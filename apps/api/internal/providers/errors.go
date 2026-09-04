package providers

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownProvider           = errors.New("unknown provider")
	ErrUnknownModel              = errors.New("unknown model")
	ErrUnsupportedCapability     = errors.New("unsupported capability")
	ErrProviderUnavailable       = errors.New("provider unavailable")
	ErrProviderExecution         = errors.New("provider execution failed")
	ErrAuthenticationUnavailable = errors.New("provider authentication or configuration unavailable")
	ErrRateLimited               = errors.New("provider rate limited")
	ErrTransientExecution        = errors.New("provider transient execution failure")
	ErrInvalidRequest            = errors.New("provider request is invalid")
	ErrMalformedResponse         = errors.New("provider response is malformed")
	ErrVideoOperationFailed      = errors.New("video operation failed")
	ErrResultUnavailable         = errors.New("generated result is unavailable")
	ErrUnknownVideoOperation     = errors.New("unknown video operation")
	ErrAmbiguousSubmit           = errors.New("video submit outcome is ambiguous")
	ErrDuplicateRegistration     = errors.New("duplicate provider registration")
	ErrSpeechInputTooLong        = errors.New("speech input is too long")
)

// Category classifies provider-boundary failures for stable handling upstream.
type Category string

const (
	CategoryUnknownProvider           Category = "unknown_provider"
	CategoryUnknownModel              Category = "unknown_model"
	CategoryUnsupportedCapability     Category = "unsupported_capability"
	CategoryProviderUnavailable       Category = "provider_unavailable"
	CategoryProviderExecution         Category = "provider_execution"
	CategoryAuthenticationUnavailable Category = "authentication_or_config_unavailable"
	CategoryRateLimited               Category = "rate_limited"
	CategoryTransientExecution        Category = "transient_execution_failure"
	CategoryInvalidRequest            Category = "invalid_request"
	CategoryMalformedResponse         Category = "malformed_response"
	CategoryVideoOperationFailed      Category = "video_operation_failed"
	CategoryResultUnavailable         Category = "result_unavailable"
	CategoryUnknownVideoOperation     Category = "unknown_video_operation"
	CategoryAmbiguousSubmit           Category = "ambiguous_submit"
	CategoryDuplicateRegistration     Category = "duplicate_registration"
	CategorySpeechInputTooLong        Category = "speech_input_too_long"
)

// BoundaryError is a provider-boundary failure with a safe presentation message.
type BoundaryError struct {
	Category Category
	Message  string
	cause    error
}

func (e *BoundaryError) Error() string {
	return e.Message
}

func (e *BoundaryError) Unwrap() error {
	return e.cause
}

func (e *BoundaryError) PresentationMessage() string {
	return e.Message
}

func NewUnknownProviderError(providerID ProviderID) error {
	return &BoundaryError{
		Category: CategoryUnknownProvider,
		Message:  fmt.Sprintf("Provider %q is not registered.", providerID),
		cause:    ErrUnknownProvider,
	}
}

func NewUnknownModelError(providerID ProviderID, modelID ModelID) error {
	return &BoundaryError{
		Category: CategoryUnknownModel,
		Message:  fmt.Sprintf("Model %q is not registered for provider %q.", modelID, providerID),
		cause:    ErrUnknownModel,
	}
}

func NewUnsupportedCapabilityError(providerID ProviderID, modelID ModelID, capability Capability) error {
	return &BoundaryError{
		Category: CategoryUnsupportedCapability,
		Message:  fmt.Sprintf("Model %q for provider %q does not support capability %q.", modelID, providerID, capability),
		cause:    ErrUnsupportedCapability,
	}
}

func NewUnavailableError(cause error) error {
	return &BoundaryError{
		Category: CategoryProviderUnavailable,
		Message:  "The selected provider is unavailable.",
		cause:    errors.Join(ErrProviderUnavailable, cause),
	}
}

func NewExecutionError(cause error) error {
	return &BoundaryError{
		Category: CategoryProviderExecution,
		Message:  "The provider could not complete text generation.",
		cause:    errors.Join(ErrProviderExecution, cause),
	}
}

func newSafeError(category Category, message string, sentinel error, cause error) error {
	return &BoundaryError{Category: category, Message: message, cause: errors.Join(sentinel, cause)}
}

func NewAuthConfigError(cause error) error {
	return newSafeError(CategoryAuthenticationUnavailable, "The provider authentication or configuration is unavailable.", ErrAuthenticationUnavailable, cause)
}

func NewRateLimitedError(cause error) error {
	return newSafeError(CategoryRateLimited, "The provider temporarily rate-limited this request.", ErrRateLimited, cause)
}

func NewTransientError(cause error) error {
	return newSafeError(CategoryTransientExecution, "The provider temporarily failed to complete this request.", ErrTransientExecution, cause)
}

func NewInvalidRequestError(cause error) error {
	return newSafeError(CategoryInvalidRequest, "The generation request is invalid.", ErrInvalidRequest, cause)
}

func NewMalformedResponseError(cause error) error {
	return newSafeError(CategoryMalformedResponse, "The provider returned an invalid response.", ErrMalformedResponse, cause)
}

func NewVideoOperationFailedError(cause error) error {
	return newSafeError(CategoryVideoOperationFailed, "The video generation operation failed.", ErrVideoOperationFailed, cause)
}

func NewResultUnavailableError(cause error) error {
	return newSafeError(CategoryResultUnavailable, "The generated result is not available yet.", ErrResultUnavailable, cause)
}

func NewUnknownVideoOperationError(cause error) error {
	return newSafeError(CategoryUnknownVideoOperation, "The video generation operation is not known.", ErrUnknownVideoOperation, cause)
}

// NewAmbiguousSubmitError marks a video submit failure where upstream acceptance cannot be ruled out.
// Callers must not automatically submit another paid operation for the same logical request.
func NewAmbiguousSubmitError(cause error) error {
	return newSafeError(CategoryAmbiguousSubmit, "The video submit outcome is uncertain and requires safe recovery.", ErrAmbiguousSubmit, cause)
}

func NewDuplicateRegistrationError(providerID ProviderID) error {
	return &BoundaryError{
		Category: CategoryDuplicateRegistration,
		Message:  fmt.Sprintf("Provider %q is already registered.", providerID),
		cause:    ErrDuplicateRegistration,
	}
}

func NewSpeechInputTooLongError() error {
	return &BoundaryError{
		Category: CategorySpeechInputTooLong,
		Message:  "The speech narration exceeds the selected provider's input limit.",
		cause:    errors.Join(ErrInvalidRequest, ErrSpeechInputTooLong),
	}
}
