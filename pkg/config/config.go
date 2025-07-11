package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	AccessToken = field.StringField(
		"access-token",
		field.WithDescription("The access token to authenticate with the GitLab API"),
		field.WithRequired(true),
	)
	BaseURL = field.StringField(
		"base-url",
		field.WithDescription("The base URL of the GitLab instance"),
		field.WithDefaultValue("https://gitlab.com/"),
		field.WithRequired(false),
	)
	AccountCreationGroup = field.StringField(
		"account-creation-group",
		field.WithDescription("The group indicated will be used as a default group for the new users. Required for account creation capability in the Cloud Version."),
		field.WithRequired(false),
	)

	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{
		AccessToken,
		BaseURL,
		AccountCreationGroup,
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
