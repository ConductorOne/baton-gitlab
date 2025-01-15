package gitlab

import (
	"context"
	"fmt"
	"strconv"

	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

func (o *Client) ListProjects(ctx context.Context, groupId string) ([]*gitlabSDK.Project, *gitlabSDK.Response, error) {
	projects, res, err := o.Groups.ListGroupProjects(groupId, &gitlabSDK.ListGroupProjectsOptions{
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

	return projects, res, nil
}

func (o *Client) ListProjectsPaginate(ctx context.Context, groupId, nextPageStr string) ([]*gitlabSDK.Project, *gitlabSDK.Response, error) {
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

	projects, res, err := o.Groups.ListGroupProjects(groupId, &gitlabSDK.ListGroupProjectsOptions{
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

	return projects, res, nil
}

func (o *Client) ListProjectMembers(ctx context.Context, projectId string) ([]*gitlabSDK.ProjectMember, *gitlabSDK.Response, error) {
	users, res, err := o.ProjectMembers.ListAllProjectMembers(projectId, &gitlabSDK.ListProjectMembersOptions{
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

func (o *Client) ListProjectMembersPaginate(ctx context.Context, projectId, nextPageStr string) ([]*gitlabSDK.ProjectMember, *gitlabSDK.Response, error) {
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
	users, res, err := o.ProjectMembers.ListAllProjectMembers(projectId, &gitlabSDK.ListProjectMembersOptions{
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

func (o *Client) AddProjectMember(ctx context.Context, projectId string, userId int, accessLevel gitlabSDK.AccessLevelValue) (*gitlabSDK.ProjectMember, error) {
	user, res, err := o.ProjectMembers.AddProjectMember(projectId, &gitlabSDK.AddProjectMemberOptions{
		UserID:      gitlabSDK.Ptr(userId),
		AccessLevel: gitlabSDK.Ptr(accessLevel),
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, err
	}

	return user, nil
}

func (o *Client) RemoveProjectMember(ctx context.Context, projectId string, userId int) error {
	res, err := o.ProjectMembers.DeleteProjectMember(projectId, userId,
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
