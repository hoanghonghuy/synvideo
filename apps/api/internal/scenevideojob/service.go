package scenevideojob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

var (
	ErrProjectNotFound           = errors.New("project not found")
	ErrScenePlanNotFound         = errors.New("scene plan not found")
	ErrScenePlanNotApproved      = errors.New("scene plan is not approved")
	ErrSceneKeyNotFound          = errors.New("scene key not found")
	ErrProviderUnavailable       = errors.New("generation provider unavailable")
	ErrGenerationRequestConflict = errors.New("generation request conflict")
	ErrJobNotFound               = errors.New("job not found")
	ErrInvalidRequestID          = errors.New("invalid request_id")
	ErrUnauthenticated           = errors.New("request principal is required")
)

type CreateGenerationInput struct {
	RequestID           uuid.UUID
	ProviderID          string
	ModelID             string
	DurationSeconds     *int
	AssignPrimaryVisual bool
}

type JobView struct {
	ID                    uuid.UUID  `json:"id"`
	State                 string     `json:"state"`
	Attempt               int        `json:"attempt"`
	MaxAttempts           int        `json:"max_attempts"`
	ErrorCode             *string    `json:"error_code"`
	MediaAssetID          *uuid.UUID `json:"media_asset_id"`
	ExternalOperationID   *string    `json:"external_operation_id,omitempty"`
	AssignedPrimaryVisual bool       `json:"assigned_primary_visual"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type ProjectRepository interface {
	Get(context.Context, uuid.UUID, uuid.UUID) (project.Project, error)
}

type ScenePlanRepository interface {
	GetByVersion(context.Context, uuid.UUID, uuid.UUID, int) (sceneplan.Plan, error)
}

type Service struct {
	runtime  VideoProviderRuntime
	jobs     jobs.Repository
	projects ProjectRepository
	plans    ScenePlanRepository
}

func NewService(runtime VideoProviderRuntime, jobsRepo jobs.Repository, projects ProjectRepository, plans ScenePlanRepository) *Service {
	return &Service{runtime: runtime, jobs: jobsRepo, projects: projects, plans: plans}
}

func (s *Service) CreateGeneration(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string, input CreateGenerationInput) (JobView, error) {
	if principal.OwnerID == uuid.Nil {
		return JobView{}, ErrUnauthenticated
	}
	if input.RequestID == uuid.Nil {
		return JobView{}, ErrInvalidRequestID
	}
	if projectID == uuid.Nil || planVersion < 1 || sceneKey == "" {
		return JobView{}, ErrScenePlanNotFound
	}
	if input.ProviderID == "" || input.ModelID == "" {
		return JobView{}, ErrProviderUnavailable
	}

	if existing, err := s.jobs.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID); err == nil {
		if err := validateExistingJob(existing, planVersion, sceneKey, input); err != nil {
			return JobView{}, err
		}
		return ToJobView(existing), nil
	} else if !errors.Is(err, jobs.ErrJobNotFound) {
		return JobView{}, err
	}

	proj, err := s.projects.Get(ctx, principal.OwnerID, projectID)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return JobView{}, ErrProjectNotFound
		}
		return JobView{}, err
	}
	plan, err := s.plans.GetByVersion(ctx, principal.OwnerID, projectID, planVersion)
	if err != nil {
		if errors.Is(err, sceneplan.ErrNotFound) {
			return JobView{}, ErrScenePlanNotFound
		}
		return JobView{}, err
	}
	if plan.ProjectID != projectID {
		return JobView{}, ErrScenePlanNotFound
	}
	if plan.Status != sceneplan.StatusApproved {
		return JobView{}, ErrScenePlanNotApproved
	}
	var scene *sceneplan.Scene
	for i := range plan.Scenes {
		if plan.Scenes[i].Key == sceneKey {
			scene = &plan.Scenes[i]
			break
		}
	}
	if scene == nil {
		return JobView{}, ErrSceneKeyNotFound
	}

	if s.runtime == nil {
		return JobView{}, ErrProviderUnavailable
	}
	generator, err := s.runtime.ResolveVideoGenerator(ctx, principal.OwnerID, providers.ProviderID(input.ProviderID), providers.ModelID(input.ModelID))
	if err != nil || generator == nil {
		return JobView{}, ErrProviderUnavailable
	}

	request := providers.VideoGenerationRequest{Prompt: scene.VisualInstruction, AspectRatio: string(proj.AspectRatio), DurationSeconds: input.DurationSeconds}
	if err := request.Validate(); err != nil {
		return JobView{}, ErrProviderUnavailable
	}
	payload := Payload{
		SchemaVersion:       SchemaVersion,
		ProviderID:          input.ProviderID,
		ModelID:             input.ModelID,
		ScenePlanVersion:    planVersion,
		SceneKey:            sceneKey,
		Prompt:              scene.VisualInstruction,
		AspectRatio:         string(proj.AspectRatio),
		DurationSeconds:     input.DurationSeconds,
		AssignPrimaryVisual: input.AssignPrimaryVisual,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return JobView{}, fmt.Errorf("marshal scene video payload: %w", err)
	}
	job, err := s.jobs.Enqueue(ctx, jobs.EnqueueInput{ID: input.RequestID, OwnerID: principal.OwnerID, ProjectID: &projectID, Kind: JobKind, MaxAttempts: 20, Payload: payloadBytes})
	if err != nil {
		if errors.Is(err, jobs.ErrDuplicateJob) {
			race, getErr := s.jobs.GetByIDForProject(ctx, principal.OwnerID, projectID, input.RequestID)
			if getErr == nil {
				if valErr := validateExistingJob(race, planVersion, sceneKey, input); valErr != nil {
					return JobView{}, valErr
				}
				return ToJobView(race), nil
			}
			return JobView{}, ErrGenerationRequestConflict
		}
		return JobView{}, fmt.Errorf("enqueue scene video job: %w", err)
	}
	return ToJobView(job), nil
}

func validateExistingJob(job jobs.Job, planVersion int, sceneKey string, input CreateGenerationInput) error {
	if job.Kind != JobKind {
		return ErrGenerationRequestConflict
	}
	var payload Payload
	if json.Unmarshal(job.Payload, &payload) != nil {
		return ErrGenerationRequestConflict
	}
	if payload.SchemaVersion != SchemaVersion || payload.ScenePlanVersion != planVersion || payload.SceneKey != sceneKey ||
		payload.ProviderID != input.ProviderID || payload.ModelID != input.ModelID || payload.AssignPrimaryVisual != input.AssignPrimaryVisual || !equalOptionalInt(payload.DurationSeconds, input.DurationSeconds) {
		return ErrGenerationRequestConflict
	}
	return nil
}

func equalOptionalInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Service) GetGeneration(ctx context.Context, principal project.Principal, projectID, jobID uuid.UUID) (JobView, error) {
	if principal.OwnerID == uuid.Nil {
		return JobView{}, ErrUnauthenticated
	}
	job, err := s.jobs.GetByIDForProject(ctx, principal.OwnerID, projectID, jobID)
	if err != nil {
		if errors.Is(err, jobs.ErrJobNotFound) {
			return JobView{}, ErrJobNotFound
		}
		return JobView{}, err
	}
	if job.Kind != JobKind {
		return JobView{}, ErrJobNotFound
	}
	return ToJobView(job), nil
}

func ToJobView(job jobs.Job) JobView {
	view := JobView{ID: job.ID, State: string(job.State), Attempt: job.Attempt, MaxAttempts: job.MaxAttempts, ErrorCode: job.ErrorCode, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt}
	if len(job.Result) > 0 {
		var result Result
		if json.Unmarshal(job.Result, &result) == nil && result.MediaAssetID != uuid.Nil {
			id := result.MediaAssetID
			view.MediaAssetID = &id
			view.AssignedPrimaryVisual = result.AssignedPrimaryVisual
			if result.ExternalOperationID != "" {
				op := result.ExternalOperationID
				view.ExternalOperationID = &op
			}
		}
	}
	return view
}
