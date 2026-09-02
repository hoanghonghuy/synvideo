package generatedimagejob

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

func TestCreateGenerationIsIdempotentAndRejectsParameterReuse(t *testing.T) {
	ownerID, projectID, requestID := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeJobsRepo{}
	projects := fakeProjectRepo{project: project.Project{ID: projectID, OwnerID: ownerID, AspectRatio: "16:9"}}
	plans := fakePlanRepo{plan: approvedPlan(projectID)}
	runtime := &fakeImageRuntime{generator: fakeImageGenerator{}}
	svc := NewService(runtime, repo, projects, plans)
	principal := project.Principal{OwnerID: ownerID}
	input := CreateGenerationInput{RequestID: requestID, ProviderID: "openai", ModelID: "image-1", AssignPrimaryVisual: true}

	first, err := svc.CreateGeneration(context.Background(), principal, projectID, 1, "scene-1", input)
	if err != nil { t.Fatalf("first create: %v", err) }
	second, err := svc.CreateGeneration(context.Background(), principal, projectID, 1, "scene-1", input)
	if err != nil { t.Fatalf("replay: %v", err) }
	if first.ID != second.ID || repo.enqueueCalls != 1 { t.Fatalf("expected one logical job, got ids %s/%s calls=%d", first.ID, second.ID, repo.enqueueCalls) }

	input.ModelID = "other"
	if _, err := svc.CreateGeneration(context.Background(), principal, projectID, 1, "scene-1", input); !errors.Is(err, ErrGenerationRequestConflict) {
		t.Fatalf("expected request conflict, got %v", err)
	}
}

func TestCreateGenerationRejectsUnavailableProviderBeforeEnqueue(t *testing.T) {
	ownerID, projectID := uuid.New(), uuid.New()
	repo := &fakeJobsRepo{}
	svc := NewService(&fakeImageRuntime{err: providers.ErrProviderUnavailable}, repo,
		fakeProjectRepo{project: project.Project{ID: projectID, OwnerID: ownerID, AspectRatio: "16:9"}},
		fakePlanRepo{plan: approvedPlan(projectID)})
	_, err := svc.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, 1, "scene-1", CreateGenerationInput{
		RequestID: uuid.New(), ProviderID: "openai", ModelID: "disabled",
	})
	if !errors.Is(err, ErrProviderUnavailable) { t.Fatalf("expected provider unavailable, got %v", err) }
	if repo.enqueueCalls != 0 { t.Fatalf("provider must fail before enqueue") }
}

func approvedPlan(projectID uuid.UUID) sceneplan.Plan {
	return sceneplan.Plan{ProjectID: projectID, Version: 1, Status: sceneplan.StatusApproved, Scenes: []sceneplan.Scene{{
		Key: "scene-1", VisualInstruction: "a lighthouse at dusk", PlannedSourceType: sceneplan.SourceTypeGeneratedImage,
	}}}
}

type fakeProjectRepo struct{ project project.Project }
func (f fakeProjectRepo) Get(_ context.Context, ownerID, id uuid.UUID) (project.Project, error) {
	if f.project.OwnerID != ownerID || f.project.ID != id { return project.Project{}, project.ErrNotFound }
	return f.project, nil
}

type fakePlanRepo struct{ plan sceneplan.Plan }
func (f fakePlanRepo) GetByVersion(_ context.Context, _ uuid.UUID, projectID uuid.UUID, version int) (sceneplan.Plan, error) {
	if f.plan.ProjectID != projectID || f.plan.Version != version { return sceneplan.Plan{}, sceneplan.ErrNotFound }
	return f.plan, nil
}

type fakeImageRuntime struct { generator providers.ImageGenerator; err error }
func (f *fakeImageRuntime) ResolveImageGenerator(context.Context, uuid.UUID, providers.ProviderID, providers.ModelID) (providers.ImageGenerator, error) {
	if f.err != nil { return nil, f.err }
	return f.generator, nil
}

type fakeImageGenerator struct{}
func (fakeImageGenerator) GenerateImage(context.Context, providers.ImageGenerationRequest) (providers.ImageGenerationResponse, error) { return providers.ImageGenerationResponse{}, nil }

type fakeJobsRepo struct { job *jobs.Job; enqueueCalls int }
func (f *fakeJobsRepo) Enqueue(_ context.Context, in jobs.EnqueueInput) (jobs.Job, error) {
	f.enqueueCalls++
	job := jobs.Job{ID: in.ID, OwnerID: in.OwnerID, ProjectID: in.ProjectID, Kind: in.Kind, State: jobs.StateQueued, MaxAttempts: in.MaxAttempts, Payload: append(json.RawMessage(nil), in.Payload...)}
	f.job = &job
	return job, nil
}
func (f *fakeJobsRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (jobs.Job, error) { return jobs.Job{}, jobs.ErrJobNotFound }
func (f *fakeJobsRepo) GetByIDForProject(_ context.Context, ownerID, projectID, id uuid.UUID) (jobs.Job, error) {
	if f.job == nil || f.job.ID != id || f.job.OwnerID != ownerID || f.job.ProjectID == nil || *f.job.ProjectID != projectID { return jobs.Job{}, jobs.ErrJobNotFound }
	return *f.job, nil
}
func (f *fakeJobsRepo) ClaimNext(context.Context, jobs.ClaimOptions) (jobs.Job, error) { return jobs.Job{}, jobs.ErrNoJobAvailable }
func (f *fakeJobsRepo) RenewLease(context.Context, uuid.UUID, uuid.UUID, time.Duration) (jobs.Job, error) { panic("unused") }
func (f *fakeJobsRepo) MarkSuccess(context.Context, uuid.UUID, uuid.UUID, json.RawMessage) (jobs.Job, error) { panic("unused") }
func (f *fakeJobsRepo) MarkRetryableFailure(context.Context, uuid.UUID, uuid.UUID, string, time.Time) (jobs.Job, error) { panic("unused") }
func (f *fakeJobsRepo) MarkTerminalFailure(context.Context, uuid.UUID, uuid.UUID, string) (jobs.Job, error) { panic("unused") }
