package scriptgenerationjob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgeneration"
)

type TextGeneratorResolver interface {
	ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error)
}

type Handler struct {
	engine   *scriptgeneration.Engine
	resolver TextGeneratorResolver
	scripts  script.Repository
}

func NewHandler(engine *scriptgeneration.Engine, scripts script.Repository) *Handler {
	return &Handler{
		engine:  engine,
		scripts: scripts,
	}
}

func NewHandlerWithResolver(resolver TextGeneratorResolver, scripts script.Repository) *Handler {
	return &Handler{
		resolver: resolver,
		scripts:  scripts,
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

	req := scriptgeneration.Request{
		Project:    payload.Project,
		Proposal:   payload.Proposal,
		ProviderID: payload.ProviderID,
		ModelID:    payload.ModelID,
	}

	var engine *scriptgeneration.Engine
	if h.resolver != nil {
		generator, err := h.resolver.ResolveTextGenerator(ctx, job.OwnerID, providers.ProviderID(payload.ProviderID), providers.ModelID(payload.ModelID))
		if err != nil {
			return nil, jobs.NewRetryableError("GENERATION_PROVIDER_UNAVAILABLE", err, nil)
		}
		engine = scriptgeneration.NewWithGenerator(generator)
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

		var genErr *scriptgeneration.Error
		if errors.As(err, &genErr) {
			switch genErr.Code {
			case scriptgeneration.CodeInvalidOutput:
				return nil, jobs.NewTerminalError("GENERATION_INVALID_OUTPUT", err)
			case scriptgeneration.CodeProviderUnavailable:
				return nil, jobs.NewRetryableError("GENERATION_PROVIDER_UNAVAILABLE", err, nil)
			case scriptgeneration.CodeProviderFailed:
				return nil, jobs.NewRetryableError("GENERATION_PROVIDER_FAILED", err, nil)
			}
		}
		return nil, jobs.NewRetryableError("GENERATION_FAILED", err, nil)
	}

	sections := make([]script.Section, len(candidate.Sections))
	for i, s := range candidate.Sections {
		sections[i] = script.Section{
			Key:     s.Key,
			Heading: s.Heading,
			Body:    s.Body,
		}
	}

	created, err := h.scripts.CreateDraft(ctx, job.OwnerID, *job.ProjectID, script.CreateDraftInput{
		SourceProposalVersion: candidate.SourceProposalVersion,
		SourceGenerationJobID: &job.ID,
		ContentLocale:         string(payload.Project.Locale),
		Content: script.Content{
			Sections:                 sections,
			EstimatedDurationSeconds: candidate.EstimatedDurationSeconds,
			Notes:                    candidate.Notes,
		},
	})
	if err != nil {
		return nil, jobs.NewRetryableError("GENERATION_PERSISTENCE_FAILED", err, nil)
	}

	result := Result{
		ScriptVersion: created.Version,
	}
	return json.Marshal(result)
}

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
	if payload.Proposal.Version < 1 {
		return fmt.Errorf("invalid proposal version: %d", payload.Proposal.Version)
	}
	if len(payload.Proposal.Structure) == 0 {
		return errors.New("proposal structure is required")
	}
	for i, s := range payload.Proposal.Structure {
		if strings.TrimSpace(s.Key) == "" {
			return fmt.Errorf("proposal structure item %d key is required", i)
		}
	}
	return nil
}
