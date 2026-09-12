package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYouTubeOAuthStateCookieBinding(t *testing.T) {
	state := "signed-owner-project-state"
	value := oauthStateCookieValue(state)
	if value == "" || value == state {
		t.Fatalf("state cookie must be a non-empty digest, got %q", value)
	}
	if !constantTimeStringEqual(value, oauthStateCookieValue(state)) {
		t.Fatal("same OAuth state must match browser binding")
	}
	if constantTimeStringEqual(value, oauthStateCookieValue(state+"-tampered")) {
		t.Fatal("tampered OAuth state must not match browser binding")
	}
}

func TestSetYouTubeOAuthCookieIsHttpOnlyLaxAndSecureBehindTLSProxy(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://api.example/api/v1/publishing/youtube/oauth/start", nil)
	request.Header.Set("X-Forwarded-Proto", "https")

	setYouTubeOAuthCookie(recorder, request, "digest", 600)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != youtubeOAuthStateCookie || cookie.Value != "digest" {
		t.Fatalf("unexpected OAuth cookie: %+v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("OAuth cookie security attributes missing: %+v", cookie)
	}
	if cookie.Path != "/api/v1/publishing/youtube/oauth/callback" || cookie.MaxAge != 600 {
		t.Fatalf("OAuth cookie scope/lifetime unexpected: %+v", cookie)
	}
}
