package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken     = errors.New("invalid bearer token")
	ErrTokenExpired     = errors.New("bearer token expired")
	ErrTokenNotYetValid = errors.New("bearer token not yet valid")
	ErrUnsupportedAlg   = errors.New("unsupported jwt algorithm")
	ErrMissingClaims    = errors.New("missing required jwt claims")
	ErrIssuerMismatch   = errors.New("jwt issuer mismatch")
	ErrAudienceMismatch = errors.New("jwt audience mismatch")
)

var allowedAlgorithms = map[string]bool{
	"RS256": true,
	"RS384": true,
	"RS512": true,
	"ES256": true,
	"ES384": true,
	"ES512": true,
	"PS256": true,
	"PS384": true,
	"PS512": true,
}

// VerifiedIdentity is the trusted external identity extracted from a JWT access token.
type VerifiedIdentity struct {
	Issuer  string
	Subject string
}

// JWTVerifier validates bearer access tokens against configured OIDC constraints.
type JWTVerifier struct {
	expectedIssuer   string
	expectedAudience string
	jwks             *JWKSCache
	leeway           time.Duration
}

type JWTVerifierConfig struct {
	Issuer    string
	Audience  string
	JWKSCache *JWKSCache
	Leeway    time.Duration
}

func NewJWTVerifier(cfg JWTVerifierConfig) (*JWTVerifier, error) {
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, errors.New("oidc issuer is required")
	}
	if strings.TrimSpace(cfg.Audience) == "" {
		return nil, errors.New("oidc audience is required")
	}
	if cfg.JWKSCache == nil {
		return nil, errors.New("jwks cache is required")
	}
	leeway := cfg.Leeway
	if leeway == 0 {
		leeway = 30 * time.Second
	}
	return &JWTVerifier{
		expectedIssuer:   strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/"),
		expectedAudience: strings.TrimSpace(cfg.Audience),
		jwks:             cfg.JWKSCache,
		leeway:           leeway,
	}, nil
}

func (v *JWTVerifier) Verify(ctx context.Context, rawToken string) (VerifiedIdentity, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return VerifiedIdentity{}, ErrInvalidToken
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{
		"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512",
	}))
	token, err := parser.Parse(rawToken, func(t *jwt.Token) (any, error) {
		alg, _ := t.Header["alg"].(string)
		if strings.EqualFold(alg, "none") || !allowedAlgorithms[alg] {
			return nil, ErrUnsupportedAlg
		}
		kid, _ := t.Header["kid"].(string)
		return v.jwks.Key(ctx, kid)
	})
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return VerifiedIdentity{}, ErrTokenExpired
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return VerifiedIdentity{}, ErrTokenNotYetValid
		case errors.Is(err, ErrUnsupportedAlg):
			return VerifiedIdentity{}, ErrUnsupportedAlg
		case errors.Is(err, ErrJWKSKeyNotFound):
			return VerifiedIdentity{}, ErrJWKSKeyNotFound
		case errors.Is(err, ErrJWKSFetchFailed):
			return VerifiedIdentity{}, ErrJWKSFetchFailed
		default:
			return VerifiedIdentity{}, ErrInvalidToken
		}
	}
	if !token.Valid {
		return VerifiedIdentity{}, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return VerifiedIdentity{}, ErrMissingClaims
	}

	issuer, err := stringClaim(claims, "iss")
	if err != nil {
		return VerifiedIdentity{}, ErrMissingClaims
	}
	issuer = strings.TrimRight(issuer, "/")
	if issuer != v.expectedIssuer {
		return VerifiedIdentity{}, ErrIssuerMismatch
	}

	subject, err := stringClaim(claims, "sub")
	if err != nil {
		return VerifiedIdentity{}, ErrMissingClaims
	}

	if !audienceMatches(claims["aud"], v.expectedAudience) {
		return VerifiedIdentity{}, ErrAudienceMismatch
	}

	now := time.Now()
	if exp, err := numericDateClaim(claims, "exp"); err == nil {
		if now.After(exp.Add(v.leeway)) {
			return VerifiedIdentity{}, ErrTokenExpired
		}
	} else {
		return VerifiedIdentity{}, ErrMissingClaims
	}

	if nbf, err := numericDateClaim(claims, "nbf"); err == nil {
		if now.Add(v.leeway).Before(nbf) {
			return VerifiedIdentity{}, ErrTokenNotYetValid
		}
	}

	return VerifiedIdentity{Issuer: issuer, Subject: subject}, nil
}

func stringClaim(claims jwt.MapClaims, key string) (string, error) {
	raw, ok := claims[key]
	if !ok {
		return "", ErrMissingClaims
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", ErrMissingClaims
	}
	return strings.TrimSpace(value), nil
}

func numericDateClaim(claims jwt.MapClaims, key string) (time.Time, error) {
	raw, ok := claims[key]
	if !ok {
		return time.Time{}, ErrMissingClaims
	}
	switch value := raw.(type) {
	case float64:
		return time.Unix(int64(value), 0), nil
	case int64:
		return time.Unix(value, 0), nil
	case int:
		return time.Unix(int64(value), 0), nil
	default:
		return time.Time{}, ErrMissingClaims
	}
}

func audienceMatches(raw any, expected string) bool {
	switch value := raw.(type) {
	case string:
		return value == expected
	case []any:
		for _, item := range value {
			if s, ok := item.(string); ok && s == expected {
				return true
			}
		}
	}
	return false
}

// ExtractBearerToken returns the bearer credential from an Authorization header.
func ExtractBearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", ErrInvalidToken
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", ErrInvalidToken
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" || strings.Contains(token, " ") {
		return "", ErrInvalidToken
	}
	return token, nil
}

// SanitizeAuthError returns a safe error message that never includes raw credentials.
func SanitizeAuthError(err error) string {
	switch {
	case errors.Is(err, ErrTokenExpired):
		return "bearer token expired"
	case errors.Is(err, ErrTokenNotYetValid):
		return "bearer token not yet valid"
	case errors.Is(err, ErrIssuerMismatch):
		return "jwt issuer mismatch"
	case errors.Is(err, ErrAudienceMismatch):
		return "jwt audience mismatch"
	case errors.Is(err, ErrUnsupportedAlg):
		return "unsupported jwt algorithm"
	case errors.Is(err, ErrJWKSFetchFailed):
		return "identity key verification unavailable"
	case errors.Is(err, ErrJWKSKeyNotFound):
		return "identity signing key not found"
	default:
		return "invalid bearer token"
	}
}

// RedactAuthorizationHeader ensures logs never include raw bearer credentials.
func RedactAuthorizationHeader(header string) string {
	if strings.TrimSpace(header) == "" {
		return ""
	}
	return "Bearer [REDACTED]"
}

func BuildJWTVerifier(ctx context.Context, issuer, audience, jwksURL string, httpClient *http.Client, cacheTTL, fetchTimeout time.Duration) (*JWTVerifier, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: fetchTimeout}
	}
	resolvedJWKSURL := strings.TrimSpace(jwksURL)
	if resolvedJWKSURL == "" {
		discovered, err := DiscoverJWKSURL(ctx, issuer, httpClient)
		if err != nil {
			return nil, err
		}
		resolvedJWKSURL = discovered
	}
	cache := NewJWKSCache(resolvedJWKSURL, httpClient, cacheTTL)
	return NewJWTVerifier(JWTVerifierConfig{
		Issuer:    issuer,
		Audience:  audience,
		JWKSCache: cache,
	})
}
