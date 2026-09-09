package identity

import (
	"context"

	"github.com/google/uuid"
)

// ExternalIdentity is the verified OIDC identity tuple used to resolve SynVideo principals.
type ExternalIdentity struct {
	Issuer  string
	Subject string
}

// Mapper resolves external identities to stable internal owner UUIDs.
type Mapper interface {
	ResolveOrCreateOwner(ctx context.Context, external ExternalIdentity) (uuid.UUID, error)
}
