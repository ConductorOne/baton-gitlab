package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// An ErrorResponse reports one or more errors caused by an API request.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/index.html#data-validation-and-error-reporting

var ErrNotFound = errors.New("404 Not Found")
var ErrForbidden = errors.New("403 Forbidden")

type ErrorResponse struct {
	Body     []byte
	Response *http.Response
	Message  string
}

// GitlabError is the body of a GitLab error response. Its "message" is a string
// for most errors and an object of field errors for validation failures, such as
// {"access_level":["should be greater than or equal to ..."]}.
type GitlabError struct {
	Detail json.RawMessage `json:"message"`
}

// Message implements the uhttp.ErrorResponse interface.
func (e *GitlabError) Message() string {
	var raw interface{}
	if len(e.Detail) == 0 || json.Unmarshal(e.Detail, &raw) != nil || raw == nil {
		return "Unknown error from Gitlab API"
	}
	if detail := parseError(raw); detail != "" {
		return detail
	}
	return "Unknown error from Gitlab API"
}

func (e *ErrorResponse) Error() string {
	path, _ := url.QueryUnescape(e.Response.Request.URL.Path)
	url := fmt.Sprintf("%s://%s%s", e.Response.Request.URL.Scheme, e.Response.Request.URL.Host, path)

	if e.Message == "" {
		return fmt.Sprintf("%s %s: %d", e.Response.Request.Method, url, e.Response.StatusCode)
	} else {
		return fmt.Sprintf("%s %s: %d %s", e.Response.Request.Method, url, e.Response.StatusCode, e.Message)
	}
}

// CheckResponse checks the API response for errors, and returns them if present.
func CheckResponse(r *http.Response) error {
	switch r.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent, http.StatusNotModified:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusForbidden:
		return ErrForbidden
	}

	errorResponse := &ErrorResponse{Response: r}
	data, err := io.ReadAll(r.Body)
	if err == nil && strings.TrimSpace(string(data)) != "" {
		errorResponse.Body = data

		var raw interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			errorResponse.Message = fmt.Sprintf("failed to parse unknown error format: %s", data)
		} else {
			errorResponse.Message = parseError(raw)
		}
	}

	return errorResponse
}

func parseError(raw interface{}) string {
	switch raw := raw.(type) {
	case string:
		return raw

	case []interface{}:
		var errs []string
		for _, v := range raw {
			errs = append(errs, parseError(v))
		}
		return fmt.Sprintf("[%s]", strings.Join(errs, ", "))

	case map[string]interface{}:
		var errs []string
		for k, v := range raw {
			errs = append(errs, fmt.Sprintf("{%s: %s}", k, parseError(v)))
		}
		sort.Strings(errs)
		return strings.Join(errs, ", ")

	default:
		return fmt.Sprintf("failed to parse unexpected error type: %T", raw)
	}
}
