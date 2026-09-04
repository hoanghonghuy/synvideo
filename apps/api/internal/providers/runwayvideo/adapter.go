package runwayvideo

import (
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

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

const (
	APIVersion               = "2024-11-06"
	defaultBaseURL           = "https://api.dev.runwayml.com"
	defaultTimeout           = 30 * time.Second
	defaultMaxResponseBytes  = int64(2 << 20)
	maxTaskFailureDetailSize = 512
	MinDurationSeconds       = 2
	MaxDurationSeconds       = 10
)

type SecretSource interface {
	Get(context.Context) (string, error)
}

type SecretSourceFunc func(context.Context) (string, error)

func (f SecretSourceFunc) Get(ctx context.Context) (string, error) {
	return f(ctx)
}

type ModelConfig struct {
	ID              providers.ModelID
	ExternalModelID string
}

type Config struct {
	ProviderID       providers.ProviderID
	BaseURL          string
	CredentialSource SecretSource
	Model            ModelConfig
	Timeout          time.Duration
	MaxResponseBytes int64
	HTTPClient       *http.Client
}

type Adapter struct {
	providerID       providers.ProviderID
	baseURL          string
	credentialSource SecretSource
	model            ModelConfig
	client           *http.Client
	maxResponseBytes int64
}

func New(cfg Config) (*Adapter, error) {
	if !cfg.ProviderID.Valid() {
		return nil, errors.New("invalid provider id")
	}
	if !cfg.Model.ID.Valid() || strings.TrimSpace(cfg.Model.ExternalModelID) == "" {
		return nil, errors.New("invalid video model")
	}
	if cfg.CredentialSource == nil {
		return nil, errors.New("credential source is required")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid Runway base URL")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	maxResponseBytes := cfg.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}

	return &Adapter{
		providerID:       cfg.ProviderID,
		baseURL:          baseURL,
		credentialSource: cfg.CredentialSource,
		model:            cfg.Model,
		client:           client,
		maxResponseBytes: maxResponseBytes,
	}, nil
}

func (a *Adapter) StartVideo(ctx context.Context, request providers.VideoGenerationRequest) (providers.VideoOperation, error) {
	if err := request.Validate(); err != nil {
		return providers.VideoOperation{}, err
	}
	if request.DurationSeconds != nil && (*request.DurationSeconds < MinDurationSeconds || *request.DurationSeconds > MaxDurationSeconds) {
		return providers.VideoOperation{}, providers.NewInvalidRequestError(fmt.Errorf("Runway Gen-4.5 duration must be between %d and %d seconds", MinDurationSeconds, MaxDurationSeconds))
	}
	if request.ReferenceImage != nil {
		return providers.VideoOperation{}, providers.NewInvalidRequestError(errors.New("Runway V1 text-to-video adapter does not accept reference images"))
	}
	ratio, err := mapAspectRatio(request.AspectRatio)
	if err != nil {
		return providers.VideoOperation{}, providers.NewInvalidRequestError(err)
	}

	body := map[string]any{
		"model":      a.model.ExternalModelID,
		"promptText": request.Prompt,
		"ratio":      ratio,
	}
	if request.DurationSeconds != nil {
		body["duration"] = *request.DurationSeconds
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return providers.VideoOperation{}, providers.NewInvalidRequestError(err)
	}

	var response struct {
		ID string `json:"id"`
	}
	if err := a.doJSON(ctx, http.MethodPost, "/v1/image_to_video", payload, &response, true); err != nil {
		return providers.VideoOperation{}, err
	}
	operation := providers.VideoOperation{ID: response.ID, State: providers.VideoOperationQueued}
	if err := operation.Validate(); err != nil {
		return providers.VideoOperation{}, err
	}
	return operation, nil
}

func (a *Adapter) GetVideoOperation(ctx context.Context, operationID string) (providers.VideoOperation, error) {
	task, err := a.getTask(ctx, operationID)
	if err != nil {
		return providers.VideoOperation{}, err
	}
	return task.operation()
}

func (a *Adapter) OpenVideoResult(ctx context.Context, operationID string) (providers.GeneratedBinary, error) {
	task, err := a.getTask(ctx, operationID)
	if err != nil {
		return nil, err
	}
	operation, err := task.operation()
	if err != nil {
		return nil, err
	}
	if operation.State != providers.VideoOperationSucceeded || len(task.Output) == 0 || strings.TrimSpace(task.Output[0]) == "" {
		return nil, providers.NewResultUnavailableError(errors.New("Runway task output is unavailable"))
	}
	outputURL, err := url.Parse(task.Output[0])
	if err != nil || outputURL.Scheme != "https" && outputURL.Scheme != "http" || outputURL.Host == "" {
		return nil, providers.NewMalformedResponseError(errors.New("Runway task output URL is invalid"))
	}
	return &remoteBinary{client: a.client, url: outputURL.String(), mime: "video/mp4"}, nil
}

type runwayTask struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	Output      []string  `json:"output"`
	Failure     string    `json:"failure"`
	FailureCode string    `json:"failureCode"`
}

func (t runwayTask) operation() (providers.VideoOperation, error) {
	state := ""
	failureCategory := providers.Category("")
	switch strings.ToUpper(strings.TrimSpace(t.Status)) {
	case "PENDING", "THROTTLED":
		state = providers.VideoOperationQueued
	case "RUNNING":
		state = providers.VideoOperationRunning
	case "SUCCEEDED":
		state = providers.VideoOperationSucceeded
	case "FAILED", "CANCELED":
		state = providers.VideoOperationFailed
		failureCategory = categoryForFailureCode(t.FailureCode)
	default:
		return providers.VideoOperation{}, providers.NewMalformedResponseError(fmt.Errorf("unknown Runway task status %q", t.Status))
	}
	operation := providers.VideoOperation{
		ID:              t.ID,
		State:           state,
		FailureCategory: failureCategory,
	}
	if !t.CreatedAt.IsZero() {
		createdAt := t.CreatedAt
		operation.CreatedAt = &createdAt
	}
	if err := operation.Validate(); err != nil {
		return providers.VideoOperation{}, err
	}
	return operation, nil
}

func categoryForFailureCode(code string) providers.Category {
	upper := strings.ToUpper(strings.TrimSpace(code))
	switch {
	case strings.HasPrefix(upper, "SAFETY."), strings.HasPrefix(upper, "INPUT_PREPROCESSING.SAFETY"), strings.HasPrefix(upper, "ASSET.INVALID"):
		return providers.CategoryInvalidRequest
	case strings.HasPrefix(upper, "THIRD_PARTY.UNAVAILABLE"), upper == "INTERNAL", strings.HasPrefix(upper, "INPUT_PREPROCESSING.INTERNAL"):
		return providers.CategoryTransientExecution
	default:
		return providers.CategoryVideoOperationFailed
	}
}

func (a *Adapter) getTask(ctx context.Context, operationID string) (runwayTask, error) {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" || strings.Contains(operationID, "/") {
		return runwayTask{}, providers.NewUnknownVideoOperationError(errors.New("invalid Runway task id"))
	}
	var task runwayTask
	if err := a.doJSON(ctx, http.MethodGet, "/v1/tasks/"+url.PathEscape(operationID), nil, &task, false); err != nil {
		return runwayTask{}, err
	}
	if task.ID != operationID {
		return runwayTask{}, providers.NewMalformedResponseError(errors.New("Runway task identity mismatch"))
	}
	return task, nil
}

