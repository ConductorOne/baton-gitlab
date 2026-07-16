package connector

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxAncestorDepth bounds the parent-group walk so a pathological/cyclic group
// hierarchy can never deadlock a sync.
const maxAncestorDepth = 20

func toGroupResourceId(groupId string) string {
	return fmt.Sprintf("g/%s", groupId)
}

func fromGroupResourceId(groupResourceId string) (string, error) {
	parts := strings.Split(groupResourceId, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid group resource id: %s", groupResourceId)
	}
	return parts[1], nil
}

func parseAccessLevelFromEntitlementID(entitlementID string) (int, error) {
	parts := strings.Split(entitlementID, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid entitlement ID: %s", entitlementID)
	}

	levelName := parts[2]
	levelValue, ok := client.GetAccessLevelByName(levelName)
	if !ok {
		return 0, fmt.Errorf("unknown access level: %s", levelName)
	}
	return int(levelValue), nil
}

// revokeInheritedError rejects a revoke of access the principal holds indirectly
// (inherited from a parent group or granted via an invited/shared group) rather than
// as a direct membership. Removing a non-existent direct membership is a silent no-op
// that reappears on the next sync (a revoke loop), so we reject with an actionable
// message instead of falsely reporting success. The indirect access must be revoked
// at its source (the parent or invited group).
func revokeInheritedError(resourceType, resourceID string) error {
	return status.Errorf(codes.InvalidArgument,
		"baton-gitlab: cannot revoke: principal is not a direct member of %s %q; access is inherited from a parent group or granted via an invited group and must be revoked at its source",
		resourceType, resourceID)
}

// isNotFoundError reports whether err is a GitLab 404. The uhttp client surfaces a 404
// as a gRPC status with codes.NotFound; the client.ErrNotFound sentinel is only returned
// by CheckResponse, which doRequest bypasses because uhttp already errors on 4xx. We
// check both so the detection is robust regardless of which path produced the error.
// (Dominant idiom across baton-openai/segment/microsoft-entra/active-directory.)
func isNotFoundError(err error) bool {
	return errors.Is(err, client.ErrNotFound) || status.Code(err) == codes.NotFound
}

// isAlreadyExistsError reports whether err is a GitLab 409 Conflict ("Member already
// exists"). uhttp maps HTTP 409 → codes.AlreadyExists; the hand-rolled *ErrorResponse
// path never fires for doRequest calls (uhttp errors on 4xx before CheckResponse runs),
// so idempotency must gate on the gRPC code, mirroring isNotFoundError.
func isAlreadyExistsError(err error) bool {
	return status.Code(err) == codes.AlreadyExists
}

func handlePermissionError(ctx context.Context, err error, resourceType, resourceId string) (bool, error) {
	if err == nil {
		return false, nil
	}

	if status.Code(err) == codes.PermissionDenied || errors.Is(err, client.ErrForbidden) {
		l := ctxzap.Extract(ctx)
		l.Debug(
			fmt.Sprintf("Permission denied while listing members for %s. Skipping.", resourceType),
			zap.String(fmt.Sprintf("%s_id", resourceType), resourceId),
		)
		return true, nil
	}

	return false, err
}

// --- Access-path helpers (only used when SyncAccessPaths is enabled) ---
//
// The access-path model labels every grant by HOW access was obtained, using
// expandable grants that reuse the existing per-access-level entitlements. It adds
// NO new entitlements and emits paths only for access levels that actually have
// members, so no expandable grant resolves to an empty set. See docs/doc-info.md.

// sortedLevelSlugs returns the slugs of the given access levels sorted ascending by
// level value, for deterministic grant emission.
func sortedLevelSlugs(set map[client.AccessLevelValue]struct{}) []string {
	levels := make([]client.AccessLevelValue, 0, len(set))
	for l := range set {
		levels = append(levels, l)
	}
	slices.Sort(levels)
	out := make([]string, 0, len(levels))
	for _, l := range levels {
		if slug := l.String(); slug != "" {
			out = append(out, slug)
		}
	}
	return out
}

