package connector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"google.golang.org/protobuf/proto"
)

type projectBuilder struct {
	client *client.GitlabClient
}

func (o *projectBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return projectResourceType
}

var projectAccessLevels = []client.AccessLevelValue{
	client.MinimalAccessPermissions,
	client.GuestPermissions,
	client.ReporterPermissions,
	client.DeveloperPermissions,
	client.MaintainerPermissions,
	client.OwnerPermissions,
}

func (o *projectBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var (
		projects          []*client.Project
		pageToken         string
		err               error
		outputAnnotations = annotations.New()
	)

	groupId, err := fromGroupResourceId(parentResourceID.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	if pToken != nil {
		pageToken = pToken.Token
	}
	projects, nextPageToken, rateLimitDesc, err := o.client.ListProjects(ctx, groupId, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	outResources := make([]*v2.Resource, 0, len(projects))
	for _, project := range projects {
		parentGroup := getParentGroupFromNamespace(project.Namespace)
		if parentGroup == nil {
			parentGroup = parentResourceID
		}

		resource, err := projectResource(project, parentGroup, o.client.IsOnPremise)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		outResources = append(outResources, resource)
	}

	return outResources, nextPageToken, outputAnnotations, nil
}

func (o *projectBuilder) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	rv := make([]*v2.Entitlement, 0, len(projectAccessLevels))

	for _, level := range projectAccessLevels {
		levelName := level.String()
		rv = append(rv, entitlement.NewAssignmentEntitlement(
			resource,
			levelName,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDisplayName(fmt.Sprintf("%s Project %s", resource.DisplayName, levelName)),
			entitlement.WithDescription(fmt.Sprintf("%s on the %s project in Gitlab", levelName, resource.DisplayName)),
		))
	}
	return rv, "", nil, nil
}

func (o *projectBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant
	var outputAnnotations = annotations.New()

	var users []*client.ProjectMember
	var err error
	var nextPageToken string
	var rateLimitDesc *v2.RateLimitDescription
	if o.client.SyncDirectMembersOnly {
		users, nextPageToken, rateLimitDesc, err = o.client.ListProjectMembers(ctx, resource.Id.Resource, pToken.Token)
	} else {
		users, nextPageToken, rateLimitDesc, err = o.client.ListAllProjectMembers(ctx, resource.Id.Resource, pToken.Token)
	}
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	groupId := resource.ParentResourceId
	if groupId == nil {
		return nil, "", outputAnnotations, fmt.Errorf("project resource has no parent group")
	}

	for _, user := range users {
		entitlementId := fmt.Sprintf("group:%s:%s", groupId.Resource, client.AccessLevelValue(user.AccessLevel).String())
		principalId, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("error creating principal ID: %w", err)
		}

		grantOptions := []grant.GrantOption{
			grant.WithAnnotation(&v2.GrantExpandable{
				EntitlementIds: []string{entitlementId},
				Shallow:        false,
			}),
		}

		outGrants = append(outGrants, grant.NewGrant(
			resource,
			client.AccessLevelValue(user.AccessLevel).String(),
			principalId,
		))

		outGrants = append(outGrants, grant.NewGrant(
			resource,
			client.AccessLevelValue(user.AccessLevel).String(),
			groupId,
			grantOptions...,
		))
	}

	return outGrants, nextPageToken, outputAnnotations, nil
}

func (o *projectBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	var outputAnnotations = annotations.New()
	if strings.HasPrefix(principal.Id.Resource, pendingInvitationUser) {
		return nil, fmt.Errorf("entitlement cannot be granted: user %q has not yet accepted the invitation to gitlab", principal.Id.Resource)
	}

	projectId := entitlement.Resource.Id.Resource
	accessLevelValue, err := parseAccessLevelFromEntitlementID(entitlement.Id)
	if err != nil {
		return nil, err
	}
	userId, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		return nil, fmt.Errorf("error converting user ID to int: %w", err)
	}

	memberRequest := &client.AddProjectMemberRequest{
		UserID:      userId,
		AccessLevel: client.AccessLevelValue(accessLevelValue),
	}
	_, rateLimitDesc, err := o.client.AddProjectMember(ctx, projectId, memberRequest)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return outputAnnotations, fmt.Errorf("error adding user to project: %w", err)
	}

	return outputAnnotations, nil
}

func (o *projectBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	var outputAnnotations = annotations.New()

	projectId := grant.Entitlement.Resource.Id.Resource
	userId, err := strconv.Atoi(grant.Principal.Id.Resource)
	if err != nil {
		return nil, fmt.Errorf("error converting user ID to int: %w", err)
	}

	rateLimitDesc, err := o.client.RemoveProjectMember(ctx, projectId, strconv.Itoa(userId))
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return annotations.New(&v2.GrantAlreadyRevoked{}), nil
		}
		return outputAnnotations, fmt.Errorf("error removing user from project: %w", err)
	}

	return outputAnnotations, nil
}

func projectResource(project *client.Project, parentResourceID *v2.ResourceId, isOnPremise bool) (*v2.Resource, error) {
	var annotations []proto.Message

	return resourceSdk.NewGroupResource(
		project.NameWithNamespace,
		projectResourceType,
		project.ID,
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(
				map[string]interface{}{
					"id":          project.ID,
					fieldName:     project.Name,
					"description": project.Description,
				},
			),
		},
		resourceSdk.WithAnnotation(annotations...),
		resourceSdk.WithParentResourceID(parentResourceID),
	)
}

func newProjectBuilder(client *client.GitlabClient) *projectBuilder {
	return &projectBuilder{
		client: client,
	}
}
