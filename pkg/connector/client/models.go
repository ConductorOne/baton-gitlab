package client

import (
	"time"
)

type User struct {
	ID              int      `json:"id"`
	Email           string   `json:"email"`
	Username        string   `json:"username"`
	Name            string   `json:"name"`
	State           string   `json:"state"`
	LastActivityOn  *ISOTime `json:"last_activity_on"`
	MembershipState string   `json:"membership_state"`
	Locked          bool     `json:"locked"`
}

type PendingInviteUser struct {
	InviteEmail string `json:"invite_email"`
}

type Group struct {
	ID                int      `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	FullName          string   `json:"full_name"`
	ParentID          int      `json:"parent_id"`
	Archived          bool     `json:"archived"`
	Visibility        string   `json:"visibility"`
	MarkedForDeletion *ISOTime `json:"marked_for_deletion"`
}

type Namespace struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	FullPath string `json:"full_path"`
	ParentId int    `json:"parent_id"`
}

type Project struct {
	ID                int        `json:"id"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	NameWithNamespace string     `json:"name_with_namespace"`
	Namespace         *Namespace `json:"namespace"`
}

type GroupMember struct {
	ID                int           `json:"id"`
	Email             string        `json:"email"`
	Username          string        `json:"username"`
	Name              string        `json:"name"`
	State             string        `json:"state"`
	AccessLevel       int           `json:"access_level"`
	GroupSAMLIdentity *SAMLIdentity `json:"group_saml_identity"`
	MembershipState   string        `json:"membership_state"`
	Locked            bool          `json:"locked"`
}

type ProjectMember struct {
	ID                int           `json:"id"`
	Username          string        `json:"username"`
	Name              string        `json:"name"`
	State             string        `json:"state"`
	AvatarURL         string        `json:"avatar_url"`
	WebURL            string        `json:"web_url"`
	AccessLevel       int           `json:"access_level"`
	ExpiresAt         *ISOTime      `json:"expires_at"`
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

// ServiceAccount is a GitLab service account user, returned by the instance
// (GET /service_accounts) and group (GET /groups/:id/service_accounts) endpoints.
// https://docs.gitlab.com/api/users/#list-service-account-users
type ServiceAccount struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// PersonalAccessToken is GitLab PAT metadata. The token value itself is never
// returned by the list endpoint, only by rotation.
// https://docs.gitlab.com/api/personal_access_tokens/
type PersonalAccessToken struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	UserID     int        `json:"user_id"`
	Scopes     []string   `json:"scopes"`
	Active     bool       `json:"active"`
	Revoked    bool       `json:"revoked"`
	CreatedAt  *time.Time `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *ISOTime   `json:"expires_at"`
}

// DeployToken is GitLab deploy token metadata (instance/project/group scoped).
// Deploy tokens are not tied to a user. The secret is only returned on creation.
// https://docs.gitlab.com/api/deploy_tokens/
type DeployToken struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	Username  string     `json:"username"`
	Scopes    []string   `json:"scopes"`
	Revoked   bool       `json:"revoked"`
	Expired   bool       `json:"expired"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// AccessToken is the shared shape of GitLab project and group access tokens.
// Each is backed by a per-resource bot user (UserID); the secret is only
// returned on creation/rotation.
// https://docs.gitlab.com/api/project_access_tokens/
// https://docs.gitlab.com/api/group_access_tokens/
type AccessToken struct {
	ID          int        `json:"id"`
	UserID      int        `json:"user_id"`
	Name        string     `json:"name"`
	Scopes      []string   `json:"scopes"`
	Active      bool       `json:"active"`
	Revoked     bool       `json:"revoked"`
	AccessLevel int        `json:"access_level"`
	CreatedAt   *time.Time `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *ISOTime   `json:"expires_at"`
}
