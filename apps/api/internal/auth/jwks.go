package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	ErrJWKSFetchFailed = errors.New("jwks fetch failed")
	ErrJWKSKeyNotFound = errors.New("jwks key not found")
)

type jwkSet struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type cachedKey struct {
	key       crypto.PublicKey
	expiresAt time.Time
}

// JWKSCache fetches and caches JWKS keys with bounded refresh.
type JWKSCache struct {
	jwksURL        string
	httpClient     *http.Client
	cacheTTL       time.Duration
	refreshBackoff time.Duration
	now            func() time.Time
	mu             sync.RWMutex
	keys           map[string]cachedKey
	setExpires     time.Time
	nextRefreshAt  time.Time
	refreshGroup   singleflight.Group
}

// SetNowFunc overrides the cache clock. Intended for deterministic tests.
func (c *JWKSCache) SetNowFunc(now func() time.Time) {
	c.now = now
}

func (c *JWKSCache) nowTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func NewJWKSCache(jwksURL string, httpClient *http.Client, cacheTTL time.Duration) *JWKSCache {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	backoff := cacheTTL
	if backoff <= 0 {
		backoff = time.Minute
	}
	return &JWKSCache{
		jwksURL:        strings.TrimSpace(jwksURL),
		httpClient:     httpClient,
		cacheTTL:       cacheTTL,
		refreshBackoff: backoff,
		keys:           make(map[string]cachedKey),
	}
}

func (c *JWKSCache) Key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	if kid == "" {
		return nil, ErrJWKSKeyNotFound
	}

	now := c.nowTime()
	c.mu.RLock()
	entry, hasKey := c.keys[kid]
	snapshotFresh := c.snapshotFresh(now)
	c.mu.RUnlock()

	if hasKey && now.Before(entry.expiresAt) {
		return entry.key, nil
	}
	if snapshotFresh {
		return nil, ErrJWKSKeyNotFound
	}

	if err := c.ensureSnapshot(ctx, now); err != nil {
		return nil, err
	}

	now = c.nowTime()
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, hasKey = c.keys[kid]
	if !hasKey || now.After(entry.expiresAt) {
		return nil, ErrJWKSKeyNotFound
	}
	return entry.key, nil
}

func (c *JWKSCache) snapshotFresh(now time.Time) bool {
	return !c.setExpires.IsZero() && now.Before(c.setExpires)
}

func (c *JWKSCache) ensureSnapshot(ctx context.Context, now time.Time) error {
	c.mu.RLock()
	if c.snapshotFresh(now) {
		c.mu.RUnlock()
		return nil
	}
	if !c.nextRefreshAt.IsZero() && now.Before(c.nextRefreshAt) {
		c.mu.RUnlock()
		return ErrJWKSFetchFailed
	}
	c.mu.RUnlock()

	_, err, _ := c.refreshGroup.Do("jwks", func() (any, error) {
		now := c.nowTime()
		c.mu.RLock()
		snapshotFresh := c.snapshotFresh(now)
		throttled := !c.nextRefreshAt.IsZero() && now.Before(c.nextRefreshAt)
		c.mu.RUnlock()
		if snapshotFresh {
			return nil, nil
		}
		if throttled {
			return nil, ErrJWKSFetchFailed
		}
		if err := c.refresh(ctx); err != nil {
			c.mu.Lock()
			c.nextRefreshAt = c.nowTime().Add(c.refreshBackoff)
			c.mu.Unlock()
			return nil, err
		}
		return nil, nil
	})
	return err
}

func (c *JWKSCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: unexpected status %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}

	var set jwkSet
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("%w: invalid jwks json", ErrJWKSFetchFailed)
	}

	keys := make(map[string]cachedKey, len(set.Keys))
	expiresAt := c.nowTime().Add(c.cacheTTL)
	for _, key := range set.Keys {
		if strings.ToUpper(key.Kty) != "RSA" || key.Kid == "" {
			continue
		}
		pub, err := parseRSAPublicKey(key)
		if err != nil {
			continue
		}
		keys[key.Kid] = cachedKey{key: pub, expiresAt: expiresAt}
	}
	if len(keys) == 0 {
		return fmt.Errorf("%w: no usable keys", ErrJWKSFetchFailed)
	}

	c.mu.Lock()
	c.keys = keys
	c.setExpires = expiresAt
	c.nextRefreshAt = time.Time{}
	c.mu.Unlock()
	return nil
}

func parseRSAPublicKey(key jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if n.Sign() <= 0 || e <= 0 {
		return nil, errors.New("invalid rsa key material")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

// DiscoverJWKSURL resolves the JWKS URI from an OIDC issuer discovery document.
func DiscoverJWKSURL(ctx context.Context, issuer string, httpClient *http.Client) (string, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	discoveryURL := issuer + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: discovery status %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrJWKSFetchFailed, err)
	}
	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || strings.TrimSpace(doc.JWKSURI) == "" {
		return "", fmt.Errorf("%w: invalid discovery document", ErrJWKSFetchFailed)
	}
	return strings.TrimSpace(doc.JWKSURI), nil
}

// ParseRSAPublicKeyFromCertificate is a test helper for certificate-based fixtures.
func ParseRSAPublicKeyFromCertificate(der []byte) (*rsa.PublicKey, error) {
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("certificate does not contain rsa public key")
	}
	return pub, nil
}
