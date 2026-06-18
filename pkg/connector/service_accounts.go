package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
)

type serviceAccountBuilder struct {
	client *client.GitlabClient
}

func (o *serviceAccountBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return serviceAccountResourceType
}

// List returns GitLab service accounts as K2 SERVICE users. On a self-managed
// instance it enumerates the instance-wide endpoint; on GitLab.com it fans out
// per top-level group (the only place service accounts are listable there).
func (o *serviceAccountBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var pageToken string
	if pToken != nil {
		pageToken = pToken.Token
	}

	outputAnnotations := annotations.New()

	var (
		accounts      []*client.ServiceAccount
		nextPageToken string
		rateLimitDesc *v2.RateLimitDescription
		err           error
	)

	if o.client.IsOnPremise {
		accounts, nextPageToken, rateLimitDesc, err = o.client.ListInstanceServiceAccounts(ctx, pageToken)
	} else {
		// Cloud: service accounts are owned by top-level groups, so this builder
		// is fanned out per group via the group's ChildResourceType annotation.
		if parentResourceID == nil {
			return nil, "", nil, nil
		}
		var groupID string
		groupID, err = fromGroupResourceId(parentResourceID.Resource)
		if err != nil {
			return nil, "", nil, fmt.Errorf("error parsing group resource id: %w", err)
		}
		accounts, nextPageToken, rateLimitDesc, err = o.client.ListGroupServiceAccounts(ctx, groupID, pageToken)
	}
	if rateLimitDesc != nil {
		outputAnnotations.WithRateLimiting(rateLimitDesc)
	}
	if err != nil {
		// The instance and group service-account endpoints require elevated
		// permissions (admin / group owner); a token without them gets 403.
		// Skip rather than failing the whole sync.
		isPermissionError, unhandledErr := handlePermissionError(ctx, err, "service_account", "")
		if unhandledErr != nil {
			return nil, "", outputAnnotations, fmt.Errorf("failed to list service accounts: %w", unhandledErr)
		}
		if isPermissionError {
			return nil, "", outputAnnotations, nil
		}
	}

	resources := make([]*v2.Resource, 0, len(accounts))
	for _, account := range accounts {
		resource, err := serviceAccountResource(account, parentResourceID)
		if err != nil {
			return nil, "", outputAnnotations, err
		}
		resources = append(resources, resource)
	}

	return resources, nextPageToken, outputAnnotations, nil
}

func (o *serviceAccountBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *serviceAccountBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func serviceAccountResource(account *client.ServiceAccount, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":          account.ID,
		fieldUsername: account.Username,
		fieldName:     account.Name,
	}
	if account.Email != "" {
		profile[fieldEmail] = account.Email
	}

	traitOpts := []resourceSdk.UserTraitOption{
		resourceSdk.WithAccountType(v2.UserTrait_ACCOUNT_TYPE_SERVICE),
		resourceSdk.WithUserProfile(profile),
		resourceSdk.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
	}
	if account.Email != "" {
		traitOpts = append(traitOpts,
			resourceSdk.WithEmail(account.Email, true),
			resourceSdk.WithUserLogin(account.Username),
		)
	} else {
		traitOpts = append(traitOpts, resourceSdk.WithUserLogin(account.Username))
	}

	var opts []resourceSdk.ResourceOption
	if parentResourceID != nil {
		opts = append(opts, resourceSdk.WithParentResourceID(parentResourceID))
	}

	displayName := account.Name
	if displayName == "" {
		displayName = account.Username
	}

	return resourceSdk.NewUserResource(displayName, serviceAccountResourceType, account.ID, traitOpts, opts...)
}

func newServiceAccountBuilder(client *client.GitlabClient) *serviceAccountBuilder {
	return &serviceAccountBuilder{
		client: client,
	}
}
