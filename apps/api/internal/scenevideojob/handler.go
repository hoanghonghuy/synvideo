package scenevideojob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
)

const (
	ErrorProviderUnavailable = "ERR_VIDEO_PROVIDER_UNAVAILABLE"
	ErrorProviderFailed      = "ERR_VIDEO_PROVIDER_FAILED"
	ErrorStorageFailed       = "ERR_VIDEO_STORAGE_FAILED"
	ErrorAssignmentFailed    = "ERR_VIDEO_ASSIGNMENT_FAILED"
	ErrorInvalidPayload      = "ERR_VIDEO_JOB_INVALID"
	ErrorPollingPending      = "ERR_VIDEO_OPERATION_PENDING"
	ErrorAmbiguousSubmit     = "ERR_VIDEO_SUBMIT_AMBIGUOUS"
	ErrorCheckpointFailed    = "ERR_VIDEO_CHECKPOINT_FAILED"
)

var defaultPollDelay = 2 * time.Second

type VideoProviderRuntime interface {
	ResolveVideoGenerator(context.Context, uuid.UUID, providers.ProviderID, providers.ModelID) (providers.VideoGenerator, error)
}

type OperationRepository interface {
	Get(context.Context, project.Principal, uuid.UUID, uuid.UUID) (OperationCheckpoint, error)
	SaveSubmitted(context.Context, project.Principal, uuid.UUID, uuid.UUID, string) (OperationCheckpoint, error)
	SaveAmbiguous(context.Context, project.Principal, uuid.UUID, uuid.UUID) (OperationCheckpoint, error)
}

type GeneratedAssetStore interface {
	FindGeneratedByJob(context.Context, project.Principal, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
	Store(context.Context, project.Principal, uuid.UUID, mediaasset.CreateInput) (mediaasset.MediaAsset, error)
}

type SceneBinder interface {
	GetCurrent(context.Context, project.Principal, uuid.UUID, int, string) (scenemedia.Binding, error)
	AssignPrimaryVisual(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenemedia.Binding, error)
}

type Handler struct {
	runtime     VideoProviderRuntime
	operations  OperationRepository
	assets      GeneratedAssetStore
	bindings    SceneBinder
}

func NewHandler(runtime VideoProviderRuntime, operations OperationRepository, assets GeneratedAssetStore, bindings SceneBinder) *Handler {
	return &Handler{runtime: runtime, operations: operations, assets: assets, bindings: bindings}
}

func (h *Handler) Handle(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
	if job.Kind != JobKind || job.OwnerID == uuid.Nil || job.ProjectID == nil || *job.ProjectID == uuid.Nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, errors.New("invalid scene video job identity"))
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.SchemaVersion != SchemaVersion || payload.ProviderID == "" || payload.ModelID == "" || payload.ScenePlanVersion < 1 || payload.SceneKey == "" || payload.Prompt == "" {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, errors.New("invalid scene video job payload"))
	}
	principal := project.Principal{OwnerID: job.OwnerID}
	projectID := *job.ProjectID

	if h.assets == nil || h.operations == nil || h.runtime == nil {
		return nil, jobs.NewTerminalError(ErrorProviderUnavailable, errors.New("scene video runtime unavailable"))
	}

	asset, err := h.assets.FindGeneratedByJob(ctx, principal, projectID, job.ID)
	if err == nil {
		return h.finish(ctx, principal, projectID, job.ID, payload, asset)
	}
	if !errors.Is(err, mediaasset.ErrNotFound) {
		return nil, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
	}

	generator, err := h.runtime.ResolveVideoGenerator(ctx, principal.OwnerID, providers.ProviderID(payload.ProviderID), providers.ModelID(payload.ModelID))
	if err != nil || generator == nil {
		return nil, jobs.NewTerminalError(ErrorProviderUnavailable, errors.New("selected video provider unavailable"))
	}

	checkpoint, err := h.operations.Get(ctx, principal, projectID, job.ID)
	if err != nil && !errors.Is(err, ErrCheckpointNotFound) {
		return nil, jobs.NewRetryableError(ErrorCheckpointFailed, err, nil)
	}
	if errors.Is(err, ErrCheckpointNotFound) {
		operation, startErr := generator.StartVideo(ctx, providers.VideoGenerationRequest{
			Prompt: payload.Prompt, AspectRatio: payload.AspectRatio, DurationSeconds: payload.DurationSeconds,
		})
		if startErr != nil {
			if errors.Is(startErr, providers.ErrAmbiguousSubmit) {
				if _, saveErr := h.operations.SaveAmbiguous(ctx, principal, projectID, job.ID); saveErr != nil {
					return nil, jobs.NewTerminalError(ErrorCheckpointFailed, saveErr)
				}
				return nil, jobs.NewTerminalError(ErrorAmbiguousSubmit, startErr)
			}
			return nil, classifyVideoProviderError(startErr)
		}
		if err := operation.Validate(); err != nil {
			return nil, jobs.NewTerminalError(ErrorProviderFailed, err)
		}
		checkpoint, err = h.operations.SaveSubmitted(ctx, principal, projectID, job.ID, operation.ID)
		if err != nil {
			// The provider may already have accepted paid work. Automatic retry here could
			// submit again, so fail closed for manual-safe recovery instead of retrying.
			return nil, jobs.NewTerminalError(ErrorCheckpointFailed, err)
		}
		if operation.State == providers.VideoOperationFailed {
			return nil, jobs.NewTerminalError(ErrorProviderFailed, providers.NewVideoOperationFailedError(errors.New("video operation failed during submit")))
		}
		if operation.State != providers.VideoOperationSucceeded {
			return nil, jobs.NewRetryableError(ErrorPollingPending, errors.New("video operation is still running"), &defaultPollDelay)
		}
	}

	if checkpoint.State == OperationStateAmbiguous {
		return nil, jobs.NewTerminalError(ErrorAmbiguousSubmit, providers.NewAmbiguousSubmitError(errors.New("previous submit outcome was ambiguous")))
	}
	if checkpoint.State != OperationStateSubmitted || checkpoint.ExternalOperationID == "" {
		return nil, jobs.NewTerminalError(ErrorCheckpointFailed, errors.New("invalid scene video operation checkpoint"))
	}

	operation, err := generator.GetVideoOperation(ctx, checkpoint.ExternalOperationID)
	if err != nil {
		return nil, classifyVideoProviderError(err)
	}
	if err := operation.Validate(); err != nil {
		return nil, jobs.NewTerminalError(ErrorProviderFailed, err)
	}
	if operation.ID != checkpoint.ExternalOperationID {
		return nil, jobs.NewTerminalError(ErrorProviderFailed, errors.New("provider returned mismatched operation identity"))
	}
	switch operation.State {
	case providers.VideoOperationQueued, providers.VideoOperationRunning:
		return nil, jobs.NewRetryableError(ErrorPollingPending, errors.New("video operation is still running"), &defaultPollDelay)
	case providers.VideoOperationFailed:
		return nil, jobs.NewTerminalError(ErrorProviderFailed, providers.NewVideoOperationFailedError(errors.New("video operation failed")))
	case providers.VideoOperationSucceeded:
		// continue to durable acquisition below
	default:
		return nil, jobs.NewTerminalError(ErrorProviderFailed, errors.New("unsupported video operation state"))
	}

	binary, err := generator.OpenVideoResult(ctx, checkpoint.ExternalOperationID)
	if err != nil {
		return nil, classifyVideoProviderError(err)
	}
	if err := providers.ValidateVideoBinary(binary); err != nil {
		return nil, jobs.NewTerminalError(ErrorProviderFailed, err)
	}
	reader, err := binary.Open(ctx)
	if err != nil {
		return nil, classifyVideoProviderError(err)
	}
	defer reader.Close()

	metadata, err := json.Marshal(map[string]any{
		"origin": "generated",
		"job_id": job.ID.String(),
		"provider_id": payload.ProviderID,
		"model_id": payload.ModelID,
		"external_operation_id": checkpoint.ExternalOperationID,
		"scene_plan_version": payload.ScenePlanVersion,
		"scene_key": payload.SceneKey,
	})
	if err != nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, err)
	}
	asset, err = h.assets.Store(ctx, principal, projectID, mediaasset.CreateInput{
		Kind: mediaasset.KindVideo, Origin: mediaasset.OriginGeneratedVideo, MimeType: binary.MIMEType(), Metadata: metadata,
		Reader: reader, MaxBytes: MaxGeneratedVideoBytes,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
	}
	return h.finish(ctx, principal, projectID, job.ID, payload, asset)
}

