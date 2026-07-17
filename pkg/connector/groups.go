package connector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type groupBuilder struct {
	client *client.GitlabClient
}

var groupAccessLevels = []client.AccessLevelValue{
	client.MinimalAccessPermissions,
	client.GuestPermissions,
	client.ReporterPermissions,
	client.DeveloperPermissions,
	client.MaintainerPermissions,
	client.OwnerPermissions,
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID != nil {
		return nil, "", nil, nil
	}
	var (
		groups            []*client.Group
		outputAnnotations = annotations.New()
		pageToken         string
		err               error
	)
	if pToken != nil {
		pageToken = pToken.Token
	}

	groups, nextPageToken, rateLimitDesc, err := o.client.ListGroups(ctx, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	outResources := make([]*v2.Resource, 0, len(groups))
	for _, group := range groups {
		parentResourceID = getParentGroup(group.ParentID)

		resource, err := groupResource(group, parentResourceID, o.client.IsOnPremise)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		outResources = append(outResources, resource)
	}

	return outResources, nextPageToken, outputAnnotations, nil
}

// parentGroupId will be 0 (zero) if the group isn't a subgroup. parentResourceID will be nil if no parent resource was received in the List function.
// Both cases are checked and handled in this function.
func getParentGroup(parentGroupId int) *v2.ResourceId {
	if parentGroupId == 0 {
		// This path occurs on the first execution of the List func. When all groups are received from the API. Subgroups will be skipped.
		return nil
	}

	parentGroupIdStr := strconv.Itoa(parentGroupId)
	parentGroupResourceId := toGroupResourceId(parentGroupIdStr)
	return &v2.ResourceId{
		ResourceType: groupResourceType.Id,
		Resource:     parentGroupResourceId,
	}
}

func getParentGroupFromNamespace(namespace *client.Namespace) *v2.ResourceId {
	if namespace == nil {
		return nil
	}
	return getParentGroup(namespace.Id)
}

func (o *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	rv := make([]*v2.Entitlement, 0, len(groupAccessLevels))

	for _, level := range groupAccessLevels {
		levelName := level.String()
		rv = append(rv, entitlement.NewAssignmentEntitlement(
			resource,
			levelName,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDisplayName(fmt.Sprintf("%s Group %s", resource.DisplayName, levelName)),
			entitlement.WithDescription(fmt.Sprintf("%s on the %s group in Gitlab", levelName, resource.DisplayName)),
		))
	}
	return rv, "", nil, nil
}

// Grants dispatches on the SyncAccessPaths flag. With it disabled (default) the
// connector emits flattened effective membership as before; with it enabled each
// access path (direct, inherited from a parent group, or via an invited group) is
// surfaced as a distinct expandable grant.
func (o *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	if o.client.SyncAccessPaths {
		return o.grantsWithAccessPaths(ctx, resource, pToken)
	}
	return o.grantsFlattened(ctx, resource, pToken)
}

// grantsFlattened emits effective membership as flat direct grants — the
// pre-access-path behavior, preserved unchanged for existing customers. When
// SyncDirectMembersOnly is set, only direct members are listed; otherwise the full
// effective set (direct + inherited) is flattened via ListAllGroupMembers.
func (o *groupBuilder) grantsFlattened(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant
	var outputAnnotations = annotations.New()
	var users []*client.GroupMember
	var err error

	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	groupId, err := fromGroupResourceId(resource.Id.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	var nextPageToken string
	var rateLimitDesc *v2.RateLimitDescription
	if o.client.SyncDirectMembersOnly {
		users, nextPageToken, rateLimitDesc, err = o.client.ListGroupMembers(ctx, groupId, pageToken)
	} else {
		users, nextPageToken, rateLimitDesc, err = o.client.ListAllGroupMembers(ctx, groupId, pageToken)
	}
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}

	if err != nil {
		isPermissionError, unhandledErr := handlePermissionError(ctx, err, "group", groupId)
		if unhandledErr != nil {
			return nil, "", outputAnnotations, unhandledErr
		}
		if isPermissionError {
			return nil, "", outputAnnotations, nil
		}
	}

	for _, user := range users {
		principalId, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("error creating principal ID: %w", err)
		}

		outGrants = append(outGrants, grant.NewGrant(
			resource,
			client.AccessLevelValue(user.AccessLevel).String(),
			principalId,
		))
	}
	return outGrants, nextPageToken, outputAnnotations, nil
}

