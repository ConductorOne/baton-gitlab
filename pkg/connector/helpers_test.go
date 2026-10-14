package connector

import (
	"net/http"
	"strings"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
)

func gitlabErrorResponse(statusCode int, body string) error {
	var gitlabErr client.GitlabError
	return uhttp.WithErrorResponse(&gitlabErr)(&uhttp.WrapperResponse{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Body:       []byte(body),
		Header:     http.Header{"Content-Type": {"application/json"}},
	})
}

func TestIsAlreadyAtOrAboveLevelError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		wantCode    codes.Code
		wantMessage string
		wantHeld    bool
	}{
		{
			name:        "access level validation object",
			body:        `{"message":{"access_level":["should be greater than or equal to Developer inherited membership from group Example"]}}`,
			wantCode:    codes.InvalidArgument,
			wantMessage: "should be greater than or equal to Developer",
			wantHeld:    true,
		},
		{
			name:        "plain string message",
			body:        `{"message":"400 Bad request - user_id is missing"}`,
			wantCode:    codes.InvalidArgument,
			wantMessage: "user_id is missing",
		},
		{
			name:        "other validation object",
			body:        `{"message":{"expires_at":["must be in the future"]}}`,
			wantCode:    codes.InvalidArgument,
			wantMessage: "{expires_at: [must be in the future]}",
		},
		{
			name:        "no message field",
			body:        `{"error":"insufficient_scope"}`,
			wantCode:    codes.InvalidArgument,
			wantMessage: "Unknown error from Gitlab API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := gitlabErrorResponse(http.StatusBadRequest, tt.body)
			if got := status.Code(err); got != tt.wantCode {
				t.Fatalf("status.Code = %v, want %v: %v", got, tt.wantCode, err)
			}
			if !strings.Contains(status.Convert(err).Message(), tt.wantMessage) {
				t.Errorf("message %q does not carry %q", status.Convert(err).Message(), tt.wantMessage)
			}
			if got := isAlreadyAtOrAboveLevelError(err); got != tt.wantHeld {
				t.Errorf("isAlreadyAtOrAboveLevelError = %v, want %v: %v", got, tt.wantHeld, err)
			}
		})
	}
}
