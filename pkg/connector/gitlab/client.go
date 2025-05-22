package gitlab

import (
	"context"
	"net/http"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type Client struct {
	*gitlabSDK.Client
	AccountCreationGroup string
	IsOnPremise          bool
}

func NewClient(ctx context.Context, accessToken, baseURL, accountCreationGroup string) (*Client, error) {
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
		Client:               client,
		AccountCreationGroup: accountCreationGroup,
		IsOnPremise:          baseURL != "https://gitlab.com/",
	}, nil
}

func (c *Client) GetAllUsers(ctx context.Context, nextPageToken string) ([]gitlabSDK.User, *gitlabSDK.Response, error) {
	var nextPage int
	var err error

	if nextPageToken != "" {
		nextPage, err = strconv.Atoi(nextPageToken)
		if err != nil {
			return nil, nil, err
		}
	}

	usersPath := "users"
	opt := &gitlabSDK.ListUsersOptions{
		ListOptions: gitlabSDK.ListOptions{
			Page: nextPage,
		},
	}
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