// grantsWithAccessPaths emits a group's direct members plus its indirect access
// paths — inheritance from ancestor groups and access via invited (shared) groups —
// as expandable grants, so each path stays distinct and reviewable in C1. It reuses
// the existing per-access-level entitlements (no synthetic membership entitlement)
// and emits paths only for levels that actually have members (no empty expansions).
func (o *groupBuilder) grantsWithAccessPaths(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant
	var outputAnnotations = annotations.New()

	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	groupId, err := fromGroupResourceId(resource.Id.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	users, nextPageToken, rateLimitDesc, err := o.client.ListGroupMembers(ctx, groupId, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		_, unhandledErr := handlePermissionError(ctx, err, "group", groupId)
		if unhandledErr != nil {
			return nil, "", outputAnnotations, unhandledErr
		}
		// Permission error listing members: skip direct members but still emit the
		// pure-expansion indirect anchors below, which need no member data of this group.
		users, nextPageToken = nil, ""
	}

	// Path 1/2: direct members.
	for _, user := range users {
		principalId, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("error creating principal ID: %w", err)
		}
		outGrants = append(outGrants, grant.NewGrant(
			resource,
			client.AccessLevelValue(user.AccessLevel).String(),
			principalId,
		))
	}

	// Indirect access paths, emitted once on the first page (unless direct-only).
	if pageToken == "" && !o.client.SyncDirectMembersOnly {
		if resource.ParentResourceId != nil {
			inherited, err := accessPathInheritanceGrants(ctx, o.client, resource, resource.ParentResourceId, &outputAnnotations)
			if err != nil {
				return nil, "", outputAnnotations, err
			}
			outGrants = append(outGrants, inherited...)
		}

		group, rlDesc, err := o.client.GetGroup(ctx, groupId)
		if rlDesc != nil {
			outputAnnotations.WithRateLimiting(rlDesc)
		}
		if err != nil {
			_, unhandledErr := handlePermissionError(ctx, err, "group", groupId)
			if unhandledErr != nil {
				return nil, "", outputAnnotations, unhandledErr
			}
			// Permission error: skip invited-group grants and continue.
		} else {
			// Path 4: groups invited into this group (group→group = direct members).
			invited, err := accessPathInvitedGrants(ctx, o.client, resource, group.SharedWithGroups, &outputAnnotations)
			if err != nil {
				return nil, "", outputAnnotations, err
			}
			outGrants = append(outGrants, invited...)
		}
	}

	return outGrants, nextPageToken, outputAnnotations, nil
}

func (o *groupBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	l := ctxzap.Extract(ctx)
	var outputAnnotations = annotations.New()

	if strings.HasPrefix(principal.Id.Resource, pendingInvitationUser) {
		return nil, fmt.Errorf("entitlement cannot be granted: user %q has not yet accepted the invitation to gitlab", principal.Id.Resource)
	}

	groupIdAndName := entitlement.Resource.Id.Resource
	groupId, err := fromGroupResourceId(groupIdAndName)
	if err != nil {
		return nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	accessLevelValue, err := parseAccessLevelFromEntitlementID(entitlement.Id)
	if err != nil {
		return nil, err
	}

	// GitLab does not allow assigning the 'Minimal Access' role to subgroups.
	// This check ensures that the group is a top-level group by verifying that
	// the 'parent_group_id' trait is not present. If the field exists, it means
	// the group is a subgroup, and the grant should be rejected to prevent API errors.
	//
	// Additionally, assigning 'Minimal Access' requires the user to have a license
	// that allows membership in top-level groups. This role is limited in scope
	// and cannot be applied to subgroups due to GitLab's permission model.
	if client.AccessLevelValue(accessLevelValue) == client.MinimalAccessPermissions {
		if entitlement.Resource.ParentResourceId != nil {
			return outputAnnotations, fmt.Errorf("cannot grant 'Minimal Access': this role is only available for top-level groups, not subgroups")
		}
	}

	userId, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		l.Debug("baton-gitlab grant: unable to parse user ID. falling back to email invite", zap.Error(err))
		ut, err := resourceSdk.GetUserTrait(principal)
		if err != nil {
			return nil, fmt.Errorf("baton-gitlab: error getting user trait: %w", err)
		}
		if len(ut.Emails) == 0 {
			return nil, fmt.Errorf("baton-gitlab: user has no email. cannot invite user to group")
		}
		userEmail := ut.Emails[0].Address
		for _, email := range ut.Emails {
			if email.IsPrimary {
				userEmail = email.Address
				break
			}
		}
		l.Info("baton-gitlab grant: inviting user to group", zap.String("email", userEmail))
		rateLimitDesc, err := o.client.InviteGroupMember(ctx, groupId, userEmail, client.AccessLevelValue(accessLevelValue))
		if rateLimitDesc != nil {
			outputAnnotations.WithRateLimiting(rateLimitDesc)
		}
		if err != nil {
			return outputAnnotations, fmt.Errorf("error inviting user to group: %w", err)
		}
		return outputAnnotations, nil
	}

	memberRequest := &client.AddGroupMemberRequest{
		UserID:      userId,
		AccessLevel: client.AccessLevelValue(accessLevelValue),
	}
	_, rateLimitDesc, err := o.client.AddGroupMember(ctx, groupId, memberRequest)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		// Idempotency: GitLab returns 409 (uhttp → codes.AlreadyExists) when the user is
		// already a member, and a 400 "should be greater than or equal to" when they already
		// hold an equal/higher role. Both mean the desired grant already holds. (The gRPC
		// code is checked directly because the hand-rolled *ErrorResponse never reaches the
		// error chain for doRequest calls — uhttp errors on 4xx before CheckResponse runs.)
		if isAlreadyExistsError(err) || isAlreadyAtOrAboveLevelError(err) {
			return annotations.New(&v2.GrantAlreadyExists{}), nil
		}
		return outputAnnotations, fmt.Errorf("error adding user to group: %w", err)
	}
	return outputAnnotations, nil
}

