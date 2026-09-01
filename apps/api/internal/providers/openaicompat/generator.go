package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

var (
	errInvalidRequest = errors.New("invalid text generation request")
	errTransport      = errors.New("upstream provider transport failure")
	errUpstreamStatus = errors.New("upstream provider returned an unsuccessful status")
	errResponseBody   = errors.New("upstream provider response exceeded the configured limit")
	errMalformedBody  = errors.New("upstream provider returned a malformed response")
)

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *responseUsage `json:"usage"`
}

type responseUsage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
	TotalTokens      *int `json:"total_tokens"`
}

// GenerateText sends one bounded, context-aware chat-completions request.
func (a *Adapter) GenerateText(ctx context.Context, req providers.TextGenerationRequest) (providers.TextGenerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.TextGenerationResponse{}, err
	}
	model, ok := a.models[req.ModelID]
	if !ok || req.ProviderID != a.providerID {
		return providers.TextGenerationResponse{}, providers.NewExecutionError(errInvalidRequest)
	}

	apiKey, err := a.credentialSource.APIKey(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return providers.TextGenerationResponse{}, ctxErr
		}
		return providers.TextGenerationResponse{}, providers.NewUnavailableError(errCredentialUnavailable)
	}
	if strings.TrimSpace(apiKey) == "" {
		return providers.TextGenerationResponse{}, providers.NewUnavailableError(errCredentialUnavailable)
	}

	messages := make([]chatMessage, len(req.Messages))
	for i, message := range req.Messages {
		messages[i] = chatMessage{Role: message.Role, Content: message.Content}
	}
	body, err := json.Marshal(chatCompletionRequest{Model: model.externalModel, Messages: messages})
	if err != nil {
		return providers.TextGenerationResponse{}, providers.NewExecutionError(errMalformedBody)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return providers.TextGenerationResponse{}, providers.NewExecutionError(errInvalidRequest)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)

	response, err := a.client.Do(httpRequest)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return providers.TextGenerationResponse{}, ctxErr
		}
		return providers.TextGenerationResponse{}, providers.NewUnavailableError(errTransport)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
		return providers.TextGenerationResponse{}, providers.NewUnavailableError(fmt.Errorf("status %d: %w", response.StatusCode, errUpstreamStatus))
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return providers.TextGenerationResponse{}, providers.NewExecutionError(fmt.Errorf("status %d: %w", response.StatusCode, errUpstreamStatus))
	}

	responseBody, err := readBounded(response.Body, a.maxResponseBytes)
	if err != nil {
		return providers.TextGenerationResponse{}, providers.NewExecutionError(err)
	}
	var decoded chatCompletionResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return providers.TextGenerationResponse{}, providers.NewExecutionError(errMalformedBody)
	}

	for _, choice := range decoded.Choices {
		if choice.Message.Role != "assistant" || strings.TrimSpace(choice.Message.Content) == "" {
			continue
		}
		usage, err := mapUsage(decoded.Usage)
		if err != nil {
			return providers.TextGenerationResponse{}, providers.NewExecutionError(err)
		}
		return providers.TextGenerationResponse{
			ProviderID: req.ProviderID,
			ModelID:    req.ModelID,
			Text:       choice.Message.Content,
			Usage:      usage,
		}, nil
	}
	return providers.TextGenerationResponse{}, providers.NewExecutionError(errMalformedBody)
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

func mapUsage(usage *responseUsage) (providers.UsageMetadata, error) {
	if usage == nil {
		return providers.UsageMetadata{}, nil
	}
	for _, value := range []*int{usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens} {
		if value != nil && *value < 0 {
			return providers.UsageMetadata{}, errMalformedBody
		}
	}
	return providers.UsageMetadata{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  usage.TotalTokens,
	}, nil
}
