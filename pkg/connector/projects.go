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

type projectBuilder struct {
	*gitlab.Client
}

func projectResource(project *gitlabSDK.Project, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	return resourceSdk.NewGroupResource(
		project.Name,
		projectResourceType,
		project.ID,
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(
				map[string]interface{}{
					"id":   project.ID,
					"name": project.Name,
				},
			),
		},
		resourceSdk.WithParentResourceID(parentResourceID),
		resourceSdk.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: userResourceType.Id},
		),
	)
}

func (o *projectBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return projectResourceType
}

func (o *projectBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var projects []*gitlabSDK.Project
	var res *gitlabSDK.Response
	var err error

	if pToken.Token == "" {
		projects, res, err = o.ListProjects(ctx, parentResourceID.Resource)
	} else {
		projects, res, err = o.ListProjectsPaginate(ctx, parentResourceID.Resource, pToken.Token)
	}
	if err != nil {
		return nil, "", nil, err
	}

	outResources := make([]*v2.Resource, 0, len(projects))
	for _, project := range projects {
		resource, err := projectResource(project, parentResourceID)
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

// Entitlements always returns an empty slice for roles.
func (o *projectBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
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
			entitlement.WithDisplayName(fmt.Sprintf("%s Project %s", resource.DisplayName, AccessLevelString(level))),
			entitlement.WithDescription(fmt.Sprintf("%s on the %s project in Gitlab", AccessLevelString(level), resource.DisplayName)),
		))
	}
	return rv, "", nil, nil
}

func (o *projectBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant

	var users []*gitlabSDK.ProjectMember
	var res *gitlabSDK.Response
	var err error
	if pToken.Token == "" {
		users, res, err = o.ListProjectMembers(ctx, resource.Id.Resource)
	} else {
		users, res, err = o.ListProjectMembersPaginate(ctx, resource.Id.Resource, pToken.Token)
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

func newProjectBuilder(client *gitlab.Client) *projectBuilder {
	return &projectBuilder{
		Client: client,
	}
}

func (r *projectBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	projectId := entitlement.Resource.Id.Resource
	accessLevel := AccessLevel(entitlement.Slug)
	userId, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		return nil, fmt.Errorf("error converting user ID to int: %w", err)
	}

	_, err = r.AddProjectMember(ctx, projectId, userId, accessLevel)

	if err != nil {
		return nil, fmt.Errorf("error adding user to group: %w", err)
	}
	return nil, nil
}

func (r *projectBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	projectId := grant.Entitlement.Resource.Id.Resource
	userId, err := strconv.Atoi(grant.Principal.Id.Resource)
	if err != nil {
		return nil, fmt.Errorf("error converting user ID to int: %w", err)
	}

	err = r.RemoveProjectMember(ctx, projectId, userId)
	if err != nil {
		return nil, fmt.Errorf("error removing user from group: %w", err)
	}

	return nil, nil
}
