package connector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func toGroupResourceId(groupId string) string {
	return fmt.Sprintf("g/%s", groupId)
}

func fromGroupResourceId(groupResourceId string) (string, error) {
	parts := strings.Split(groupResourceId, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid group resource id: %s", groupResourceId)
	}
	return parts[1], nil
}

func parseAccessLevelFromEntitlementID(entitlementID string) (int, error) {
	parts := strings.Split(entitlementID, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid entitlement ID: %s", entitlementID)
	}

	levelName := parts[2]
	levelValue, ok := client.GetAccessLevelByName(levelName)
	if !ok {
		return 0, fmt.Errorf("unknown access level: %s", levelName)
	}
	return int(levelValue), nil
}

func handlePermissionError(ctx context.Context, err error, resourceType, resourceId string) (bool, error) {
	if err == nil {
		return false, nil
	}

	// Some GitLab endpoints (group/project access tokens, service accounts)
	// return 401 rather than 403 when the token lacks the required role on a
	// specific resource. The token itself is valid (Validate already exercised
	// it), so treat both as "skip this resource" rather than failing the sync.
	code := status.Code(err)
	if code == codes.PermissionDenied || code == codes.Unauthenticated ||
		errors.Is(err, client.ErrForbidden) || errors.Is(err, client.ErrUnauthorized) {
		l := ctxzap.Extract(ctx)
		l.Warn(
			fmt.Sprintf("Permission denied while listing %s. Skipping.", resourceType),
			zap.String(fmt.Sprintf("%s_id", resourceType), resourceId),
		)
		return true, nil
	}

	return false, err
}
