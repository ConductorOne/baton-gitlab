package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"

	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type groupBuilder struct {
	*gitlab.Client
}

const groupMembership = "member"

func groupResource(group *gitlabSDK.Group, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
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
		resourceSdk.WithParentResourceID(parentResourceID),
	)
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

// TODO: check rate limiting
// TODO: check pagination
// TODO: check list
func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	groups, res, _, err := o.ListGroups(ctx, pToken.Token)
	if err != nil {
		return nil, "", nil, fmt.Errorf("gitlab-connector: ListGroups failed: %w", err)
	}

	outResources := make([]*v2.Resource, 0, len(groups))
	for _, group := range groups {
		resource, err := groupResource(group, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		outResources = append(outResources, resource)
	}

	var cursor string
	if res.NextLink != "" {
		cursor = res.NextLink
	}

	return outResources, cursor, nil, nil
}

// Entitlements always returns an empty slice for roles.
func (o *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
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
