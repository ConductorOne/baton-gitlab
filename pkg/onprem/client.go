package onprem

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type Client struct {
	*uhttp.BaseHttpClient
}

type transport struct {
	BaseURL string
	rt      http.RoundTripper
}

// roundtrip will ensure that the request URL is properly resolved against the base URL.
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "" {
		baseURL, err := url.Parse(t.BaseURL)
		if err != nil {
			return nil, err
		}
		req.URL = baseURL.ResolveReference(req.URL)
	}
	return t.rt.RoundTrip(req)
}

func New(ctx context.Context, token, baseURL string) (*Client, error) {
	// adds token to every request
	client, err := uhttp.NewBearerAuth(token).GetClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	client = &http.Client{
		// adds token to every request and base URL to every request
		Transport: &transport{
			BaseURL: baseURL,
			rt:      client.Transport,
		},
	}

	return &Client{
		BaseHttpClient: uhttp.NewBaseHttpClient(client),
	}, nil
}

func (c *Client) GetUsers(ctx context.Context, pToken *pagination.Token) ([]User, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/api/v4/users", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// pagination docs https://docs.gitlab.com/api/rest/#keyset-based-pagination
	query := req.URL.Query()
	query.Set("pagination", "keyset")
	query.Set("order_by", "id")
	query.Set("sort", "asc")

	if pToken != nil && pToken.Token != "" {
		query.Set("cursor", pToken.Token)
	}
	req.URL.RawQuery = query.Encode()
	var target []User
	res, err := c.Do(
		req,
		uhttp.WithJSONResponse(&target),
	)

	if err != nil {
		logBody(ctx, res)
		return nil, nil, fmt.Errorf("failed to get users: %w", err)
	}

	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		logBody(ctx, res)
		return nil, nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return target, res, nil
}