// inheritableLevelSlugs returns the distinct access levels actually held by the
// given members, restricted to the inheritable range (Guest..Owner). Minimal and
// None confer no access to child subgroups/projects, so they are excluded.
func inheritableLevelSlugs(members []*client.GroupMember) []string {
	present := make(map[client.AccessLevelValue]struct{})
	for _, member := range members {
		lvl := client.AccessLevelValue(member.AccessLevel)
		if lvl < client.GuestPermissions || lvl > client.OwnerPermissions {
			continue
		}
		present[lvl] = struct{}{}
	}
	return sortedLevelSlugs(present)
}

// inheritanceGrants emits one expandable grant per level the ancestor group holds:
// principal = the ancestor group, expandable into that group's per-level entitlement.
// Shallow keeps each path a single crisp hop ("access via <ancestor> membership");
// the ancestor's own inheritance grants carry deeper paths as their own edges.
func inheritanceGrants(target *v2.Resource, ancestor *v2.ResourceId, levelSlugs []string) []*v2.Grant {
	grants := make([]*v2.Grant, 0, len(levelSlugs))
	for _, slug := range levelSlugs {
		grants = append(grants, grant.NewGrant(
			target,
			slug,
			ancestor,
			// GrantExpandable surfaces the ancestor's members as access-through-this-path;
			// GrantImmutable marks the edge non-revocable — inherited access is structural
			// and can only be removed at its source, so C1 must not offer a direct revoke.
			grant.WithAnnotation(
				&v2.GrantExpandable{
					EntitlementIds:  []string{fmt.Sprintf("%s:%s:%s", groupResourceType.Id, ancestor.Resource, slug)},
					Shallow:         true,
					ResourceTypeIds: []string{userResourceType.Id},
				},
				&v2.GrantImmutable{},
			),
		))
	}
	return grants
}

// invitedGroupGrants emits access-path grants for groups invited (shared) into the
// target. GitLab caps a shared member's effective access at min(their level in the
// invited group, the share's group_access_level); we map each level present in the
// invited group to that capped level and emit one expandable grant per resulting
// target level, expanding the invited group's matching per-level entitlements.
// Shallow, only real levels — no empty paths, no synthetic membership entitlement.
func invitedGroupGrants(target *v2.Resource, shared []client.SharedGroup, membersByGroup map[int][]*client.GroupMember) []*v2.Grant {
	var grants []*v2.Grant
	for _, sharedGroup := range shared {
		share := client.AccessLevelValue(sharedGroup.GroupAccessLevel)
		invitedResourceID := toGroupResourceId(strconv.Itoa(sharedGroup.GroupID))
		principalID := &v2.ResourceId{ResourceType: groupResourceType.Id, Resource: invitedResourceID}

		// capped target level -> set of the invited group's per-level entitlements
		// whose members land on that level.
		sources := make(map[client.AccessLevelValue]map[string]struct{})
		for _, m := range membersByGroup[sharedGroup.GroupID] {
			lvl := client.AccessLevelValue(m.AccessLevel)
			// A member level in range but not one of GitLab's defined levels has no
			// entitlement slug; skip it so we never build a malformed source ID.
			if lvl < client.MinimalAccessPermissions || lvl > client.OwnerPermissions || lvl.String() == "" {
				continue
			}
			eff := lvl
			if share >= client.MinimalAccessPermissions && share < eff {
				eff = share
			}
			// A non-standard share level (in range but undefined) has no slug either;
			// skip rather than emit a grant on an empty-slug entitlement.
			if eff.String() == "" {
				continue
			}
			src := fmt.Sprintf("%s:%s:%s", groupResourceType.Id, invitedResourceID, lvl.String())
			if sources[eff] == nil {
				sources[eff] = make(map[string]struct{})
			}
			sources[eff][src] = struct{}{}
		}

		effLevels := make([]client.AccessLevelValue, 0, len(sources))
		for l := range sources {
			effLevels = append(effLevels, l)
		}
		slices.Sort(effLevels)
		for _, eff := range effLevels {
			srcIDs := make([]string, 0, len(sources[eff]))
			for s := range sources[eff] {
				srcIDs = append(srcIDs, s)
			}
			slices.Sort(srcIDs)
			grants = append(grants, grant.NewGrant(
				target,
				eff.String(),
				principalID,
				// Invited-group access flows through the invited group's membership
				// (GrantExpandable) and is not revocable at the target (GrantImmutable):
				// it must be removed by un-sharing or editing the invited group.
				grant.WithAnnotation(
					&v2.GrantExpandable{
						EntitlementIds:  srcIDs,
						Shallow:         true,
						ResourceTypeIds: []string{userResourceType.Id},
					},
					&v2.GrantImmutable{},
				),
			))
		}
	}
	return grants
}

