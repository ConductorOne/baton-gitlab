package connector

import (
	"context"
	"fmt"
	"time"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

// accessTokenResource builds a K1 STATIC_SECRET resource for a project or group
// access token. The backing bot user (UserID) is set as the SecretTrait identity
// back-reference.
func accessTokenResource(token *client.AccessToken, resourceType *v2.ResourceType, detail string, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	var createdAt, lastUsedAt, expiresAt time.Time
	if token.CreatedAt != nil {
		createdAt = *token.CreatedAt
	}
	if token.LastUsedAt != nil {
		lastUsedAt = *token.LastUsedAt
	}
	if token.ExpiresAt != nil {
		expiresAt = time.Time(*token.ExpiresAt)
	}

	return newStaticSecretResource(
		token.Name,
		resourceType,
		token.ID,
		detail,
		createdAt,
		lastUsedAt,
		expiresAt,
		token.UserID,
		parentResourceID,
	)
}

// projectAccessTokenBuilder syncs access tokens for each project (per-project
// fan-out via the project's ChildResourceType annotation).
type projectAccessTokenBuilder struct {
	client *client.GitlabClient
}

func (o *projectAccessTokenBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return projectAccessTokenResourceType
}

func (o *projectAccessTokenBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	outputAnnotations := annotations.New()
	tokens, nextPageToken, rateLimitDesc, err := o.client.ListProjectAccessTokens(ctx, parentResourceID.Resource, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		isPermissionError, unhandledErr := handlePermissionError(ctx, err, "project", parentResourceID.Resource)
		if unhandledErr != nil {
			return nil, "", outputAnnotations, unhandledErr
		}
		if isPermissionError {
			return nil, "", outputAnnotations, nil
		}
	}

	resources := make([]*v2.Resource, 0, len(tokens))
	for _, token := range tokens {
		resource, err := accessTokenResource(token, projectAccessTokenResourceType, subtypeProjectAccess, parentResourceID)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		resources = append(resources, resource)
	}

	return resources, nextPageToken, outputAnnotations, nil
}

func (o *projectAccessTokenBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *projectAccessTokenBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newProjectAccessTokenBuilder(client *client.GitlabClient) *projectAccessTokenBuilder {
	return &projectAccessTokenBuilder{client: client}
}

// groupAccessTokenBuilder syncs access tokens for each group (per-group fan-out
// via the group's ChildResourceType annotation).
type groupAccessTokenBuilder struct {
	client *client.GitlabClient
}

func (o *groupAccessTokenBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return groupAccessTokenResourceType
}

func (o *groupAccessTokenBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	groupID, err := fromGroupResourceId(parentResourceID.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	outputAnnotations := annotations.New()
	tokens, nextPageToken, rateLimitDesc, err := o.client.ListGroupAccessTokens(ctx, groupID, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		isPermissionError, unhandledErr := handlePermissionError(ctx, err, "group", groupID)
		if unhandledErr != nil {
			return nil, "", outputAnnotations, unhandledErr
		}
		if isPermissionError {
			return nil, "", outputAnnotations, nil
		}
	}

	resources := make([]*v2.Resource, 0, len(tokens))
	for _, token := range tokens {
		resource, err := accessTokenResource(token, groupAccessTokenResourceType, subtypeGroupAccess, parentResourceID)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		resources = append(resources, resource)
	}

	return resources, nextPageToken, outputAnnotations, nil
}

func (o *groupAccessTokenBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *groupAccessTokenBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newGroupAccessTokenBuilder(client *client.GitlabClient) *groupAccessTokenBuilder {
	return &groupAccessTokenBuilder{client: client}
}
