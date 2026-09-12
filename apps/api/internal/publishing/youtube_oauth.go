package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrOAuthConfiguration    = errors.New("invalid oauth configuration")
	ErrOAuthExchange         = errors.New("oauth token exchange failed")
	ErrOAuthRefresh          = errors.New("oauth token refresh failed")
	ErrOAuthRefreshReconnect = errors.New("oauth refresh credential rejected")
)

const (
	youtubeUploadScope        = "https://www.googleapis.com/auth/youtube.upload"
	youtubeReadonlyScope      = "https://www.googleapis.com/auth/youtube.readonly"
	youtubeAuthorizationScope = youtubeUploadScope + " " + youtubeReadonlyScope
)

type YouTubeOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
}

type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    time.Time
}

type YouTubeOAuth struct {
	config YouTubeOAuthConfig
	client *http.Client
	now    func() time.Time
}

func NewYouTubeOAuth(config YouTubeOAuthConfig, client *http.Client) (*YouTubeOAuth, error) {
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	config.RedirectURL = strings.TrimSpace(config.RedirectURL)
	if config.AuthURL == "" {
		config.AuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if config.TokenURL == "" {
		config.TokenURL = "https://oauth2.googleapis.com/token"
	}
	if config.ClientID == "" || config.ClientSecret == "" || config.RedirectURL == "" {
		return nil, ErrOAuthConfiguration
	}
	if _, err := url.ParseRequestURI(config.RedirectURL); err != nil {
		return nil, ErrOAuthConfiguration
	}
	if _, err := url.ParseRequestURI(config.AuthURL); err != nil {
		return nil, ErrOAuthConfiguration
	}
	if _, err := url.ParseRequestURI(config.TokenURL); err != nil {
		return nil, ErrOAuthConfiguration
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &YouTubeOAuth{config: config, client: client, now: time.Now}, nil
}

func (o *YouTubeOAuth) AuthorizationURL(state string) (string, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		return "", ErrOAuthConfiguration
	}
	u, err := url.Parse(o.config.AuthURL)
	if err != nil {
		return "", ErrOAuthConfiguration
	}
	q := u.Query()
	q.Set("client_id", o.config.ClientID)
	q.Set("redirect_uri", o.config.RedirectURL)
	q.Set("response_type", "code")
	// youtube.upload is sufficient for videos.insert but does not authorize the
	// channels.list(mine=true) identity read required to persist the canonical
	// destination. youtube.readonly is the narrow read scope used only for that
	// channel identity/capability lookup; no management scope is requested.
	q.Set("scope", youtubeAuthorizationScope)
	q.Set("access_type", "offline")
	q.Set("include_granted_scopes", "true")
	q.Set("prompt", "consent")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (o *YouTubeOAuth) ExchangeCode(ctx context.Context, code string) (OAuthToken, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return OAuthToken{}, ErrOAuthExchange
	}
	values := url.Values{
		"client_id":     {o.config.ClientID},
		"client_secret": {o.config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {o.config.RedirectURL},
	}
	return o.requestToken(ctx, values, ErrOAuthExchange)
}

func (o *YouTubeOAuth) Refresh(ctx context.Context, refreshToken string) (OAuthToken, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return OAuthToken{}, ErrOAuthRefresh
	}
	values := url.Values{
		"client_id":     {o.config.ClientID},
		"client_secret": {o.config.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	token, err := o.requestToken(ctx, values, ErrOAuthRefresh)
	if err != nil {
		return OAuthToken{}, err
	}
	// Google commonly omits refresh_token on refresh. Keep the caller's durable token.
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

func (o *YouTubeOAuth) requestToken(ctx context.Context, values url.Values, sentinel error) (OAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.config.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return OAuthToken{}, fmt.Errorf("%w: build request", sentinel)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.client.Do(req)
	if err != nil {
		return OAuthToken{}, fmt.Errorf("%w: transport", sentinel)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return OAuthToken{}, fmt.Errorf("%w: read response", sentinel)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Google returns invalid_grant when a refresh credential was revoked,
		// expired, or otherwise cannot be used anymore. Classify only that stable
		// recovery signal; never surface provider payload/token details.
		if errors.Is(sentinel, ErrOAuthRefresh) {
			var providerError struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(body, &providerError) == nil && strings.EqualFold(strings.TrimSpace(providerError.Error), "invalid_grant") {
				return OAuthToken{}, fmt.Errorf("%w: %w", sentinel, ErrOAuthRefreshReconnect)
			}
		}
		return OAuthToken{}, fmt.Errorf("%w: provider status %d", sentinel, resp.StatusCode)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return OAuthToken{}, fmt.Errorf("%w: invalid response", sentinel)
	}
	payload.AccessToken = strings.TrimSpace(payload.AccessToken)
	if payload.AccessToken == "" || payload.ExpiresIn <= 0 {
		return OAuthToken{}, fmt.Errorf("%w: incomplete response", sentinel)
	}
	return OAuthToken{
		AccessToken:  payload.AccessToken,
		RefreshToken: strings.TrimSpace(payload.RefreshToken),
		TokenType:    strings.TrimSpace(payload.TokenType),
		ExpiresAt:    o.now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}
