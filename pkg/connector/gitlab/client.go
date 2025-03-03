package gitlab

import (
	"context"
	"net/http"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type Client struct {
	*gitlabSDK.Client
}

func NewClient(ctx context.Context, accessToken, baseURL string) (*Client, error) {
	httpClient, err := uhttp.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	client, err := gitlabSDK.NewClient(accessToken,
		gitlabSDK.WithBaseURL(baseURL),
		gitlabSDK.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		Client: client,
	}, nil
}

// GetAllUsers retrieves the whole list of Users of the GitLab instance.
// Endpoint: /api/v4/users
func (c *Client) GetAllUsers(ctx context.Context) ([]gitlabSDK.User, *gitlabSDK.Response, error) {
	usersPath := "users"
	opt := gitlabSDK.ListGroupMembersOptions{}
	var options []gitlabSDK.RequestOptionFunc
	options = append(options, gitlabSDK.WithContext(ctx))

	req, err := c.NewRequest(http.MethodGet, usersPath, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var users []gitlabSDK.User
	resp, err := c.Do(req, &users)
	if err != nil {
		return nil, nil, err
	}

	return users, resp, nil
}
