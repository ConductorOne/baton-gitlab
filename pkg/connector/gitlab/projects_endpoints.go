package gitlab

import (
	"context"
	"fmt"
	"strconv"

	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

const DefaultProjectLimit = 2

func (o *Client) ListProjects(ctx context.Context, groupId string, nextPageStr string) ([]*gitlabSDK.Project, *gitlabSDK.Response, error) {
	// __AUTO_GENERATED_PRINTF_START__
	fmt.Println("ListProjects 1") // __AUTO_GENERATED_PRINTF_END__
	projects, res, err := o.Groups.ListGroupProjects(groupId, &gitlabSDK.ListGroupProjectsOptions{
		ListOptions: gitlabSDK.ListOptions{
			PerPage: DefaultProjectLimit,
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

func (o *Client) ListProjectsPaginate(ctx context.Context, groupId, nextPageStr string) ([]*gitlabSDK.Project, *gitlabSDK.Response, error) {
	// __AUTO_GENERATED_PRINTF_START__
	fmt.Println("ListProjectsPaginate 1") // __AUTO_GENERATED_PRINTF_END__
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

	// __AUTO_GENERATED_PRINT_VAR_START__
	fmt.Println(fmt.Sprintf("ListProjectsPaginate nextPage: %+v", nextPage)) // __AUTO_GENERATED_PRINT_VAR_END__

	projects, res, err := o.Groups.ListGroupProjects(groupId, &gitlabSDK.ListGroupProjectsOptions{
		ListOptions: gitlabSDK.ListOptions{
			Page:    nextPage,
			PerPage: DefaultProjectLimit,
		},
	},
		gitlabSDK.WithContext(ctx),
	)

	if err != nil {
		return nil, res, err
	}

	// __AUTO_GENERATED_PRINT_VAR_START__
	fmt.Println(fmt.Sprintf("ListProjectsPaginate projects: %+v", projects)) // __AUTO_GENERATED_PRINT_VAR_END__
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res, err
	}

	return projects, res, nil
}
