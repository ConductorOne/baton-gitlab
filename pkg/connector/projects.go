package connector

import (
	"context"
	"strconv"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"

	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type projectBuilder struct {
	*gitlab.Client
}

const projectMembership = "member"

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
	)
}

func (o *projectBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return projectResourceType
}

// TODO: check rate limiting
// TODO: check pagination
// TODO: check list
func (o *projectBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {

	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var projects []*gitlabSDK.Project
	var res *gitlabSDK.Response
	var err error

	if pToken.Token == "" {
		projects, res, err = o.ListProjects(ctx, parentResourceID.Resource, pToken.Token)
		if err != nil {
			return nil, "", nil, err
		}
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
	return nil, "", nil, nil
}

func (o *projectBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
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
	return nil, nil
}

func (r *projectBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return nil, nil
}
