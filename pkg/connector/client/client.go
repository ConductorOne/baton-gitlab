package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const sortAscending = "asc"

type GitlabClient struct {
	httpClient            *uhttp.BaseHttpClient
	baseURL               string
	AccountCreationGroup  string
	IsOnPremise           bool
	accessToken           string
	SyncDirectMembersOnly bool
}

func New(ctx context.Context, accessToken, baseURL, accountCreationGroup string, syncDirectMembersOnly bool) (*GitlabClient, error) {
	options := []uhttp.Option{uhttp.WithLogger(true, ctxzap.Extract(ctx)), uhttp.WithUserAgent("baton-gitlab/1.0")}

	client, err := uhttp.NewClient(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client failed: %w", err)
	}

	httpClient, err := uhttp.NewBaseHttpClientWithContext(ctx, client)
	if err != nil {
		return nil, err
	}

	baseURLTrimmed := strings.TrimRight(baseURL, "/")

	return &GitlabClient{
		httpClient:            httpClient,
		baseURL:               baseURLTrimmed,
		AccountCreationGroup:  accountCreationGroup,
		IsOnPremise:           baseURLTrimmed != "https://gitlab.com",
		accessToken:           accessToken,
		SyncDirectMembersOnly: syncDirectMembersOnly,
	}, nil
}

func (c *GitlabClient) doRequest(ctx context.Context, method string, endpoint string, target interface{}, body interface{}) (*http.Header, *v2.RateLimitDescription, error) {
	endpoint = fmt.Sprintf("%s%s", c.baseURL, endpoint)
	relativeURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	var requestOptions []uhttp.RequestOption
	requestOptions = append(requestOptions,
		uhttp.WithAcceptJSONHeader(),
		uhttp.WithHeader("Private-Token", c.accessToken))
	if body != nil {
		requestOptions = append(requestOptions, uhttp.WithContentTypeJSONHeader(), uhttp.WithJSONBody(body))
	}

	request, err := c.httpClient.NewRequest(ctx, method, relativeURL, requestOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	var rateLimitData v2.RateLimitDescription
	var errorResponse GitlabError
	doOptions := []uhttp.DoOption{
		uhttp.WithRatelimitData(&rateLimitData),
		uhttp.WithErrorResponse(&errorResponse),
	}
	if target != nil {
		doOptions = append(doOptions, uhttp.WithJSONResponse(target))
	}

	response, err := c.httpClient.Do(request, doOptions...)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if closeErr != nil {
			log.Printf("warning: failed to close response body: %v", closeErr)
		}
	}()

	if response.StatusCode >= 300 {
		apiError := CheckResponse(response)
		if apiError != nil {
			return &response.Header, &rateLimitData, apiError
		}
	}

	return &response.Header, &rateLimitData, nil
}

// ListUsers retrieves all users from GitLab API.
func (c *GitlabClient) ListUsers(ctx context.Context, nextLink string) ([]*User, string, *v2.RateLimitDescription, error) {
	var users []*User
	opts := KeysetPaginationOpts{OrderBy: "id", Sort: sortAscending}

	newNextLink, rateLimitDesc, err := c.listWithKeysetPagination(ctx, "/api/v4/users", nextLink, &users, opts)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return users, newNextLink, rateLimitDesc, nil
}

// CreateUser creates a new user in GitLab.
func (c *GitlabClient) CreateUser(ctx context.Context, userRequest *CreateUserOptions) (*User, *v2.RateLimitDescription, error) {
	var user User

	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodPost, "/api/v4/users", &user, userRequest)
	if err != nil {
		return nil, rateLimitDesc, err
	}

	return &user, rateLimitDesc, nil
}

func (c *GitlabClient) GetCurrentlyAuthenticatedUser(ctx context.Context) (*User, *v2.RateLimitDescription, error) {
	var user User

	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, "/api/v4/user", &user, nil)
	if err != nil {
		return nil, rateLimitDesc, err
	}

	return &user, rateLimitDesc, nil
}

