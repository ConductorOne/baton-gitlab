package gitlab

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (o *Client) GetAllUsers(ctx context.Context, nextPageToken string) ([]gitlabSDK.User, *gitlabSDK.Response, error) {
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

	req, err := o.NewRequest(http.MethodGet, usersPath, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var users []gitlabSDK.User
	resp, err := o.Do(req, &users)
	if err != nil {
		return nil, nil, err
	}

	return users, resp, nil
}

// wrapError takes the error from the request and validates the code. It wraps the error with the expected code for the SDK to handle it.
//
// TODO: Include the other codes expected by the SDK for it to behave properly.
func wrapError(err error, response *gitlabSDK.Response) error {
	if response == nil {
		return err
	}

	// Validates if the code error was a 429 (rate limit) and wraps the error with the expected code by the baton-sdk.
	if response.StatusCode == http.StatusTooManyRequests {
		st := status.New(codes.Unavailable, response.Status)
		allErrs := append([]error{st.Err()}, err)
		err = errors.Join(allErrs...)
	}

	return err
}
