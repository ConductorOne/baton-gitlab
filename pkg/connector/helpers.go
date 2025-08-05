package connector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
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

func (u *userBuilder) getGroupID(ctx context.Context) (string, *v2.RateLimitDescription, error) {
	groupName := u.client.AccountCreationGroup
	var matchingGroups []*client.Group
	var lastRateLimitDesc *v2.RateLimitDescription
	nextPageToken := ""

	for {
		groups, returnedNextPageToken, rateLimitDesc, err := u.client.ListGroups(ctx, nextPageToken)
		lastRateLimitDesc = rateLimitDesc
		if err != nil {
			return "", lastRateLimitDesc, fmt.Errorf("error listing groups to find account creation group: %w", err)
		}

		for _, group := range groups {
			if group.Name == groupName {
				matchingGroups = append(matchingGroups, group)
			}
		}
		if returnedNextPageToken == "" {
			break
		}

		nextPageToken = returnedNextPageToken
	}

	if len(matchingGroups) == 0 {
		return "", lastRateLimitDesc, fmt.Errorf("account creation group '%s' not found", groupName)
	}
	if len(matchingGroups) > 1 {
		return "", lastRateLimitDesc, fmt.Errorf("search for account creation group '%s' returned multiple results with that exact name", groupName)
	}

	return strconv.Itoa(matchingGroups[0].ID), lastRateLimitDesc, nil
}
