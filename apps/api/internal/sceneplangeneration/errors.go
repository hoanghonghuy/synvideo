package sceneplangeneration

import (
	"context"
	"errors"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

var (
	ErrGenerationProviderUnavailable = errors.New("generation provider unavailable")
	ErrGenerationProviderFailed      = errors.New("generation provider failed")
	ErrGenerationInvalidOutput       = errors.New("generation invalid output")
)

type Code string

const (
	CodeProviderUnavailable Code = "GENERATION_PROVIDER_UNAVAILABLE"
	CodeProviderFailed      Code = "GENERATION_PROVIDER_FAILED"
	CodeInvalidOutput       Code = "GENERATION_INVALID_OUTPUT"
)

type Error struct {
	Code    Code
	Message string
	cause   error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.cause }

func (e *Error) PresentationMessage() string { return e.Message }

func newProviderUnavailableError(cause error) error {
	return &Error{
		Code:    CodeProviderUnavailable,
		Message: "The selected generation provider is unavailable.",
		cause:   errors.Join(ErrGenerationProviderUnavailable, cause),
	}
}

func newProviderFailedError(cause error) error {
	return &Error{
		Code:    CodeProviderFailed,
		Message: "Scene plan generation could not be completed.",
		cause:   errors.Join(ErrGenerationProviderFailed, cause),
	}
}

func newInvalidOutputError(cause error) error {
	if cause == nil {
		cause = ErrGenerationInvalidOutput
	}
	return &Error{
		Code:    CodeInvalidOutput,
		Message: "The model output did not satisfy the scene plan contract.",
		cause:   errors.Join(ErrGenerationInvalidOutput, cause),
	}
}

func mapProviderError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	switch {
	case errors.Is(err, providers.ErrUnknownProvider),
		errors.Is(err, providers.ErrUnknownModel),
		errors.Is(err, providers.ErrUnsupportedCapability),
		errors.Is(err, providers.ErrProviderUnavailable):
		return newProviderUnavailableError(err)
	default:
		return newProviderFailedError(err)
	}
}