// DeleteUser deletes a user from GitLab.
func (c *GitlabClient) DeleteUser(ctx context.Context, userID string) (*v2.RateLimitDescription, error) {
	path := fmt.Sprintf("/api/v4/users/%s", PathEscape(userID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodDelete, path, nil, nil)

	return rateLimitDesc, err
}

// ListGroups retrieves all groups from GitLab API using keyset pagination.
func (c *GitlabClient) ListGroups(ctx context.Context, nextLink string) ([]*Group, string, *v2.RateLimitDescription, error) {
	var groups []*Group
	opts := KeysetPaginationOpts{OrderBy: "name", Sort: sortAscending}

	newNextLink, rateLimitDesc, err := c.listWithKeysetPagination(ctx, "/api/v4/groups", nextLink, &groups, opts)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return groups, newNextLink, rateLimitDesc, nil
}

// GetGroup retrieves a specific group by ID.
func (c *GitlabClient) GetGroup(ctx context.Context, groupID string) (*Group, *v2.RateLimitDescription, error) {
	var group Group

	path := fmt.Sprintf("/api/v4/groups/%s", PathEscape(groupID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, path, &group, nil)
	if err != nil {
		return nil, rateLimitDesc, err
	}

	return &group, rateLimitDesc, nil
}

// ListAllGroupMembers retrieves members and pending invites of a specific group including inherited members through ancestor groups.
func (c *GitlabClient) ListAllGroupMembers(ctx context.Context, groupID string, nextPageToken string) ([]*GroupMember, string, *v2.RateLimitDescription, error) {
	var members []*GroupMember

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/groups/%s/members/all", PathEscape(groupID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &members, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	nextToken := headers.Get("X-Next-Page")
	return members, nextToken, rateLimitDesc, nil
}

// ListGroupMembers retrieves members of a specific group.
func (c *GitlabClient) ListGroupMembers(ctx context.Context, groupID string, nextPageToken string) ([]*GroupMember, string, *v2.RateLimitDescription, error) {
	var members []*GroupMember

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/groups/%s/members", PathEscape(groupID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &members, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	nextToken := headers.Get("X-Next-Page")
	return members, nextToken, rateLimitDesc, nil
}

// AddGroupMember adds a member to a group.
func (c *GitlabClient) AddGroupMember(ctx context.Context, groupID string, memberRequest *AddGroupMemberRequest) (*GroupMember, *v2.RateLimitDescription, error) {
	var member GroupMember

	path := fmt.Sprintf("/api/v4/groups/%s/members", PathEscape(groupID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodPost, path, &member, memberRequest)
	if err != nil {
		return nil, rateLimitDesc, err
	}

	return &member, rateLimitDesc, nil
}

// RemoveGroupMember removes a member from a group.
func (c *GitlabClient) RemoveGroupMember(ctx context.Context, groupID, userID string) (*v2.RateLimitDescription, error) {
	path := fmt.Sprintf("/api/v4/groups/%s/members/%s", PathEscape(groupID), PathEscape(userID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodDelete, path, nil, nil)

	return rateLimitDesc, err
}

// SearchGroups searches for groups by name.
func (c *GitlabClient) SearchGroups(ctx context.Context, search string, nextPageToken string) ([]*Group, string, *v2.RateLimitDescription, error) {
	var groups []*Group

	apiURL, _ := url.Parse("/api/v4/groups")
	WithSearch(apiURL, search)
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &groups, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	nextToken := headers.Get("X-Next-Page")
	return groups, nextToken, rateLimitDesc, nil
}

// InviteGroupMember invites a member to a group.
func (c *GitlabClient) InviteGroupMember(ctx context.Context, groupID string, userEmail string, accessLevel AccessLevelValue) (*v2.RateLimitDescription, error) {
	inviteRequest := &InviteGroupMemberRequest{
		Email:       userEmail,
		AccessLevel: accessLevel,
	}
	path := fmt.Sprintf("/api/v4/groups/%s/invitations", PathEscape(groupID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodPost, path, nil, inviteRequest)

	return rateLimitDesc, err
}

// ListPendingGroupInvitations lists pending invitations for a group.
func (c *GitlabClient) ListPendingGroupInvitations(ctx context.Context, groupID string, nextPageToken string) ([]*PendingInviteUser, string, *v2.RateLimitDescription, error) {
	var pendingInvites []*PendingInviteUser

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/groups/%s/invitations", PathEscape(groupID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &pendingInvites, nil)
	if err != nil {
		if status.Code(err) == codes.PermissionDenied || errors.Is(err, ErrForbidden) {
			return nil, "", nil, nil
		}
		return nil, "", rateLimitDesc, err
	}

	nextToken := headers.Get("X-Next-Page")
	return pendingInvites, nextToken, rateLimitDesc, nil
}

// ListProjects retrieves all projects from GitLab API.
func (c *GitlabClient) ListProjects(ctx context.Context, groupID string, nextLink string) ([]*Project, string, *v2.RateLimitDescription, error) {
	var projects []*Project
	endpoint := fmt.Sprintf("/api/v4/groups/%s/projects", PathEscape(groupID))
	opts := KeysetPaginationOpts{OrderBy: "id", Sort: sortAscending}

	newNextLink, rateLimitDesc, err := c.listWithKeysetPagination(ctx, endpoint, nextLink, &projects, opts,
		WithQueryParam("include_subgroups", "true"),
		WithQueryParam("simple", "true"),
	)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	return projects, newNextLink, rateLimitDesc, nil
}

// GetProject retrieves a specific project by ID.
func (c *GitlabClient) GetProject(ctx context.Context, projectID string) (*Project, *v2.RateLimitDescription, error) {
	var project Project

	path := fmt.Sprintf("/api/v4/projects/%s", PathEscape(projectID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, path, &project, nil)
	if err != nil {
		return nil, rateLimitDesc, err
	}

	return &project, rateLimitDesc, nil
}

// ListProjectMembers retrieves members of a specific project.
func (c *GitlabClient) ListProjectMembers(ctx context.Context, projectID string, nextPageToken string) ([]*ProjectMember, string, *v2.RateLimitDescription, error) {
	var members []*ProjectMember

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/projects/%s/members", PathEscape(projectID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &members, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	nextToken := headers.Get("X-Next-Page")
	return members, nextToken, rateLimitDesc, nil
}

func (c *GitlabClient) ListAllProjectMembers(ctx context.Context, projectID string, nextPageToken string) ([]*ProjectMember, string, *v2.RateLimitDescription, error) {
	var members []*ProjectMember

	apiURL, _ := url.Parse(fmt.Sprintf("/api/v4/projects/%s/members/all", PathEscape(projectID)))
	WithOffsetPagination(apiURL, nextPageToken)
	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), &members, nil)
	if err != nil {
		return nil, "", rateLimitDesc, err
	}

	nextToken := headers.Get("X-Next-Page")
	return members, nextToken, rateLimitDesc, nil
}

// AddProjectMember adds a member to a project.
func (c *GitlabClient) AddProjectMember(ctx context.Context, projectID string, memberRequest *AddProjectMemberRequest) (*ProjectMember, *v2.RateLimitDescription, error) {
	var member ProjectMember

	path := fmt.Sprintf("/api/v4/projects/%s/members", PathEscape(projectID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodPost, path, &member, memberRequest)
	if err != nil {
		return nil, rateLimitDesc, err
	}

	return &member, rateLimitDesc, nil
}

// RemoveProjectMember removes a member from a project.
func (c *GitlabClient) RemoveProjectMember(ctx context.Context, projectID, userID string) (*v2.RateLimitDescription, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/members/%s", PathEscape(projectID), PathEscape(userID))
	_, rateLimitDesc, err := c.doRequest(ctx, http.MethodDelete, path, nil, nil)

	return rateLimitDesc, err
}
