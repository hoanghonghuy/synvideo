package actor_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth/testutil"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/identity"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type memoryIdentityMapper struct {
	mu    sync.Mutex
	byKey map[string]uuid.UUID
}

func (m *memoryIdentityMapper) ResolveOrCreateOwner(_ context.Context, external identity.ExternalIdentity) (uuid.UUID, error) {
	key := external.Issuer + "\x00" + external.Subject
	m.mu.Lock()
	defer m.mu.Unlock()
	if ownerID, ok := m.byKey[key]; ok {
		return ownerID, nil
	}
	ownerID := uuid.New()
	m.byKey[key] = ownerID
	return ownerID, nil
}

func TestProductionResolverMapsValidJWTToStablePrincipal(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()

	verifier := mustBuildVerifier(t, fixture)
	mapper := &memoryIdentityMapper{byKey: make(map[string]uuid.UUID)}
	resolver := actor.NewProductionResolver(verifier, mapper)
	token := fixture.SignToken("subject-a")

	first := resolveWithToken(t, resolver, token)
	second := resolveWithToken(t, resolver, token)
	if first.OwnerID != second.OwnerID {
		t.Fatalf("expected stable owner mapping, got %s then %s", first.OwnerID, second.OwnerID)
	}
}

func TestProductionResolverRejectsMissingOrInvalidCredentials(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	resolver := actor.NewProductionResolver(mustBuildVerifier(t, fixture), &memoryIdentityMapper{byKey: make(map[string]uuid.UUID)})

	if _, err := resolver.Resolve(httptest.NewRequest(http.MethodGet, "/", nil)); err == nil {
		t.Fatal("expected missing authorization to fail closed")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	if _, err := resolver.Resolve(req); err == nil {
		t.Fatal("expected malformed token to fail closed")
	}
}

func TestProductionResolverDoesNotAliasDifferentIssuerOrSubject(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	mapper := &memoryIdentityMapper{byKey: make(map[string]uuid.UUID)}
	resolver := actor.NewProductionResolver(mustBuildVerifier(t, fixture), mapper)

	subjectA := resolveWithToken(t, resolver, fixture.SignToken("subject-a"))
	subjectB := resolveWithToken(t, resolver, fixture.SignToken("subject-b"))
	if subjectA.OwnerID == subjectB.OwnerID {
		t.Fatal("different subjects must not alias the same principal")
	}

	otherIssuer := testutil.StartJWKSFixture()
	defer otherIssuer.Close()
	otherResolver := actor.NewProductionResolver(mustBuildVerifier(t, otherIssuer), mapper)
	otherSubject := resolveWithToken(t, otherResolver, otherIssuer.SignToken("subject-a"))
	if subjectA.OwnerID == otherSubject.OwnerID {
		t.Fatal("different issuers must not alias the same principal")
	}
}

func mustBuildVerifier(t *testing.T, fixture *testutil.JWKSFixture) *auth.JWTVerifier {
	verifier, err := auth.BuildJWTVerifier(
		context.Background(),
		fixture.Issuer,
		fixture.Audience,
		fixture.Server.URL+"/jwks",
		fixture.Server.Client(),
		time.Minute,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf("build verifier: %v", err)
	}
	return verifier
}

func resolveWithToken(t *testing.T, resolver actor.Resolver, token string) project.Principal {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	principal, err := resolver.Resolve(req)
	if err != nil {
		t.Fatalf("resolve principal: %v", err)
	}
	return principal
}
