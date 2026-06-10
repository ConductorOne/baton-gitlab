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

type personalAccessTokenBuilder struct {
	client *client.GitlabClient
}

func (o *personalAccessTokenBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return personalAccessTokenResourceType
}

// List returns personal access tokens as K1 STATIC_SECRET resources. With an
// admin token this enumerates every PAT on the instance; otherwise it returns
// the authenticated user's tokens. Each token's owning user (user_id) is set as
// the SecretTrait identity back-reference.
func (o *personalAccessTokenBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	outputAnnotations := annotations.New()
	tokens, nextPageToken, rateLimitDesc, err := o.client.ListPersonalAccessTokens(ctx, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		// Listing every instance PAT requires an admin token; a non-admin token
		// gets 403. Skip rather than failing the whole sync.
		isPermissionError, unhandledErr := handlePermissionError(ctx, err, "personal_access_token", "")
		if unhandledErr != nil {
			return nil, "", outputAnnotations, fmt.Errorf("failed to list personal access tokens: %w", unhandledErr)
		}
		if isPermissionError {
			return nil, "", outputAnnotations, nil
		}
	}

	resources := make([]*v2.Resource, 0, len(tokens))
	for _, token := range tokens {
		resource, err := personalAccessTokenResource(token)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		resources = append(resources, resource)
	}

	return resources, nextPageToken, outputAnnotations, nil
}

func (o *personalAccessTokenBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *personalAccessTokenBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func personalAccessTokenResource(token *client.PersonalAccessToken) (*v2.Resource, error) {
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
		personalAccessTokenResourceType,
		token.ID,
		subtypePAT,
		createdAt,
		lastUsedAt,
		expiresAt,
		token.UserID,
		nil,
	)
}

func newPersonalAccessTokenBuilder(client *client.GitlabClient) *personalAccessTokenBuilder {
	return &personalAccessTokenBuilder{
		client: client,
	}
}
