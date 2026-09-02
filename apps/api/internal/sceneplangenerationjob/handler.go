package sceneplangenerationjob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangeneration"
)

type TextGeneratorResolver interface {
	ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error)
}

type Handler struct {
	engine     *sceneplangeneration.Engine
	resolver   TextGeneratorResolver
	scenePlans sceneplan.Repository
}

func NewHandler(engine *sceneplangeneration.Engine, scenePlans sceneplan.Repository) *Handler {
	return &Handler{
		engine:     engine,
		scenePlans: scenePlans,
	}
}

func NewHandlerWithResolver(resolver TextGeneratorResolver, scenePlans sceneplan.Repository) *Handler {
	return &Handler{
		resolver:   resolver,
		scenePlans: scenePlans,
	}
}

func (h *Handler) Handle(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
	if job.ProjectID == nil {
		return nil, jobs.NewTerminalError("GENERATION_INVALID_PAYLOAD", errors.New("job project_id is required"))
	}

	var payload Payload
	dec := json.NewDecoder(bytes.NewReader(job.Payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return nil, jobs.NewTerminalError("GENERATION_INVALID_PAYLOAD", fmt.Errorf("decode payload: %w", err))
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, jobs.NewTerminalError("GENERATION_INVALID_PAYLOAD", errors.New("unexpected trailing content after json payload"))
	}
	if err := validatePayload(&payload, job); err != nil {
		return nil, jobs.NewTerminalError("GENERATION_INVALID_PAYLOAD", fmt.Errorf("validate payload: %w", err))
	}

	req := sceneplangeneration.Request{
		Project:    payload.Project,
		Script:     payload.Script,
		Proposal:   payload.Proposal,
		ProviderID: payload.ProviderID,
		ModelID:    payload.ModelID,
	}

	var engine *sceneplangeneration.Engine
	if h.resolver != nil {
		generator, err := h.resolver.ResolveTextGenerator(ctx, job.OwnerID, providers.ProviderID(payload.ProviderID), providers.ModelID(payload.ModelID))
		if err != nil {
			return nil, jobs.NewRetryableError("GENERATION_PROVIDER_UNAVAILABLE", err, nil)
		}
		engine = sceneplangeneration.NewWithGenerator(generator)
	} else if h.engine != nil {
		engine = h.engine
	} else {
		return nil, jobs.NewRetryableError("GENERATION_PROVIDER_UNAVAILABLE", errors.New("no engine or resolver available"), nil)
	}

	candidate, err := engine.Generate(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		var genErr *sceneplangeneration.Error
		if errors.As(err, &genErr) {
			switch genErr.Code {
			case sceneplangeneration.CodeInvalidOutput:
				return nil, jobs.NewTerminalError("GENERATION_INVALID_OUTPUT", err)
			case sceneplangeneration.CodeProviderUnavailable:
				return nil, jobs.NewRetryableError("GENERATION_PROVIDER_UNAVAILABLE", err, nil)
			case sceneplangeneration.CodeProviderFailed:
				return nil, jobs.NewRetryableError("GENERATION_PROVIDER_FAILED", err, nil)
			}
		}
		return nil, jobs.NewRetryableError("GENERATION_FAILED", err, nil)
	}

	scenes := make([]sceneplan.Scene, len(candidate.Scenes))
	for i, s := range candidate.Scenes {
		scenes[i] = sceneplan.Scene{
			Key:                     s.Key,
			ScriptSectionKey:        s.ScriptSectionKey,
			Narration:               s.Narration,
			VisualInstruction:       s.VisualInstruction,
			PlannedSourceType:       sceneplan.SourceType(s.PlannedSourceType),
			ExpectedDurationSeconds: s.ExpectedDurationSeconds,
			CaptionIntent:           s.CaptionIntent,
			TransitionNotes:         s.TransitionNotes,
		}
	}

	created, err := h.scenePlans.CreateDraft(ctx, job.OwnerID, *job.ProjectID, sceneplan.CreateDraftInput{
		SourceScriptVersion:   candidate.SourceScriptVersion,
		SourceGenerationJobID: &job.ID,
		ContentLocale:         string(payload.Project.Locale),
		Content: sceneplan.Content{
			Scenes: scenes,
		},
	})
	if err != nil {
		return nil, jobs.NewRetryableError("GENERATION_PERSISTENCE_FAILED", err, nil)
	}

	result := Result{
		ScenePlanVersion: created.Version,
	}
	return json.Marshal(result)
}

var sectionKeyRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

func validatePayload(payload *Payload, job jobs.Job) error {
	if payload.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unexpected schema_version: %q", payload.SchemaVersion)
	}
	if strings.TrimSpace(payload.ProviderID) == "" {
		return errors.New("provider_id is required")
	}
	if strings.TrimSpace(payload.ModelID) == "" {
		return errors.New("model_id is required")
	}
	if payload.Project.ID == uuid.Nil {
		return errors.New("project id is required")
	}
	if job.ProjectID == nil || payload.Project.ID != *job.ProjectID {
		return errors.New("payload project id does not match job project id")
	}
	switch payload.Project.ContentFormat {
	case project.ContentFormatShort, project.ContentFormatLong, project.ContentFormatFlexible:
		// valid
	default:
		return fmt.Errorf("invalid project content format: %q", payload.Project.ContentFormat)
	}
	switch payload.Project.AspectRatio {
	case project.AspectRatio16x9, project.AspectRatio9x16, project.AspectRatio1x1, project.AspectRatio4x5:
		// valid
	default:
		return fmt.Errorf("invalid project aspect ratio: %q", payload.Project.AspectRatio)
	}
	if payload.Project.TargetDurationSeconds != nil {
		dur := *payload.Project.TargetDurationSeconds
		if dur < 1 || dur > 43200 {
			return fmt.Errorf("invalid project target duration: %d", dur)
		}
	}
	if payload.Project.Locale != project.LocaleVI && payload.Project.Locale != project.LocaleEN {
		return fmt.Errorf("invalid project locale: %q", payload.Project.Locale)
	}
	if payload.Script.Version < 1 {
		return fmt.Errorf("invalid script version: %d", payload.Script.Version)
	}
	if payload.Script.SourceProposalVersion < 1 {
		return fmt.Errorf("invalid script source proposal version: %d", payload.Script.SourceProposalVersion)
	}
	if len(payload.Script.Sections) == 0 {
		return errors.New("script sections are required")
	}
	if len(payload.Script.Sections) > 200 {
		return fmt.Errorf("script sections exceed maximum (200): %d", len(payload.Script.Sections))
	}
	seenKeys := make(map[string]bool, len(payload.Script.Sections))
	for i, s := range payload.Script.Sections {
		trimmedKey := strings.TrimSpace(s.Key)
		if trimmedKey == "" {
			return fmt.Errorf("script section %d key is required", i)
		}
		if len([]rune(trimmedKey)) > 64 {
			return fmt.Errorf("script section %d key exceeds maximum length (64): %q", i, trimmedKey)
		}
		if !sectionKeyRegex.MatchString(trimmedKey) {
			return fmt.Errorf("script section %d key format is invalid: %q", i, trimmedKey)
		}
		if seenKeys[trimmedKey] {
			return fmt.Errorf("script section %d key is duplicate: %q", i, trimmedKey)
		}
		seenKeys[trimmedKey] = true

		if len([]rune(strings.TrimSpace(s.Heading))) > 300 {
			return fmt.Errorf("script section %d heading exceeds maximum length (300)", i)
		}
		trimmedBody := strings.TrimSpace(s.Body)
		if trimmedBody == "" {
			return fmt.Errorf("script section %d body is required", i)
		}
		if len([]rune(trimmedBody)) > 20000 {
			return fmt.Errorf("script section %d body exceeds maximum length (20000)", i)
		}
	}
	if payload.Script.EstimatedDurationSeconds != nil {
		dur := *payload.Script.EstimatedDurationSeconds
		if dur < 1 || dur > 43200 {
			return fmt.Errorf("invalid script estimated duration: %d", dur)
		}
	}
	if len([]rune(strings.TrimSpace(payload.Script.Notes))) > 10000 {
		return errors.New("script notes exceed maximum length (10000)")
	}

	if payload.Proposal.Version < 1 || payload.Proposal.Version != payload.Script.SourceProposalVersion {
		return fmt.Errorf("invalid proposal version %d for script source proposal version %d", payload.Proposal.Version, payload.Script.SourceProposalVersion)
	}
	if len([]rune(strings.TrimSpace(payload.Proposal.VisualDirection))) > 5000 {
		return errors.New("proposal visual_direction exceeds maximum length (5000)")
	}
	if len([]rune(strings.TrimSpace(payload.Proposal.VoiceDirection))) > 3000 {
		return errors.New("proposal voice_direction exceeds maximum length (3000)")
	}
	if len([]rune(strings.TrimSpace(payload.Proposal.MusicDirection))) > 3000 {
		return errors.New("proposal music_direction exceeds maximum length (3000)")
	}
	if len([]rune(strings.TrimSpace(payload.Proposal.CaptionDirection))) > 3000 {
		return errors.New("proposal caption_direction exceeds maximum length (3000)")
	}
	if len(payload.Proposal.Warnings) > 20 {
		return fmt.Errorf("proposal warnings exceed maximum items (20): %d", len(payload.Proposal.Warnings))
	}
	for i, w := range payload.Proposal.Warnings {
		trimmed := strings.TrimSpace(w)
		if trimmed == "" {
			return fmt.Errorf("proposal warning %d is required", i)
		}
		if len([]rune(trimmed)) > 1000 {
			return fmt.Errorf("proposal warning %d exceeds maximum length (1000)", i)
		}
	}
	if len(payload.Proposal.ResearchGaps) > 20 {
		return fmt.Errorf("proposal research_gaps exceed maximum items (20): %d", len(payload.Proposal.ResearchGaps))
	}
	for i, g := range payload.Proposal.ResearchGaps {
		trimmed := strings.TrimSpace(g)
		if trimmed == "" {
			return fmt.Errorf("proposal research gap %d is required", i)
		}
		if len([]rune(trimmed)) > 1000 {
			return fmt.Errorf("proposal research gap %d exceeds maximum length (1000)", i)
		}
	}
	return nil
}
