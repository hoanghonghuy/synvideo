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
	ErrYouTubeStatusConfiguration = errors.New("invalid youtube status configuration")
	ErrYouTubeStatusRequest       = errors.New("youtube status request failed")
	ErrYouTubeStatusProtocol      = errors.New("invalid youtube status response")
)

const defaultYouTubeStatusURL = "https://www.googleapis.com/youtube/v3/videos"

type RemoteStatusFailure string

const (
	RemoteStatusFailureNone              RemoteStatusFailure = ""
	RemoteStatusFailureRetryable         RemoteStatusFailure = "retryable"
	RemoteStatusFailureReconnectRequired RemoteStatusFailure = "reconnect_required"
	RemoteStatusFailureRejected          RemoteStatusFailure = "rejected"
)

type YouTubeRemoteStatus struct {
	ProcessingStatus string
	PrivacyStatus    string
	PublishAt        *time.Time
	Failure          RemoteStatusFailure
	ProviderCode     int
}

type YouTubeStatusClient struct {
	endpoint string
	client   *http.Client
}

func NewYouTubeStatusClient(endpoint string, client *http.Client) (*YouTubeStatusClient, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultYouTubeStatusURL
	}
	u, err := url.ParseRequestURI(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, ErrYouTubeStatusConfiguration
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &YouTubeStatusClient{endpoint: endpoint, client: client}, nil
}

func (c *YouTubeStatusClient) Get(ctx context.Context, accessToken, remoteVideoID string) (YouTubeRemoteStatus, error) {
	accessToken = strings.TrimSpace(accessToken)
	remoteVideoID = strings.TrimSpace(remoteVideoID)
	if accessToken == "" || remoteVideoID == "" {
		return YouTubeRemoteStatus{}, ErrYouTubeStatusRequest
	}

	u, err := url.Parse(c.endpoint)
	if err != nil {
		return YouTubeRemoteStatus{}, ErrYouTubeStatusConfiguration
	}
	q := u.Query()
	q.Set("part", "status,processingDetails")
	q.Set("id", remoteVideoID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return YouTubeRemoteStatus{}, fmt.Errorf("%w: build request", ErrYouTubeStatusRequest)
	}
	setBearer(req, accessToken)
	resp, err := c.client.Do(req)
	if err != nil {
		return YouTubeRemoteStatus{Failure: RemoteStatusFailureRetryable}, fmt.Errorf("%w: transport", ErrYouTubeStatusRequest)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classifyRemoteStatusFailure(resp.StatusCode), nil
	}

	var payload struct {
		Items []struct {
			ID     string `json:"id"`
			Status struct {
				PrivacyStatus string `json:"privacyStatus"`
				PublishAt     string `json:"publishAt"`
			} `json:"status"`
			ProcessingDetails struct {
				ProcessingStatus string `json:"processingStatus"`
			} `json:"processingDetails"`
		} `json:"items"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return YouTubeRemoteStatus{}, fmt.Errorf("%w: decode response", ErrYouTubeStatusProtocol)
	}
	if len(payload.Items) != 1 || strings.TrimSpace(payload.Items[0].ID) != remoteVideoID {
		return YouTubeRemoteStatus{Failure: RemoteStatusFailureRejected, ProviderCode: http.StatusNotFound}, nil
	}

	item := payload.Items[0]
	result := YouTubeRemoteStatus{
		ProcessingStatus: strings.TrimSpace(item.ProcessingDetails.ProcessingStatus),
		PrivacyStatus:    strings.TrimSpace(item.Status.PrivacyStatus),
		ProviderCode:     resp.StatusCode,
	}
	if value := strings.TrimSpace(item.Status.PublishAt); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return YouTubeRemoteStatus{}, fmt.Errorf("%w: invalid publishAt", ErrYouTubeStatusProtocol)
		}
		parsed = parsed.UTC()
		result.PublishAt = &parsed
	}
	if result.ProcessingStatus == "" || result.PrivacyStatus == "" {
		return YouTubeRemoteStatus{}, fmt.Errorf("%w: incomplete response", ErrYouTubeStatusProtocol)
	}
	return result, nil
}

func classifyRemoteStatusFailure(statusCode int) YouTubeRemoteStatus {
	result := YouTubeRemoteStatus{Failure: RemoteStatusFailureRejected, ProviderCode: statusCode}
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		result.Failure = RemoteStatusFailureReconnectRequired
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		result.Failure = RemoteStatusFailureRetryable
	}
	return result
}
