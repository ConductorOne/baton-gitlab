package connector

import (
	"fmt"
	"strings"
)

func toGroupResourceId(groupId, groupName string) string {
	return fmt.Sprintf("%s/%s", groupId, groupName)
}

func toProjectResourceId(groupName, projectName string) string {
	return fmt.Sprintf("%s/%s", groupName, projectName)
}

func fromGroupResourceId(groupResourceId string) (string, string, error) {
	parts := strings.Split(groupResourceId, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid group resource id: %s", groupResourceId)
	}
	return parts[0], parts[1], nil
}
