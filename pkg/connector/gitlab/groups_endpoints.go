package gitlab

import (
	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

// TODO: how to handle ratelimits?
func (o *Client) ListGroups(ctx context.Context, cursor string) ([]*gitlabSDK.Group, *gitlabSDK.Response, *v2.RateLimitDescription, error) {
	groups, res, err := o.Groups.ListGroups(&gitlabSDK.ListGroupsOptions{
		ListOptions: gitlabSDK.ListOptions{
			PageToken: cursor,
		},
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return nil, res, nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, nil, err
	}
	return groups, res, nil, nil
}
