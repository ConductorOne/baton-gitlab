package connector

import (
	"context"
	"fmt"
	"io"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type Connector struct {
	client *client.GitlabClient
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (d *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		newUserBuilder(d.client),
		newGroupBuilder(d.client),
		newProjectBuilder(d.client),
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
	if !d.client.IsOnPremise && d.client.AccountCreationGroup != "" {
		groupName := d.client.AccountCreationGroup
		var matchingGroups []*client.Group
		nextPageToken := ""
		for {
			groups, returnedNextPageToken, _, err := d.client.ListGroups(ctx, nextPageToken)
			if err != nil {
				return nil, fmt.Errorf("error listing groups to validate account creation group: %w", err)
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
			return nil, fmt.Errorf("account creation group '%s' not found", groupName)
		}
		if len(matchingGroups) > 1 {
			return nil, fmt.Errorf("search for account creation group '%s' returned multiple results with that exact name", groupName)
		}
	} else if d.client.IsOnPremise {
		_, _, _, err := d.client.ListUsers(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("error validating token with ListUsers: %w", err)
		}
	}

	return nil, nil
}

// New returns a new instance of the connector.
func New(ctx context.Context, accessToken, baseURL, accountCreationGroup string) (*Connector, error) {
	l := ctxzap.Extract(ctx)

	gitlabClient, err := client.New(ctx, accessToken, baseURL, accountCreationGroup)
	if err != nil {
		l.Error("error creating gitlab client", zap.Error(err))
		return nil, err
	}

	return &Connector{
		client: gitlabClient,
	}, nil
}
