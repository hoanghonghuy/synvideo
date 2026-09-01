package scriptgenerationjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgeneration"
)

var (
	ErrProjectNotFound           = errors.New("project not found")
	ErrApprovedProposalRequired  = errors.New("approved creative proposal is required")
	ErrProviderUnavailable       = errors.New("generation provider unavailable")
	ErrGenerationRequestConflict = errors.New("generation request conflict")
	ErrJobNotFound               = errors.New("job not found")
	ErrInvalidRequestID          = errors.New("invalid request_id")
	ErrUnauthenticated           = errors.New("request principal is required")
)

type CreateScriptGenerationInput struct {
	RequestID  uuid.UUID
	ProviderID string
	ModelID    string
}

type ScriptGenerationJobView struct {
	ID            uuid.UUID `json:"id"`
	State         string    `json:"state"`
	Attempt       int       `json:"attempt"`
	MaxAttempts   int       `json:"max_attempts"`
	ErrorCode     *string   `json:"error_code"`
	ScriptVersion *int      `json:"script_version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProjectRepository interface {
	Get(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (project.Project, error)
}

type CreativeProposalRepository interface {
	List(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error)
}

type TextProviderRuntime interface {
	GetOwnerTextGenerationOptions(ctx context.Context, ownerID uuid.UUID) (providersettings.TextGenerationOptionsResponse, error)
	ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error)
}

type Service struct {
	registry  *providers.Registry
	runtime   TextProviderRuntime
	jobsRepo  jobs.Repository
	projects  ProjectRepository
	proposals CreativeProposalRepository
}

func NewService(
	registry *providers.Registry,
	jobsRepo jobs.Repository,
	projects ProjectRepository,
	proposals CreativeProposalRepository,
) *Service {
	return &Service{
		registry:  registry,
		jobsRepo:  jobsRepo,
		projects:  projects,
		proposals: proposals,
	}
}

func NewServiceWithRuntime(
	runtime TextProviderRuntime,
	jobsRepo jobs.Repository,
	projects ProjectRepository,
	proposals CreativeProposalRepository,
) *Service {
	return &Service{
		runtime:   runtime,
		jobsRepo:  jobsRepo,
		projects:  projects,
		proposals: proposals,
	}
}

func (s *Service) CreateGeneration(
	ctx context.Context,
	principal project.Principal,
	projectID uuid.UUID,
	input CreateScriptGenerationInput,
) (ScriptGenerationJobView, error) {
	if principal.OwnerID == uuid.Nil {
		return ScriptGenerationJobView{}, ErrUnauthenticated
	}
	if input.RequestID == uuid.Nil {
		return ScriptGenerationJobView{}, ErrInvalidRequestID
	}
	if input.ProviderID == "" || input.ModelID == "" {
		return ScriptGenerationJobView{}, ErrProviderUnavailable
	}

	// 1. Check if a job with input.RequestID already exists (for idempotent replay)
	existingJob, err := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID)
	if err == nil {
		if valErr := validateExistingJob(existingJob, projectID, input); valErr != nil {
			return ScriptGenerationJobView{}, valErr
		}
		return ToJobView(existingJob), nil
	} else if !errors.Is(err, jobs.ErrJobNotFound) {
		return ScriptGenerationJobView{}, err
	}

	// 2. Verify project exists and is visible to principal
	proj, err := s.projects.Get(ctx, principal.OwnerID, projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return ScriptGenerationJobView{}, ErrProjectNotFound
		}
		return ScriptGenerationJobView{}, err
	}

	// 3. Load creative proposals and pick highest approved version
	proposals, err := s.proposals.List(ctx, principal.OwnerID, projectID)
	if err != nil {
		if errors.Is(err, creativeproposal.ErrNotFound) {
			return ScriptGenerationJobView{}, ErrProjectNotFound
		}
		return ScriptGenerationJobView{}, err
	}

	var approvedProp *creativeproposal.CreativeProposal
	for i := range proposals {
		if proposals[i].Status == creativeproposal.StatusApproved {
			if approvedProp == nil || proposals[i].Version > approvedProp.Version {
				approvedProp = &proposals[i]
			}
		}
	}
	if approvedProp == nil {
		return ScriptGenerationJobView{}, ErrApprovedProposalRequired
	}

	// 4. Resolve selected text-generation capability through runtime or registry
	if s.runtime != nil {
		generator, err := s.runtime.ResolveTextGenerator(ctx, principal.OwnerID, providers.ProviderID(input.ProviderID), providers.ModelID(input.ModelID))
		if err != nil || generator == nil {
			return ScriptGenerationJobView{}, ErrProviderUnavailable
		}
	} else if s.registry != nil {
		generator, _, err := s.registry.ResolveTextGenerator(providers.ProviderID(input.ProviderID), providers.ModelID(input.ModelID))
		if err != nil || generator == nil {
			return ScriptGenerationJobView{}, ErrProviderUnavailable
		}
	} else {
		return ScriptGenerationJobView{}, ErrProviderUnavailable
	}

	// 5. Build snapshot payload
	propStructure := make([]scriptgeneration.ProposalStructureItem, len(approvedProp.Structure))
	for i, item := range approvedProp.Structure {
		propStructure[i] = scriptgeneration.ProposalStructureItem{
			Key:     item.Key,
			Title:   item.Title,
			Purpose: item.Purpose,
		}
	}

	payload := Payload{
		SchemaVersion: SchemaVersion,
		ProviderID:    input.ProviderID,
		ModelID:       input.ModelID,
		Project: scriptgeneration.ProjectContext{
			ID:                    proj.ID,
			ContentFormat:         proj.ContentFormat,
			AspectRatio:           proj.AspectRatio,
			TargetDurationSeconds: proj.TargetDurationSeconds,
			Locale:                proj.Locale,
		},
		Proposal: scriptgeneration.ProposalContext{
			Version:                  approvedProp.Version,
			TitleOptions:             approvedProp.TitleOptions,
			HookOptions:              approvedProp.HookOptions,
			AudienceSummary:          approvedProp.AudienceSummary,
			ObjectiveSummary:         approvedProp.ObjectiveSummary,
			NarrativeAngle:           approvedProp.NarrativeAngle,
			EstimatedDurationSeconds: approvedProp.EstimatedDurationSeconds,
			FormatRationale:          approvedProp.FormatRationale,
			Structure:                propStructure,
			VisualDirection:          approvedProp.VisualDirection,
			VoiceDirection:           approvedProp.VoiceDirection,
			MusicDirection:           approvedProp.MusicDirection,
			CaptionDirection:         approvedProp.CaptionDirection,
			CallToAction:             approvedProp.CallToAction,
			ResearchGaps:             approvedProp.ResearchGaps,
			Warnings:                 approvedProp.Warnings,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return ScriptGenerationJobView{}, fmt.Errorf("marshal payload: %w", err)
	}

	enqueuedJob, err := s.jobsRepo.Enqueue(ctx, jobs.EnqueueInput{
		ID:          input.RequestID,
		OwnerID:     principal.OwnerID,
		ProjectID:   &projectID,
		Kind:        JobKind,
		MaxAttempts: 3,
		Payload:     payloadBytes,
	})
	if err != nil {
		if errors.Is(err, jobs.ErrDuplicateJob) {
			// Concurrent race or duplicate key: retrieve and validate parameters
			raceJob, getErr := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID)
			if getErr == nil {
				if valErr := validateExistingJob(raceJob, projectID, input); valErr != nil {
					return ScriptGenerationJobView{}, valErr
				}
				return ToJobView(raceJob), nil
			}
			return ScriptGenerationJobView{}, ErrGenerationRequestConflict
		}
		return ScriptGenerationJobView{}, fmt.Errorf("enqueue generation job: %w", err)
	}

	return ToJobView(enqueuedJob), nil
}

func validateExistingJob(existingJob jobs.Job, projectID uuid.UUID, input CreateScriptGenerationInput) error {
	if existingJob.ProjectID == nil || *existingJob.ProjectID != projectID {
		return ErrGenerationRequestConflict
	}
	if existingJob.Kind != JobKind {
		return ErrGenerationRequestConflict
	}
	var existingPayload Payload
	if err := json.Unmarshal(existingJob.Payload, &existingPayload); err != nil {
		return ErrGenerationRequestConflict
	}
	if existingPayload.ProviderID != input.ProviderID || existingPayload.ModelID != input.ModelID {
		return ErrGenerationRequestConflict
	}
	return nil
}

func (s *Service) GetGeneration(
	ctx context.Context,
	principal project.Principal,
	projectID uuid.UUID,
	jobID uuid.UUID,
) (ScriptGenerationJobView, error) {
	if principal.OwnerID == uuid.Nil {
		return ScriptGenerationJobView{}, ErrUnauthenticated
	}

	job, err := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, jobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return ScriptGenerationJobView{}, ErrJobNotFound
		}
		return ScriptGenerationJobView{}, err
	}

	return ToJobView(job), nil
}

func ToJobView(job jobs.Job) ScriptGenerationJobView {
	view := ScriptGenerationJobView{
		ID:          job.ID,
		State:       string(job.State),
		Attempt:     job.Attempt,
		MaxAttempts: job.MaxAttempts,
		ErrorCode:   job.ErrorCode,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}

	if job.State == jobs.StateSucceeded && len(job.Result) > 0 {
		var res Result
		if err := json.Unmarshal(job.Result, &res); err == nil && res.ScriptVersion > 0 {
			v := res.ScriptVersion
			view.ScriptVersion = &v
		}
	}

	return view
}
