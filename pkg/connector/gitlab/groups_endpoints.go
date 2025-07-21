package gitlab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

func (o *Client) ListGroups(ctx context.Context, nextPageStr string) ([]*gitlabSDK.Group, *gitlabSDK.Response, error) {
	var nextPage int
	var err error

	if nextPageStr != "" {
		nextPage, err = strconv.Atoi(nextPageStr)
		if err != nil {
			return nil, nil, err
		}
	}

	groups, res, err := o.Groups.ListGroups(&gitlabSDK.ListGroupsOptions{
		ListOptions: gitlabSDK.ListOptions{
			Page: nextPage,
		},
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return nil, res, wrapError(err, res)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, wrapError(err, res)
	}

	return groups, res, nil
}

func (o *Client) ListGroupMembers(ctx context.Context, groupId string) ([]*gitlabSDK.GroupMember, *gitlabSDK.Response, error) {
	users, res, err := o.Groups.ListAllGroupMembers(groupId, &gitlabSDK.ListGroupMembersOptions{
		ListOptions: gitlabSDK.ListOptions{},
	},
		gitlabSDK.WithContext(ctx),
	)
	if err != nil {
		return nil, res, wrapError(err, res)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, wrapError(err, res)
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
	users, res, err := o.Groups.ListAllGroupMembers(groupId, &gitlabSDK.ListGroupMembersOptions{
		ListOptions: gitlabSDK.ListOptions{
			Page: nextPage,
		},
	},
		gitlabSDK.WithContext(ctx),
	)
	if err != nil {
		return nil, res, wrapError(err, res)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, wrapError(err, res)
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
		return wrapError(err, res)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return wrapError(err, res)
	}

	return nil
}

func (o *Client) InviteGroupMember(ctx context.Context, groupId, userEmail string, accessLevel gitlabSDK.AccessLevelValue) error {
	_, res, err := o.Invites.GroupInvites(groupId, &gitlabSDK.InvitesOptions{
		Email:       gitlabSDK.Ptr(userEmail),
		AccessLevel: gitlabSDK.Ptr(accessLevel),
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return wrapError(err, res)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return errors.Join(wrapError(err, res), fmt.Errorf("failed to invite user: status=%d, could not read response body: %w", res.StatusCode, readErr))
		}
		return errors.Join(wrapError(err, res), fmt.Errorf("failed to invite user: status=%d body=%s", res.StatusCode, string(body)))
	}

	return nil
}

func (o *Client) RemoveGroupMember(ctx context.Context, groupId string, userId int) error {
	res, err := o.GroupMembers.RemoveGroupMember(groupId, userId,
		&gitlabSDK.RemoveGroupMemberOptions{},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return wrapError(err, res)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return wrapError(err, res)
	}

	return nil
}
