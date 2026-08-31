package actor

import (
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
)

func TestLocalResolverReturnsConfiguredActorInDevelopment(t *testing.T) {
	actorID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	resolver := NewLocalResolver(config.Config{
		Environment:  config.EnvironmentDevelopment,
		LocalActorID: &actorID,
	})

	principal, err := resolver.Resolve(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("resolve principal: %v", err)
	}
	if principal.OwnerID != actorID {
		t.Fatalf("expected owner %s, got %s", actorID, principal.OwnerID)
	}
}

func TestLocalResolverFailsClosedInProduction(t *testing.T) {
	actorID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	resolver := NewLocalResolver(config.Config{
		Environment:  config.EnvironmentProduction,
		LocalActorID: &actorID,
	})

	if _, err := resolver.Resolve(httptest.NewRequest("GET", "/", nil)); err == nil {
		t.Fatal("expected production local fallback to fail closed")
	}
}
