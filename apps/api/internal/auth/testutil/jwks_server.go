package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWKSFixture struct {
	Server     *httptest.Server
	Issuer     string
	Audience   string
	PrivateKey *rsa.PrivateKey
	KeyID      string
}

func StartJWKSFixture() *JWKSFixture {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("generate rsa key: %v", err))
	}

	fixture := &JWKSFixture{
		Audience:   "synvideo-api",
		PrivateKey: privateKey,
		KeyID:      "test-key-1",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   fixture.Server.URL,
			"jwks_uri": fixture.Server.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{rsaPublicJWK(privateKey.PublicKey, fixture.KeyID)},
		})
	})

	fixture.Server = httptest.NewServer(mux)
	fixture.Issuer = fixture.Server.URL
	return fixture
}

func (f *JWKSFixture) Close() {
	f.Server.Close()
}

func (f *JWKSFixture) SignToken(subject string, opts ...TokenOption) string {
	claims := jwt.MapClaims{
		"iss": f.Issuer,
		"sub": subject,
		"aud": f.Audience,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	}
	for _, opt := range opts {
		opt(claims)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = f.KeyID
	signed, err := token.SignedString(f.PrivateKey)
	if err != nil {
		panic(fmt.Sprintf("sign token: %v", err))
	}
	return signed
}

type TokenOption func(jwt.MapClaims)

func WithIssuer(issuer string) TokenOption {
	return func(claims jwt.MapClaims) {
		claims["iss"] = issuer
	}
}

func WithAudience(audience string) TokenOption {
	return func(claims jwt.MapClaims) {
		claims["aud"] = audience
	}
}

func WithExpiry(exp time.Time) TokenOption {
	return func(claims jwt.MapClaims) {
		claims["exp"] = exp.Unix()
	}
}

func WithNotBefore(nbf time.Time) TokenOption {
	return func(claims jwt.MapClaims) {
		claims["nbf"] = nbf.Unix()
	}
}

func rsaPublicJWK(pub rsa.PublicKey, kid string) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}
