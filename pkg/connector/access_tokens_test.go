package connector

import (
	"testing"
	"time"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

func TestProjectAccessTokenResource(t *testing.T) {
	created := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	expires := client.ISOTime(time.Date(2027, 2, 2, 0, 0, 0, 0, time.UTC))

	parent := &v2.ResourceId{ResourceType: projectResourceType.Id, Resource: "55"}
	resource, err := accessTokenResource(&client.AccessToken{
		ID:        3,
		UserID:    900,
		Name:      "project-bot-token",
		CreatedAt: &created,
		ExpiresAt: &expires,
	}, projectAccessTokenResourceType, subtypeProjectAccess, parent)
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}

	if resource.GetId().GetResourceType() != projectAccessTokenResourceType.Id {
		t.Errorf("resource type = %q, want %q", resource.GetId().GetResourceType(), projectAccessTokenResourceType.Id)
	}
	if resource.GetParentResourceId().GetResource() != "55" {
		t.Errorf("parent = %v, want project/55", resource.GetParentResourceId())
	}

	st := pickSecretTrait(t, resource)
	if st.GetCredentialType() != v2.SecretTrait_CREDENTIAL_TYPE_STATIC_SECRET {
		t.Errorf("credential_type = %v, want STATIC_SECRET", st.GetCredentialType())
	}
	if st.GetCredentialDetail() != subtypeProjectAccess {
		t.Errorf("credential_detail = %q, want %q", st.GetCredentialDetail(), subtypeProjectAccess)
	}
	// Backing bot user must be the SecretTrait identity back-reference.
	if got := st.GetIdentityId(); got == nil || got.GetResource() != "900" || got.GetResourceType() != userResourceType.Id {
		t.Errorf("identity_id = %v, want user/900", got)
	}
}

func TestGroupAccessTokenResourceDetail(t *testing.T) {
	resource, err := accessTokenResource(&client.AccessToken{
		ID:     4,
		UserID: 901,
		Name:   "group-bot-token",
	}, groupAccessTokenResourceType, subtypeGroupAccess, nil)
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}

	st := pickSecretTrait(t, resource)
	if st.GetCredentialDetail() != subtypeGroupAccess {
		t.Errorf("credential_detail = %q, want %q", st.GetCredentialDetail(), subtypeGroupAccess)
	}
	if st.GetCredentialType() != v2.SecretTrait_CREDENTIAL_TYPE_STATIC_SECRET {
		t.Errorf("credential_type = %v, want STATIC_SECRET", st.GetCredentialType())
	}
}
