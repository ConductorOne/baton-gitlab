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

const (
	profileFieldName  = "name"
	profileFieldEmail = "email"
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
				profileFieldName: {
					DisplayName: "Name",
					Required:    true,
					Description: "This name will be used for the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Name",
					Order:       1,
				},
				profileFieldEmail: {
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

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials.
func (d *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	var outputAnnotations = annotations.New()
	if !d.client.IsOnPremise {
		// If no account creation group is set, perform a general token validation.

		// Logic for GitLab Cloud.
		if d.client.AccountCreationGroup != "" {
			// If an account creation group is set, validate it exists and is unique.
			groupName := d.client.AccountCreationGroup
			_, rateLimitDesc, err := d.client.FindGroupByName(ctx, groupName)
			if rateLimitDesc != nil {
				outputAnnotations.WithRateLimiting(rateLimitDesc)
			}
			if err != nil {
				return outputAnnotations, fmt.Errorf("failed to validate account creation group: %w", err)
			}
		} else {
			// If no account creation group is set, perform a general token validation.
			_, rateLimitDesc, err := d.client.GetCurrentlyAuthenticatedUser(ctx)
			if rateLimitDesc != nil {
				outputAnnotations.WithRateLimiting(rateLimitDesc)
			}
			if err != nil {
				return outputAnnotations, fmt.Errorf("error validating token with GetCurrentlyAuthenticatedUser: %w", err)
			}
		}
	} else {
		// For On-Premise, validate the token can list users (requires admin permissions).
		_, _, rateLimitDesc, err := d.client.ListUsers(ctx, "")
		if rateLimitDesc != nil {
			outputAnnotations.WithRateLimiting(rateLimitDesc)
		}
		if err != nil {
			return outputAnnotations, fmt.Errorf("error validating token with ListUsers: %w", err)
		}
	}

	return outputAnnotations, nil
}

// New returns a new instance of the connector.
func New(ctx context.Context, accessToken string, baseURL string, accountCreationGroup string, syncDirectMembersOnly bool) (*Connector, error) {
	l := ctxzap.Extract(ctx)

	gitlabClient, err := client.New(ctx, accessToken, baseURL, accountCreationGroup, syncDirectMembersOnly)
	if err != nil {
		l.Error("error creating gitlab client", zap.Error(err))
		return nil, err
	}

	return &Connector{
		client: gitlabClient,
	}, nil
}
