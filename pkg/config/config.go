package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	AccessToken = field.StringField(
		"access-token",
		field.WithDisplayName("Personal access token"),
		field.WithDescription("The access token to authenticate with the GitLab API"),
		field.WithRequired(true),
		field.WithIsSecret(true),
	)
	BaseURL = field.StringField(
		"base-url",
		field.WithDisplayName("URL"),
		field.WithDescription("The base URL of the GitLab instance"),
		field.WithDefaultValue("https://gitlab.com/"),
		field.WithRequired(false),
	)
	AccountCreationGroup = field.StringField(
		"account-creation-group",
		field.WithDisplayName("Group"),
		field.WithDescription("The group indicated will be used as a default group for the new users. Required for account creation capability in the Cloud Version."),
		field.WithRequired(false),
	)
	SyncDirectMembersOnly = field.BoolField(
		"sync-direct-members-only",
		field.WithDisplayName("Sync direct members only"),
		field.WithDescription("When enabled, only direct members of groups and projects are synced. Access inherited from parent groups or granted via invited (shared) groups is excluded."),
	)
	SyncAccessPaths = field.BoolField(
		"sync-access-paths",
		field.WithDisplayName("Sync access paths"),
		field.WithDescription("Label grants by access path (direct, inherited, or invited group). Disabled (default) keeps the previous flattened effective-membership grants."),
	)

	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{
		AccessToken,
		BaseURL,
		AccountCreationGroup,
		SyncDirectMembersOnly,
		SyncAccessPaths,
	}

	// FieldRelationships defines relationships between the fields listed in
	// ConfigurationFields that can be automatically validated. For example, a
	// username and password can be required together, or an access token can be
	// marked as mutually exclusive from the username password pair.
	FieldRelationships = []field.SchemaFieldRelationship{}
)

//go:generate go run ./gen
var Config = field.NewConfiguration(
	ConfigurationFields,
	field.WithConstraints(FieldRelationships...),
	field.WithConnectorDisplayName("GitLab"),
	field.WithHelpUrl("/docs/baton/gitlab-v2"),
	field.WithIconUrl("/static/app-icons/gitlab.svg"),
)