func (a *Adapter) doJSON(ctx context.Context, method, path string, body []byte, output any, submit bool) error {
	credential, err := a.credentialSource.Get(ctx)
	if err != nil || strings.TrimSpace(credential) == "" {
		return providers.NewAuthConfigError(errors.New("Runway credential unavailable"))
	}
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, reader)
	if err != nil {
		return providers.NewInvalidRequestError(err)
	}
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set("X-Runway-Version", APIVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if submit {
			return providers.NewAmbiguousSubmitError(err)
		}
		return providers.NewTransientError(err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, a.maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return providers.NewTransientError(err)
	}
	if int64(len(data)) > a.maxResponseBytes {
		return providers.NewMalformedResponseError(errors.New("Runway response exceeded size limit"))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := classifyHTTPError(resp.StatusCode, data)
		if submit && resp.StatusCode >= 500 {
			return providers.NewAmbiguousSubmitError(httpErr)
		}
		return httpErr
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return providers.NewMalformedResponseError(err)
	}
	return nil
}

func classifyHTTPError(status int, body []byte) error {
	safeDetail := strings.TrimSpace(string(body))
	if len(safeDetail) > maxTaskFailureDetailSize {
		safeDetail = safeDetail[:maxTaskFailureDetailSize]
	}
	cause := fmt.Errorf("Runway HTTP %d", status)
	if safeDetail != "" {
		cause = fmt.Errorf("Runway HTTP %d response length %d", status, len(body))
	}
	switch {
	case status == http.StatusTooManyRequests:
		return providers.NewRateLimitedError(cause)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return providers.NewAuthConfigError(cause)
	case status == http.StatusNotFound:
		return providers.NewUnknownVideoOperationError(cause)
	case status >= 400 && status < 500:
		return providers.NewInvalidRequestError(cause)
	case status >= 500:
		return providers.NewTransientError(cause)
	default:
		return providers.NewTransientError(cause)
	}
}

func mapAspectRatio(aspectRatio string) (string, error) {
	switch strings.TrimSpace(aspectRatio) {
	case "", "16:9":
		return "1280:720", nil
	case "9:16":
		return "720:1280", nil
	default:
		return "", fmt.Errorf("Runway Gen-4.5 text-to-video does not support aspect ratio %q", aspectRatio)
	}
}

type remoteBinary struct {
	client *http.Client
	url    string
	mime   string
}

func (b *remoteBinary) MIMEType() string {
	return b.mime
}

func (b *remoteBinary) Size() int64 {
	return -1
}

func (b *remoteBinary) Open(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url, nil)
	if err != nil {
		return nil, providers.NewResultUnavailableError(err)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, providers.NewTransientError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, providers.NewResultUnavailableError(fmt.Errorf("Runway output HTTP %d", resp.StatusCode))
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if contentType != "" && contentType != "video/mp4" && contentType != "application/octet-stream" {
		resp.Body.Close()
		return nil, providers.NewMalformedResponseError(fmt.Errorf("unexpected Runway output content type %q", contentType))
	}
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, parseErr := strconv.ParseInt(contentLength, 10, 64); parseErr == nil && size < 0 {
			resp.Body.Close()
			return nil, providers.NewMalformedResponseError(errors.New("invalid Runway output content length"))
		}
	}
	return resp.Body, nil
}
