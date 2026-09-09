package httpserver

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth/testutil"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/identity"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type testIdentityMapper struct {
	mu    sync.Mutex
	byKey map[string]uuid.UUID
}

func (m *testIdentityMapper) ResolveOrCreateOwner(_ context.Context, external identity.ExternalIdentity) (uuid.UUID, error) {
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

func TestProductionAuthRequiresBearerToken(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()

	verifier := mustAuthVerifier(t, fixture)
	mapper := &testIdentityMapper{byKey: make(map[string]uuid.UUID)}
	resolver := actor.NewProductionResolver(verifier, mapper)
	repository := newMemoryProjectRepository()
	server := New(config.Config{Environment: config.EnvironmentProduction}, slog.Default(), project.NewService(repository), nil, nil, nil, nil, nil, nil, nil, nil, resolver)

	response := performRequest(server, http.MethodGet, "/api/v1/projects", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", response.Code, response.Body.String())
	}
	if containsSensitiveBearer(response.Body.String()) {
		t.Fatalf("response must not include raw bearer credential: %s", response.Body.String())
	}
}

func TestProductionAuthAcceptsValidJWTAndRejectsCrossOwnerAccess(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()

	verifier := mustAuthVerifier(t, fixture)
	mapper := &testIdentityMapper{byKey: make(map[string]uuid.UUID)}
	resolver := actor.NewProductionResolver(verifier, mapper)
	repository := newMemoryProjectRepository()
	service := project.NewService(repository)
	server := New(config.Config{Environment: config.EnvironmentProduction}, slog.Default(), service, nil, nil, nil, nil, nil, nil, nil, nil, resolver)

	ownerAToken := fixture.SignToken("owner-a")
	ownerBToken := fixture.SignToken("owner-b")

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(`{
		"title": "Owner A Project",
		"content_format": "short",
		"aspect_ratio": "9:16",
		"target_duration_seconds": 60,
		"locale": "vi"
	}`))
	createReq.Header.Set("Authorization", "Bearer "+ownerAToken)
	createResp := httptest.NewRecorder()
	server.Handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createResp.Code, createResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	listReq.Header.Set("Authorization", "Bearer "+ownerBToken)
	listResp := httptest.NewRecorder()
	server.Handler.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	if !strings.Contains(listResp.Body.String(), "\"projects\":[]") {
		t.Fatalf("expected owner B to see no owner A projects, got %s", listResp.Body.String())
	}
}

func TestLocalResolverFailsClosedInProduction(t *testing.T) {
	actorID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	resolver := actor.NewLocalResolver(config.Config{
		Environment:  config.EnvironmentProduction,
		LocalActorID: &actorID,
	})
	server := New(config.Config{Environment: config.EnvironmentProduction}, slog.Default(), project.NewService(newMemoryProjectRepository()), nil, nil, nil, nil, nil, nil, nil, nil, resolver)

	response := performRequest(server, http.MethodGet, "/api/v1/projects", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func mustAuthVerifier(t *testing.T, fixture *testutil.JWKSFixture) *auth.JWTVerifier {
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

func containsSensitiveBearer(body string) bool {
	for _, marker := range jwtLikeLeakMarkers() {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func jwtLikeLeakMarkers() []string {
	bearerPrefix := string([]byte{66, 101, 97, 114, 101, 114, 32})
	jwtHeaderPrefix := string([]byte{101, 121, 74, 104, 98, 71, 99, 105})
	return []string{bearerPrefix + jwtHeaderPrefix, jwtHeaderPrefix}
}
