package proposalgenerationjob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

type TextGeneratorResolver interface {
	ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error)
}

type Handler struct {
	engine    *proposalgeneration.Engine
	resolver  TextGeneratorResolver
	proposals creativeproposal.Repository
}

func NewHandler(engine *proposalgeneration.Engine, proposals creativeproposal.Repository) *Handler {
	return &Handler{
		engine:    engine,
		proposals: proposals,
	}
}

func NewHandlerWithResolver(resolver TextGeneratorResolver, proposals creativeproposal.Repository) *Handler {
	return &Handler{
		resolver:  resolver,
		proposals: proposals,
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
	if payload.SchemaVersion != SchemaVersion {
		return nil, jobs.NewTerminalError("GENERATION_INVALID_PAYLOAD", fmt.Errorf("unexpected schema_version: %q", payload.SchemaVersion))
	}

	req := proposalgeneration.Request{
		Project:    payload.Project,
		Brief:      payload.Brief,
		ProviderID: payload.ProviderID,
		ModelID:    payload.ModelID,
	}

	var engine *proposalgeneration.Engine
	if h.resolver != nil {
		generator, err := h.resolver.ResolveTextGenerator(ctx, job.OwnerID, providers.ProviderID(payload.ProviderID), providers.ModelID(payload.ModelID))
		if err != nil {
			return nil, jobs.NewRetryableError("GENERATION_PROVIDER_UNAVAILABLE", err, nil)
		}
		engine = proposalgeneration.NewWithGenerator(generator)
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

		var genErr *proposalgeneration.Error
		if errors.As(err, &genErr) {
			switch genErr.Code {
			case proposalgeneration.CodeInvalidOutput:
				return nil, jobs.NewTerminalError("GENERATION_INVALID_OUTPUT", err)
			case proposalgeneration.CodeProviderUnavailable:
				return nil, jobs.NewRetryableError("GENERATION_PROVIDER_UNAVAILABLE", err, nil)
			case proposalgeneration.CodeProviderFailed:
				return nil, jobs.NewRetryableError("GENERATION_PROVIDER_FAILED", err, nil)
			}
		}
		return nil, jobs.NewRetryableError("GENERATION_FAILED", err, nil)
	}

	structureItems := make([]creativeproposal.StructureItem, len(candidate.Structure))
	for i, item := range candidate.Structure {
		structureItems[i] = creativeproposal.StructureItem{
			Key:     item.Key,
			Title:   item.Title,
			Purpose: item.Purpose,
		}
	}

	created, err := h.proposals.CreateDraft(ctx, job.OwnerID, *job.ProjectID, creativeproposal.CreateDraftInput{
		SourceBriefRevision:   candidate.SourceBriefRevision,
		SourceGenerationJobID: &job.ID,
		Content: creativeproposal.Content{
			TitleOptions:             candidate.TitleOptions,
			HookOptions:              candidate.HookOptions,
			AudienceSummary:          candidate.AudienceSummary,
			ObjectiveSummary:         candidate.ObjectiveSummary,
			NarrativeAngle:           candidate.NarrativeAngle,
			EstimatedDurationSeconds: candidate.EstimatedDurationSeconds,
			FormatRationale:          candidate.FormatRationale,
			Structure:                structureItems,
			VisualDirection:          candidate.VisualDirection,
			VoiceDirection:           candidate.VoiceDirection,
			MusicDirection:           candidate.MusicDirection,
			CaptionDirection:         candidate.CaptionDirection,
			CallToAction:             candidate.CallToAction,
			ResearchGaps:             candidate.ResearchGaps,
			Warnings:                 candidate.Warnings,
		},
	})
	if err != nil {
		return nil, jobs.NewRetryableError("GENERATION_PERSISTENCE_FAILED", err, nil)
	}

	result := Result{
		ProposalVersion: created.Version,
	}
	return json.Marshal(result)
}
