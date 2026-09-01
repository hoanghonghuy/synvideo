package fake

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

// NewBinaryInput returns a deterministic immutable reference input for tests.
func NewBinaryInput(mime string, data []byte) providers.BinaryInput {
	input, err := providers.NewBinaryInput(mime, data)
	if err != nil {
		panic(err)
	}
	return input
}

// ImageGenerator is a deterministic fake image provider.
type ImageGenerator struct {
	mu          sync.Mutex
	outputs     [][]byte
	mime        string
	generateErr error
	delay       time.Duration
	requests    []providers.ImageGenerationRequest
}

func NewImageGenerator(data []byte) *ImageGenerator {
	return &ImageGenerator{outputs: [][]byte{append([]byte(nil), data...)}, mime: "image/png"}
}

func (g *ImageGenerator) WithMIMEType(mime string) *ImageGenerator {
	g.mime = mime
	return g
}

func (g *ImageGenerator) WithOutputs(outputs ...[]byte) *ImageGenerator {
	g.outputs = cloneBytes(outputs)
	return g
}

func (g *ImageGenerator) WithError(err error) *ImageGenerator {
	g.generateErr = err
	return g
}

func (g *ImageGenerator) WithDelay(delay time.Duration) *ImageGenerator {
	g.delay = delay
	return g
}

func cloneImageRequest(ctx context.Context, req providers.ImageGenerationRequest) (providers.ImageGenerationRequest, error) {
	cloned := req
	if req.OutputCount != nil {
		value := *req.OutputCount
		cloned.OutputCount = &value
	}
	if req.Seed != nil {
		value := *req.Seed
		cloned.Seed = &value
	}
	if len(req.ReferenceImages) > 0 {
		cloned.ReferenceImages = make([]providers.BinaryInput, len(req.ReferenceImages))
		for i, reference := range req.ReferenceImages {
			var err error
			cloned.ReferenceImages[i], err = snapshotBinaryInput(ctx, reference)
			if err != nil {
				return providers.ImageGenerationRequest{}, err
			}
		}
	}
	return cloned, nil
}

func (g *ImageGenerator) Requests() []providers.ImageGenerationRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	requests := make([]providers.ImageGenerationRequest, len(g.requests))
	for i, request := range g.requests {
		requests[i], _ = cloneImageRequest(context.Background(), request)
	}
	return requests
}

func (g *ImageGenerator) GenerateImage(ctx context.Context, req providers.ImageGenerationRequest) (providers.ImageGenerationResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.ImageGenerationResponse{}, err
	}
	if err := req.Validate(); err != nil {
		return providers.ImageGenerationResponse{}, err
	}
	clonedRequest, err := cloneImageRequest(ctx, req)
	if err != nil {
		return providers.ImageGenerationResponse{}, providers.NewInvalidRequestError(err)
	}
	if g.delay > 0 {
		timer := time.NewTimer(g.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return providers.ImageGenerationResponse{}, ctx.Err()
		case <-timer.C:
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.generateErr != nil {
		return providers.ImageGenerationResponse{}, g.generateErr
	}
	g.requests = append(g.requests, clonedRequest)
	count := 1
	if req.OutputCount != nil {
		count = *req.OutputCount
	}
	outputs := make([]providers.GeneratedImage, 0, count)
	for i := 0; i < count; i++ {
		data := []byte(nil)
		if len(g.outputs) > 0 {
			data = g.outputs[i%len(g.outputs)]
		}
		binary, err := providers.NewGeneratedBinary(g.mime, data)
		if err != nil {
			return providers.ImageGenerationResponse{}, err
		}
		outputs = append(outputs, providers.GeneratedImage{Binary: binary})
	}
	response := providers.ImageGenerationResponse{ProviderID: req.ProviderID, ModelID: req.ModelID, Outputs: outputs}
	if err := response.Validate(); err != nil {
		return providers.ImageGenerationResponse{}, err
	}
	return response, nil
}

// VideoGenerator is a deterministic fake with queued -> running -> succeeded states.
type VideoGenerator struct {
	mu               sync.Mutex
	result           []byte
	mime             string
	startErr         error
	pollErr          error
	resultErr        error
	operationFailure error
	delay            time.Duration
	nextID           uint64
	operations       map[string]*fakeVideoOperation
	requests         []providers.VideoGenerationRequest
}

type fakeVideoOperation struct {
	operation providers.VideoOperation
	polls     int
}

func NewVideoGenerator(data []byte) *VideoGenerator {
	return &VideoGenerator{result: append([]byte(nil), data...), mime: "video/mp4", operations: make(map[string]*fakeVideoOperation)}
}

func (g *VideoGenerator) WithMIMEType(mime string) *VideoGenerator  { g.mime = mime; return g }
func (g *VideoGenerator) WithStartError(err error) *VideoGenerator  { g.startErr = err; return g }
func (g *VideoGenerator) WithPollError(err error) *VideoGenerator   { g.pollErr = err; return g }
func (g *VideoGenerator) WithResultError(err error) *VideoGenerator { g.resultErr = err; return g }
func (g *VideoGenerator) WithOperationFailure(err error) *VideoGenerator {
	g.operationFailure = err
	return g
}
func (g *VideoGenerator) WithDelay(delay time.Duration) *VideoGenerator { g.delay = delay; return g }

func cloneVideoRequest(ctx context.Context, req providers.VideoGenerationRequest) (providers.VideoGenerationRequest, error) {
	cloned := req
	if req.DurationSeconds != nil {
		value := *req.DurationSeconds
		cloned.DurationSeconds = &value
	}
	if req.ReferenceImage != nil {
		var err error
		cloned.ReferenceImage, err = snapshotBinaryInput(ctx, req.ReferenceImage)
		if err != nil {
			return providers.VideoGenerationRequest{}, err
		}
	}
	return cloned, nil
}

func (g *VideoGenerator) Requests() []providers.VideoGenerationRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	requests := make([]providers.VideoGenerationRequest, len(g.requests))
	for i, request := range g.requests {
		requests[i], _ = cloneVideoRequest(context.Background(), request)
	}
	return requests
}

