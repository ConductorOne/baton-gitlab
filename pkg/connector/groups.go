package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type groupBuilder struct {
	*gitlab.Client
}

const groupMembership = "member"

func groupResource(group *gitlabSDK.Group) (*v2.Resource, error) {
	return resourceSdk.NewGroupResource(
		group.Name,
		groupResourceType,
		group.ID,
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(
				map[string]interface{}{
					"id":   group.ID,
					"name": group.Name,
				},
			),
		},
		resourceSdk.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: projectResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: userResourceType.Id},
		),
	)
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {

	var groups []*gitlabSDK.Group
	var res *gitlabSDK.Response
	var err error

	if pToken.Token == "" {
		groups, res, err = o.ListGroups(ctx, pToken.Token)
		if err != nil {
			return nil, "", nil, err
		}
	} else {
		groups, res, err = o.ListGroupsPaginate(ctx, pToken.Token)
	}
	if err != nil {
		return nil, "", nil, err
	}

	outResources := make([]*v2.Resource, 0, len(groups))
	for _, group := range groups {
		resource, err := groupResource(group)
		if err != nil {
			return nil, "", nil, err
		}
		outResources = append(outResources, resource)
	}

	var nextPage string
	if res.NextPage != 0 {
		nextPage = strconv.Itoa(res.NextPage)
	}
	return outResources, nextPage, nil, nil
}

func AccessLevelString(level gitlabSDK.AccessLevelValue) string {
	switch level {
	case gitlabSDK.NoPermissions:
		return "No Permissions"
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
	default:
		return "Unknown"
	}
}

// Entitlements always returns an empty slice for roles.
func (o *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	levels := []gitlabSDK.AccessLevelValue{
		gitlabSDK.MinimalAccessPermissions,
		gitlabSDK.GuestPermissions,
		gitlabSDK.ReporterPermissions,
		gitlabSDK.DeveloperPermissions,
		gitlabSDK.MaintainerPermissions,
		gitlabSDK.OwnerPermissions,
	}

	rv := make([]*v2.Entitlement, 0, len(levels))
	for _, level := range levels {
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
	if pToken.Token == "" {
		users, res, err = o.ListGroupMembers(ctx, resource.Id.Resource)
	} else {
		users, res, err = o.ListGroupMembersPaginate(ctx, resource.Id.Resource, pToken.Token)
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
	return nil, nil
}

func (r *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return nil, nil
}
