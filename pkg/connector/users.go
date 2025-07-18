package connector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

const pendingInvitationUser = "pending-invite-"

type userBuilder struct {
	*gitlab.Client
}

func (u *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return userResourceType
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

	users, res, err = u.getUsers(ctx, parentResourceID, pToken)
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

func (u *userBuilder) getUsers(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]any, *gitlabSDK.Response, error) {
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

		for _, member := range groupMembers {
			users = append(users, member)
		}

		pending, _, err := u.ListExternalGroupMembers(ctx, groupId)
		if err != nil {
			return nil, nil, fmt.Errorf("error listing external group members: %w", err)
		}

		for _, invite := range pending {
			users = append(users, invite)
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
	return u.createCloudUser(ctx, accountInfo)
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
	profile := accountInfo.GetProfile().AsMap()

	email, ok := profile["email"].(string)
	if !ok || email == "" {
		return nil, nil, nil, fmt.Errorf("missing required field: email")
	}

	groupID, err := u.getGroupID(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get group ID: %w", err)
	}

	members, _, err := u.ListGroupMembers(ctx, groupID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list group members: %w", err)
	}

	var user *gitlabSDK.GroupMember
	for _, m := range members {
		if m.Email == email {
			user = m
			break
		}
	}

	if user == nil {
		err = u.InviteGroupMember(ctx, groupID, email, gitlabSDK.GuestPermissions)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to invite user to group: %w", err)
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

func newUserBuilder(client *gitlab.Client) *userBuilder {
	return &userBuilder{
		Client: client,
	}
}

func (u *userBuilder) getGroupID(ctx context.Context) (string, error) {
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
	// NOTE: The email attribute is only visible in the DC version (on-premise/self-hosted) to group owners for enterprise users of the group when an API request is sent to the group itself,
	// or that group's subgroups or projects.
	// https://docs.gitlab.com/ee/api/members.html#known-issues
	var email string
	var username string
	var name string
	var state string
	var accessLevel int
	// NOTE: The last login attribute is only visible in the DC version (on-premise/self-hosted). To get this attribute you need admin permissions and in the cloud version it does not exist.
	// https://docs.gitlab.com/api/users/
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
	case *gitlabSDK.PendingInvite:
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
