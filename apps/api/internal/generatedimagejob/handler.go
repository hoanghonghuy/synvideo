package generatedimagejob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
)

const (
	ErrorProviderUnavailable = "ERR_IMAGE_PROVIDER_UNAVAILABLE"
	ErrorProviderFailed = "ERR_IMAGE_PROVIDER_FAILED"
	ErrorStorageFailed = "ERR_IMAGE_STORAGE_FAILED"
	ErrorAssignmentFailed = "ERR_IMAGE_ASSIGNMENT_FAILED"
	ErrorInvalidPayload = "ERR_IMAGE_JOB_INVALID"
)

type GeneratedAssetStore interface {
	FindGeneratedByJob(context.Context, project.Principal, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
	Store(context.Context, project.Principal, uuid.UUID, mediaasset.CreateInput) (mediaasset.MediaAsset, error)
}

type SceneBinder interface {
	GetCurrent(context.Context, project.Principal, uuid.UUID, int, string) (scenemedia.Binding, error)
	AssignPrimaryVisual(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenemedia.Binding, error)
}

type Handler struct {
	runtime ImageProviderRuntime
	assets GeneratedAssetStore
	bindings SceneBinder
}

func NewHandler(runtime ImageProviderRuntime, assets GeneratedAssetStore, bindings SceneBinder) *Handler {
	return &Handler{runtime: runtime, assets: assets, bindings: bindings}
}

func (h *Handler) Handle(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
	if job.Kind != JobKind || job.OwnerID == uuid.Nil || job.ProjectID == nil || *job.ProjectID == uuid.Nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, errors.New("invalid generated image job identity"))
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.SchemaVersion != SchemaVersion || payload.ProviderID == "" || payload.ModelID == "" || payload.ScenePlanVersion < 1 || payload.SceneKey == "" || payload.Prompt == "" {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, errors.New("invalid generated image job payload"))
	}
	principal := project.Principal{OwnerID: job.OwnerID}
	projectID := *job.ProjectID

	asset, err := h.assets.FindGeneratedByJob(ctx, principal, projectID, job.ID)
	if err != nil && !errors.Is(err, mediaasset.ErrNotFound) {
		return nil, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
	}
	if errors.Is(err, mediaasset.ErrNotFound) {
		asset, err = h.generateAndStore(ctx, principal, projectID, job.ID, payload)
		if err != nil { return nil, err }
	}

	assigned := false
	if payload.AssignPrimaryVisual {
		if h.bindings == nil {
			return nil, jobs.NewRetryableError(ErrorAssignmentFailed, errors.New("scene binding service unavailable"), nil)
		}
		current, currentErr := h.bindings.GetCurrent(ctx, principal, projectID, payload.ScenePlanVersion, payload.SceneKey)
		if currentErr == nil && current.AssetID == asset.ID {
			assigned = true
		} else if currentErr != nil && !errors.Is(currentErr, scenemedia.ErrNotFound) {
			return nil, jobs.NewRetryableError(ErrorAssignmentFailed, currentErr, nil)
		} else {
			if _, assignErr := h.bindings.AssignPrimaryVisual(ctx, principal, projectID, payload.ScenePlanVersion, payload.SceneKey, asset.ID); assignErr != nil {
				return nil, jobs.NewRetryableError(ErrorAssignmentFailed, assignErr, nil)
			}
			assigned = true
		}
	}

	result, err := json.Marshal(Result{MediaAssetID: asset.ID, AssignedPrimaryVisual: assigned})
	if err != nil { return nil, jobs.NewTerminalError(ErrorInvalidPayload, err) }
	return result, nil
}

func (h *Handler) generateAndStore(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID, payload Payload) (mediaasset.MediaAsset, error) {
	if h.runtime == nil || h.assets == nil {
		return mediaasset.MediaAsset{}, jobs.NewTerminalError(ErrorProviderUnavailable, errors.New("generated image runtime unavailable"))
	}
	generator, err := h.runtime.ResolveImageGenerator(ctx, principal.OwnerID, providers.ProviderID(payload.ProviderID), providers.ModelID(payload.ModelID))
	if err != nil || generator == nil {
		return mediaasset.MediaAsset{}, jobs.NewTerminalError(ErrorProviderUnavailable, errors.New("selected image provider unavailable"))
	}
	one := 1
	response, err := generator.GenerateImage(ctx, providers.ImageGenerationRequest{Prompt: payload.Prompt, AspectRatio: payload.AspectRatio, OutputCount: &one})
	if err != nil { return mediaasset.MediaAsset{}, classifyProviderError(err) }
	if err := response.Validate(); err != nil { return mediaasset.MediaAsset{}, jobs.NewTerminalError(ErrorProviderFailed, err) }
	binary := response.Outputs[0].Binary
	reader, err := binary.Open(ctx)
	if err != nil { return mediaasset.MediaAsset{}, classifyProviderError(err) }
	defer reader.Close()

	metadata, err := json.Marshal(map[string]any{
		"origin": "generated", "job_id": jobID.String(), "provider_id": payload.ProviderID, "model_id": payload.ModelID,
		"scene_plan_version": payload.ScenePlanVersion, "scene_key": payload.SceneKey,
	})
	if err != nil { return mediaasset.MediaAsset{}, jobs.NewTerminalError(ErrorInvalidPayload, err) }
	asset, err := h.assets.Store(ctx, principal, projectID, mediaasset.CreateInput{
		Kind: mediaasset.KindImage, Origin: mediaasset.OriginGeneratedImage, MimeType: binary.MIMEType(), Metadata: metadata,
		Reader: reader, MaxBytes: MaxGeneratedImageBytes,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) { return mediaasset.MediaAsset{}, err }
		return mediaasset.MediaAsset{}, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
	}
	return asset, nil
}

func classifyProviderError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) { return err }
	if errors.Is(err, providers.ErrRateLimited) || errors.Is(err, providers.ErrTransientExecution) || errors.Is(err, providers.ErrProviderUnavailable) || errors.Is(err, providers.ErrAuthenticationUnavailable) {
		return jobs.NewRetryableError(ErrorProviderFailed, err, nil)
	}
	if errors.Is(err, providers.ErrInvalidRequest) || errors.Is(err, providers.ErrMalformedResponse) || errors.Is(err, providers.ErrUnknownProvider) || errors.Is(err, providers.ErrUnknownModel) || errors.Is(err, providers.ErrUnsupportedCapability) {
		return jobs.NewTerminalError(ErrorProviderFailed, err)
	}
	return jobs.NewRetryableError(ErrorProviderFailed, fmt.Errorf("provider generation failed: %w", err), nil)
}
