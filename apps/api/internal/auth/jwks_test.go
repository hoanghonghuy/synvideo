package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/auth/testutil"
)

func TestJWKSCacheBoundsUnknownKidFetchesWithinSnapshotTTL(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()

	cache := auth.NewJWKSCache(fixture.Server.URL+"/jwks", fixture.Server.Client(), 2*time.Second)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const attempts = 25
	for i := 0; i < attempts; i++ {
		token := signTokenWithKid(fixture, privateKey, "unknown-kid-"+itoa(i))
		verifier := mustVerifierWithCache(t, fixture, cache)
		if _, err := verifier.Verify(context.Background(), token); err == nil {
			t.Fatal("expected unknown kid verification to fail closed")
		}
	}

	if fixture.JWKSFetchCount() != 1 {
		t.Fatalf("expected one JWKS fetch within snapshot TTL, got %d", fixture.JWKSFetchCount())
	}
}

func TestJWKSCacheConcurrentUnknownKidMissesTriggerSingleFetch(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()

	cache := auth.NewJWKSCache(fixture.Server.URL+"/jwks", fixture.Server.Client(), 2*time.Second)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	verifier := mustVerifierWithCache(t, fixture, cache)

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer wg.Done()
			token := signTokenWithKid(fixture, privateKey, "concurrent-unknown-"+itoa(index))
			if _, err := verifier.Verify(context.Background(), token); err == nil {
				t.Error("expected unknown kid verification to fail closed")
			}
		}(i)
	}
	wg.Wait()

	if fixture.JWKSFetchCount() != 1 {
		t.Fatalf("expected one JWKS fetch for concurrent unknown kids, got %d", fixture.JWKSFetchCount())
	}
}

func TestJWKSCacheAllowsRotatedKeyAfterSnapshotExpires(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()

	cacheTTL := 40 * time.Millisecond
	cache := auth.NewJWKSCache(fixture.Server.URL+"/jwks", fixture.Server.Client(), cacheTTL)
	verifier := mustVerifierWithCache(t, fixture, cache)

	if _, err := verifier.Verify(context.Background(), fixture.SignToken("prime-user")); err != nil {
		t.Fatalf("prime snapshot with initial key: %v", err)
	}
	if fixture.JWKSFetchCount() != 1 {
		t.Fatalf("expected initial JWKS fetch, got %d", fixture.JWKSFetchCount())
	}

	rotatedKey := fixture.AddRotatedKey("rotated-key-2")
	rotatedToken := fixture.SignTokenWithKey("rotated-key-2", rotatedKey, "user-rotated")

	if _, err := verifier.Verify(context.Background(), rotatedToken); err == nil {
		t.Fatal("expected rotated key to fail before snapshot refresh")
	}
	if fixture.JWKSFetchCount() != 1 {
		t.Fatalf("expected no additional fetch before snapshot expiry, got %d", fixture.JWKSFetchCount())
	}

	time.Sleep(cacheTTL + 20*time.Millisecond)

	if _, err := verifier.Verify(context.Background(), rotatedToken); err != nil {
		t.Fatalf("expected rotated key after snapshot refresh, got %v", err)
	}
	if fixture.JWKSFetchCount() != 2 {
		t.Fatalf("expected one refresh after snapshot expiry, got %d fetches", fixture.JWKSFetchCount())
	}
}

func mustVerifierWithCache(t *testing.T, fixture *testutil.JWKSFixture, cache *auth.JWKSCache) *auth.JWTVerifier {
	verifier, err := auth.NewJWTVerifier(auth.JWTVerifierConfig{
		Issuer:    fixture.Issuer,
		Audience:  fixture.Audience,
		JWKSCache: cache,
	})
	if err != nil {
		t.Fatalf("build verifier: %v", err)
	}
	return verifier
}

func signTokenWithKid(fixture *testutil.JWKSFixture, privateKey *rsa.PrivateKey, kid string) string {
	claims := jwt.MapClaims{
		"iss": fixture.Issuer,
		"sub": "attacker",
		"aud": fixture.Audience,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(privateKey)
	if err != nil {
		panic(err)
	}
	return signed
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func TestJWKSCacheStillRejectsUnknownKidAfterRefresh(t *testing.T) {
	fixture := testutil.StartJWKSFixture()
	defer fixture.Close()
	cache := auth.NewJWKSCache(fixture.Server.URL+"/jwks", fixture.Server.Client(), time.Second)

	_, err := cache.Key(context.Background(), "missing-kid")
	if !errors.Is(err, auth.ErrJWKSKeyNotFound) {
		t.Fatalf("expected key not found, got %v", err)
	}
	if fixture.JWKSFetchCount() != 1 {
		t.Fatalf("expected one fetch, got %d", fixture.JWKSFetchCount())
	}

	_, err = cache.Key(context.Background(), "another-missing-kid")
	if !errors.Is(err, auth.ErrJWKSKeyNotFound) {
		t.Fatalf("expected key not found, got %v", err)
	}
	if fixture.JWKSFetchCount() != 1 {
		t.Fatalf("expected bounded fetch count after negative miss, got %d", fixture.JWKSFetchCount())
	}
}
