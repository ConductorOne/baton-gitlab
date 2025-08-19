package connector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type groupBuilder struct {
	client *client.GitlabClient
}

var groupAccessLevels = []client.AccessLevelValue{
	client.MinimalAccessPermissions,
	client.GuestPermissions,
	client.ReporterPermissions,
	client.DeveloperPermissions,
	client.MaintainerPermissions,
	client.OwnerPermissions,
}

func (o *groupBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return groupResourceType
}

func (o *groupBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	l.Info("gitlab-connector: list groups", zap.Any("parentResourceID", parentResourceID))
	if parentResourceID != nil {
		return nil, "", nil, nil
	}
	var (
		groups            []*client.Group
		outputAnnotations = annotations.New()
		pageToken         string
		err               error
	)
	if pToken != nil {
		pageToken = pToken.Token
	}

	groups, nextPageToken, rateLimitDesc, err := o.client.ListGroups(ctx, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	outResources := make([]*v2.Resource, 0, len(groups))
	for _, group := range groups {
		parentResourceID = getParentGroup(group.ParentID)

		resource, err := groupResource(group, parentResourceID, o.client.IsOnPremise)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		outResources = append(outResources, resource)
	}

	return outResources, nextPageToken, outputAnnotations, nil
}

// parentGroupId will be 0 (zero) if the group isn't a subgroup. parentResourceID will be nil if no parent resource was received in the List function.
// Both cases are checked and handled in this function.
func getParentGroup(parentGroupId int) *v2.ResourceId {
	if parentGroupId == 0 {
		// This path occurs on the first execution of the List func. When all groups are received from the API. Subgroups will be skipped.
		return nil
	}

	parentGroupIdStr := strconv.Itoa(parentGroupId)
	parentGroupResourceId := toGroupResourceId(parentGroupIdStr)
	return &v2.ResourceId{
		ResourceType: groupResourceType.Id,
		Resource:     parentGroupResourceId,
	}
}

func (o *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	rv := make([]*v2.Entitlement, 0, len(groupAccessLevels))

	for _, level := range groupAccessLevels {
		levelName := level.String()
		rv = append(rv, entitlement.NewAssignmentEntitlement(
			resource,
			levelName,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDisplayName(fmt.Sprintf("%s Group %s", resource.DisplayName, levelName)),
			entitlement.WithDescription(fmt.Sprintf("%s on the %s group in Gitlab", levelName, resource.DisplayName)),
		))
	}
	return rv, "", nil, nil
}

func (o *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant
	var outputAnnotations = annotations.New()
	var users []*client.GroupMember

	groupId, err := fromGroupResourceId(resource.Id.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	users, nextPageToken, rateLimitDesc, err := o.client.ListGroupMembers(ctx, groupId, pToken.Token)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}

	if err != nil {
		l := ctxzap.Extract(ctx)
		l.Warn("Permission denied while listing members for group. Skipping.",
			zap.String("group_id", resource.Id.Resource),
		)
		if status.Code(err) == codes.PermissionDenied || errors.Is(err, client.ErrForbidden) {
			return nil, "", nil, nil
		}
		return nil, "", outputAnnotations, err
	}

	for _, user := range users {
		principalId, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("error creating principal ID: %w", err)
		}

		outGrants = append(outGrants, grant.NewGrant(
			resource,
			client.AccessLevelValue(user.AccessLevel).String(),
			principalId,
		))
	}
	return outGrants, nextPageToken, outputAnnotations, nil
}

