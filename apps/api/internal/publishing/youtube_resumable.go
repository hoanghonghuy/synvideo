package publishing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrResumableConfiguration = errors.New("invalid resumable upload configuration")
	ErrResumableRequest       = errors.New("resumable upload request failed")
	ErrResumableProtocol      = errors.New("invalid resumable upload response")
)

const defaultYouTubeResumableURL = "https://www.googleapis.com/upload/youtube/v3/videos"

type UploadFailure string

const (
	UploadFailureNone              UploadFailure = ""
	UploadFailureRetryable         UploadFailure = "retryable"
	UploadFailureReconnectRequired UploadFailure = "reconnect_required"
	UploadFailureSessionExpired    UploadFailure = "session_expired"
	UploadFailureRejected          UploadFailure = "rejected"
)

type YouTubeUploadMetadata struct {
	Title       string
	Description string
}

type ResumableUploadResult struct {
	SessionURI    string
	UploadedBytes int64
	RemoteVideoID string
	Complete      bool
	Failure       UploadFailure
	RetryAfter    time.Duration
	ProviderCode  int
}

type YouTubeResumableClient struct {
	initiateURL string
	client      *http.Client
}

func NewYouTubeResumableClient(initiateURL string, client *http.Client) (*YouTubeResumableClient, error) {
	initiateURL = strings.TrimSpace(initiateURL)
	if initiateURL == "" {
		initiateURL = defaultYouTubeResumableURL
	}
	u, err := url.ParseRequestURI(initiateURL)
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
		return nil, ErrResumableConfiguration
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &YouTubeResumableClient{initiateURL: initiateURL, client: client}, nil
}

func (c *YouTubeResumableClient) Initiate(ctx context.Context, accessToken string, metadata YouTubeUploadMetadata, contentType string, totalBytes int64) (ResumableUploadResult, error) {
	accessToken = strings.TrimSpace(accessToken)
	metadata.Title = strings.TrimSpace(metadata.Title)
	contentType = strings.TrimSpace(contentType)
	if accessToken == "" || metadata.Title == "" || contentType == "" || totalBytes <= 0 {
		return ResumableUploadResult{}, ErrResumableRequest
	}

	body, err := json.Marshal(struct {
		Snippet struct {
			Title       string `json:"title"`
			Description string `json:"description,omitempty"`
		} `json:"snippet"`
		Status struct {
			PrivacyStatus string `json:"privacyStatus"`
		} `json:"status"`
	}{
		Snippet: struct {
			Title       string `json:"title"`
			Description string `json:"description,omitempty"`
		}{Title: metadata.Title, Description: metadata.Description},
		Status: struct {
			PrivacyStatus string `json:"privacyStatus"`
		}{PrivacyStatus: "private"},
	})
	if err != nil {
		return ResumableUploadResult{}, fmt.Errorf("%w: encode metadata", ErrResumableRequest)
	}

	u, err := url.Parse(c.initiateURL)
	if err != nil {
		return ResumableUploadResult{}, ErrResumableConfiguration
	}
	q := u.Query()
	q.Set("uploadType", "resumable")
	q.Set("part", "snippet,status")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return ResumableUploadResult{}, fmt.Errorf("%w: build initiation request", ErrResumableRequest)
	}
	setBearer(req, accessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(totalBytes, 10))
	req.Header.Set("X-Upload-Content-Type", contentType)

	resp, err := c.client.Do(req)
	if err != nil {
		return ResumableUploadResult{Failure: UploadFailureRetryable}, fmt.Errorf("%w: initiation transport", ErrResumableRequest)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return classifyUploadFailure(resp), nil
	}
	sessionURI := strings.TrimSpace(resp.Header.Get("Location"))
	if !validSessionURI(sessionURI) {
		return ResumableUploadResult{}, fmt.Errorf("%w: missing session URI", ErrResumableProtocol)
	}
	return ResumableUploadResult{SessionURI: sessionURI}, nil
}

func (c *YouTubeResumableClient) Query(ctx context.Context, accessToken, sessionURI string, totalBytes int64) (ResumableUploadResult, error) {
	if strings.TrimSpace(accessToken) == "" || !validSessionURI(sessionURI) || totalBytes <= 0 {
		return ResumableUploadResult{}, ErrResumableRequest
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURI, http.NoBody)
	if err != nil {
		return ResumableUploadResult{}, fmt.Errorf("%w: build status request", ErrResumableRequest)
	}
	setBearer(req, accessToken)
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", totalBytes))
	return c.doResumable(req, sessionURI, totalBytes)
}

