package client

import (
	"fmt"
	"net/http"
	"net/url"

	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

// ListPersonalAccessTokens lists personal access token metadata. With an admin
// token this returns all PATs on the instance; otherwise it returns the
// authenticated user's PATs. The token value is never returned.
// https://docs.gitlab.com/api/personal_access_tokens/#list-personal-access-tokens
func (c *GitlabClient) ListPersonalAccessTokens(ctx context.Context, nextPageToken string) ([]*PersonalAccessToken, string, *v2.RateLimitDescription, error) {
	var tokens []*PersonalAccessToken

	apiURL, _ := url.Parse("/api/v4/personal_access_tokens")
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &tokens, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return tokens, headers.Get("X-Next-Page"), rateLimitDesc, nil
}

// ListDeployTokens lists all deploy token metadata on the instance. Requires an
// administrator token. The token value is never returned.
// https://docs.gitlab.com/api/deploy_tokens/#list-all-deploy-tokens
func (c *GitlabClient) ListDeployTokens(ctx context.Context, nextPageToken string) ([]*DeployToken, string, *v2.RateLimitDescription, error) {
	var tokens []*DeployToken

	apiURL, _ := url.Parse("/api/v4/deploy_tokens")
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &tokens, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return tokens, headers.Get("X-Next-Page"), rateLimitDesc, nil
}

// ListInstanceServiceAccounts lists service account users on a self-managed
// instance. Requires an administrator token.
// https://docs.gitlab.com/api/users/#list-service-account-users
func (c *GitlabClient) ListInstanceServiceAccounts(ctx context.Context, nextPageToken string) ([]*ServiceAccount, string, *v2.RateLimitDescription, error) {
	var accounts []*ServiceAccount

	apiURL, _ := url.Parse("/api/v4/service_accounts")
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &accounts, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return accounts, headers.Get("X-Next-Page"), rateLimitDesc, nil
}

// ListGroupServiceAccounts lists service account users owned by a top-level
// group (GitLab 17.1+). Used on GitLab.com where the instance endpoint is
// unavailable.
// https://docs.gitlab.com/api/group_service_accounts/#list-service-account-users
func (c *GitlabClient) ListGroupServiceAccounts(ctx context.Context, groupID string, nextPageToken string) ([]*ServiceAccount, string, *v2.RateLimitDescription, error) {
	var accounts []*ServiceAccount

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/groups/%s/service_accounts", PathEscape(groupID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &accounts, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return accounts, headers.Get("X-Next-Page"), rateLimitDesc, nil
}

// ListProjectAccessTokens lists access token metadata for a single project.
// Each token is backed by a per-project bot user. The token value is never
// returned by the list endpoint.
// https://docs.gitlab.com/api/project_access_tokens/#list-all-project-access-tokens
func (c *GitlabClient) ListProjectAccessTokens(ctx context.Context, projectID string, nextPageToken string) ([]*AccessToken, string, *v2.RateLimitDescription, error) {
	var tokens []*AccessToken

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/projects/%s/access_tokens", PathEscape(projectID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &tokens, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return tokens, headers.Get("X-Next-Page"), rateLimitDesc, nil
}

// ListGroupAccessTokens lists access token metadata for a single group. Each
// token is backed by a per-group bot user. The token value is never returned by
// the list endpoint.
// https://docs.gitlab.com/api/group_access_tokens/#list-all-group-access-tokens
func (c *GitlabClient) ListGroupAccessTokens(ctx context.Context, groupID string, nextPageToken string) ([]*AccessToken, string, *v2.RateLimitDescription, error) {
	var tokens []*AccessToken

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/groups/%s/access_tokens", PathEscape(groupID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &tokens, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return tokens, headers.Get("X-Next-Page"), rateLimitDesc, nil
}