func (o *groupBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	l := ctxzap.Extract(ctx)
	var outputAnnotations = annotations.New()

	if strings.HasPrefix(principal.Id.Resource, pendingInvitationUser) {
		return nil, fmt.Errorf("entitlement cannot be granted: user %q has not yet accepted the invitation to gitlab", principal.Id.Resource)
	}

	groupIdAndName := entitlement.Resource.Id.Resource
	groupId, err := fromGroupResourceId(groupIdAndName)
	if err != nil {
		return nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	accessLevelValue, err := parseAccessLevelFromEntitlementID(entitlement.Id)
	if err != nil {
		return nil, err
	}

	// GitLab does not allow assigning the 'Minimal Access' role to subgroups.
	// This check ensures that the group is a top-level group by verifying that
	// the 'parent_group_id' trait is not present. If the field exists, it means
	// the group is a subgroup, and the grant should be rejected to prevent API errors.
	//
	// Additionally, assigning 'Minimal Access' requires the user to have a license
	// that allows membership in top-level groups. This role is limited in scope
	// and cannot be applied to subgroups due to GitLab's permission model.
	if client.AccessLevelValue(accessLevelValue) == client.MinimalAccessPermissions {
		if entitlement.Resource.ParentResourceId != nil {
			return outputAnnotations, fmt.Errorf("cannot grant 'Minimal Access': this role is only available for top-level groups, not subgroups")
		}
	}

	userId, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		l.Warn("baton-gitlab grant: unable to parse user ID. falling back to email invite", zap.Error(err))
		ut, err := resourceSdk.GetUserTrait(principal)
		if err != nil {
			return nil, fmt.Errorf("baton-gitlab: error getting user trait: %w", err)
		}
		if len(ut.Emails) == 0 {
			return nil, fmt.Errorf("baton-gitlab: user has no email. cannot invite user to group")
		}
		userEmail := ut.Emails[0].Address
		for _, email := range ut.Emails {
			if email.IsPrimary {
				userEmail = email.Address
				break
			}
		}
		l.Info("baton-gitlab grant: inviting user to group", zap.String("email", userEmail))
		rateLimitDesc, err := o.client.InviteGroupMember(ctx, groupId, userEmail, client.AccessLevelValue(accessLevelValue))
		if rateLimitDesc != nil {
			outputAnnotations.WithRateLimiting(rateLimitDesc)
		}
		if err != nil {
			return outputAnnotations, fmt.Errorf("error inviting user to group: %w", err)
		}
		return outputAnnotations, nil
	}

	memberRequest := &client.AddGroupMemberRequest{
		UserID:      userId,
		AccessLevel: client.AccessLevelValue(accessLevelValue),
	}
	_, rateLimitDesc, err := o.client.AddGroupMember(ctx, groupId, memberRequest)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		var errResp *client.ErrorResponse
		if errors.As(err, &errResp) {
			if errResp.Response != nil && errResp.Response.StatusCode == http.StatusConflict {
				return annotations.New(&v2.GrantAlreadyExists{}), nil
			}
			if strings.Contains(err.Error(), "should be greater than or equal to") {
				return annotations.New(&v2.GrantAlreadyExists{}), nil
			}
		}
		return outputAnnotations, fmt.Errorf("error adding user to group: %w", err)
	}
	return outputAnnotations, nil
}

func (o *groupBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	var outputAnnotations = annotations.New()

	groupIdAndName := grant.Entitlement.Resource.Id.Resource
	groupId, err := fromGroupResourceId(groupIdAndName)
	if err != nil {
		return nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	rateLimitDesc, err := o.client.RemoveGroupMember(ctx, groupId, grant.Principal.Id.Resource)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return annotations.New(&v2.GrantAlreadyRevoked{}), nil
		}
		return outputAnnotations, fmt.Errorf("error removing user from group: %w", err)
	}

	return outputAnnotations, nil
}

func groupResource(group *client.Group, parentResourceID *v2.ResourceId, isOnPremise bool) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":          group.ID,
		"name":        group.Name,
		"full_name":   group.FullName,
		"description": group.Description,
	}
	if group.ParentID != 0 {
		profile["parent_group_id"] = group.ParentID
	}

	annotations := []proto.Message{
		&v2.ChildResourceType{ResourceTypeId: projectResourceType.Id},
	}

	return resourceSdk.NewGroupResource(
		group.FullName,
		groupResourceType,
		toGroupResourceId(strconv.Itoa(group.ID)),
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(profile),
		},
		resourceSdk.WithAnnotation(annotations...),
		resourceSdk.WithParentResourceID(parentResourceID),
	)
}

func newGroupBuilder(client *client.GitlabClient) *groupBuilder {
	return &groupBuilder{
		client: client,
	}
}
