package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth/testutil"
)

func errorsIsJWKSKeyMissing(err error) bool {
	return errors.Is(err, auth.ErrJWKSKeyNotFound) || errors.Is(err, auth.ErrInvalidToken)
}

func TestJWTVerifierAcceptsValidToken(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()

	verifier := newTestVerifier(t, fixture)
	token := fixture.SignToken("user-123")

	identity, err := verifier.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if identity.Issuer != fixture.Issuer || identity.Subject != "user-123" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestJWTVerifierRejectsWrongIssuerAudienceAndSignature(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	verifier := newTestVerifier(t, fixture)

	if _, err := verifier.Verify(context.Background(), fixture.SignToken("user-123", testutil.WithIssuer("https://evil.example"))); err != auth.ErrIssuerMismatch {
		t.Fatalf("expected issuer mismatch, got %v", err)
	}
	if _, err := verifier.Verify(context.Background(), fixture.SignToken("user-123", testutil.WithAudience("wrong-aud"))); err != auth.ErrAudienceMismatch {
		t.Fatalf("expected audience mismatch, got %v", err)
	}

	other := testutil.StartJWKSFixture()
	defer other.Close()
	if _, err := verifier.Verify(context.Background(), other.SignToken("user-123")); err == nil {
		t.Fatal("expected invalid signature to fail closed")
	}
}

func TestJWTVerifierRejectsExpiredAndNotYetValidTokens(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	verifier := newTestVerifier(t, fixture)

	if _, err := verifier.Verify(context.Background(), fixture.SignToken("user-123", testutil.WithExpiry(time.Now().Add(-time.Minute)))); err != auth.ErrTokenExpired {
		t.Fatalf("expected expired token rejection, got %v", err)
	}
	if _, err := verifier.Verify(context.Background(), fixture.SignToken("user-123", testutil.WithNotBefore(time.Now().Add(time.Minute)))); err != auth.ErrTokenNotYetValid {
		t.Fatalf("expected not-yet-valid rejection, got %v", err)
	}
}

func TestJWTVerifierRejectsUnsupportedAlgorithm(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	verifier := newTestVerifier(t, fixture)

	claims := jwt.MapClaims{
		"iss": fixture.Issuer,
		"sub": "user-123",
		"aud": fixture.Audience,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	token.Header["alg"] = "none"
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}
	if _, err := verifier.Verify(context.Background(), unsigned); err == nil {
		t.Fatal("expected unsupported alg rejection")
	}
}

func TestJWTVerifierRejectsUnknownKey(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	verifier := newTestVerifier(t, fixture)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	claims := jwt.MapClaims{
		"iss": fixture.Issuer,
		"sub": "user-123",
		"aud": fixture.Audience,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "missing-key"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := verifier.Verify(context.Background(), signed); err == nil || !errorsIsJWKSKeyMissing(err) {
		t.Fatalf("expected unknown key rejection, got %v", err)
	}
}

func TestJWKSDiscoveryTimeoutDoesNotAuthenticate(t *testing.T) {
	hanging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer hanging.Close()

	client := &http.Client{Timeout: 20 * time.Millisecond}
	verifier, err := auth.BuildJWTVerifier(context.Background(), hanging.URL, "synvideo-api", hanging.URL+"/jwks", client, time.Minute, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("build verifier: %v", err)
	}

	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	if _, err := verifier.Verify(context.Background(), fixture.SignToken("user-123")); err == nil {
		t.Fatal("expected jwks timeout to fail closed")
	}
}

func TestSanitizeAuthErrorNeverIncludesRawToken(t *testing.T) {
	raw := testBearerHeaderForRedaction()
	message := auth.SanitizeAuthError(auth.ErrInvalidToken)
	if message == "" || message == raw {
		t.Fatalf("expected sanitized message, got %q", message)
	}
	if auth.RedactAuthorizationHeader(raw) == raw {
		t.Fatal("expected authorization header redaction")
	}
}

func testBearerHeaderForRedaction() string {
	return "Bearer " + "synvideo-redaction-test-fixture"
}

func newTestVerifier(t *testing.T, fixture *testutil.JWKSFixture) *auth.JWTVerifier {
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