func (c *YouTubeResumableClient) UploadChunk(ctx context.Context, accessToken, sessionURI, contentType string, body io.Reader, start, length, totalBytes int64) (ResumableUploadResult, error) {
	contentType = strings.TrimSpace(contentType)
	if strings.TrimSpace(accessToken) == "" || !validSessionURI(sessionURI) || contentType == "" || body == nil || start < 0 || length <= 0 || totalBytes <= 0 || start+length > totalBytes {
		return ResumableUploadResult{}, ErrResumableRequest
	}
	end := start + length - 1
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURI, io.LimitReader(body, length))
	if err != nil {
		return ResumableUploadResult{}, fmt.Errorf("%w: build chunk request", ErrResumableRequest)
	}
	setBearer(req, accessToken)
	req.ContentLength = length
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, totalBytes))
	return c.doResumable(req, sessionURI, totalBytes)
}

func (c *YouTubeResumableClient) doResumable(req *http.Request, sessionURI string, totalBytes int64) (ResumableUploadResult, error) {
	client := *c.client
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		return ResumableUploadResult{SessionURI: sessionURI, Failure: UploadFailureRetryable}, fmt.Errorf("%w: transport", ErrResumableRequest)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 308 {
		uploaded, err := uploadedBytes(resp.Header.Get("Range"), totalBytes)
		if err != nil {
			return ResumableUploadResult{}, err
		}
		result := ResumableUploadResult{
			SessionURI:    updatedSessionURI(sessionURI, resp.Header.Get("Location")),
			UploadedBytes: uploaded,
			RetryAfter:    parseRetryAfter(resp.Header.Get("Retry-After")),
			ProviderCode:  resp.StatusCode,
		}
		return result, nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		videoID, err := decodeVideoID(resp.Body)
		if err != nil {
			return ResumableUploadResult{}, err
		}
		return ResumableUploadResult{
			SessionURI:    updatedSessionURI(sessionURI, resp.Header.Get("Location")),
			UploadedBytes: totalBytes,
			RemoteVideoID: videoID,
			Complete:      true,
			ProviderCode:  resp.StatusCode,
		}, nil
	}
	result := classifyUploadFailure(resp)
	result.SessionURI = sessionURI
	return result, nil
}

func classifyUploadFailure(resp *http.Response) ResumableUploadResult {
	result := ResumableUploadResult{
		Failure:      UploadFailureRejected,
		RetryAfter:   parseRetryAfter(resp.Header.Get("Retry-After")),
		ProviderCode: resp.StatusCode,
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		result.Failure = UploadFailureReconnectRequired
	case http.StatusNotFound, http.StatusGone:
		result.Failure = UploadFailureSessionExpired
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		result.Failure = UploadFailureRetryable
	}
	return result
}

func uploadedBytes(value string, totalBytes int64) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if !strings.HasPrefix(value, "bytes=") {
		return 0, fmt.Errorf("%w: invalid range", ErrResumableProtocol)
	}
	bounds := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
	if len(bounds) != 2 || bounds[0] != "0" {
		return 0, fmt.Errorf("%w: invalid range", ErrResumableProtocol)
	}
	last, err := strconv.ParseInt(bounds[1], 10, 64)
	if err != nil || last < 0 || last >= totalBytes {
		return 0, fmt.Errorf("%w: invalid range", ErrResumableProtocol)
	}
	return last + 1, nil
}

func decodeVideoID(body io.Reader) (string, error) {
	var payload struct {
		ID string `json:"id"`
	}
	decoder := json.NewDecoder(io.LimitReader(body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return "", fmt.Errorf("%w: invalid completion response", ErrResumableProtocol)
	}
	payload.ID = strings.TrimSpace(payload.ID)
	if payload.ID == "" {
		return "", fmt.Errorf("%w: missing video id", ErrResumableProtocol)
	}
	return payload.ID, nil
}

func validSessionURI(value string) bool {
	u, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func updatedSessionURI(current, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if validSessionURI(candidate) {
		return candidate
	}
	return current
}

func setBearer(req *http.Request, accessToken string) {
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