func (h *Handler) finish(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID, payload Payload, asset mediaasset.MediaAsset) (json.RawMessage, error) {
	assigned := false
	if payload.AssignPrimaryVisual {
		if h.bindings == nil {
			return nil, jobs.NewRetryableError(ErrorAssignmentFailed, errors.New("scene binding service unavailable"), nil)
		}
		current, err := h.bindings.GetCurrent(ctx, principal, projectID, payload.ScenePlanVersion, payload.SceneKey)
		if err == nil && current.AssetID == asset.ID {
			assigned = true
		} else if err != nil && !errors.Is(err, scenemedia.ErrNotFound) {
			return nil, jobs.NewRetryableError(ErrorAssignmentFailed, err, nil)
		} else {
			if _, err := h.bindings.AssignPrimaryVisual(ctx, principal, projectID, payload.ScenePlanVersion, payload.SceneKey, asset.ID); err != nil {
				return nil, jobs.NewRetryableError(ErrorAssignmentFailed, err, nil)
			}
			assigned = true
		}
	}
	checkpoint, err := h.operations.Get(ctx, principal, projectID, jobID)
	if err != nil {
		return nil, jobs.NewRetryableError(ErrorCheckpointFailed, err, nil)
	}
	result, err := json.Marshal(Result{MediaAssetID: asset.ID, ExternalOperationID: checkpoint.ExternalOperationID, AssignedPrimaryVisual: assigned})
	if err != nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, err)
	}
	return result, nil
}

func classifyVideoProviderError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, providers.ErrRateLimited) || errors.Is(err, providers.ErrTransientExecution) || errors.Is(err, providers.ErrProviderUnavailable) || errors.Is(err, providers.ErrAuthenticationUnavailable) || errors.Is(err, providers.ErrResultUnavailable) {
		return jobs.NewRetryableError(ErrorProviderFailed, err, nil)
	}
	if errors.Is(err, providers.ErrInvalidRequest) || errors.Is(err, providers.ErrMalformedResponse) || errors.Is(err, providers.ErrUnknownProvider) || errors.Is(err, providers.ErrUnknownModel) || errors.Is(err, providers.ErrUnsupportedCapability) || errors.Is(err, providers.ErrUnknownVideoOperation) || errors.Is(err, providers.ErrVideoOperationFailed) {
		return jobs.NewTerminalError(ErrorProviderFailed, err)
	}
	if errors.Is(err, providers.ErrAmbiguousSubmit) {
		return jobs.NewTerminalError(ErrorAmbiguousSubmit, err)
	}
	return jobs.NewRetryableError(ErrorProviderFailed, fmt.Errorf("video provider operation failed: %w", err), nil)
}
