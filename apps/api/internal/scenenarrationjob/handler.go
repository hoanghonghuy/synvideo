package scenenarrationjob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/audio"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
)

const (
	ErrorProviderUnavailable = "ERR_TTS_PROVIDER_UNAVAILABLE"
	ErrorProviderFailed      = "ERR_TTS_PROVIDER_FAILED"
	ErrorStorageFailed       = "ERR_TTS_STORAGE_FAILED"
	ErrorAssignmentFailed    = "ERR_TTS_ASSIGNMENT_FAILED"
	ErrorInvalidPayload      = "ERR_TTS_JOB_INVALID"
)

type GeneratedAssetStore interface {
	FindGeneratedByJob(context.Context, project.Principal, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
	Store(context.Context, project.Principal, uuid.UUID, mediaasset.CreateInput) (mediaasset.MediaAsset, error)
}

type SceneNarrationBinder interface {
	GetActive(context.Context, project.Principal, uuid.UUID, int, string) (scenenarration.Binding, error)
	AssignNarration(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenenarration.Binding, error)
}

type Handler struct {
	runtime    TTSProviderRuntime
	assets     GeneratedAssetStore
	bindings   SceneNarrationBinder
	chunkStore ChunkStore
}

func NewHandler(runtime TTSProviderRuntime, assets GeneratedAssetStore, bindings SceneNarrationBinder, chunkStore ChunkStore) *Handler {
	return &Handler{
		runtime:    runtime,
		assets:     assets,
		bindings:   bindings,
		chunkStore: chunkStore,
	}
}

func (h *Handler) Handle(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
	if job.Kind != JobKind || job.OwnerID == uuid.Nil || job.ProjectID == nil || *job.ProjectID == uuid.Nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, errors.New("invalid scene narration job identity"))
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.SchemaVersion != SchemaVersion ||
		payload.ProviderID == "" || payload.ModelID == "" || payload.VoiceID == "" ||
		payload.ScenePlanVersion < 1 || payload.SceneKey == "" || strings.TrimSpace(payload.NarrationText) == "" {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, errors.New("invalid scene narration job payload"))
	}

	principal := project.Principal{OwnerID: job.OwnerID}
	projectID := *job.ProjectID

	asset, err := h.assets.FindGeneratedByJob(ctx, principal, projectID, job.ID)
	if err != nil && !errors.Is(err, mediaasset.ErrNotFound) {
		return nil, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
	}

	var duration float64
	if errors.Is(err, mediaasset.ErrNotFound) {
		var genErr error
		asset, duration, genErr = h.synthesizeAndStore(ctx, principal, projectID, job.ID, payload)
		if genErr != nil {
			return nil, genErr
		}
	} else {
		// Read duration from asset metadata
		var meta struct {
			DurationSeconds float64 `json:"duration_seconds"`
		}
		if json.Unmarshal(asset.Metadata, &meta) == nil {
			duration = meta.DurationSeconds
		}
	}

	assigned := false
	if payload.AssignCurrent {
		if h.bindings == nil {
			return nil, jobs.NewRetryableError(ErrorAssignmentFailed, errors.New("scene narration binding service unavailable"), nil)
		}
		active, activeErr := h.bindings.GetActive(ctx, principal, projectID, payload.ScenePlanVersion, payload.SceneKey)
		if activeErr == nil && active.AssetID == asset.ID {
			assigned = true
		} else if activeErr != nil && !errors.Is(activeErr, scenenarration.ErrNotFound) {
			return nil, jobs.NewRetryableError(ErrorAssignmentFailed, activeErr, nil)
		} else {
			if _, assignErr := h.bindings.AssignNarration(ctx, principal, projectID, payload.ScenePlanVersion, payload.SceneKey, asset.ID); assignErr != nil {
				return nil, jobs.NewRetryableError(ErrorAssignmentFailed, assignErr, nil)
			}
			assigned = true
		}
	}

	result, err := json.Marshal(Result{
		MediaAssetID:      asset.ID,
		DurationSeconds:   duration,
		AssignedNarration: assigned,
	})
	if err != nil {
		return nil, jobs.NewTerminalError(ErrorInvalidPayload, err)
	}
	return result, nil
}