func (o *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	var outputAnnotations = annotations.New()

	groupIdAndName := grant.Entitlement.Resource.Id.Resource
	groupId, err := fromGroupResourceId(groupIdAndName)
	if err != nil {
		return nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	rateLimitDesc, err := o.client.RemoveGroupMember(ctx, groupId, grant.Principal.Id.Resource)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		if isNotFoundError(err) {
			// With access-path labeling off (default), preserve the historical behavior:
			// an absent direct membership is reported as already revoked. The effective-
			// access check below only applies in access-path mode, so default-mode
			// deployments see no provisioning behavior change.
			if !o.client.SyncAccessPaths {
				outputAnnotations.Update(&v2.GrantAlreadyRevoked{})
				return outputAnnotations, nil
			}
			// Not a direct member. If the principal still has effective access, it is
			// inherited/invited — reject instead of falsely reporting success (which
			// would reappear next sync). Otherwise the membership is genuinely gone.
			_, rlDesc, checkErr := o.client.GetGroupMemberAll(ctx, groupId, grant.Principal.Id.Resource)
			if rlDesc != nil {
				outputAnnotations.WithRateLimiting(rlDesc)
			}
			// GetGroupMemberAll always returns a non-nil member when checkErr is nil,
			// so a nil-check would be dead here; keying on checkErr is sufficient.
			switch {
			case checkErr == nil:
				return outputAnnotations, revokeInheritedError("group", groupIdAndName)
			case !isNotFoundError(checkErr):
				return outputAnnotations, fmt.Errorf("baton-gitlab: error verifying group membership: %w", checkErr)
			default:
				outputAnnotations.Update(&v2.GrantAlreadyRevoked{})
				return outputAnnotations, nil
			}
		}
		return outputAnnotations, fmt.Errorf("error removing user from group: %w", err)
	}

	return outputAnnotations, nil
}

func groupResource(group *client.Group, parentResourceID *v2.ResourceId, isOnPremise bool) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":             group.ID,
		profileFieldName: group.Name,
		"full_name":      group.FullName,
		"description":    group.Description,
		"archived":       group.Archived,
		"visibility":     group.Visibility,
	}

	if group.MarkedForDeletion != nil && !time.Time(*group.MarkedForDeletion).IsZero() {
		profile["marked_for_deletion"] = time.Time(*group.MarkedForDeletion)
	}

	if group.ParentID != 0 {
		profile["parent_group_id"] = group.ParentID
	}

	annos := make([]proto.Message, 0)
	if parentResourceID == nil {
		annos = append(annos, &v2.ChildResourceType{ResourceTypeId: projectResourceType.Id})
	}

	// We get all members of subgroups so only need top level
	if !isOnPremise && parentResourceID == nil {
		annos = append(annos, &v2.ChildResourceType{ResourceTypeId: userResourceType.Id})
	}

	return resourceSdk.NewGroupResource(
		group.FullName,
		groupResourceType,
		toGroupResourceId(strconv.Itoa(group.ID)),
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(profile),
		},
		resourceSdk.WithAnnotation(annos...),
		resourceSdk.WithParentResourceID(parentResourceID),
	)
}

func newGroupBuilder(client *client.GitlabClient) *groupBuilder {
	return &groupBuilder{
		client: client,
	}
}
