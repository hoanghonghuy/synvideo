package openaiimage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

var (
	errTransport     = errors.New("upstream provider transport failure")
	errResponseBody  = errors.New("upstream provider response exceeded configured limit")
	errMalformedBody = errors.New("upstream provider response was malformed")
	errInvalidImage  = errors.New("upstream provider returned invalid image data")
)

type modelGenerator struct {
	adapter *Adapter
	model   modelConfig
}

type generationRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size,omitempty"`
}

type imagesResponse struct {
	ID    string         `json:"id"`
	Data  []imageData    `json:"data"`
	Usage *responseUsage `json:"usage"`
}

type imageData struct {
	B64JSON string `json:"b64_json"`
	URL     string `json:"url"`
}

type responseUsage struct {
	InputTokens      *int `json:"input_tokens"`
	OutputTokens     *int `json:"output_tokens"`
	TotalTokens      *int `json:"total_tokens"`
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
}

// GenerateImage sends one bounded, context-aware Images API request.
func (g *modelGenerator) GenerateImage(ctx context.Context, req providers.ImageGenerationRequest) (providers.ImageGenerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.ImageGenerationResponse{}, err
	}
	if req.NegativePrompt != "" || req.Seed != nil || len(req.ReferenceImages) != 0 {
		return providers.ImageGenerationResponse{}, providers.NewInvalidRequestError(ErrUnsupportedParameter)
	}
	if err := req.Validate(); err != nil {
		return providers.ImageGenerationResponse{}, err
	}

	count := 1
	if req.OutputCount != nil {
		count = *req.OutputCount
	}
	if count > g.adapter.maxOutputCount {
		return providers.ImageGenerationResponse{}, providers.NewInvalidRequestError(errors.New("output count exceeds adapter bound"))
	}
	body, err := json.Marshal(generationRequest{
		Model:  g.model.externalModel,
		Prompt: req.Prompt,
		N:      count,
		Size:   mapAspectRatio(req.AspectRatio),
	})
	if err != nil {
		return providers.ImageGenerationResponse{}, providers.NewInvalidRequestError(err)
	}

	apiKey, err := g.adapter.credentialSource.APIKey(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return providers.ImageGenerationResponse{}, ctxErr
		}
		return providers.ImageGenerationResponse{}, providers.NewAuthConfigError(errCredentialUnavailable)
	}
	if strings.TrimSpace(apiKey) == "" {
		return providers.ImageGenerationResponse{}, providers.NewAuthConfigError(errCredentialUnavailable)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, g.adapter.endpoint, bytes.NewReader(body))
	if err != nil {
		return providers.ImageGenerationResponse{}, providers.NewUnavailableError(errTransport)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := g.adapter.client.Do(httpRequest)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return providers.ImageGenerationResponse{}, ctxErr
		}
		return providers.ImageGenerationResponse{}, providers.NewUnavailableError(errTransport)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		return providers.ImageGenerationResponse{}, providers.NewAuthConfigError(errUpstreamStatus(response.StatusCode))
	case response.StatusCode == http.StatusTooManyRequests:
		return providers.ImageGenerationResponse{}, providers.NewRateLimitedError(errUpstreamStatus(response.StatusCode))
	case response.StatusCode >= http.StatusInternalServerError:
		return providers.ImageGenerationResponse{}, providers.NewTransientError(errUpstreamStatus(response.StatusCode))
	case response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices:
		return providers.ImageGenerationResponse{}, providers.NewInvalidRequestError(errUpstreamStatus(response.StatusCode))
	}

	responseBody, err := readBounded(response.Body, g.adapter.maxResponseBytes)
	if err != nil {
		return providers.ImageGenerationResponse{}, providers.NewMalformedResponseError(err)
	}
	var decoded imagesResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return providers.ImageGenerationResponse{}, providers.NewMalformedResponseError(errMalformedBody)
	}
	if len(decoded.Data) == 0 || len(decoded.Data) > g.adapter.maxOutputCount || len(decoded.Data) > providers.MaxImageOutputs {
		return providers.ImageGenerationResponse{}, providers.NewMalformedResponseError(errMalformedBody)
	}

	outputs := make([]providers.GeneratedImage, 0, len(decoded.Data))
	var totalBytes int64
	for _, item := range decoded.Data {
		data, mime, err := decodeImage(item.B64JSON, g.adapter.maxDecodedImageBytes)
		if err != nil {
			return providers.ImageGenerationResponse{}, providers.NewMalformedResponseError(err)
		}
		if int64(len(data)) > g.adapter.maxAggregateImageBytes-totalBytes {
			return providers.ImageGenerationResponse{}, providers.NewMalformedResponseError(errInvalidImage)
		}
		totalBytes += int64(len(data))
		binary, err := providers.NewGeneratedBinary(mime, data)
		if err != nil {
			return providers.ImageGenerationResponse{}, providers.NewMalformedResponseError(errInvalidImage)
		}
		outputs = append(outputs, providers.GeneratedImage{Binary: binary})
	}

	usage, err := mapUsage(decoded.Usage)
	if err != nil {
		return providers.ImageGenerationResponse{}, providers.NewMalformedResponseError(err)
	}
	requestID := safeRequestID(response.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = safeRequestID(decoded.ID)
	}
	return providers.ImageGenerationResponse{RequestID: requestID, Outputs: outputs, Usage: usage}, nil
}

func mapAspectRatio(aspectRatio string) string {
	switch aspectRatio {
	case "1:1":
		return "1024x1024"
	case "9:16", "4:5":
		return "1024x1536"
	case "16:9":
		return "1536x1024"
	default:
		return ""
	}
}

func decodeImage(encoded string, maxBytes int64) ([]byte, string, error) {
	if encoded == "" || strings.TrimSpace(encoded) != encoded {
		return nil, "", errInvalidImage
	}
	if int64(base64.StdEncoding.DecodedLen(len(encoded))) > maxBytes {
		return nil, "", errInvalidImage
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || int64(len(data)) > maxBytes {
		return nil, "", errInvalidImage
	}
	mime := imageMIME(data)
	if mime == "" {
		return nil, "", errInvalidImage
	}
	return data, mime, nil
}

func imageMIME(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"
	default:
		return ""
	}
}

func mapUsage(usage *responseUsage) (providers.UsageMetadata, error) {
	if usage == nil {
		return providers.UsageMetadata{}, nil
	}
	input := usage.InputTokens
	output := usage.OutputTokens
	if input == nil {
		input = usage.PromptTokens
	}
	if output == nil {
		output = usage.CompletionTokens
	}
	for _, value := range []*int{input, output, usage.TotalTokens} {
		if value != nil && *value < 0 {
			return providers.UsageMetadata{}, errMalformedBody
		}
	}
	return providers.UsageMetadata{InputTokens: input, OutputTokens: output, TotalTokens: usage.TotalTokens}, nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, errMalformedBody
	}
	if int64(len(body)) > limit {
		return nil, errResponseBody
	}
	return body, nil
}

func errUpstreamStatus(status int) error {
	return fmt.Errorf("upstream status %d", status)
}

func safeRequestID(value string) string {
	if value == "" || len(value) > 256 || strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return ""
	}
	return value
}
