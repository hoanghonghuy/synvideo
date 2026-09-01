package proposalgenerationjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

var (
	ErrProjectNotFound           = errors.New("project not found")
	ErrCreativeBriefRequired     = errors.New("creative brief is required")
	ErrProviderUnavailable       = errors.New("generation provider unavailable")
	ErrGenerationRequestConflict = errors.New("generation request conflict")
	ErrJobNotFound               = errors.New("job not found")
	ErrInvalidRequestID          = errors.New("invalid request_id")
	ErrUnauthenticated           = errors.New("request principal is required")
)

type TextGenerationOptionModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type TextGenerationOptionProvider struct {
	ID          string                      `json:"id"`
	DisplayName string                      `json:"display_name"`
	Models      []TextGenerationOptionModel `json:"models"`
}

type TextGenerationOptionsResponse struct {
	Providers []TextGenerationOptionProvider `json:"providers"`
}

type CreateProposalGenerationInput struct {
	RequestID  uuid.UUID
	ProviderID string
	ModelID    string
}

type ProposalGenerationJobView struct {
	ID              uuid.UUID  `json:"id"`
	State           string     `json:"state"`
	Attempt         int        `json:"attempt"`
	MaxAttempts     int        `json:"max_attempts"`
	ErrorCode       *string    `json:"error_code"`
	ProposalVersion *int       `json:"proposal_version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

type ProjectRepository interface {
	Get(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (project.Project, error)
}

type CreativeBriefRepository interface {
	Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) (creativebrief.CreativeBrief, error)
}

type Service struct {
	registry *providers.Registry
	jobsRepo jobs.Repository
	projects ProjectRepository
	briefs   CreativeBriefRepository
}

func NewService(
	registry *providers.Registry,
	jobsRepo jobs.Repository,
	projects ProjectRepository,
	briefs CreativeBriefRepository,
) *Service {
	return &Service{
		registry: registry,
		jobsRepo: jobsRepo,
		projects: projects,
		briefs:   briefs,
	}
}

func (s *Service) GetTextGenerationOptions(ctx context.Context) (TextGenerationOptionsResponse, error) {
	if s.registry == nil {
		return TextGenerationOptionsResponse{Providers: []TextGenerationOptionProvider{}}, nil
	}

	providerMetaList := s.registry.ListProviders()
	var providerOptions []TextGenerationOptionProvider

	for _, prov := range providerMetaList {
		modelsMetaList, err := s.registry.ListModels(prov.ID)
		if err != nil {
			continue
		}
		var textModels []TextGenerationOptionModel
		for _, mod := range modelsMetaList {
			if mod.Supports(providers.CapabilityTextGeneration) {
				textModels = append(textModels, TextGenerationOptionModel{
					ID:          string(mod.ID),
					DisplayName: mod.DisplayName,
				})
			}
		}
		if len(textModels) > 0 {
			sort.Slice(textModels, func(i, j int) bool {
				return textModels[i].ID < textModels[j].ID
			})
			providerOptions = append(providerOptions, TextGenerationOptionProvider{
				ID:          string(prov.ID),
				DisplayName: prov.DisplayName,
				Models:      textModels,
			})
		}
	}

	sort.Slice(providerOptions, func(i, j int) bool {
		return providerOptions[i].ID < providerOptions[j].ID
	})

	if providerOptions == nil {
		providerOptions = []TextGenerationOptionProvider{}
	}

	return TextGenerationOptionsResponse{
		Providers: providerOptions,
	}, nil
}

func (s *Service) CreateGeneration(
	ctx context.Context,
	principal project.Principal,
	projectID uuid.UUID,
	input CreateProposalGenerationInput,
) (ProposalGenerationJobView, error) {
	if principal.OwnerID == uuid.Nil {
		return ProposalGenerationJobView{}, ErrUnauthenticated
	}
	if input.RequestID == uuid.Nil {
		return ProposalGenerationJobView{}, ErrInvalidRequestID
	}
	if input.ProviderID == "" || input.ModelID == "" {
		return ProposalGenerationJobView{}, ErrProviderUnavailable
	}

	// 1. Verify project exists and is visible to principal
	proj, err := s.projects.Get(ctx, principal.OwnerID, projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return ProposalGenerationJobView{}, ErrProjectNotFound
		}
		return ProposalGenerationJobView{}, err
	}

	// 2. Load current creative brief under same owner scope
	brief, err := s.briefs.Get(ctx, principal.OwnerID, projectID)
	if err != nil {
		if errors.Is(err, creativebrief.ErrNotFound) {
			return ProposalGenerationJobView{}, ErrCreativeBriefRequired
		}
		return ProposalGenerationJobView{}, err
	}

	// 3. Resolve selected text-generation capability through accepted provider registry
	if s.registry == nil {
		return ProposalGenerationJobView{}, ErrProviderUnavailable
	}
	generator, _, err := s.registry.ResolveTextGenerator(providers.ProviderID(input.ProviderID), providers.ModelID(input.ModelID))
	if err != nil || generator == nil {
		return ProposalGenerationJobView{}, ErrProviderUnavailable
	}

	// 4. Check if a job with input.RequestID already exists
	existingJob, err := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID)
	if err == nil {
		// Existing job found: verify parameters match for idempotency
		if existingJob.Kind != JobKind {
			return ProposalGenerationJobView{}, ErrGenerationRequestConflict
		}
		var existingPayload Payload
		if err := json.Unmarshal(existingJob.Payload, &existingPayload); err != nil {
			return ProposalGenerationJobView{}, ErrGenerationRequestConflict
		}
		if existingPayload.ProviderID != input.ProviderID || existingPayload.ModelID != input.ModelID {
			return ProposalGenerationJobView{}, ErrGenerationRequestConflict
		}
		return ToJobView(existingJob), nil
	} else if !errors.Is(err, jobs.ErrJobNotFound) {
		return ProposalGenerationJobView{}, err
	}

	// 5. Build snapshot payload
	payload := Payload{
		SchemaVersion: SchemaVersion,
		ProviderID:    input.ProviderID,
		ModelID:       input.ModelID,
		Project: proposalgeneration.ProjectContext{
			ID:                    proj.ID,
			ContentFormat:         proj.ContentFormat,
			AspectRatio:           proj.AspectRatio,
			TargetDurationSeconds: proj.TargetDurationSeconds,
			Locale:                proj.Locale,
		},
		Brief: proposalgeneration.BriefContext{
			Revision:            brief.Revision,
			SourceText:          brief.SourceText,
			TargetAudience:      brief.TargetAudience,
			Objective:           brief.Objective,
			DesiredStyle:        brief.DesiredStyle,
			Tone:                brief.Tone,
			DistributionTargets: brief.DistributionTargets,
			CallToAction:        brief.CallToAction,
			MustInclude:         brief.MustInclude,
			MustAvoid:           brief.MustAvoid,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return ProposalGenerationJobView{}, fmt.Errorf("marshal payload: %w", err)
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
			// Concurrent race: retrieve and inspect
			raceJob, getErr := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID)
			if getErr == nil {
				return ToJobView(raceJob), nil
			}
		}
		return ProposalGenerationJobView{}, fmt.Errorf("enqueue generation job: %w", err)
	}

	return ToJobView(enqueuedJob), nil
}

func (s *Service) GetGeneration(
	ctx context.Context,
	principal project.Principal,
	projectID uuid.UUID,
	jobID uuid.UUID,
) (ProposalGenerationJobView, error) {
	if principal.OwnerID == uuid.Nil {
		return ProposalGenerationJobView{}, ErrUnauthenticated
	}

	job, err := s.jobsRepo.GetByIDForProject(ctx, principal.OwnerID, projectID, jobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return ProposalGenerationJobView{}, ErrJobNotFound
		}
		return ProposalGenerationJobView{}, err
	}

	return ToJobView(job), nil
}

func ToJobView(job jobs.Job) ProposalGenerationJobView {
	view := ProposalGenerationJobView{
		ID:          job.ID,
		State:       string(job.State),
		Attempt:     job.Attempt,
		MaxAttempts: job.MaxAttempts,
		ErrorCode:   job.ErrorCode,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		StartedAt:   job.StartedAt,
		FinishedAt:  job.FinishedAt,
	}

	if job.State == jobs.StateSucceeded && len(job.Result) > 0 {
		var res Result
		if err := json.Unmarshal(job.Result, &res); err == nil && res.ProposalVersion > 0 {
			v := res.ProposalVersion
			view.ProposalVersion = &v
		}
	}

	return view
}
