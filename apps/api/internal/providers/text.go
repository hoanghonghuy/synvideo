package providers

import "context"

// TextMessage is provider-neutral conversational input.
type TextMessage struct {
	Role    string
	Content string
}

// TextGenerationRequest carries provider-neutral text generation input.
type TextGenerationRequest struct {
	ProviderID ProviderID
	ModelID    ModelID
	Messages   []TextMessage
}

// UsageMetadata reports token usage when an adapter can provide it.
type UsageMetadata struct {
	InputTokens  *int
	OutputTokens *int
	TotalTokens  *int
}

// TextGenerationResponse carries provider-neutral generated text and usage.
type TextGenerationResponse struct {
	ProviderID ProviderID
	ModelID    ModelID
	Text       string
	Usage      UsageMetadata
}

// TextGenerator is the typed text-generation boundary for future proposal work.
type TextGenerator interface {
	GenerateText(ctx context.Context, req TextGenerationRequest) (TextGenerationResponse, error)
}
