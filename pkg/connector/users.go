package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type userBuilder struct {
	*gitlab.Client
}

func (o *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return userResourceType
}

func userResource(user any, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	var id int
	var email string
	var name string
	var accessLevel int

	switch user := user.(type) {
	case *gitlabSDK.GroupMember:
		id = user.ID
		email = user.Email
		name = user.Name
		accessLevel = int(user.AccessLevel)
	case *gitlabSDK.ProjectMember:
		id = user.ID
		email = user.Email
		name = user.Name
		accessLevel = int(user.AccessLevel)
	default:
		return nil, fmt.Errorf("unknown user type: %T", user)
	}

	profile := map[string]interface{}{
		"id":           id,
		"email":        email,
		"first_name":   name,
		"access_level": accessLevel,
	}

	userTraitOptions := []resourceSdk.UserTraitOption{
		resourceSdk.WithEmail(email, true),
		resourceSdk.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
		resourceSdk.WithUserProfile(profile),
		resourceSdk.WithUserLogin(email),
	}

	return resourceSdk.NewUserResource(
		name,
		userResourceType,
		id,
		userTraitOptions,
		resourceSdk.WithParentResourceID(parentResourceID),
	)
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *userBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var users []any
	var res *gitlabSDK.Response
	var err error

	var groupMembers []*gitlabSDK.GroupMember

	if parentResourceID.ResourceType == groupResourceType.Id {
		if pToken.Token == "" {
			groupMembers, res, err = o.ListGroupMembers(ctx, parentResourceID.Resource)
		} else {
			groupMembers, res, err = o.ListGroupMembersPaginate(ctx, parentResourceID.Resource, pToken.Token)
		}
	}
	if err != nil {
		return nil, "", nil, err
	}

	for _, member := range groupMembers {
		users = append(users, member)
	}

	var projectMembers []*gitlabSDK.ProjectMember
	if parentResourceID.ResourceType == projectResourceType.Id {
		if pToken.Token == "" {
			projectMembers, res, err = o.ListProjectMembers(ctx, parentResourceID.Resource)
		} else {
			projectMembers, res, err = o.ListProjectMembersPaginate(ctx, parentResourceID.Resource, pToken.Token)
		}
	}
	if err != nil {
		return nil, "", nil, err
	}

	for _, member := range projectMembers {
		users = append(users, member)
	}

	outResources := make([]*v2.Resource, 0, len(users))
	for _, user := range users {
		resource, err := userResource(user, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		outResources = append(outResources, resource)
	}

	var nextPage string
	if res.NextPage != 0 {
		nextPage = strconv.Itoa(res.NextPage)
	}

	return outResources, nextPage, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *userBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *userBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newUserBuilder(client *gitlab.Client) *userBuilder {
	return &userBuilder{
		Client: client,
	}
}
