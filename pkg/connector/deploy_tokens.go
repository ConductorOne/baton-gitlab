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

type deployTokenBuilder struct {
	client *client.GitlabClient
}

func (o *deployTokenBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return deployTokenResourceType
}

// List returns instance deploy tokens as K1 STATIC_SECRET resources. Requires an
// administrator token. Deploy tokens are not tied to a user, so no SecretTrait
// identity back-reference is emitted.
func (o *deployTokenBuilder) List(ctx context.Context, _ *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	outputAnnotations := annotations.New()
	tokens, nextPageToken, rateLimitDesc, err := o.client.ListDeployTokens(ctx, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, fmt.Errorf("failed to list deploy tokens: %w", err)
	}

	resources := make([]*v2.Resource, 0, len(tokens))
	for _, token := range tokens {
		resource, err := deployTokenResource(token)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		resources = append(resources, resource)
	}

	return resources, nextPageToken, outputAnnotations, nil
}

func (o *deployTokenBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *deployTokenBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func deployTokenResource(token *client.DeployToken) (*v2.Resource, error) {
	var expiresAt time.Time
	if token.ExpiresAt != nil {
		expiresAt = *token.ExpiresAt
	}

	return newStaticSecretResource(
		token.Name,
		deployTokenResourceType,
		token.ID,
		subtypeDeploy,
		time.Time{},
		time.Time{},
		expiresAt,
		0,
		nil,
	)
}

func newDeployTokenBuilder(client *client.GitlabClient) *deployTokenBuilder {
	return &deployTokenBuilder{
		client: client,
	}
}
