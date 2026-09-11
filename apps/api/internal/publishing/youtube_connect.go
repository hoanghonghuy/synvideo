package publishing

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOAuthState     = errors.New("invalid oauth state")
	ErrYouTubeChannel = errors.New("youtube channel lookup failed")
)

type youtubeStatePayload struct {
	OwnerID   uuid.UUID `json:"owner_id"`
	ProjectID uuid.UUID `json:"project_id"`
	ExpiresAt int64     `json:"expires_at"`
	Nonce     string    `json:"nonce"`
}

type YouTubeConnectService struct {
	oauth       *YouTubeOAuth
	connections *ConnectionService
	client      *http.Client
	stateSecret []byte
	returnBase  *url.URL
	now         func() time.Time
}

func NewYouTubeConnectService(oauth *YouTubeOAuth, connections *ConnectionService, stateSecret, returnBase string, client *http.Client) (*YouTubeConnectService, error) {
	stateSecret = strings.TrimSpace(stateSecret)
	returnBase = strings.TrimSpace(returnBase)
	if oauth == nil || connections == nil || len(stateSecret) < 32 || returnBase == "" {
		return nil, ErrOAuthConfiguration
	}
	parsed, err := url.Parse(returnBase)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, ErrOAuthConfiguration
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &YouTubeConnectService{oauth: oauth, connections: connections, client: client, stateSecret: []byte(stateSecret), returnBase: parsed, now: time.Now}, nil
}

func (s *YouTubeConnectService) Start(ownerID, projectID uuid.UUID) (string, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil {
		return "", ErrOAuthState
	}
	nonce := make([]byte, 18)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("%w: nonce", ErrOAuthState)
	}
	payload := youtubeStatePayload{
		OwnerID: ownerID, ProjectID: projectID,
		ExpiresAt: s.now().UTC().Add(10 * time.Minute).Unix(),
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
	}
	state, err := s.signState(payload)
	if err != nil {
		return "", err
	}
	return s.oauth.AuthorizationURL(state)
}

func (s *YouTubeConnectService) ProjectFromState(rawState string) (uuid.UUID, error) {
	payload, err := s.verifyState(rawState)
	if err != nil {
		return uuid.Nil, err
	}
	return payload.ProjectID, nil
}

func (s *YouTubeConnectService) Complete(ctx context.Context, rawState, code string) (ChannelConnection, uuid.UUID, error) {
	payload, err := s.verifyState(rawState)
	if err != nil {
		return ChannelConnection{}, uuid.Nil, err
	}
	token, err := s.oauth.ExchangeCode(ctx, code)
	if err != nil {
		return ChannelConnection{}, uuid.Nil, err
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return ChannelConnection{}, uuid.Nil, fmt.Errorf("%w: provider did not return refresh token", ErrOAuthExchange)
	}
	remoteID, displayName, err := s.lookupChannel(ctx, token.AccessToken)
	if err != nil {
		return ChannelConnection{}, uuid.Nil, err
	}
	now := s.now().UTC()
	connection := ChannelConnection{
		ID: uuid.New(), OwnerID: payload.OwnerID, Provider: ProviderYouTube,
		RemoteChannelID: remoteID, DisplayName: displayName, State: ConnectionConnected,
		Capabilities: Capabilities{CanUpload: true, CanPublish: true, CanSchedule: true},
		CreatedAt:    now, UpdatedAt: now,
	}
	saved, err := s.connections.SaveConnectedChannel(ctx, connection, token.RefreshToken)
	if err != nil {
		return ChannelConnection{}, uuid.Nil, err
	}
	return saved, payload.ProjectID, nil
}

func (s *YouTubeConnectService) ReturnURL(projectID uuid.UUID, status string) string {
	u := *s.returnBase
	basePath := strings.TrimRight(u.Path, "/")
	u.Path = basePath + "/projects/" + projectID.String() + "/publishing"
	q := u.Query()
	q.Set("youtube", status)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *YouTubeConnectService) signState(payload youtubeStatePayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("%w: encode", ErrOAuthState)
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, s.stateSecret)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func (s *YouTubeConnectService) verifyState(raw string) (youtubeStatePayload, error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 2 {
		return youtubeStatePayload{}, ErrOAuthState
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return youtubeStatePayload{}, ErrOAuthState
	}
	mac := hmac.New(sha256.New, s.stateSecret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return youtubeStatePayload{}, ErrOAuthState
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return youtubeStatePayload{}, ErrOAuthState
	}
	var payload youtubeStatePayload
	if err := json.Unmarshal(body, &payload); err != nil || payload.OwnerID == uuid.Nil || payload.ProjectID == uuid.Nil || strings.TrimSpace(payload.Nonce) == "" {
		return youtubeStatePayload{}, ErrOAuthState
	}
	now := s.now().UTC().Unix()
	if payload.ExpiresAt < now || payload.ExpiresAt > now+11*60 {
		return youtubeStatePayload{}, ErrOAuthState
	}
	return payload, nil
}

func (s *YouTubeConnectService) lookupChannel(ctx context.Context, accessToken string) (string, string, error) {
	u, _ := url.Parse("https://www.googleapis.com/youtube/v3/channels")
	q := u.Query()
	q.Set("part", "id,snippet")
	q.Set("mine", "true")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", ErrYouTubeChannel
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: transport", ErrYouTubeChannel)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("%w: provider status %d", ErrYouTubeChannel, resp.StatusCode)
	}
	var payload struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil || len(payload.Items) != 1 {
		return "", "", ErrYouTubeChannel
	}
	remoteID := strings.TrimSpace(payload.Items[0].ID)
	displayName := strings.TrimSpace(payload.Items[0].Snippet.Title)
	if remoteID == "" || displayName == "" {
		return "", "", ErrYouTubeChannel
	}
	return remoteID, displayName, nil
}
