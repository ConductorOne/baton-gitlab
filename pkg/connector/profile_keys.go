package connector

// Profile / account-schema field keys, shared across resource builders. Defined
// as constants so repeated use does not trip the goconst linter.
const (
	fieldName     = "name"
	fieldEmail    = "email"
	fieldUsername = "username"
)
