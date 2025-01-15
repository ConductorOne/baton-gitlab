package gitlab

import (
	"context"
	"fmt"
	"strconv"

	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

func (o *Client) ListGroups(ctx context.Context) ([]*gitlabSDK.Group, *gitlabSDK.Response, error) {
	groups, res, err := o.Groups.ListGroups(&gitlabSDK.ListGroupsOptions{
		ListOptions: gitlabSDK.ListOptions{},
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
			Page: nextPage,
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

func (o *Client) ListGroupMembers(ctx context.Context, groupId string) ([]*gitlabSDK.GroupMember, *gitlabSDK.Response, error) {
	users, res, err := o.Groups.ListGroupMembers(groupId, &gitlabSDK.ListGroupMembersOptions{
		ListOptions: gitlabSDK.ListOptions{},
	},
		gitlabSDK.WithContext(ctx),
	)
	if err != nil {
		return nil, res, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, err
	}

	return users, res, nil
}

func (o *Client) ListGroupMembersPaginate(ctx context.Context, groupId string, nextPageStr string) ([]*gitlabSDK.GroupMember, *gitlabSDK.Response, error) {
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
	users, res, err := o.Groups.ListGroupMembers(groupId, &gitlabSDK.ListGroupMembersOptions{
		ListOptions: gitlabSDK.ListOptions{
			Page: nextPage,
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

	return users, res, nil
}

func (o *Client) AddGroupMember(ctx context.Context, groupId string, userId int, accessLevel gitlabSDK.AccessLevelValue) error {
	_, res, err := o.GroupMembers.AddGroupMember(groupId, &gitlabSDK.AddGroupMemberOptions{
		UserID:      gitlabSDK.Ptr(userId),
		AccessLevel: gitlabSDK.Ptr(accessLevel),
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return err
	}

	return nil
}

func (o *Client) RemoveGroupMember(ctx context.Context, groupId string, userId int) error {
	res, err := o.GroupMembers.RemoveGroupMember(groupId, userId,
		&gitlabSDK.RemoveGroupMemberOptions{},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return err
	}

	return nil
}
