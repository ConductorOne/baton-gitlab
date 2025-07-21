package connector

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	"github.com/conductorone/baton-gitlab/pkg/onprem"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type userOnPremBuilder struct {
	*gitlab.Client
	onpremClient *onprem.Client
}

func (u *userOnPremBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return userResourceType
}

func userOnPremResource(user onprem.User) (*v2.Resource, error) {
	var id int
	// NOTE: The email attribute is only visible in the DC version (on-premise/self-hosted) to group owners for enterprise users of the group when an API request is sent to the group itself,
	// or that group's subgroups or projects.
	// https://docs.gitlab.com/ee/api/members.html#known-issues
	var email string
	var username string
	var name string
	var state string
	// NOTE: The last login attribute is only visible in the DC version (on-premise/self-hosted). To get this attribute you need admin permissions and in the cloud version it does not exist.
	// https://docs.gitlab.com/api/users/
	var lastLogin time.Time

	id = user.ID
	email = user.Email
	state = user.State
	name = user.Name
	username = user.Username
	if user.LastActivityOn != nil && !time.Time(*user.LastActivityOn).IsZero() {
		lastLogin = time.Time(*user.LastActivityOn)
	}

	userStatus := v2.UserTrait_Status_STATUS_ENABLED
	switch state {
	case "blocked", "deactivated", "ldap_blocked", "banned":
		userStatus = v2.UserTrait_Status_STATUS_DISABLED
	case "pending":
		userStatus = v2.UserTrait_Status_STATUS_UNSPECIFIED
	}

	profile := map[string]interface{}{
		"first_name": name,
		"username":   username,
		"email":      email,
		"state":      state,
		"id":         id,
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

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (u *userOnPremBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var (
		users []onprem.User
		res   *http.Response
		err   error
	)

	users, res, err = u.onpremClient.GetUsers(ctx, pToken)
	if err != nil {
		return nil, "", nil, err
	}

	outResources := make([]*v2.Resource, 0, len(users))
	for _, user := range users {
		resource, err := userOnPremResource(user)
		if err != nil {
			return nil, "", nil, err
		}
		outResources = append(outResources, resource)
	}

	var nextToken string
	if onprem.HasNextToken(res) {
		nextToken = onprem.NextToken(ctx, res)
	}

	return outResources, nextToken, nil, nil
}

// Entitlements always returns an empty slice for users.
func (u *userOnPremBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (u *userOnPremBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (u *userOnPremBuilder) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_RANDOM_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

func (u *userOnPremBuilder) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	credentialOptions *v2.CredentialOptions,
) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	return u.createOnPremUser(accountInfo, credentialOptions)
}

func (u *userOnPremBuilder) createOnPremUser(
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

func (u *userOnPremBuilder) getCreateUserOptions(accountInfo *v2.AccountInfo, credentialOptions *v2.CredentialOptions) (*gitlabSDK.CreateUserOptions, string, error) {
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

func newUserOnPremBuilder(client *gitlab.Client, httpClient *onprem.Client) *userOnPremBuilder {
	return &userOnPremBuilder{
		Client:       client,
		onpremClient: httpClient,
	}
}
