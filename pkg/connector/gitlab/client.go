package gitlab

import (
	"context"

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
