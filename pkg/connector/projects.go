package connector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type projectBuilder struct {
	client *client.GitlabClient
}

func (o *projectBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return projectResourceType
}

var projectAccessLevels = []client.AccessLevelValue{
	client.MinimalAccessPermissions,
	client.GuestPermissions,
	client.ReporterPermissions,
	client.DeveloperPermissions,
	client.MaintainerPermissions,
	client.OwnerPermissions,
}

func (o *projectBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var (
		projects          []*client.Project
		pageToken         string
		err               error
		outputAnnotations = annotations.New()
	)

	groupId, err := fromGroupResourceId(parentResourceID.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
	}

	if pToken != nil {
		pageToken = pToken.Token
	}
	projects, nextPageToken, rateLimitDesc, err := o.client.ListProjects(ctx, groupId, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	outResources := make([]*v2.Resource, 0, len(projects))
	for _, project := range projects {
		parentGroup := getParentGroupFromNamespace(project.Namespace)
		if parentGroup == nil {
			parentGroup = parentResourceID
		}

		resource, err := projectResource(project, parentGroup, o.client.IsOnPremise)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		outResources = append(outResources, resource)
	}

	return outResources, nextPageToken, outputAnnotations, nil
}

func (o *projectBuilder) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	rv := make([]*v2.Entitlement, 0, len(projectAccessLevels))

	for _, level := range projectAccessLevels {
		levelName := level.String()
		rv = append(rv, entitlement.NewAssignmentEntitlement(
			resource,
			levelName,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDisplayName(fmt.Sprintf("%s Project %s", resource.DisplayName, levelName)),
			entitlement.WithDescription(fmt.Sprintf("%s on the %s project in Gitlab", levelName, resource.DisplayName)),
		))
	}
	return rv, "", nil, nil
}

// Grants dispatches on the SyncAccessPaths flag. With it disabled (default) the
// connector emits flattened effective membership as before; with it enabled each
// access path (direct, inherited from a parent group, or via an invited group) is
// surfaced as a distinct expandable grant.
func (o *projectBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	if o.client.SyncAccessPaths {
		return o.grantsWithAccessPaths(ctx, resource, pToken)
	}
	return o.grantsFlattened(ctx, resource, pToken)
}

// grantsFlattened emits effective membership as flat direct grants plus a
// parent-group expandable grant per member — the pre-access-path behavior, preserved
// unchanged for existing customers. When SyncDirectMembersOnly is set, only direct
// members are listed; otherwise the full effective set is flattened via
// ListAllProjectMembers.
func (o *projectBuilder) grantsFlattened(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant
	var outputAnnotations = annotations.New()

	var users []*client.ProjectMember
	var err error
	var nextPageToken string
	var rateLimitDesc *v2.RateLimitDescription
	if o.client.SyncDirectMembersOnly {
		users, nextPageToken, rateLimitDesc, err = o.client.ListProjectMembers(ctx, resource.Id.Resource, pToken.Token)
	} else {
		users, nextPageToken, rateLimitDesc, err = o.client.ListAllProjectMembers(ctx, resource.Id.Resource, pToken.Token)
	}
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, err
	}

	groupId := resource.ParentResourceId
	if groupId == nil {
		return nil, "", outputAnnotations, fmt.Errorf("project resource has no parent group")
	}

	for _, user := range users {
		entitlementId := fmt.Sprintf("group:%s:%s", groupId.Resource, client.AccessLevelValue(user.AccessLevel).String())
		principalId, err := resourceSdk.NewResourceID(userResourceType, user.ID)
		if err != nil {
			return nil, "", outputAnnotations, fmt.Errorf("error creating principal ID: %w", err)
		}

		grantOptions := []grant.GrantOption{
			grant.WithAnnotation(&v2.GrantExpandable{
				EntitlementIds: []string{entitlementId},
				Shallow:        false,
			}),
		}

		outGrants = append(outGrants, grant.NewGrant(
			resource,
			client.AccessLevelValue(user.AccessLevel).String(),
			principalId,
		))

		outGrants = append(outGrants, grant.NewGrant(
			resource,
			client.AccessLevelValue(user.AccessLevel).String(),
			groupId,
			grantOptions...,
		))
	}

	return outGrants, nextPageToken, outputAnnotations, nil
}

