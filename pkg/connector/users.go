package connector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/crypto"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type userBuilder struct {
	*gitlab.Client
}

func (u *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return userResourceType
}

func (u *userBuilder) setEmailsGroupMembers(ctx context.Context, users []*gitlabSDK.GroupMember) []*gitlabSDK.GroupMember {
	for i, user := range users {
		details, _, err := u.Users.GetUser(user.ID, gitlabSDK.GetUsersOptions{}, gitlabSDK.WithContext(ctx))
		if err == nil {
			if details.PublicEmail != "" {
				users[i].Email = details.PublicEmail
			}
			if details.Email != "" {
				users[i].Email = details.Email
			}
		}
	}
	return users
}

func (u *userBuilder) setEmailsProjectMembers(ctx context.Context, users []*gitlabSDK.ProjectMember) []*gitlabSDK.ProjectMember {
	for i, user := range users {
		details, _, err := u.Users.GetUser(user.ID, gitlabSDK.GetUsersOptions{}, gitlabSDK.WithContext(ctx))
		if err == nil {
			if details.PublicEmail != "" {
				users[i].Email = details.PublicEmail
			}
			if details.Email != "" {
				users[i].Email = details.Email
			}
		}
	}
	return users
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (u *userBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var (
		users []any
		res   *gitlabSDK.Response
		err   error
	)

	if u.IsOnPremise {
		users, res, err = u.listOnPremiseVersion(ctx, pToken)
	} else {
		users, res, err = u.listCloudVersion(ctx, parentResourceID, pToken)
	}

	if err != nil {
		return nil, "", nil, err
	}

	outResources := make([]*v2.Resource, 0, len(users))
	for _, user := range users {
		resource, err := userResource(user)
		if err != nil {
			return nil, "", nil, err
		}
		outResources = append(outResources, resource)
	}

	var nextPage string
	if res != nil && res.NextPage != 0 {
		nextPage = strconv.Itoa(res.NextPage)
	}

	return outResources, nextPage, nil, nil
}

func (u *userBuilder) listOnPremiseVersion(ctx context.Context, pToken *pagination.Token) ([]any, *gitlabSDK.Response, error) {
	var pageToken string

	if pToken != nil {
		pageToken = pToken.Token
	}

	users, res, err := u.GetAllUsers(ctx, pageToken)
	if err != nil {
		return nil, nil, err
	}
	resources := make([]any, len(users))
	for i, user := range users {
		resources[i] = &user
	}

	return resources, res, nil
}

func (u *userBuilder) listCloudVersion(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]any, *gitlabSDK.Response, error) {
	var users []any
	var res *gitlabSDK.Response
	var err error

	switch parentResourceID.ResourceType {
	case groupResourceType.Id:
		groupId, err := fromGroupResourceId(parentResourceID.Resource)
		if err != nil {
			return nil, nil, fmt.Errorf("error parsing group resource id: %w", err)
		}
		var groupMembers []*gitlabSDK.GroupMember
		if pToken.Token == "" {
			groupMembers, res, err = u.ListGroupMembers(ctx, groupId)
		} else {
			groupMembers, res, err = u.ListGroupMembersPaginate(ctx, groupId, pToken.Token)
		}
		if err != nil {
			return nil, nil, err
		}
		groupMembers = u.setEmailsGroupMembers(ctx, groupMembers)
		for _, member := range groupMembers {
			users = append(users, member)
		}

	case projectResourceType.Id:
		var projectMembers []*gitlabSDK.ProjectMember
		if pToken.Token == "" {
			projectMembers, res, err = u.ListProjectMembers(ctx, parentResourceID.Resource)
		} else {
			projectMembers, res, err = u.ListProjectMembersPaginate(ctx, parentResourceID.Resource, pToken.Token)
		}
		if err != nil {
			return nil, nil, err
		}
		projectMembers = u.setEmailsProjectMembers(ctx, projectMembers)
		for _, member := range projectMembers {
			users = append(users, member)
		}

	default:
		return nil, nil, fmt.Errorf("unsupported parent resource type: %s", parentResourceID.ResourceType)
	}

	return users, res, nil
}