// allDirectGroupMembers pages through a group's direct members, accumulating rate-limit
// annotations. Used by the access-path helpers to enumerate ancestor/invited-group
// membership levels.
func allDirectGroupMembers(ctx context.Context, c *client.GitlabClient, groupID string, annos *annotations.Annotations) ([]*client.GroupMember, error) {
	var all []*client.GroupMember
	pageToken := ""
	for {
		members, next, rateLimitDesc, err := c.ListGroupMembers(ctx, groupID, pageToken)
		if rateLimitDesc != nil {
			annos.WithRateLimiting(rateLimitDesc)
		}
		if err != nil {
			return all, err
		}
		all = append(all, members...)
		if next == "" {
			return all, nil
		}
		pageToken = next
	}
}

// accessPathInheritanceGrants walks the target's ancestor group chain and emits, per
// ancestor, "access via <ancestor> membership" expandable grants for the levels that
// ancestor actually has direct members at. Together the per-ancestor edges surface
// both intermediate-subgroup and top-level access as distinct, reviewable paths.
func accessPathInheritanceGrants(ctx context.Context, c *client.GitlabClient, target *v2.Resource, immediateParent *v2.ResourceId, annos *annotations.Annotations) ([]*v2.Grant, error) {
	var grants []*v2.Grant
	cursor := immediateParent
	for depth := 0; cursor != nil && depth < maxAncestorDepth; depth++ {
		groupID, err := fromGroupResourceId(cursor.Resource)
		if err != nil {
			return nil, err
		}

		members, err := allDirectGroupMembers(ctx, c, groupID, annos)
		if err != nil {
			isPermissionError, unhandledErr := handlePermissionError(ctx, err, "group", groupID)
			if unhandledErr != nil {
				return nil, unhandledErr
			}
			if isPermissionError {
				members = nil
			}
		}
		grants = append(grants, inheritanceGrants(target, cursor, inheritableLevelSlugs(members))...)

		group, rateLimitDesc, err := c.GetGroup(ctx, groupID)
		if rateLimitDesc != nil {
			annos.WithRateLimiting(rateLimitDesc)
		}
		if err != nil {
			isPermissionError, unhandledErr := handlePermissionError(ctx, err, "group", groupID)
			if unhandledErr != nil {
				return nil, unhandledErr
			}
			if isPermissionError {
				return grants, nil
			}
		}
		if group == nil || group.ParentID == 0 {
			return grants, nil
		}
		cursor = getParentGroup(group.ParentID)
	}
	return grants, nil
}

// accessPathInvitedGrants resolves each invited (shared) group's direct members and
// emits the invited-group access paths for the target. GitLab group→group sharing
// confers access only to the invited group's direct members; group→project sharing
// also confers it to the invited group's inherited members, so for an invited
// *subgroup* shared into a project those inherited members are surfaced via their own
// group's path rather than this one (see docs/doc-info.md).
func accessPathInvitedGrants(ctx context.Context, c *client.GitlabClient, target *v2.Resource, shared []client.SharedGroup, annos *annotations.Annotations) ([]*v2.Grant, error) {
	if len(shared) == 0 {
		return nil, nil
	}
	membersByGroup := make(map[int][]*client.GroupMember, len(shared))
	for _, sg := range shared {
		groupID := strconv.Itoa(sg.GroupID)
		members, err := allDirectGroupMembers(ctx, c, groupID, annos)
		if err != nil {
			isPermissionError, unhandledErr := handlePermissionError(ctx, err, "group", groupID)
			if unhandledErr != nil {
				return nil, unhandledErr
			}
			if isPermissionError {
				continue
			}
		}
		membersByGroup[sg.GroupID] = members
	}
	return invitedGroupGrants(target, shared, membersByGroup), nil
}
