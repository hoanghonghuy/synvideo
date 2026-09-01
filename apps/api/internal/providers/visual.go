package providers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxVisualPromptRunes         = 16_000
	MaxVisualNegativePromptRunes = 4_000
	MaxImageOutputs              = 4
	MaxVideoDurationSeconds      = 3_600
	MaxReferenceImageBytes       = 20 << 20
)

var supportedAspectRatios = map[string]struct{}{
	"16:9": {},
	"9:16": {},
	"1:1":  {},
	"4:5":  {},
}

// BinaryInput is a provider-neutral, finite binary input supplied by the caller.
type BinaryInput interface {
	MIMEType() string
	Size() int64
	Open(context.Context) (io.ReadCloser, error)
}

// BinaryInputCloner is implemented by immutable in-memory inputs used by fakes.
type BinaryInputCloner interface {
	Clone() BinaryInput
}

// GeneratedBinary is a provider-neutral stream opener for generated media.
type GeneratedBinary interface {
	MIMEType() string
	// Size returns the known byte size, or -1 when the size is unknown.
	Size() int64
	Open(context.Context) (io.ReadCloser, error)
}

type memoryBinary struct {
	mime string
	data []byte
}

// NewGeneratedBinary creates an immutable in-memory generated binary.
func NewGeneratedBinary(mime string, data []byte) (GeneratedBinary, error) {
	if !validGeneratedMIME(mime) {
		return nil, NewInvalidRequestError(fmt.Errorf("unsupported generated MIME type"))
	}
	return &memoryBinary{mime: mime, data: append([]byte(nil), data...)}, nil
}

// NewBinaryInput creates an immutable in-memory reference binary.
func NewBinaryInput(mime string, data []byte) (BinaryInput, error) {
	if !validReferenceMIME(mime) || len(data) > MaxReferenceImageBytes {
		return nil, NewInvalidRequestError(fmt.Errorf("invalid reference image"))
	}
	return &memoryBinary{mime: mime, data: append([]byte(nil), data...)}, nil
}

func (b *memoryBinary) MIMEType() string { return b.mime }

func (b *memoryBinary) Size() int64 { return int64(len(b.data)) }

func (b *memoryBinary) Open(ctx context.Context) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &contextReadCloser{ctx: ctx, reader: bytes.NewReader(b.data)}, nil
}

type contextReadCloser struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (r *contextReadCloser) Close() error { return nil }

func (b *memoryBinary) Clone() BinaryInput {
	return &memoryBinary{mime: b.mime, data: append([]byte(nil), b.data...)}
}

// ImageGenerationRequest contains provider-neutral image generation input.
type ImageGenerationRequest struct {
	Prompt          string
	NegativePrompt  string
	AspectRatio     string
	OutputCount     *int
	Seed            *int64
	ReferenceImages []BinaryInput
}

func (r ImageGenerationRequest) Validate() error {
	if err := validateVisualPrompt(r.Prompt, MaxVisualPromptRunes); err != nil {
		return NewInvalidRequestError(err)
	}
	if err := validateOptionalPrompt(r.NegativePrompt, MaxVisualNegativePromptRunes); err != nil {
		return NewInvalidRequestError(err)
	}
	if r.AspectRatio != "" {
		if _, ok := supportedAspectRatios[r.AspectRatio]; !ok {
			return NewInvalidRequestError(errors.New("unsupported aspect ratio"))
		}
	}
	if r.OutputCount != nil && (*r.OutputCount < 1 || *r.OutputCount > MaxImageOutputs) {
		return NewInvalidRequestError(errors.New("output count out of range"))
	}
	for _, reference := range r.ReferenceImages {
		if err := validateReferenceImage(reference); err != nil {
			return NewInvalidRequestError(err)
		}
	}
	return nil
}

// ImageGenerationResponse carries one or more provider-neutral image streams.
type ImageGenerationResponse struct {
	RequestID string
	Outputs   []GeneratedImage
	Usage     UsageMetadata
}

type GeneratedImage struct {
	Binary GeneratedBinary
	Width  *int
	Height *int
}

func (r ImageGenerationResponse) Validate() error {
	if len(r.Outputs) == 0 || len(r.Outputs) > MaxImageOutputs {
		return NewMalformedResponseError(errors.New("image output count is invalid"))
	}
	for _, output := range r.Outputs {
		if err := ValidateImageBinary(output.Binary); err != nil {
			return err
		}
		if output.Width != nil && *output.Width < 1 || output.Height != nil && *output.Height < 1 {
			return NewMalformedResponseError(errors.New("image dimensions are invalid"))
		}
	}
	return nil
}

