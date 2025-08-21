package connector

import (
	"fmt"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
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

func gitlabFullGroupID(groupId int) string {
	return fmt.Sprintf("gid://gitlab/Group/%d", groupId)
}
