package client

import (
	"time"
)

type User struct {
	ID             int      `json:"id"`
	Email          string   `json:"email"`
	Username       string   `json:"username"`
	Name           string   `json:"name"`
	State          string   `json:"state"`
	LastActivityOn *ISOTime `json:"last_activity_on"`
}

type PendingInviteUser struct {
	InviteEmail string `json:"invite_email"`
}

type Group struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	FullName    string `json:"full_name"`
	ParentID    int    `json:"parent_id"`
}

type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	NameWithNamespace string `json:"name_with_namespace"`
}

type GroupMember struct {
	ID                int           `json:"id"`
	Email             string        `json:"email"`
	Username          string        `json:"username"`
	Name              string        `json:"name"`
	State             string        `json:"state"`
	AccessLevel       int           `json:"access_level"`
	GroupSAMLIdentity *SAMLIdentity `json:"group_saml_identity"`
}

type ProjectMember struct {
	ID                int           `json:"id"`
	Username          string        `json:"username"`
	Name              string        `json:"name"`
	State             string        `json:"state"`
	AvatarURL         string        `json:"avatar_url"`
	WebURL            string        `json:"web_url"`
	AccessLevel       int           `json:"access_level"`
	ExpiresAt         *ISOTime    `json:"expires_at"`
	GroupSAMLIdentity *SAMLIdentity `json:"group_saml_identity"`
	Email             string        `json:"email"`
}

// SAMLIdentity represents SAML identity information.
type SAMLIdentity struct {
	ExternUID      string `json:"extern_uid"`
	Provider       string `json:"provider"`
	SAMLProviderID int    `json:"saml_provider_id"`
}

type CreateUserOptions struct {
	Email               *string `url:"email,omitempty" json:"email,omitempty"`
	ForceRandomPassword *bool   `url:"force_random_password,omitempty" json:"force_random_password,omitempty"`
	GroupIDForSAML      *string `url:"group_id_for_saml,omitempty" json:"group_id_for_saml,omitempty"`
	Name                *string `url:"name,omitempty" json:"name,omitempty"`
	Password            *string `url:"password,omitempty" json:"password,omitempty"`
	Username            *string `url:"username,omitempty" json:"username,omitempty"`
}

type InviteGroupMemberRequest struct {
	Email       string           `json:"email"`
	AccessLevel AccessLevelValue `json:"access_level"`
}

type AddGroupMemberRequest struct {
	UserID      int              `json:"user_id"`
	AccessLevel AccessLevelValue `json:"access_level"`
}

type AddProjectMemberRequest struct {
	UserID      int              `json:"user_id"`
	AccessLevel AccessLevelValue `json:"access_level"`
}

type ISOTime time.Time

// ISO 8601 date format.
const iso8601 = "2006-01-02"
