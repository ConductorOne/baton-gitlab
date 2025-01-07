package gitlab

import (
	"context"
	"fmt"
	"strconv"

	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

const DefaultGroupLimit = 2

func (o *Client) ListGroups(ctx context.Context, nextPageStr string) ([]*gitlabSDK.Group, *gitlabSDK.Response, error) {
	groups, res, err := o.Groups.ListGroups(&gitlabSDK.ListGroupsOptions{
		ListOptions: gitlabSDK.ListOptions{
			PerPage: DefaultGroupLimit,
		},
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return nil, res, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, err
	}

	return groups, res, nil
}

func (o *Client) ListGroupsPaginate(ctx context.Context, nextPageStr string) ([]*gitlabSDK.Group, *gitlabSDK.Response, error) {
	if nextPageStr == "" {
		return nil, nil, fmt.Errorf("gitlab-connector: no page given for pagination")
	}

	var nextPage int
	var err error

	if nextPageStr != "" {
		nextPage, err = strconv.Atoi(nextPageStr)
		if err != nil {
			return nil, nil, err
		}
	}

	if nextPage < 1 {
		return nil, nil, fmt.Errorf("gitlab-connector: invalid page given for pagination: %d", nextPage)
	}

	groups, res, err := o.Groups.ListGroups(&gitlabSDK.ListGroupsOptions{
		ListOptions: gitlabSDK.ListOptions{
			Page:    nextPage,
			PerPage: DefaultGroupLimit,
		},
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return nil, res, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, err
	}

	return groups, res, nil
}