// grantsWithAccessPaths emits a project's direct members plus its indirect access
// paths — inheritance from ancestor groups (up to and including the top-level group)
// and access via invited (shared) groups — as expandable grants. It reuses the
// existing per-access-level entitlements (no synthetic membership entitlement) and
// emits paths only for levels that actually have members (no empty expansions).
func (o *projectBuilder) grantsWithAccessPaths(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var outGrants []*v2.Grant
	var outputAnnotations = annotations.New()

	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	users, nextPageToken, rateLimitDesc, err := o.client.ListProjectMembers(ctx, resource.Id.Resource, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		_, unhandledErr := handlePermissionError(ctx, err, "project", resource.Id.Resource)
		if unhandledErr != nil {
			return nil, "", outputAnnotations, unhandledErr
		}
		// Permission error listing members: skip direct members but still emit the
		// pure-expansion indirect anchors below, which need no project member data.
		users, nextPageToken = nil, ""
	}

	// Path 1: direct members.
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

	// Indirect access paths, emitted once on the first page (unless direct-only).
	if pageToken == "" && !o.client.SyncDirectMembersOnly {
		// Path 2: inheritance from ancestor groups (immediate subgroup up to top-level).
		if resource.ParentResourceId != nil {
			inherited, err := accessPathInheritanceGrants(ctx, o.client, resource, resource.ParentResourceId, &outputAnnotations)
			if err != nil {
				return nil, "", outputAnnotations, err
			}
			outGrants = append(outGrants, inherited...)
		}

		// Path 3: groups invited into this project.
		project, rlDesc, err := o.client.GetProject(ctx, resource.Id.Resource)
		if rlDesc != nil {
			outputAnnotations.WithRateLimiting(rlDesc)
		}
		if err != nil {
			_, unhandledErr := handlePermissionError(ctx, err, "project", resource.Id.Resource)
			if unhandledErr != nil {
				return nil, "", outputAnnotations, unhandledErr
			}
			// Permission error: skip invited-group grants and continue.
		} else {
			invited, err := accessPathInvitedGrants(ctx, o.client, resource, project.SharedWithGroups, &outputAnnotations)
			if err != nil {
				return nil, "", outputAnnotations, err
			}
			outGrants = append(outGrants, invited...)
		}
	}

	return outGrants, nextPageToken, outputAnnotations, nil
}

func (o *projectBuilder) Grant(
	ctx context.Context,
	principal *v2.Resource,
	entitlement *v2.Entitlement,
) (
	annotations.Annotations,
	error,
) {
	var outputAnnotations = annotations.New()
	if strings.HasPrefix(principal.Id.Resource, pendingInvitationUser) {
		return nil, fmt.Errorf("entitlement cannot be granted: user %q has not yet accepted the invitation to gitlab", principal.Id.Resource)
	}

	projectId := entitlement.Resource.Id.Resource
	accessLevelValue, err := parseAccessLevelFromEntitlementID(entitlement.Id)
	if err != nil {
		return nil, err
	}
	userId, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "baton-gitlab: invalid user ID: %v", err)
	}

	memberRequest := &client.AddProjectMemberRequest{
		UserID:      userId,
		AccessLevel: client.AccessLevelValue(accessLevelValue),
	}
	_, rateLimitDesc, err := o.client.AddProjectMember(ctx, projectId, memberRequest)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		// Idempotency: GitLab returns 409 (uhttp → codes.AlreadyExists) when the user is
		// already a member, and a 400 "should be greater than or equal to" when they already
		// hold an equal/higher role. Both mean the desired grant already holds.
		if isAlreadyExistsError(err) || strings.Contains(err.Error(), "should be greater than or equal to") {
			return annotations.New(&v2.GrantAlreadyExists{}), nil
		}
		return outputAnnotations, fmt.Errorf("error adding user to project: %w", err)
	}

	return outputAnnotations, nil
}

func (o *projectBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	var outputAnnotations = annotations.New()

	projectId := grant.Entitlement.Resource.Id.Resource
	userId, err := strconv.Atoi(grant.Principal.Id.Resource)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "baton-gitlab: invalid user ID: %v", err)
	}

	rateLimitDesc, err := o.client.RemoveProjectMember(ctx, projectId, strconv.Itoa(userId))
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		if isNotFoundError(err) {
			// With access-path labeling off (default), preserve the historical behavior:
			// an absent direct membership is reported as already revoked. The effective-
			// access check below only applies in access-path mode, so default-mode
			// deployments see no provisioning behavior change.
			if !o.client.SyncAccessPaths {
				return annotations.New(&v2.GrantAlreadyRevoked{}), nil
			}
			// Not a direct member. If the principal still has effective access, it is
			// inherited/invited — reject instead of falsely reporting success (which
			// would reappear next sync). Otherwise the membership is genuinely gone.
			member, rlDesc, checkErr := o.client.GetProjectMemberAll(ctx, projectId, strconv.Itoa(userId))
			if rlDesc != nil {
				outputAnnotations.WithRateLimiting(rlDesc)
			}
			switch {
			case checkErr == nil && member != nil:
				return outputAnnotations, revokeInheritedError("project", projectId)
			case checkErr != nil && !isNotFoundError(checkErr):
				return outputAnnotations, fmt.Errorf("error verifying project membership: %w", checkErr)
			default:
				outputAnnotations.Update(&v2.GrantAlreadyRevoked{})
				return outputAnnotations, nil
			}
		}
		return outputAnnotations, fmt.Errorf("error removing user from project: %w", err)
	}

	return outputAnnotations, nil
}

func projectResource(project *client.Project, parentResourceID *v2.ResourceId, isOnPremise bool) (*v2.Resource, error) {
	var annotations []proto.Message

	return resourceSdk.NewGroupResource(
		project.NameWithNamespace,
		projectResourceType,
		project.ID,
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(
				map[string]interface{}{
					"id":             project.ID,
					profileFieldName: project.Name,
					"description":    project.Description,
				},
			),
		},
		resourceSdk.WithAnnotation(annotations...),
		resourceSdk.WithParentResourceID(parentResourceID),
	)
}

func newProjectBuilder(client *client.GitlabClient) *projectBuilder {
	return &projectBuilder{
		client: client,
	}
}
