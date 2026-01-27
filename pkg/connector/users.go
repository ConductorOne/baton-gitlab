package connector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/crypto"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
)

const (
	pendingInvitationUser = "pending-invite-"
)

type userBuilder struct {
	client *client.GitlabClient
}

func (u *userBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return userResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (u *userBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var (
		users         []any
		nextPageToken string
		ann           annotations.Annotations
		err           error
	)

	if u.client.IsOnPremise {
		users, nextPageToken, ann, err = u.listOnPremiseVersion(ctx, pToken)
	} else {
		if parentResourceID == nil {
			return nil, "", nil, nil
		}
		users, nextPageToken, ann, err = u.listCloudVersion(ctx, parentResourceID, pToken)
	}
	if err != nil {
		return nil, "", ann, err
	}

	outResources := make([]*v2.Resource, 0, len(users))
	for _, user := range users {
		resource, err := userResource(user)
		if err != nil {
			return nil, "", ann, err
		}
		outResources = append(outResources, resource)
	}

	return outResources, nextPageToken, ann, nil
}

func (u *userBuilder) listOnPremiseVersion(ctx context.Context, pToken *pagination.Token) ([]any, string, annotations.Annotations, error) {
	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	outputAnnotations := annotations.New()
	users, nextPageToken, rateLimitDesc, err := u.client.ListUsers(ctx, pageToken)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, "", outputAnnotations, fmt.Errorf("failed to list users: %w", err)
	}

	resources := make([]any, len(users))
	for i, user := range users {
		resources[i] = user
	}

	return resources, nextPageToken, outputAnnotations, nil
}

func (u *userBuilder) listCloudVersion(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]any, string, annotations.Annotations, error) {
	var users []any
	var err error
	var nextPageToken string
	var outputAnnotations = annotations.New()

	switch parentResourceID.ResourceType {
	case groupResourceType.Id:
		groupId, err := fromGroupResourceId(parentResourceID.Resource)
		if err != nil {
			return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
		}
		var groupMembers []*client.GroupMember
		var rateLimitDescGroupMembers *v2.RateLimitDescription
		groupMembers, nextPageToken, rateLimitDescGroupMembers, err = u.client.ListAllGroupMembers(ctx, groupId, pToken.Token)
		if rateLimitDescGroupMembers != nil {
			outputAnnotations.WithRateLimiting(rateLimitDescGroupMembers)
		}
		if err != nil {
			isPermissionError, unhandledErr := handlePermissionError(ctx, err, "group", groupId)
			if unhandledErr != nil {
				return nil, "", outputAnnotations, unhandledErr
			}
			if isPermissionError {
				return nil, "", outputAnnotations, nil
			}
		}

		for _, member := range groupMembers {
			users = append(users, member)
		}
	case projectResourceType.Id:
		var projectMembers []*client.ProjectMember
		var rateLimitDescProjectMembers *v2.RateLimitDescription
		projectMembers, nextPageToken, rateLimitDescProjectMembers, err = u.client.ListAllProjectMembers(ctx, parentResourceID.Resource, pToken.Token)
		if rateLimitDescProjectMembers != nil {
			outputAnnotations.WithRateLimiting(rateLimitDescProjectMembers)
		}
		if err != nil {
			return nil, "", outputAnnotations, err
		}

		for _, member := range projectMembers {
			users = append(users, member)
		}
	}

	return users, nextPageToken, outputAnnotations, nil
}