func (h *Handler) synthesizeAndStore(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID, payload Payload) (mediaasset.MediaAsset, float64, error) {
	if h.runtime == nil || h.assets == nil {
		return mediaasset.MediaAsset{}, 0, jobs.NewTerminalError(ErrorProviderUnavailable, errors.New("tts runtime unavailable"))
	}
	synthesizer, err := h.runtime.ResolveSpeechSynthesizer(ctx, principal.OwnerID, providers.ProviderID(payload.ProviderID), providers.ModelID(payload.ModelID))
	if err != nil || synthesizer == nil {
		return mediaasset.MediaAsset{}, 0, jobs.NewTerminalError(ErrorProviderUnavailable, errors.New("selected tts provider unavailable"))
	}

	format := providers.AudioFormat(strings.ToLower(payload.Format))
	if format == "" {
		format = providers.AudioFormatMP3
	}

	chunks := audio.ChunkText(payload.NarrationText, DefaultMaxChunkRunes)
	if len(chunks) == 0 {
		return mediaasset.MediaAsset{}, 0, jobs.NewTerminalError(ErrorInvalidPayload, errors.New("empty narration chunks"))
	}

	var audioChunks [][]byte
	for i, chunkText := range chunks {
		// Try recovering chunk from chunkStore if available
		if h.chunkStore != nil {
			cached, err := h.chunkStore.GetChunk(ctx, projectID, jobID, i)
			if err != nil {
				return mediaasset.MediaAsset{}, 0, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
			}
			if len(cached) > 0 {
				audioChunks = append(audioChunks, cached)
				continue
			}
		}

		resp, err := synthesizer.SynthesizeSpeech(ctx, providers.SpeechSynthesisRequest{
			Text:    chunkText,
			VoiceID: providers.VoiceID(payload.VoiceID),
			Locale:  payload.Locale,
			Format:  format,
		})
		if err != nil {
			return mediaasset.MediaAsset{}, 0, classifyTTSProviderError(err)
		}
		if err := resp.Validate(); err != nil {
			return mediaasset.MediaAsset{}, 0, jobs.NewTerminalError(ErrorProviderFailed, err)
		}

		reader, err := resp.Audio.Open(ctx)
		if err != nil {
			return mediaasset.MediaAsset{}, 0, classifyTTSProviderError(err)
		}
		chunkBytes, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return mediaasset.MediaAsset{}, 0, classifyTTSProviderError(err)
		}

		if h.chunkStore != nil {
			if err := h.chunkStore.PutChunk(ctx, projectID, jobID, i, chunkBytes); err != nil {
				return mediaasset.MediaAsset{}, 0, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
			}
		}
		audioChunks = append(audioChunks, chunkBytes)
	}

	mimeType := format.MIMEType()
	stitchedAudio, duration, err := audio.StitchAudio(mimeType, audioChunks)
	if err != nil {
		return mediaasset.MediaAsset{}, 0, jobs.NewTerminalError(ErrorProviderFailed, fmt.Errorf("stitch audio chunks: %w", err))
	}

	metadata, err := json.Marshal(map[string]any{
		"origin":             "generated_audio",
		"job_id":             jobID.String(),
		"provider_id":        payload.ProviderID,
		"model_id":           payload.ModelID,
		"voice_id":           payload.VoiceID,
		"scene_plan_version": payload.ScenePlanVersion,
		"scene_key":          payload.SceneKey,
		"duration_seconds":   duration,
	})
	if err != nil {
		return mediaasset.MediaAsset{}, 0, jobs.NewTerminalError(ErrorInvalidPayload, err)
	}

	asset, err := h.assets.Store(ctx, principal, projectID, mediaasset.CreateInput{
		Kind:     mediaasset.KindAudio,
		Origin:   mediaasset.OriginGeneratedAudio,
		MimeType: mimeType,
		Metadata: metadata,
		Reader:   bytes.NewReader(stitchedAudio),
		MaxBytes: MaxGeneratedAudioBytes,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return mediaasset.MediaAsset{}, 0, err
		}
		return mediaasset.MediaAsset{}, 0, jobs.NewRetryableError(ErrorStorageFailed, err, nil)
	}

	if h.chunkStore != nil {
		_ = h.chunkStore.DeleteChunks(ctx, projectID, jobID, len(chunks))
	}

	return asset, duration, nil
}

func classifyTTSProviderError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, providers.ErrRateLimited) || errors.Is(err, providers.ErrTransientExecution) || errors.Is(err, providers.ErrProviderUnavailable) || errors.Is(err, providers.ErrAuthenticationUnavailable) {
		return jobs.NewRetryableError(ErrorProviderFailed, err, nil)
	}
	if errors.Is(err, providers.ErrInvalidRequest) || errors.Is(err, providers.ErrMalformedResponse) || errors.Is(err, providers.ErrUnknownProvider) || errors.Is(err, providers.ErrUnknownModel) || errors.Is(err, providers.ErrUnsupportedCapability) {
		return jobs.NewTerminalError(ErrorProviderFailed, err)
	}
	return jobs.NewRetryableError(ErrorProviderFailed, fmt.Errorf("tts generation failed: %w", err), nil)
}
