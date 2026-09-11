package publishing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestYouTubeOAuthAuthorizationURL(t *testing.T) {
	oauth, err := NewYouTubeOAuth(YouTubeOAuthConfig{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "https://synvideo.test/oauth/youtube/callback",
	}, nil)
	if err != nil { t.Fatal(err) }
	raw, err := oauth.AuthorizationURL("csrf-state")
	if err != nil { t.Fatal(err) }
	u, err := url.Parse(raw)
	if err != nil { t.Fatal(err) }
	q := u.Query()
	if q.Get("state") != "csrf-state" || q.Get("scope") != youtubeUploadScope || q.Get("access_type") != "offline" || q.Get("prompt") != "consent" {
		t.Fatalf("unexpected authorization query: %v", q)
	}
}

func TestYouTubeOAuthExchangeAndRefresh(t *testing.T) {
	var requests []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil { t.Fatal(err) }
		copyValues := make(url.Values, len(r.Form))
		for k, v := range r.Form { copyValues[k] = append([]string(nil), v...) }
		requests = append(requests, copyValues)
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("grant_type") == "authorization_code" {
			_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"access-2","token_type":"Bearer","expires_in":1800}`))
	}))
	defer server.Close()

	oauth, err := NewYouTubeOAuth(YouTubeOAuthConfig{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "https://synvideo.test/callback", TokenURL: server.URL,
	}, server.Client())
	if err != nil { t.Fatal(err) }
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	oauth.now = func() time.Time { return now }

	exchanged, err := oauth.ExchangeCode(context.Background(), "auth-code")
	if err != nil { t.Fatal(err) }
	if exchanged.AccessToken != "access-1" || exchanged.RefreshToken != "refresh-1" || !exchanged.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected exchange token: %+v", exchanged)
	}
	refreshed, err := oauth.Refresh(context.Background(), "refresh-1")
	if err != nil { t.Fatal(err) }
	if refreshed.AccessToken != "access-2" || refreshed.RefreshToken != "refresh-1" || !refreshed.ExpiresAt.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("unexpected refreshed token: %+v", refreshed)
	}
	if len(requests) != 2 || requests[0].Get("client_secret") != "secret" || requests[0].Get("code") != "auth-code" || requests[1].Get("refresh_token") != "refresh-1" {
		t.Fatalf("unexpected token requests: %#v", requests)
	}
}

func TestYouTubeOAuthProviderFailureDoesNotLeakBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","refresh_token":"must-not-leak"}`))
	}))
	defer server.Close()
	oauth, err := NewYouTubeOAuth(YouTubeOAuthConfig{ClientID: "client", ClientSecret: "secret", RedirectURL: "https://synvideo.test/callback", TokenURL: server.URL}, server.Client())
	if err != nil { t.Fatal(err) }
	_, err = oauth.Refresh(context.Background(), "refresh-secret")
	if !errors.Is(err, ErrOAuthRefresh) { t.Fatalf("expected refresh sentinel, got %v", err) }
	if err != nil && (contains(err.Error(), "must-not-leak") || contains(err.Error(), "refresh-secret")) { t.Fatalf("credential leaked in error: %v", err) }
}

func TestYouTubeOAuthRejectsIncompleteConfigAndEmptyState(t *testing.T) {
	if _, err := NewYouTubeOAuth(YouTubeOAuthConfig{}, nil); !errors.Is(err, ErrOAuthConfiguration) { t.Fatalf("expected config error, got %v", err) }
	oauth, err := NewYouTubeOAuth(YouTubeOAuthConfig{ClientID: "client", ClientSecret: "secret", RedirectURL: "https://synvideo.test/callback"}, nil)
	if err != nil { t.Fatal(err) }
	if _, err := oauth.AuthorizationURL(" "); !errors.Is(err, ErrOAuthConfiguration) { t.Fatalf("expected state error, got %v", err) }
}

func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ { if s[i:i+len(part)] == part { return true } }
	return false
}
