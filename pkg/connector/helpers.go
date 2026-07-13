package connector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

// inheritedAccessLevels are the levels a parent group confers on child
// subgroups/projects (Minimal and None excluded — they confer no child access).
var inheritedAccessLevels = []client.AccessLevelValue{
	client.GuestPermissions,
	client.ReporterPermissions,
	client.DeveloperPermissions,
	client.MaintainerPermissions,
	client.OwnerPermissions,
}

// parentGroupInheritanceGrants surfaces "access via parent/top-level group" as a
// distinct path: per level, the parent group as principal, expandable to the
// parent's members at that level (non-shallow, so ancestors resolve transitively).
func parentGroupInheritanceGrants(target *v2.Resource, parentGroup *v2.ResourceId) []*v2.Grant {
	grants := make([]*v2.Grant, 0, len(inheritedAccessLevels))
	for _, level := range inheritedAccessLevels {
		levelName := level.String()
		grants = append(grants, grant.NewGrant(
			target,
			levelName,
			parentGroup,
			grant.WithAnnotation(&v2.GrantExpandable{
				EntitlementIds:  []string{fmt.Sprintf("%s:%s:%s", groupResourceType.Id, parentGroup.Resource, levelName)},
				Shallow:         false,
				ResourceTypeIds: []string{userResourceType.Id, groupResourceType.Id},
			}),
		))
	}
	return grants
}

// sharedGroupGrants emits, for each invited (shared) group, that group as
// principal on the target's access-level entitlement, expandable to the invited
// group's membership. expansionSlug picks which membership entitlement to expand
// into (group→group = member/direct, group→project = effective-member); see
// docs/doc-info.md for the sharing-semantics rationale.
//
// Unlike parent inheritance (which excludes Minimal because it confers no child
// access), a share carries whatever group_access_level GitLab reports, so the
// full Minimal..Owner range is accepted here — both groups and projects expose a
// Minimal access-level entitlement, so the grant still resolves.
func sharedGroupGrants(ctx context.Context, target *v2.Resource, shared []client.SharedGroup, expansionSlug string) []*v2.Grant {
	l := ctxzap.Extract(ctx)
	grants := make([]*v2.Grant, 0, len(shared))
	for _, sg := range shared {
		level := client.AccessLevelValue(sg.GroupAccessLevel)
		levelName := level.String()
		// Skip levels with no matching entitlement (only Minimal..Owner exist),
		// which would otherwise orphan the grant.
		if levelName == "" || level < client.MinimalAccessPermissions || level > client.OwnerPermissions {
			l.Debug("baton-gitlab: skipping shared group with unsupported access level",
				zap.String("target", target.Id.Resource),
				zap.Int("shared_group_id", sg.GroupID),
				zap.Int("group_access_level", sg.GroupAccessLevel),
			)
			continue
		}

		invitedGroupResourceID := toGroupResourceId(strconv.Itoa(sg.GroupID))
		principalID := &v2.ResourceId{ResourceType: groupResourceType.Id, Resource: invitedGroupResourceID}

		grants = append(grants, grant.NewGrant(
			target,
			levelName,
			principalID,
			grant.WithAnnotation(&v2.GrantExpandable{
				EntitlementIds:  []string{fmt.Sprintf("%s:%s:%s", groupResourceType.Id, invitedGroupResourceID, expansionSlug)},
				Shallow:         false,
				ResourceTypeIds: []string{userResourceType.Id, groupResourceType.Id},
			}),
		))
	}
	return grants
}

// effectiveMemberChainGrants composes a group's effective-member entitlement
// (direct + ancestor-inherited members) purely via expansion — no extra API call.
// It excludes the group's own inbound shares because group→project sharing is
// non-transitive. See docs/doc-info.md for the semantics.
func effectiveMemberChainGrants(group *v2.Resource, parentGroup *v2.ResourceId) []*v2.Grant {
	expandable := func(srcEntitlementID string) grant.GrantOption {
		return grant.WithAnnotation(&v2.GrantExpandable{
			EntitlementIds:  []string{srcEntitlementID},
			Shallow:         false,
			ResourceTypeIds: []string{userResourceType.Id, groupResourceType.Id},
		})
	}

	grants := make([]*v2.Grant, 0, 2)

	// Direct members.
	grants = append(grants, grant.NewGrant(
		group,
		groupEffectiveMemberEntitlement,
		group.Id,
		expandable(groupMemberEntitlementID(group.Id.Resource)),
	))

	// Inherited members (transitive up the ancestor chain).
	if parentGroup != nil {
		grants = append(grants, grant.NewGrant(
			group,
			groupEffectiveMemberEntitlement,
			parentGroup,
			expandable(groupEffectiveMemberEntitlementID(parentGroup.Resource)),
		))
	}
	return grants
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
