package fake

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

// TextGenerator is a deterministic fake text provider for tests.
type TextGenerator struct {
	response string
	delay    time.Duration

	mu       sync.Mutex
	requests []providers.TextGenerationRequest
}

func NewTextGenerator(response string) *TextGenerator {
	return &TextGenerator{response: response}
}

func (g *TextGenerator) WithDelay(delay time.Duration) *TextGenerator {
	g.delay = delay
	return g
}

func (g *TextGenerator) Requests() []providers.TextGenerationRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	copied := make([]providers.TextGenerationRequest, len(g.requests))
	copy(copied, g.requests)
	return copied
}

func (g *TextGenerator) GenerateText(ctx context.Context, req providers.TextGenerationRequest) (providers.TextGenerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.TextGenerationResponse{}, err
	}

	if g.delay > 0 {
		timer := time.NewTimer(g.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return providers.TextGenerationResponse{}, ctx.Err()
		case <-timer.C:
		}
	}

	g.mu.Lock()
	g.requests = append(g.requests, req)
	g.mu.Unlock()

	inputTokens := 0
	for _, message := range req.Messages {
		inputTokens += len(strings.Fields(message.Content))
	}
	outputTokens := len(strings.Fields(g.response))
	totalTokens := inputTokens + outputTokens

	return providers.TextGenerationResponse{
		ProviderID: req.ProviderID,
		ModelID:    req.ModelID,
		Text:       g.response,
		Usage: providers.UsageMetadata{
			InputTokens:  intPtr(inputTokens),
			OutputTokens: intPtr(outputTokens),
			TotalTokens:  intPtr(totalTokens),
		},
	}, nil
}

func intPtr(value int) *int {
	return &value
}
