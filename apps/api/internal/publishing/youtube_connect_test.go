package publishing

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestYouTubeConnectServiceSignedStateAndReconnect(t *testing.T) {
	repository := &fakeConnectionRepository{}
	protector, err := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	connections, err := NewConnectionService(repository, protector)
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"items":[{"id":"UC-secure","snippet":{"title":"Secure Channel"}}]}`
		if strings.Contains(r.URL.Host, "oauth.example") {
			body = `{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	oauth, err := NewYouTubeOAuth(YouTubeOAuthConfig{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "https://api.example/oauth/callback",
		AuthURL: "https://accounts.example/auth", TokenURL: "https://oauth.example/token",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewYouTubeConnectService(oauth, connections, "0123456789abcdef0123456789abcdef", "https://app.example", client)
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, 9, 12, 6, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	ownerID := uuid.New()
	projectID := uuid.New()
	authorizationURL, err := service.Start(ownerID, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(authorizationURL, "access_type=offline") || !strings.Contains(authorizationURL, "prompt=consent") {
		t.Fatalf("authorization URL missing offline consent: %s", authorizationURL)
	}
	stateStart := strings.Index(authorizationURL, "state=")
	if stateStart < 0 {
		t.Fatalf("authorization URL missing state: %s", authorizationURL)
	}
	parsed, err := http.NewRequest(http.MethodGet, authorizationURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	state := parsed.URL.Query().Get("state")
	if got, err := service.ProjectFromState(state); err != nil || got != projectID {
		t.Fatalf("project from state = %s, %v", got, err)
	}
	if _, err := service.ProjectFromState(state + "tampered"); err != ErrOAuthState {
		t.Fatalf("tampered state error = %v", err)
	}

	saved, completedProjectID, err := service.Complete(t.Context(), state, "code-1")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completedProjectID != projectID || saved.OwnerID != ownerID || saved.RemoteChannelID != "UC-secure" || saved.State != ConnectionConnected {
		t.Fatalf("unexpected saved connection: %+v project=%s", saved, completedProjectID)
	}
	refreshToken, err := connections.RefreshToken(t.Context(), ownerID, saved.ID)
	if err != nil || refreshToken != "refresh-1" {
		t.Fatalf("refresh token = %q, %v", refreshToken, err)
	}
	if got := service.ReturnURL(projectID, "connected"); got != "https://app.example/projects/"+projectID.String()+"/publishing?youtube=connected" {
		t.Fatalf("return URL = %s", got)
	}
}

func TestYouTubeConnectServiceRejectsExpiredState(t *testing.T) {
	repository := &fakeConnectionRepository{}
	protector, _ := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": []byte("0123456789abcdef0123456789abcdef")})
	connections, _ := NewConnectionService(repository, protector)
	oauth, err := NewYouTubeOAuth(YouTubeOAuthConfig{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "https://api.example/oauth/callback",
	}, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewYouTubeConnectService(oauth, connections, "0123456789abcdef0123456789abcdef", "https://app.example", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	startTime := time.Date(2026, 9, 12, 6, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return startTime }
	ownerID, projectID := uuid.New(), uuid.New()
	authorizationURL, err := service.Start(ownerID, projectID)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, authorizationURL, nil)
	state := req.URL.Query().Get("state")
	service.now = func() time.Time { return startTime.Add(11 * time.Minute) }
	if _, err := service.ProjectFromState(state); err != ErrOAuthState {
		t.Fatalf("expired state error = %v", err)
	}
}