func (g *VideoGenerator) wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if g.delay == 0 {
		return nil
	}
	timer := time.NewTimer(g.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func cloneOperation(operation providers.VideoOperation) providers.VideoOperation {
	cloned := operation
	if operation.Progress != nil {
		value := *operation.Progress
		cloned.Progress = &value
	}
	return cloned
}

func (g *VideoGenerator) StartVideo(ctx context.Context, req providers.VideoGenerationRequest) (providers.VideoOperation, error) {
	if err := g.wait(ctx); err != nil {
		return providers.VideoOperation{}, err
	}
	if err := req.Validate(); err != nil {
		return providers.VideoOperation{}, err
	}
	clonedRequest, err := cloneVideoRequest(ctx, req)
	if err != nil {
		return providers.VideoOperation{}, providers.NewInvalidRequestError(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.startErr != nil {
		return providers.VideoOperation{}, g.startErr
	}
	g.nextID++
	id := "op-" + formatID(g.nextID)
	operation := providers.VideoOperation{ID: id, State: providers.VideoOperationQueued}
	g.operations[id] = &fakeVideoOperation{operation: operation}
	g.requests = append(g.requests, clonedRequest)
	return cloneOperation(operation), nil
}

func (g *VideoGenerator) GetVideoOperation(ctx context.Context, id string) (providers.VideoOperation, error) {
	if err := g.wait(ctx); err != nil {
		return providers.VideoOperation{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	operation, ok := g.operations[id]
	if !ok {
		return providers.VideoOperation{}, providers.NewUnknownVideoOperationError(errors.New("operation not found"))
	}
	if g.pollErr != nil {
		return providers.VideoOperation{}, g.pollErr
	}
	operation.polls++
	if operation.polls >= 2 && g.operationFailure != nil {
		operation.operation.State = providers.VideoOperationFailed
		operation.operation.FailureCategory = providers.CategoryVideoOperationFailed
		return cloneOperation(operation.operation), nil
	}
	switch operation.polls {
	case 1:
		operation.operation.State = providers.VideoOperationRunning
		progress := 50
		operation.operation.Progress = &progress
	default:
		operation.operation.State = providers.VideoOperationSucceeded
		progress := 100
		operation.operation.Progress = &progress
	}
	return cloneOperation(operation.operation), nil
}

func (g *VideoGenerator) OpenVideoResult(ctx context.Context, id string) (providers.GeneratedBinary, error) {
	if err := g.wait(ctx); err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	operation, ok := g.operations[id]
	if !ok {
		return nil, providers.NewUnknownVideoOperationError(errors.New("operation not found"))
	}
	if g.resultErr != nil {
		return nil, g.resultErr
	}
	if operation.operation.State == providers.VideoOperationFailed {
		return nil, providers.NewVideoOperationFailedError(errors.New("configured fake operation failure"))
	}
	if operation.operation.State != providers.VideoOperationSucceeded {
		return nil, providers.NewResultUnavailableError(errors.New("operation has not succeeded"))
	}
	binary, err := providers.NewGeneratedBinary(g.mime, g.result)
	if err != nil {
		return nil, err
	}
	if err := providers.ValidateVideoBinary(binary); err != nil {
		return nil, err
	}
	return binary, nil
}

func snapshotBinaryInput(ctx context.Context, input providers.BinaryInput) (providers.BinaryInput, error) {
	if input == nil {
		return nil, errors.New("reference input size is invalid")
	}
	size := input.Size()
	if size < 0 || size > providers.MaxReferenceImageBytes {
		return nil, errors.New("reference input size is invalid")
	}
	mime := input.MIMEType()
	stream, err := input.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, size+1))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(data)) > size {
		return nil, errors.New("reference input exceeded declared size")
	}
	cloned, err := providers.NewBinaryInput(mime, data)
	if err != nil {
		return nil, err
	}
	return cloned, nil
}

func cloneBytes(values [][]byte) [][]byte {
	cloned := make([][]byte, len(values))
	for i, value := range values {
		cloned[i] = append([]byte(nil), value...)
	}
	return cloned
}

func formatID(value uint64) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var reversed [20]byte
	index := len(reversed)
	for value > 0 {
		index--
		reversed[index] = digits[value%10]
		value /= 10
	}
	return string(reversed[index:])
}
