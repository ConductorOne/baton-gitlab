package onprem

import (
	gitlabSDK "gitlab.com/gitlab-org/api/client-go"
)

type User struct {
	ID                  int                `json:"id"`
	Username            string             `json:"username"`
	PublicEmail         *string            `json:"public_email"`
	Name                string             `json:"name"`
	State               string             `json:"state"`
	Locked              bool               `json:"locked"`
	AvatarURL           string             `json:"avatar_url"`
	WebURL              string             `json:"web_url"`
	CreatedAt           string             `json:"created_at"`
	Bio                 string             `json:"bio"`
	Location            string             `json:"location"`
	LinkedIn            string             `json:"linkedin"`
	Twitter             string             `json:"twitter"`
	Discord             string             `json:"discord"`
	WebsiteURL          string             `json:"website_url"`
	Github              string             `json:"github"`
	JobTitle            string             `json:"job_title"`
	Pronouns            *string            `json:"pronouns"`
	Organization        string             `json:"organization"`
	Bot                 bool               `json:"bot"`
	WorkInformation     *string            `json:"work_information"`
	Followers           *int               `json:"followers,omitempty"`
	Following           *int               `json:"following,omitempty"`
	IsFollowed          *bool              `json:"is_followed,omitempty"`
	LocalTime           *string            `json:"local_time"`
	LastSignInAt        *string            `json:"last_sign_in_at"`
	ConfirmedAt         string             `json:"confirmed_at"`
	LastActivityOn      *gitlabSDK.ISOTime `json:"last_activity_on"`
	Email               string             `json:"email"`
	ThemeID             int                `json:"theme_id"`
	ColorSchemeID       int                `json:"color_scheme_id"`
	ProjectsLimit       int                `json:"projects_limit"`
	CurrentSignInAt     *string            `json:"current_sign_in_at"`
	Identities          []string           `json:"identities"`
	CanCreateGroup      bool               `json:"can_create_group"`
	CanCreateProject    bool               `json:"can_create_project"`
	TwoFactorEnabled    bool               `json:"two_factor_enabled"`
	External            bool               `json:"external"`
	PrivateProfile      bool               `json:"private_profile"`
	CommitEmail         string             `json:"commit_email"`
	IsAdmin             bool               `json:"is_admin"`
	Note                *string            `json:"note"`
	NamespaceID         int                `json:"namespace_id"`
	CreatedBy           *CreatedBy         `json:"created_by"`
	EmailResetOfferedAt *string            `json:"email_reset_offered_at"`
}

type CreatedBy struct {
	ID          int     `json:"id"`
	Username    string  `json:"username"`
	PublicEmail *string `json:"public_email"`
	Name        string  `json:"name"`
	State       string  `json:"state"`
	Locked      bool    `json:"locked"`
	AvatarURL   string  `json:"avatar_url"`
	WebURL      string  `json:"web_url"`
}