// Entitlements always returns an empty slice for users.
func (u *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (u *userBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (u *userBuilder) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
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
	if u.IsOnPremise {
		return u.createOnPremUser(accountInfo, credentialOptions)
	}
	return u.createCloudUser(ctx, accountInfo)
}

func (u *userBuilder) createOnPremUser(
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

	user, _, err := u.Users.CreateUser(createUserOpts)
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

// ************************* THIS FUNCTION IS NOT FINISHED YET, IT IS FOR CREATE NEW USER IN THE CLOUD VERSION *********************************.
func (u *userBuilder) createCloudUser(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	profile := accountInfo.GetProfile().AsMap()

	email, ok := profile["email"].(string)
	if !ok || email == "" {
		return nil, nil, nil, fmt.Errorf("missing required field: email")
	}

	groupID, err := u.getGroupID(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get group ID: %w", err)
	}

	err = u.InviteGroupMember(ctx, groupID, email, gitlabSDK.GuestPermissions)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to invite user to group: %w", err)
	}

	members, _, err := u.ListGroupMembers(ctx, groupID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list group members after invite: %w", err)
	}

	var user *gitlabSDK.GroupMember
	for _, m := range members {
		if m.Email == email {
			user = m
			break
		}
	}

	parentResourceID, err := resourceSdk.NewResourceID(groupResourceType, groupID)
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
			email,
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
	plaintextPassword, err := crypto.GenerateRandomPassword(&v2.CredentialOptions_RandomPassword{
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

func (u *userBuilder) getCreateUserOptions(accountInfo *v2.AccountInfo, credentialOptions *v2.CredentialOptions) (*gitlabSDK.CreateUserOptions, string, error) {
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

	createUserOpts := &gitlabSDK.CreateUserOptions{
		Email:    &email,
		Username: &username,
		Name:     &name,
	}

	if generatedPassword {
		createUserOpts.Password = &password
	} else {
		createUserOpts.ForceRandomPassword = ToPtr(true)
	}

	if samlGroupID, ok := pMap["group_id_for_saml"].(string); ok && samlGroupID != "" {
		createUserOpts.GroupIDForSAML = &samlGroupID
	}

	return createUserOpts, password, nil
}

func newUserBuilder(client *gitlab.Client) *userBuilder {
	return &userBuilder{
		Client: client,
	}
}

func (u *userBuilder) getGroupID(ctx context.Context) (string, error) {
	if u.AccountCreationGroup == "" {
		return "", fmt.Errorf("account creation group not set. use --account-creation-group when running the connector")
	}

	groupName := u.AccountCreationGroup
	groups, _, err := u.Groups.ListGroups(&gitlabSDK.ListGroupsOptions{
		Search: &groupName,
	},
		gitlabSDK.WithContext(ctx),
	)
	if err != nil {
		return "", fmt.Errorf("error listing groups: %w", err)
	}

	for _, group := range groups {
		if group.Name == groupName {
			return strconv.Itoa(group.ID), nil
		}
	}

	return "", fmt.Errorf("account creation group %s not found", groupName)
}

func userResource(user any) (*v2.Resource, error) {
	var id int
	// NOTE: The email attribute is only visible to group owners for enterprise users of the group when an API request is sent to the group itself, or that group's subgroups or projects.
	// https://docs.gitlab.com/ee/api/members.html#known-issues
	var email string
	var username string
	var name string
	var state string
	var accessLevel int
	var lastLogin time.Time

	switch user := user.(type) {
	case *gitlabSDK.GroupMember:
		id = user.ID
		email = user.Email
		state = user.State
		name = user.Name
		username = user.Username
		accessLevel = int(user.AccessLevel)
	case *gitlabSDK.ProjectMember:
		id = user.ID
		email = user.Email
		state = user.State
		name = user.Name
		username = user.Username
		accessLevel = int(user.AccessLevel)
	case *gitlabSDK.User:
		id = user.ID
		email = user.Email
		state = user.State
		name = user.Name
		username = user.Username
		if user.LastActivityOn != nil && !time.Time(*user.LastActivityOn).IsZero() {
			lastLogin = time.Time(*user.LastActivityOn)
		}
	default:
		return nil, fmt.Errorf("unknown user type: %T", user)
	}

	profile := map[string]interface{}{
		"first_name":   name,
		"username":     username,
		"email":        email,
		"state":        state,
		"access_level": accessLevel,
		"id":           id,
	}

	userTraitOptions := []resourceSdk.UserTraitOption{
		resourceSdk.WithEmail(email, true),
		resourceSdk.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
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
