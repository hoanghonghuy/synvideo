package actor

import (
	"net/http"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/identity"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

// ProductionResolver authenticates Bearer access tokens and maps them to SynVideo principals.
type ProductionResolver struct {
	verifier *auth.JWTVerifier
	mapper   identity.Mapper
}

func NewProductionResolver(verifier *auth.JWTVerifier, mapper identity.Mapper) *ProductionResolver {
	return &ProductionResolver{verifier: verifier, mapper: mapper}
}

func (r *ProductionResolver) Resolve(req *http.Request) (project.Principal, error) {
	if r.verifier == nil || r.mapper == nil {
		return project.Principal{}, ErrNoPrincipal
	}

	rawToken, err := auth.ExtractBearerToken(req.Header.Get("Authorization"))
	if err != nil {
		return project.Principal{}, ErrNoPrincipal
	}

	verified, err := r.verifier.Verify(req.Context(), rawToken)
	if err != nil {
		return project.Principal{}, ErrNoPrincipal
	}

	ownerID, err := r.mapper.ResolveOrCreateOwner(req.Context(), identity.ExternalIdentity{
		Issuer:  verified.Issuer,
		Subject: verified.Subject,
	})
	if err != nil {
		return project.Principal{}, ErrNoPrincipal
	}

	return project.Principal{OwnerID: ownerID}, nil
}
