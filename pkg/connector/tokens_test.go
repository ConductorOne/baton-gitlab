package connector

import (
	"testing"
	"time"

	"github.com/conductorone/baton-gitlab/pkg/connector/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
)

func pickSecretTrait(t *testing.T, resource *v2.Resource) *v2.SecretTrait {
	t.Helper()
	st := &v2.SecretTrait{}
	annos := annotations.Annotations(resource.GetAnnotations())
	ok, err := annos.Pick(st)
	if err != nil {
		t.Fatalf("pick secret trait: %v", err)
	}
	if !ok {
		t.Fatal("resource has no SecretTrait")
	}
	return st
}

func TestPersonalAccessTokenResource(t *testing.T) {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	lastUsed := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	expires := client.ISOTime(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))

	resource, err := personalAccessTokenResource(&client.PersonalAccessToken{
		ID:         42,
		Name:       "ci-token",
		UserID:     7,
		Active:     true,
		CreatedAt:  &created,
		LastUsedAt: &lastUsed,
		ExpiresAt:  &expires,
	})
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}

	if resource.GetId().GetResourceType() != personalAccessTokenResourceType.Id {
		t.Errorf("resource type = %q, want %q", resource.GetId().GetResourceType(), personalAccessTokenResourceType.Id)
	}

	st := pickSecretTrait(t, resource)
	if st.GetCredentialType() != v2.SecretTrait_CREDENTIAL_TYPE_STATIC_SECRET {
		t.Errorf("credential_type = %v, want STATIC_SECRET", st.GetCredentialType())
	}
	if st.GetCredentialDetail() != subtypePAT {
		t.Errorf("credential_detail = %q, want %q", st.GetCredentialDetail(), subtypePAT)
	}
	// Owner back-reference must point at the owning user (K2 identity link).
	if got := st.GetIdentityId(); got == nil || got.GetResourceType() != userResourceType.Id || got.GetResource() != "7" {
		t.Errorf("identity_id = %v, want user/7", got)
	}
	if st.GetCreatedAt() == nil || st.GetExpiresAt() == nil || st.GetLastUsedAt() == nil {
		t.Error("expected created_at, expires_at and last_used_at to be set")
	}
}

func TestDeployTokenResourceHasNoOwner(t *testing.T) {
	expires := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)

	resource, err := deployTokenResource(&client.DeployToken{
		ID:        9,
		Name:      "registry-token",
		Username:  "gitlab+deploy-token-9",
		ExpiresAt: &expires,
	})
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}

	st := pickSecretTrait(t, resource)
	if st.GetCredentialType() != v2.SecretTrait_CREDENTIAL_TYPE_STATIC_SECRET {
		t.Errorf("credential_type = %v, want STATIC_SECRET", st.GetCredentialType())
	}
	if st.GetCredentialDetail() != subtypeDeploy {
		t.Errorf("credential_detail = %q, want %q", st.GetCredentialDetail(), subtypeDeploy)
	}
	// Deploy tokens are not tied to a user: no identity back-reference.
	if st.GetIdentityId() != nil {
		t.Errorf("identity_id = %v, want nil (deploy tokens have no user owner)", st.GetIdentityId())
	}
	if st.GetExpiresAt() == nil {
		t.Error("expected expires_at to be set")
	}
	if st.GetCreatedAt() != nil || st.GetLastUsedAt() != nil {
		t.Error("expected created_at and last_used_at to be unset for deploy tokens")
	}
}

func TestServiceAccountResourceIsServiceTyped(t *testing.T) {
	resource, err := serviceAccountResource(&client.ServiceAccount{
		ID:       101,
		Username: "svc_bot",
		Name:     "Service Bot",
		Email:    "svc@example.com",
	}, nil)
	if err != nil {
		t.Fatalf("build resource: %v", err)
	}

	if resource.GetId().GetResourceType() != serviceAccountResourceType.Id {
		t.Errorf("resource type = %q, want %q", resource.GetId().GetResourceType(), serviceAccountResourceType.Id)
	}

	ut, err := resourceSdk.GetUserTrait(resource)
	if err != nil {
		t.Fatalf("get user trait: %v", err)
	}
	if ut.GetAccountType() != v2.UserTrait_ACCOUNT_TYPE_SERVICE {
		t.Errorf("account_type = %v, want SERVICE", ut.GetAccountType())
	}
}