// ValidateImageBinary rejects generated binaries that are not image media.
func ValidateImageBinary(binary GeneratedBinary) error {
	if binary == nil || !validImageMIME(binary.MIMEType()) || binary.Size() < -1 {
		return NewMalformedResponseError(errors.New("image output binary is invalid"))
	}
	return nil
}

// ValidateVideoBinary rejects generated binaries that are not video media.
func ValidateVideoBinary(binary GeneratedBinary) error {
	if binary == nil || !validVideoMIME(binary.MIMEType()) || binary.Size() < -1 {
		return NewMalformedResponseError(errors.New("video output binary is invalid"))
	}
	return nil
}

// ImageGenerator is the provider-neutral synchronous image generation port.
type ImageGenerator interface {
	GenerateImage(context.Context, ImageGenerationRequest) (ImageGenerationResponse, error)
}

// VideoGenerationRequest contains provider-neutral asynchronous video input.
type VideoGenerationRequest struct {
	Prompt          string
	ReferenceImage  BinaryInput
	AspectRatio     string
	DurationSeconds *int
}

func (r VideoGenerationRequest) Validate() error {
	if err := validateVisualPrompt(r.Prompt, MaxVisualPromptRunes); err != nil {
		return NewInvalidRequestError(err)
	}
	if r.AspectRatio != "" {
		if _, ok := supportedAspectRatios[r.AspectRatio]; !ok {
			return NewInvalidRequestError(errors.New("unsupported aspect ratio"))
		}
	}
	if r.DurationSeconds != nil && (*r.DurationSeconds < 1 || *r.DurationSeconds > MaxVideoDurationSeconds) {
		return NewInvalidRequestError(errors.New("duration out of range"))
	}
	if r.ReferenceImage != nil {
		if err := validateReferenceImage(r.ReferenceImage); err != nil {
			return NewInvalidRequestError(err)
		}
	}
	return nil
}

const (
	VideoOperationQueued    = "queued"
	VideoOperationRunning   = "running"
	VideoOperationSucceeded = "succeeded"
	VideoOperationFailed    = "failed"
)

// VideoOperation is the safe, provider-neutral state of an external operation.
type VideoOperation struct {
	ID              string
	State           string
	Progress        *int
	FailureCategory Category
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
	Usage           UsageMetadata
}

func (o VideoOperation) Validate() error {
	if strings.TrimSpace(o.ID) == "" {
		return NewMalformedResponseError(errors.New("video operation ID is missing"))
	}
	switch o.State {
	case VideoOperationQueued, VideoOperationRunning, VideoOperationSucceeded, VideoOperationFailed:
	default:
		return NewMalformedResponseError(errors.New("video operation state is invalid"))
	}
	if o.Progress != nil && (*o.Progress < 0 || *o.Progress > 100) {
		return NewMalformedResponseError(errors.New("video operation progress is invalid"))
	}
	return nil
}

// VideoGenerator preserves the external operation lifecycle for video.
type VideoGenerator interface {
	StartVideo(context.Context, VideoGenerationRequest) (VideoOperation, error)
	GetVideoOperation(context.Context, string) (VideoOperation, error)
	OpenVideoResult(context.Context, string) (GeneratedBinary, error)
}

func validateVisualPrompt(value string, limit int) error {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return errors.New("prompt is required")
	}
	if utf8.RuneCountInString(value) > limit {
		return errors.New("prompt exceeds maximum length")
	}
	return nil
}

func validateOptionalPrompt(value string, limit int) error {
	if value == "" {
		return nil
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return errors.New("negative prompt exceeds maximum length")
	}
	return nil
}

func validateReferenceImage(reference BinaryInput) error {
	if reference == nil || !validReferenceMIME(reference.MIMEType()) {
		return errors.New("reference image MIME type is invalid")
	}
	if reference.Size() < 0 || reference.Size() > MaxReferenceImageBytes {
		return errors.New("reference image size is invalid")
	}
	return nil
}

func validReferenceMIME(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}

func validGeneratedMIME(mime string) bool {
	return validImageMIME(mime) || validVideoMIME(mime)
}

func validImageMIME(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}

func validVideoMIME(mime string) bool {
	switch mime {
	case "video/mp4", "video/webm":
		return true
	default:
		return false
	}
}
