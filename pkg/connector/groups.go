package connector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

type groupBuilder struct {
	*gitlab.Client
	GroupsCache []*gitlabSDK.Group
}

var accessLevels = []gitlabSDK.AccessLevelValue{
	gitlabSDK.MinimalAccessPermissions,
	gitlabSDK.GuestPermissions,
	gitlabSDK.ReporterPermissions,
	gitlabSDK.DeveloperPermissions,
	gitlabSDK.MaintainerPermissions,
	gitlabSDK.OwnerPermissions,
}

func groupResource(group *gitlabSDK.Group, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":          group.ID,
		"name":        group.Name,
		"full_name":   group.FullName,
		"description": group.Description,
	}
	if group.ParentID != 0 {
		profile["parent_group_id"] = group.ParentID
	}

	return resourceSdk.NewGroupResource(
		group.FullName,
		groupResourceType,
		strconv.Itoa(group.ID),
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(
				profile,
			),
		},
		resourceSdk.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: projectResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: groupResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: userResourceType.Id},
		),
		resourceSdk.WithParentResourceID(parentResourceID),
	)
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var (
		groups    []*gitlabSDK.Group
		res       *gitlabSDK.Response
		pageToken string
		err       error
	)
	if pToken != nil {
		pageToken = pToken.Token
	}

	if len(o.GroupsCache) > 0 && pageToken == "" {
		groups = o.GroupsCache
	} else {
		groups, res, err = o.ListGroups(ctx, pageToken)
		if err != nil {
			return nil, "", nil, err
		}
		o.loadIntoCache(groups)
	}

	outResources := make([]*v2.Resource, 0, len(groups))
	for _, group := range groups {
		shouldCreateResource, err := parentResourceIsParentGroup(group.ParentID, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}

		if !shouldCreateResource {
			continue
		}

		resource, err := groupResource(group, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		outResources = append(outResources, resource)
	}

	var nextPage string
	if res != nil && res.NextPage != 0 {
		nextPage = strconv.Itoa(res.NextPage)
	}
	return outResources, nextPage, nil, nil
}

// parentResourceIsParentGroup validates that the parentResourceID received by the function call belongs to the corresponding parent group.
// The validation occurs between the ID on the group data (given by the API) and the ID of the parentResource.
// parentGroupId will be 0 (zero) if the group isn't a subgroup. parentResourceID will be nil if no parent resource was received in the List function.
// Both cases are checked and handled in this function.
func parentResourceIsParentGroup(parentGroupId int, parentResourceID *v2.ResourceId) (bool, error) {
	if parentGroupId != 0 && parentResourceID == nil {
		// This path occurs on the first execution of the List func. When all groups are received from the API. Subgroups will be skipped.
		return false, nil
	}

	if parentResourceID == nil {
		// If parentResourceID is null, is because it's a root group, not a subgroup, so there is no need to validate a parent group id, and it shouldn't be skipped.
		return true, nil
	}

	parentId, err := strconv.Atoi(parentResourceID.Resource)
	if err != nil {
		return false, err
	}

	return parentGroupId == parentId, nil
}

func AccessLevelString(level gitlabSDK.AccessLevelValue) string {
	switch level {
	case gitlabSDK.NoPermissions:
		return "None"
	case gitlabSDK.MinimalAccessPermissions:
		return "Minimal"
	case gitlabSDK.GuestPermissions:
		return "Guest"
	case gitlabSDK.ReporterPermissions:
		return "Reporter"
	case gitlabSDK.DeveloperPermissions:
		return "Developer"
	case gitlabSDK.MaintainerPermissions:
		return "Maintainer"
	case gitlabSDK.OwnerPermissions:
		return "Owner"
	case gitlabSDK.AdminPermissions:
		return "Admin"
	}
	return ""
}

