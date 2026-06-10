package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

// The user resource type is for all user objects from the database.
var userResourceType = &v2.ResourceType{
	Id:          "user",
	DisplayName: "User",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

var groupResourceType = &v2.ResourceType{
	Id:          "group",
	DisplayName: "Group",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
}

var projectResourceType = &v2.ResourceType{
	Id:          "project",
	DisplayName: "Project",
}

// serviceAccountResourceType is a GitLab service account: a distinct, non-human
// user type (NHI kind K2). Emitted with account type SERVICE.
var serviceAccountResourceType = &v2.ResourceType{
	Id:          "service_account",
	DisplayName: "Service Account",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

// personalAccessTokenResourceType is a GitLab personal access token: an opaque
// static credential (NHI kind K1, STATIC_SECRET).
var personalAccessTokenResourceType = &v2.ResourceType{
	Id:          "personal_access_token",
	DisplayName: "Personal Access Token",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_SECRET},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

// deployTokenResourceType is a GitLab deploy token: an opaque static credential
// (NHI kind K1, STATIC_SECRET).
var deployTokenResourceType = &v2.ResourceType{
	Id:          "deploy_token",
	DisplayName: "Deploy Token",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_SECRET},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

// projectAccessTokenResourceType is a GitLab project access token: an opaque
// static credential backed by a per-project bot user (NHI kind K1, STATIC_SECRET).
var projectAccessTokenResourceType = &v2.ResourceType{
	Id:          "project_access_token",
	DisplayName: "Project Access Token",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_SECRET},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

// groupAccessTokenResourceType is a GitLab group access token: an opaque static
// credential backed by a per-group bot user (NHI kind K1, STATIC_SECRET).
var groupAccessTokenResourceType = &v2.ResourceType{
	Id:          "group_access_token",
	DisplayName: "Group Access Token",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_SECRET},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}
