package onprem

import (
	"context"
	"net/http"
	"net/url"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/peterhellberg/link"
	"go.uber.org/zap"
)

func logBody(ctx context.Context, response *http.Response) {
	if response == nil {
		return
	}
	l := ctxzap.Extract(ctx)
	body := make([]byte, 10000)
	n, err := response.Body.Read(body)
	if err != nil {
		l.Error("error reading response body", zap.Error(err))
		return
	}
	l.Info("response body: ", zap.String("body", string(body[:n])))
}

func HasNextToken(res *http.Response) bool {
	for _, l := range link.ParseResponse(res) {
		if l.Rel == "next" {
			return true
		}
	}
	return false
}

// https://docs.sentry.io/api/pagination/
func NextToken(ctx context.Context, res *http.Response) string {
	logger := ctxzap.Extract(ctx)
	for _, l := range link.ParseResponse(res) {
		if l.Rel == "next" {
			nextURL, err := url.Parse(l.URI)
			if err != nil {
				logger.Error("failed to parse next token URL", zap.Error(err))
				return ""
			}
			return nextURL.Query().Get("cursor")
		}
	}
	return ""
}