func AccessLevel(level string) gitlabSDK.AccessLevelValue {
	switch level {
	case "None":
		return gitlabSDK.NoPermissions
	case "Minimal":
		return gitlabSDK.MinimalAccessPermissions
	case "Guest":
		return gitlabSDK.GuestPermissions
	case "Reporter":
		return gitlabSDK.ReporterPermissions
	case "Developer":
		return gitlabSDK.DeveloperPermissions
	case "Maintainer":
		return gitlabSDK.MaintainerPermissions
	case "Owner":
		return gitlabSDK.OwnerPermissions
	case "Admin":
		return gitlabSDK.AdminPermissions
	default:
		return gitlabSDK.NoPermissions
	}
}

func (o *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	rv := make([]*v2.Entitlement, 0, len(accessLevels))
	for _, level := range accessLevels {
		rv = append(rv, entitlement.NewAssignmentEntitlement(
			resource,
			AccessLevelString(level),
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDisplayName(fmt.Sprintf("%s Group %s", resource.DisplayName, AccessLevelString(level))),
			entitlement.WithDescription(fmt.Sprintf("%s on the %s group in Gitlab", AccessLevelString(level), resource.DisplayName)),
		))
	}
	return rv, "", nil, nil
}

func (o *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant

	var users []*gitlabSDK.GroupMember
	var res *gitlabSDK.Response
	var err error
	groupId, err := fromGroupResourceId(resource.Id.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
	}
	if pToken.Token == "" {
		users, res, err = o.ListGroupMembers(ctx, groupId)
	} else {
		users, res, err = o.ListGroupMembersPaginate(ctx, groupId, pToken.Token)
	}
	if err != nil {
		return nil, "", nil, err
	}

	var nextPage string
	if res.NextPage != 0 {
		nextPage = strconv.Itoa(res.NextPage)
	}

	for _, user := range users {
		principalId, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("error creating principal ID: %w", err)
		}

		outGrants = append(outGrants, grant.NewGrant(
			resource,
			AccessLevelString(user.AccessLevel),
			principalId,
		))
	}
	return outGrants, nextPage, nil, nil
}

func newGroupBuilder(client *gitlab.Client) *groupBuilder {
	return &groupBuilder{
		Client: client,
	}
}

func (r *groupBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	l := ctxzap.Extract(ctx)
	groupIdAndName := entitlement.Resource.Id.Resource
	groupId, err := fromGroupResourceId(groupIdAndName)
	if err != nil {
		return nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	parts := strings.Split(entitlement.Id, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid entitlement ID: %s", entitlement.Id)
	}
	accessLevelValue := AccessLevel(parts[2])
	userId, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		l.Warn("baton-gitlab grant: unable to parse user ID. falling back to email invite", zap.Error(err))
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
		err = r.InviteGroupMember(ctx, groupId, userEmail, accessLevelValue)
		if err != nil {
			return nil, fmt.Errorf("error inviting user to group: %w", err)
		}
		return nil, nil
	}

	err = r.AddGroupMember(ctx, groupId, userId, accessLevelValue)
	if err != nil {
		errResp := &gitlabSDK.ErrorResponse{}
		if errors.As(err, &errResp) {
			if errResp.Response != nil && errResp.Response.StatusCode == http.StatusConflict {
				return annotations.New(&v2.GrantAlreadyExists{}), nil
			}
		}
		return nil, fmt.Errorf("error adding user to group: %w", err)
	}
	return nil, nil
}

func (r *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	groupIdAndName := grant.Entitlement.Resource.Id.Resource
	groupId, err := fromGroupResourceId(groupIdAndName)
	if err != nil {
		return nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	userId, err := strconv.Atoi(grant.Principal.Id.Resource)
	if err != nil {
		return nil, fmt.Errorf("error converting user ID to int: %w", err)
	}

	err = r.RemoveGroupMember(ctx, groupId, userId)
	if err != nil {
		if errors.Is(err, gitlabSDK.ErrNotFound) {
			return annotations.New(&v2.GrantAlreadyRevoked{}), nil
		}
		return nil, fmt.Errorf("error removing user from group: %w", err)
	}

	return nil, nil
}

func (o *groupBuilder) loadIntoCache(groups []*gitlabSDK.Group) {
	o.GroupsCache = append(o.GroupsCache, groups...)
}
