package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

type KeysetPaginationOpts struct {
	OrderBy string
	Sort    string
}

func (c *GitlabClient) listWithKeysetPagination(
	ctx context.Context,
	baseEndpoint string,
	nextLink string,
	target interface{},
	opts KeysetPaginationOpts,
) (string, *v2.RateLimitDescription, error) {
	endpointURL := baseEndpoint
	if nextLink != "" {
		nextURL, err := url.Parse(nextLink)
		if err != nil {
			return "", nil, fmt.Errorf("invalid pagination URL:  %w", err)
		}
		endpointURL = fmt.Sprintf("%s?%s", nextURL.Path, nextURL.RawQuery)
	}

	apiURL, err := url.Parse(endpointURL)
	if err != nil {
		return "", nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if nextLink == "" {
		q := apiURL.Query()
		q.Set("pagination", "keyset")
		q.Set("order_by", opts.OrderBy)
		q.Set("sort", opts.Sort)
		q.Set("per_page", "100")
		apiURL.RawQuery = q.Encode()
	}

	headers, rateLimitDesc, err := c.doRequest(ctx, http.MethodGet, apiURL.String(), target, nil)
	if err != nil {
		return "", rateLimitDesc, err
	}

	newNextLink := parseNextLinkHeader(headers.Get("Link"))
	return newNextLink, rateLimitDesc, nil
}

func parseNextLinkHeader(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		if strings.Contains(link, `rel="next"`) {
			parts := strings.Split(link, ";")
			if len(parts) > 0 {
				return strings.Trim(strings.TrimSpace(parts[0]), "<>")
			}
		}
	}

	return ""
}

func WithOffsetPagination(url *url.URL, nextPageToken string) {
	q := url.Query()
	q.Set("per_page", "100")
	if nextPageToken != "" {
		q.Set("page", nextPageToken)
	}
	url.RawQuery = q.Encode()
}

func WithSearch(url *url.URL, search string) {
	q := url.Query()
	if search != "" {
		q.Set("search", search)
	}
	url.RawQuery = q.Encode()
}

func PathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), ".", "%2E")
}

func (t *ISOTime) MarshalJSON() ([]byte, error) {
	if t == nil || (time.Time(*t)).IsZero() {
		return []byte(`null`), nil
	}

	if y := time.Time(*t).Year(); y < 0 || y >= 10000 {
		// ISO 8601 uses 4 digits for the years.
		return nil, errors.New("json: ISOTime year outside of range [0,9999]")
	}

	b := make([]byte, 0, len(iso8601)+2)
	b = append(b, '"')
	b = (time.Time(*t)).AppendFormat(b, iso8601)
	b = append(b, '"')

	return b, nil
}

func (t *ISOTime) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if string(data) == "null" {
		return nil
	}

	isoTime, err := time.Parse(`"`+iso8601+`"`, string(data))
	if err != nil {
		return err
	}
	*t = ISOTime(isoTime)

	return nil
}

func (t *ISOTime) EncodeValues(key string, v *url.Values) error {
	if t == nil || (time.Time(*t)).IsZero() {
		return nil
	}
	v.Add(key, t.String())
	return nil
}

func (t *ISOTime) String() string {
	if t == nil {
		return ""
	}
	return time.Time(*t).Format(iso8601)
}

type AccessLevelValue int

const (
	NoPermissions            AccessLevelValue = 0
	MinimalAccessPermissions AccessLevelValue = 5
	GuestPermissions         AccessLevelValue = 10
	ReporterPermissions      AccessLevelValue = 20
	DeveloperPermissions     AccessLevelValue = 30
	MaintainerPermissions    AccessLevelValue = 40
	OwnerPermissions         AccessLevelValue = 50
	AdminPermissions         AccessLevelValue = 60
)

var accessLevelMap = map[string]AccessLevelValue{
	"None":       NoPermissions,
	"Minimal":    MinimalAccessPermissions,
	"Guest":      GuestPermissions,
	"Reporter":   ReporterPermissions,
	"Developer":  DeveloperPermissions,
	"Maintainer": MaintainerPermissions,
	"Owner":      OwnerPermissions,
	"Admin":      AdminPermissions,
}

func GetAccessLevelByName(name string) (AccessLevelValue, bool) {
	level, ok := accessLevelMap[name]
	return level, ok
}

func (a AccessLevelValue) String() string {
	switch a {
	case NoPermissions:
		return "None"
	case MinimalAccessPermissions:
		return "Minimal"
	case GuestPermissions:
		return "Guest"
	case ReporterPermissions:
		return "Reporter"
	case DeveloperPermissions:
		return "Developer"
	case MaintainerPermissions:
		return "Maintainer"
	case OwnerPermissions:
		return "Owner"
	case AdminPermissions:
		return "Admin"
	default:
		return ""
	}
}

// FindGroupByName searches for groups across all pages and then filters for an exact match.
func (c *GitlabClient) FindGroupByName(ctx context.Context, groupName string) (*Group, *v2.RateLimitDescription, error) {
	var allCandidateGroups []*Group
	var lastRateLimitDesc *v2.RateLimitDescription
	nextPageToken := ""

	for {
		candidateGroups, returnedNextPageToken, rateLimitDesc, err := c.SearchGroups(ctx, groupName, nextPageToken)
		lastRateLimitDesc = rateLimitDesc
		if err != nil {
			return nil, lastRateLimitDesc, fmt.Errorf("error searching for group '%s': %w", groupName, err)
		}

		allCandidateGroups = append(allCandidateGroups, candidateGroups...)

		if returnedNextPageToken == "" {
			break
		}
		nextPageToken = returnedNextPageToken
	}

	var exactMatches []*Group
	for _, group := range allCandidateGroups {
		if group.Name == groupName {
			exactMatches = append(exactMatches, group)
		}
	}

	if len(exactMatches) == 0 {
		return nil, lastRateLimitDesc, fmt.Errorf("group '%s' not found among potential matches", groupName)
	}

	if len(exactMatches) > 1 {
		return nil, lastRateLimitDesc, fmt.Errorf("search for group '%s' returned multiple exact matches by name", groupName)
	}

	return exactMatches[0], lastRateLimitDesc, nil
}
