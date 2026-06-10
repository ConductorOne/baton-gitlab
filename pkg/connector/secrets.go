package connector

import (
	"strconv"
	"time"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
)

// Axis-2 credential subtype strings (NHI RFC §2.8 convention:
// <platform>.<object>.<purpose>). These refine, but do not replace, the
// indexed credential_type spine value.
const (
	subtypePAT    = "gitlab.token.pat"
	subtypeDeploy = "gitlab.token.deploy"
)

// newStaticSecretResource builds a TRAIT_SECRET resource carrying a SecretTrait
// stamped with credential_type STATIC_SECRET (NHI kind K1) and the given axis-2
// detail. Timestamps are only set when non-zero. When ownerUserID > 0 the
// SecretTrait identity back-reference points at the owning GitLab user so the
// credential links to its K2 identity.
func newStaticSecretResource(
	name string,
	resourceType *v2.ResourceType,
	objectID int,
	detail string,
	createdAt time.Time,
	lastUsedAt time.Time,
	expiresAt time.Time,
	ownerUserID int,
	parentResourceID *v2.ResourceId,
) (*v2.Resource, error) {
	traitOpts := []resourceSdk.SecretTraitOption{
		resourceSdk.WithSecretType(v2.SecretTrait_CREDENTIAL_TYPE_STATIC_SECRET),
		resourceSdk.WithSecretDetail(detail),
	}
	if !createdAt.IsZero() {
		traitOpts = append(traitOpts, resourceSdk.WithSecretCreatedAt(createdAt))
	}
	if !lastUsedAt.IsZero() {
		traitOpts = append(traitOpts, resourceSdk.WithSecretLastUsedAt(lastUsedAt))
	}
	if !expiresAt.IsZero() {
		traitOpts = append(traitOpts, resourceSdk.WithSecretExpiresAt(expiresAt))
	}
	if ownerUserID > 0 {
		ownerID, err := resourceSdk.NewResourceID(userResourceType, strconv.Itoa(ownerUserID))
		if err != nil {
			return nil, err
		}
		traitOpts = append(traitOpts, resourceSdk.WithSecretIdentityID(ownerID))
	}

	var opts []resourceSdk.ResourceOption
	if parentResourceID != nil {
		opts = append(opts, resourceSdk.WithParentResourceID(parentResourceID))
	}

	return resourceSdk.NewSecretResource(name, resourceType, objectID, traitOpts, opts...)
}
