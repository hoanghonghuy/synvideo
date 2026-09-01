package sceneplangenerationjob

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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

var (
	ErrProjectNotFound           = errors.New("project not found")
	ErrScriptApprovalRequired    = errors.New("approved script is required")
	ErrScenePlanSourceInvalid    = errors.New("scene plan source is invalid")
	ErrProviderUnavailable       = errors.New("generation provider unavailable")
	ErrGenerationRequestConflict = errors.New("generation request conflict")
	ErrJobNotFound               = errors.New("job not found")
	ErrInvalidRequestID          = errors.New("invalid request_id")
	ErrUnauthenticated           = errors.New("request principal is required")
)

type CreateScenePlanGenerationInput struct {
	RequestID  uuid.UUID
	ProviderID string
	ModelID    string
}

type ScenePlanGenerationJobView struct {
	ID               uuid.UUID `json:"id"`
	State            string    `json:"state"`
	Attempt          int       `json:"attempt"`
	MaxAttempts      int       `json:"max_attempts"`
	ErrorCode        *string   `json:"error_code"`
	ScenePlanVersion *int      `json:"scene_plan_version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ProjectRepository interface {
	Get(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (project.Project, error)
}

type ScriptRepository interface {
	ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]script.Script, error)
	GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (script.Script, error)
}

type CreativeProposalRepository interface {
	List(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error)
	Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (creativeproposal.CreativeProposal, error)
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
	scripts   ScriptRepository
	proposals CreativeProposalRepository
}

func NewService(
	registry *providers.Registry,
	jobsRepo jobs.Repository,
	projects ProjectRepository,
	scripts ScriptRepository,
	proposals CreativeProposalRepository,
) *Service {
	return &Service{
		registry:  registry,
		jobsRepo:  jobsRepo,
		projects:  projects,
		scripts:   scripts,
		proposals: proposals,
	}
}

func NewServiceWithRuntime(
	runtime TextProviderRuntime,
	jobsRepo jobs.Repository,
	projects ProjectRepository,
	scripts ScriptRepository,
	proposals CreativeProposalRepository,
) *Service {
	return &Service{
		runtime:   runtime,
		jobsRepo:  jobsRepo,
		projects:  projects,
		scripts:   scripts,
		proposals: proposals,
	}
}

func (s *Service) CreateGeneration(
	ctx context.Context,
	principal project.Principal,
	projectID uuid.UUID,
	input CreateScenePlanGenerationInput,
) (ScenePlanGenerationJobView, error) {
	if principal.OwnerID == uuid.Nil {
		return ScenePlanGenerationJobView{}, ErrUnauthenticated
	}
	if input.RequestID == uuid.Nil {
		return ScenePlanGenerationJobView{}, ErrInvalidRequestID
	}
	if input.ProviderID == "" || input.ModelID == "" {
		return ScenePlanGenerationJobView{}, ErrProviderUnavailable
	}

	// 1. Check if a job with input.RequestID already exists (for idempotent replay)
	existingJob, err := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID)
	if err == nil {
		if valErr := validateExistingJob(existingJob, projectID, input); valErr != nil {
			return ScenePlanGenerationJobView{}, valErr
		}
		return ToJobView(existingJob), nil
	} else if !errors.Is(err, jobs.ErrJobNotFound) {
		return ScenePlanGenerationJobView{}, err
	}

	// 2. Verify project exists and is visible to principal
	proj, err := s.projects.Get(ctx, principal.OwnerID, projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return ScenePlanGenerationJobView{}, ErrProjectNotFound
		}
		return ScenePlanGenerationJobView{}, err
	}

	// 3. Load scripts and pick highest approved version
	scriptsList, err := s.scripts.ListVersions(ctx, principal.OwnerID, projectID)
	if err != nil {
		if errors.Is(err, script.ErrNotFound) {
			return ScenePlanGenerationJobView{}, ErrProjectNotFound
		}
		return ScenePlanGenerationJobView{}, err
	}

	var approvedScript *script.Script
	for i := range scriptsList {
		if scriptsList[i].Status == script.StatusApproved {
			if approvedScript == nil || scriptsList[i].Version > approvedScript.Version {
				approvedScript = &scriptsList[i]
			}
		}
	}
	if approvedScript == nil {
		return ScenePlanGenerationJobView{}, ErrScriptApprovalRequired
	}

	// 4. Load matching approved proposal
	if approvedScript.SourceProposalVersion < 1 {
		return ScenePlanGenerationJobView{}, ErrScenePlanSourceInvalid
	}
	matchingProposal, err := s.proposals.Get(ctx, principal.OwnerID, projectID, approvedScript.SourceProposalVersion)
	if err != nil {
		if errors.Is(err, creativeproposal.ErrNotFound) {
			return ScenePlanGenerationJobView{}, ErrScenePlanSourceInvalid
		}
		return ScenePlanGenerationJobView{}, err
	}
	if matchingProposal.Status != creativeproposal.StatusApproved || matchingProposal.Version != approvedScript.SourceProposalVersion {
		return ScenePlanGenerationJobView{}, ErrScenePlanSourceInvalid
	}

	// 5. Resolve selected text-generation capability through runtime or registry
	if s.runtime != nil {
		generator, err := s.runtime.ResolveTextGenerator(ctx, principal.OwnerID, providers.ProviderID(input.ProviderID), providers.ModelID(input.ModelID))
		if err != nil || generator == nil {
			return ScenePlanGenerationJobView{}, ErrProviderUnavailable
		}
	} else if s.registry != nil {
		generator, _, err := s.registry.ResolveTextGenerator(providers.ProviderID(input.ProviderID), providers.ModelID(input.ModelID))
		if err != nil || generator == nil {
			return ScenePlanGenerationJobView{}, ErrProviderUnavailable
		}
	} else {
		return ScenePlanGenerationJobView{}, ErrProviderUnavailable
	}

	// 6. Build snapshot payload
	scriptSections := make([]sceneplangeneration.ScriptSection, len(approvedScript.Sections))
	for i, s := range approvedScript.Sections {
		scriptSections[i] = sceneplangeneration.ScriptSection{
			Key:     s.Key,
			Heading: s.Heading,
			Body:    s.Body,
		}
	}

	payload := Payload{
		SchemaVersion: SchemaVersion,
		ProviderID:    input.ProviderID,
		ModelID:       input.ModelID,
		Project: sceneplangeneration.ProjectContext{
			ID:                    proj.ID,
			ContentFormat:         proj.ContentFormat,
			AspectRatio:           proj.AspectRatio,
			TargetDurationSeconds: proj.TargetDurationSeconds,
			Locale:                proj.Locale,
		},
		Script: sceneplangeneration.ScriptContext{
			Version:                  approvedScript.Version,
			SourceProposalVersion:    approvedScript.SourceProposalVersion,
			Sections:                 scriptSections,
			EstimatedDurationSeconds: approvedScript.EstimatedDurationSeconds,
			Notes:                    approvedScript.Notes,
		},
		Proposal: sceneplangeneration.ProposalContext{
			Version:          matchingProposal.Version,
			VisualDirection:  matchingProposal.VisualDirection,
			VoiceDirection:   matchingProposal.VoiceDirection,
			MusicDirection:   matchingProposal.MusicDirection,
			CaptionDirection: matchingProposal.CaptionDirection,
			Warnings:         matchingProposal.Warnings,
			ResearchGaps:     matchingProposal.ResearchGaps,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return ScenePlanGenerationJobView{}, fmt.Errorf("marshal payload: %w", err)
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
			raceJob, getErr := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID)
			if getErr == nil {
				if valErr := validateExistingJob(raceJob, projectID, input); valErr != nil {
					return ScenePlanGenerationJobView{}, valErr
				}
				return ToJobView(raceJob), nil
			}
			return ScenePlanGenerationJobView{}, ErrGenerationRequestConflict
		}
		return ScenePlanGenerationJobView{}, fmt.Errorf("enqueue generation job: %w", err)
	}

	return ToJobView(enqueuedJob), nil
}

func validateExistingJob(existingJob jobs.Job, projectID uuid.UUID, input CreateScenePlanGenerationInput) error {
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
) (ScenePlanGenerationJobView, error) {
	if principal.OwnerID == uuid.Nil {
		return ScenePlanGenerationJobView{}, ErrUnauthenticated
	}

	job, err := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, jobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return ScenePlanGenerationJobView{}, ErrJobNotFound
		}
		return ScenePlanGenerationJobView{}, err
	}

	return ToJobView(job), nil
}

func ToJobView(job jobs.Job) ScenePlanGenerationJobView {
	view := ScenePlanGenerationJobView{
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
		if err := json.Unmarshal(job.Result, &res); err == nil && res.ScenePlanVersion > 0 {
			v := res.ScenePlanVersion
			view.ScenePlanVersion = &v
		}
	}

	return view
}