// Entitlements always returns an empty slice for users.
func (u *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (u *userBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (u *userBuilder) CreateAccountCapabilityDetails(_ context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_RANDOM_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

func (u *userBuilder) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.CredentialOptions,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	if u.client.IsOnPremise {
		return u.createOnPremUser(ctx, accountInfo, credentialOptions)
	}
	return u.createCloudUser(ctx, accountInfo)
}

func (u *userBuilder) createOnPremUser(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.CredentialOptions,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	createUserOpts, generatedPassword, err := u.getCreateUserOptions(accountInfo, credentialOptions)
	if err != nil {
		return nil, nil, nil, err
	}

	user, _, err := u.client.CreateUser(ctx, createUserOpts)
	if err != nil {
		return nil, nil, nil, err
	}

	userResource, err := userResource(user)
	if err != nil {
		return nil, nil, nil, err
	}

	car := &v2.CreateAccountResponse_SuccessResult{
		Resource: userResource,
	}

	return car, []*v2.PlaintextData{{Bytes: []byte(generatedPassword)}}, nil, nil
}

func (u *userBuilder) createCloudUser(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	var outputAnnotations = annotations.New()
	groupName := u.client.AccountCreationGroup
	profile := accountInfo.GetProfile().AsMap()

	email, ok := profile["email"].(string)
	if !ok || email == "" {
		return nil, nil, nil, fmt.Errorf("missing required field: email")
	}

	group, rateLimitDesc, err := u.client.FindGroupByName(ctx, groupName)
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		return nil, nil, outputAnnotations, fmt.Errorf("failed to get group ID: %w", err)
	}

	members, _, rateLimitDescMembers, err := u.client.ListGroupMembers(ctx, strconv.Itoa(group.ID), "")
	if rateLimitDescMembers != nil {
		outputAnnotations.WithRateLimiting(rateLimitDescMembers)
	}
	if err != nil {
		return nil, nil, outputAnnotations, fmt.Errorf("failed to list group members: %w", err)
	}

	var user *client.GroupMember
	for _, m := range members {
		if m.Email == email {
			user = m
			break
		}
	}

	if user == nil {
		rateLimitDescInvite, err := u.client.InviteGroupMember(ctx, strconv.Itoa(group.ID), email, client.GuestPermissions)
		if rateLimitDescInvite != nil {
			outputAnnotations.WithRateLimiting(rateLimitDescInvite)
		}
		if err != nil {
			return nil, nil, outputAnnotations, fmt.Errorf("failed to invite user to group: %w", err)
		}
	}

	parentResourceID, err := resourceSdk.NewResourceID(groupResourceType, strconv.Itoa(group.ID))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create parent resource ID: %w", err)
	}

	var userRes *v2.Resource
	if user != nil {
		userRes, err = userResource(user)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to build user resource: %w", err)
		}
		userRes.ParentResourceId = parentResourceID
	} else {
		userRes, err = resourceSdk.NewUserResource(
			email,
			userResourceType,
			pendingInvitationUser+strings.ToLower(email),
			[]resourceSdk.UserTraitOption{
				resourceSdk.WithEmail(email, true),
				resourceSdk.WithUserLogin(email),
				resourceSdk.WithStatus(v2.UserTrait_Status_STATUS_DISABLED),
			},
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to build pending user resource: %w", err)
		}
		userRes.ParentResourceId = parentResourceID
	}

	return &v2.CreateAccountResponse_SuccessResult{
		Resource: userRes,
	}, nil, nil, nil
}

func getCredentialOption(credentialOptions *v2.CredentialOptions) (string, bool, error) {
	if credentialOptions.GetNoPassword() != nil {
		return "", false, nil
	}

	if credentialOptions.GetRandomPassword() == nil {
		return "", false, errors.New("unsupported credential options")
	}

	length := min(8, credentialOptions.GetRandomPassword().GetLength())
	plaintextPassword, err := crypto.GenerateRandomPassword(&v2.LocalCredentialOptions_RandomPassword{
		Length: length,
	})
	if err != nil {
		return "", false, err
	}

	return plaintextPassword, true, nil
}

func ToPtr[T any](v T) *T {
	return &v
}

func (u *userBuilder) getCreateUserOptions(accountInfo *v2.AccountInfo, credentialOptions *v2.CredentialOptions) (*client.CreateUserOptions, string, error) {
	pMap := accountInfo.Profile.AsMap()

	email, ok := pMap["email"].(string)
	if !ok || email == "" {
		return nil, "", fmt.Errorf("email is required")
	}

	username, ok := pMap["username"].(string)
	if !ok || username == "" {
		return nil, "", fmt.Errorf("username is required")
	}

	name, ok := pMap["name"].(string)
	if !ok || name == "" {
		return nil, "", fmt.Errorf("name is required")
	}

	password, generatedPassword, err := getCredentialOption(credentialOptions)
	if err != nil {
		return nil, "", err
	}

	createUserReq := &client.CreateUserOptions{
		Email:    &email,
		Username: &username,
		Name:     &name,
	}

	if generatedPassword {
		createUserReq.Password = &password
	} else {
		createUserReq.ForceRandomPassword = ToPtr(true)
	}

	if samlGroupID, ok := pMap["group_id_for_saml"].(string); ok && samlGroupID != "" {
		createUserReq.GroupIDForSAML = &samlGroupID
	}

	return createUserReq, password, nil
}

