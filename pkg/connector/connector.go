package connector

import (
	"context"
	"fmt"
	"io"

	"github.com/conductorone/baton-gitlab/pkg/connector/gitlab"
	"github.com/conductorone/baton-gitlab/pkg/onprem"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type Connector struct {
	SdkClient    *gitlab.Client
	onpremClient *onprem.Client
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (d *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	// the user resource syncers are different for on-premise and cloud GitLab due to
	// pagination being different: https://conductorone.atlassian.net/browse/BB-1128
	var userResource connectorbuilder.ResourceSyncer
	if d.SdkClient.IsOnPremise {
		userResource = newUserOnPremBuilder(d.SdkClient, d.onpremClient)
	} else {
		userResource = newUserBuilder(d.SdkClient)
	}

	return []connectorbuilder.ResourceSyncer{
		userResource,
		newGroupBuilder(d.SdkClient),
		newProjectBuilder(d.SdkClient),
	}
}

// Asset takes an input AssetRef and attempts to fetch it using the connector's authenticated http client
// It streams a response, always starting with a metadata object, following by chunked payloads for the asset.
func (d *Connector) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

// Metadata returns metadata about the connector.
func (d *Connector) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "GitLab",
		Description: "GitLab is a web-based Git repository manager with built-in CI/CD pipeline functionality.",
		AccountCreationSchema: &v2.ConnectorAccountCreationSchema{
			FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
				"name": {
					DisplayName: "Name",
					Required:    true,
					Description: "This name will be used for the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Name",
					Order:       1,
				},
				"email": {
					DisplayName: "Email",
					Required:    true,
					Description: "This email will be used as the login for the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Email",
					Order:       2,
				},
				"username": {
					DisplayName: "Username",
					Required:    true,
					Description: "This username will be used for the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Username",
					Order:       3,
				},
				"group_id_for_saml": {
					DisplayName: "Group ID for SAML",
					Required:    false,
					Description: "If this is set, the user will use the specified group for SAML authentication.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Group ID for SAML",
					Order:       4,
				},
			},
		},
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials
// to be sure that they are valid.
func (d *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	if !d.SdkClient.IsOnPremise && d.SdkClient.AccountCreationGroup != "" {
		groupName := d.SdkClient.AccountCreationGroup
		groups, _, err := d.SdkClient.Groups.ListGroups(&gitlabSDK.ListGroupsOptions{
			Search: &groupName,
		},
			gitlabSDK.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("error getting account creation group: %w", err)
		}

		if len(groups) == 0 {
			return nil, fmt.Errorf("account creation group not found")
		}
	} else {
		_, _, err := d.SdkClient.ListGroups(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("error listing groups: %w", err)
		}
	}

	return nil, nil
}

// New returns a new instance of the connector.
func New(ctx context.Context, accessToken, baseURL, accountCreationGroup string) (*Connector, error) {
	gitlabClient, err := gitlab.NewClient(ctx, accessToken, baseURL, accountCreationGroup)
	if err != nil {
		return nil, fmt.Errorf("error creating gitlab client: %w", err)
	}

	httpClient, err := onprem.New(ctx, accessToken, baseURL)
	if err != nil {
		return nil, fmt.Errorf("error creating http client: %w", err)
	}

	return &Connector{
		SdkClient:    gitlabClient,
		onpremClient: httpClient,
	}, nil
}
