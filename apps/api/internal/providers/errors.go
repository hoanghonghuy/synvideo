package providers

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownProvider       = errors.New("unknown provider")
	ErrUnknownModel          = errors.New("unknown model")
	ErrUnsupportedCapability = errors.New("unsupported capability")
	ErrProviderUnavailable   = errors.New("provider unavailable")
	ErrProviderExecution     = errors.New("provider execution failed")
	ErrDuplicateRegistration = errors.New("duplicate provider registration")
)

// Category classifies provider-boundary failures for stable handling upstream.
type Category string

const (
	CategoryUnknownProvider       Category = "unknown_provider"
	CategoryUnknownModel          Category = "unknown_model"
	CategoryUnsupportedCapability Category = "unsupported_capability"
	CategoryProviderUnavailable   Category = "provider_unavailable"
	CategoryProviderExecution     Category = "provider_execution"
	CategoryDuplicateRegistration Category = "duplicate_registration"
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

func NewDuplicateRegistrationError(providerID ProviderID) error {
	return &BoundaryError{
		Category: CategoryDuplicateRegistration,
		Message:  fmt.Sprintf("Provider %q is already registered.", providerID),
		cause:    ErrDuplicateRegistration,
	}
}