func (u *userBuilder) Delete(ctx context.Context, id *v2.ResourceId) (annotations.Annotations, error) {
	outputAnnotations := annotations.New()
	groupName := u.client.AccountCreationGroup

	if u.client.IsOnPremise {
		rateLimitDesc, err := u.client.DeleteUser(ctx, id.Resource)
		if rateLimitDesc != nil {
			outputAnnotations.WithRateLimiting(rateLimitDesc)
		}
		if err != nil {
			return outputAnnotations, fmt.Errorf("failed to delete user %s from GitLab: %w", id.Resource, err)
		}
	} else {
		group, rateLimitDesc, err := u.client.FindGroupByName(ctx, groupName)
		if err != nil {
			if rateLimitDesc != nil {
				outputAnnotations.WithRateLimiting(rateLimitDesc)
			}
			return outputAnnotations, fmt.Errorf("failed to get group ID for deprovisioning: %w", err)
		}

		rateLimitDesc, err = u.client.RemoveGroupMember(ctx, strconv.Itoa(group.ID), id.Resource)
		if rateLimitDesc != nil {
			outputAnnotations.WithRateLimiting(rateLimitDesc)
		}
		if err != nil {
			return outputAnnotations, fmt.Errorf("failed to remove user %s from group %s: %w", id.Resource, strconv.Itoa(group.ID), err)
		}
	}

	return outputAnnotations, nil
}

func userResource(user any) (*v2.Resource, error) {
	var id int
	// NOTE: The email attribute is only visible in the DC version (on-premise/self-hosted) to group owners for enterprise users of the group when an API request is sent to the group itself,
	// or that group's subgroups or projects.
	// https://docs.gitlab.com/ee/api/members.html#known-issues
	var email string
	var username string
	var name string
	var state string
	var accessLevel int
	var membershipState string
	var locked *bool
	// NOTE: The last login attribute is only visible in the DC version (on-premise/self-hosted). To get this attribute you need admin permissions and in the cloud version it does not exist.
	// https://docs.gitlab.com/api/users/
	var lastLogin time.Time

	switch user := user.(type) {
	case *client.GroupMember:
		id = user.ID
		email = user.Email
		state = user.State
		name = user.Name
		username = user.Username
		accessLevel = user.AccessLevel
		membershipState = user.MembershipState
		lockedVal := user.Locked
		locked = &lockedVal
	case *client.ProjectMember:
		id = user.ID
		email = user.Email
		state = user.State
		name = user.Name
		username = user.Username
		accessLevel = user.AccessLevel
	case *client.User:
		id = user.ID
		email = user.Email
		state = user.State
		name = user.Name
		username = user.Username
		if user.LastActivityOn != nil && !time.Time(*user.LastActivityOn).IsZero() {
			lastLogin = time.Time(*user.LastActivityOn)
		}
	case *client.PendingInviteUser:
		email := user.InviteEmail
		name := pendingInvitationUser + strings.ToLower(email)

		profile := map[string]interface{}{
			"email": email,
		}

		return resourceSdk.NewUserResource(
			name,
			userResourceType,
			name,
			[]resourceSdk.UserTraitOption{
				resourceSdk.WithEmail(email, true),
				resourceSdk.WithUserLogin(email),
				resourceSdk.WithUserProfile(profile),
				resourceSdk.WithStatus(v2.UserTrait_Status_STATUS_DISABLED),
			},
		)
	default:
		return nil, fmt.Errorf("unknown user type: %T", user)
	}

	userStatus := v2.UserTrait_Status_STATUS_ENABLED
	switch state {
	case "blocked", "deactivated", "ldap_blocked", "banned":
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	case "pending":
		userStatus = v2.UserTrait_Status_STATUS_UNSPECIFIED
		name = pendingInvitationUser + strings.ToLower(email)
	}

	profile := map[string]interface{}{
		"first_name":       name,
		"username":         username,
		"email":            email,
		"state":            state,
		"access_level":     accessLevel,
		"id":               id,
		"membership_state": membershipState,
	}
	if locked != nil {
		profile["locked"] = *locked
	}

	userTraitOptions := []resourceSdk.UserTraitOption{
		resourceSdk.WithEmail(email, true),
		resourceSdk.WithStatus(userStatus),
		resourceSdk.WithUserProfile(profile),
		resourceSdk.WithUserLogin(email),
	}

	if !lastLogin.IsZero() {
		userTraitOptions = append(userTraitOptions, resourceSdk.WithLastLogin(lastLogin))
	}

	return resourceSdk.NewUserResource(
		name,
		userResourceType,
		id,
		userTraitOptions,
	)
}

func newUserBuilder(client *client.GitlabClient) *userBuilder {
	return &userBuilder{
		client: client,
	}
}
