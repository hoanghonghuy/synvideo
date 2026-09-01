package proposalgenerationjob_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgenerationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

type mockProjectRepo struct {
	proj project.Project
	err  error
}

func (m *mockProjectRepo) Get(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (project.Project, error) {
	if m.err != nil {
		return project.Project{}, m.err
	}
	return m.proj, nil
}

type mockBriefRepo struct {
	brief creativebrief.CreativeBrief
	err   error
}

func (m *mockBriefRepo) Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) (creativebrief.CreativeBrief, error) {
	if m.err != nil {
		return creativebrief.CreativeBrief{}, m.err
	}
	return m.brief, nil
}

type mockJobsRepo struct {
	jobs                map[uuid.UUID]jobs.Job
	enqueuedInput       jobs.EnqueueInput
	enqueueErr          error
	getByIDForProjectFn func(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error)
}

func newMockJobsRepo() *mockJobsRepo {
	return &mockJobsRepo{
		jobs: make(map[uuid.UUID]jobs.Job),
	}
}

func (m *mockJobsRepo) Enqueue(ctx context.Context, input jobs.EnqueueInput) (jobs.Job, error) {
	m.enqueuedInput = input
	if m.enqueueErr != nil {
		return jobs.Job{}, m.enqueueErr
	}
	if _, exists := m.jobs[input.ID]; exists {
		return jobs.Job{}, jobs.ErrDuplicateJob
	}
	j := jobs.Job{
		ID:          input.ID,
		OwnerID:     input.OwnerID,
		ProjectID:   input.ProjectID,
		Kind:        input.Kind,
		State:       jobs.StateQueued,
		Attempt:     0,
		MaxAttempts: input.MaxAttempts,
		Payload:     input.Payload,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	m.jobs[input.ID] = j
	return j, nil
}

func (m *mockJobsRepo) GetByID(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	j, ok := m.jobs[id]
	if !ok || j.OwnerID != ownerID {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	return j, nil
}

func (m *mockJobsRepo) GetByIDForProject(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
	if m.getByIDForProjectFn != nil {
		return m.getByIDForProjectFn(ctx, ownerID, projectID, id)
	}
	j, ok := m.jobs[id]
	if !ok || j.OwnerID != ownerID || j.ProjectID == nil || *j.ProjectID != projectID {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	return j, nil
}

func (m *mockJobsRepo) ClaimNext(ctx context.Context, options jobs.ClaimOptions) (jobs.Job, error) {
	return jobs.Job{}, jobs.ErrNoJobAvailable
}
func (m *mockJobsRepo) RenewLease(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, extendDuration time.Duration) (jobs.Job, error) {
	return jobs.Job{}, nil
}
func (m *mockJobsRepo) MarkSuccess(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, result json.RawMessage) (jobs.Job, error) {
	return jobs.Job{}, nil
}
func (m *mockJobsRepo) MarkRetryableFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string, nextAvailableAt time.Time) (jobs.Job, error) {
	return jobs.Job{}, nil
}
func (m *mockJobsRepo) MarkTerminalFailure(ctx context.Context, id uuid.UUID, leaseToken uuid.UUID, errorCode string) (jobs.Job, error) {
	return jobs.Job{}, nil
}

func registerFakeRegistry(t *testing.T) *providers.Registry {
	t.Helper()
	reg := providers.NewRegistry()
	err := reg.Register(providers.Registration{
		Provider: providers.ProviderMetadata{
			ID:          "lab-provider",
			DisplayName: "Lab Provider",
		},
		Models: []providers.ModelRegistration{
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "lab-provider",
					ID:                    "lab-model-v1",
					DisplayName:           "Lab Model V1",
					SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
				},
				TextGenerator: fake.NewTextGenerator("{}"),
			},
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "lab-provider",
					ID:                    "lab-model-v2",
					DisplayName:           "Lab Model V2",
					SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
				},
				TextGenerator: fake.NewTextGenerator("{}"),
			},
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return reg
}

func TestService_GetTextGenerationOptions(t *testing.T) {
	t.Run("empty registry", func(t *testing.T) {
		svc := proposalgenerationjob.NewService(providers.NewRegistry(), newMockJobsRepo(), &mockProjectRepo{}, &mockBriefRepo{})
		resp, err := svc.GetTextGenerationOptions(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(resp.Providers) != 0 {
			t.Fatalf("expected 0 providers, got %d", len(resp.Providers))
		}
	})

	t.Run("returns sorted text providers and models", func(t *testing.T) {
		reg := registerFakeRegistry(t)
		svc := proposalgenerationjob.NewService(reg, newMockJobsRepo(), &mockProjectRepo{}, &mockBriefRepo{})
		resp, err := svc.GetTextGenerationOptions(context.Background())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(resp.Providers) != 1 || len(resp.Providers[0].Models) != 2 {
			t.Fatalf("expected 1 provider with 2 models, got %d providers: %+v", len(resp.Providers), resp.Providers)
		}
		if resp.Providers[0].ID != "lab-provider" || resp.Providers[0].Models[0].ID != "lab-model-v1" || resp.Providers[0].Models[1].ID != "lab-model-v2" {
			t.Fatalf("unexpected provider/model: %+v", resp.Providers[0])
		}
	})
}

func TestService_CreateGeneration(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	principal := project.Principal{OwnerID: ownerID}
	reg := registerFakeRegistry(t)

	validProj := project.Project{
		ID:            projectID,
		OwnerID:       ownerID,
		ContentFormat: project.ContentFormatShort,
		AspectRatio:   project.AspectRatio9x16,
		Locale:        project.LocaleVI,
	}
	validBrief := creativebrief.CreativeBrief{
		ProjectID:  projectID,
		Revision:   3,
		SourceText: "Test Brief",
	}

	t.Run("unauthenticated", func(t *testing.T) {
		svc := proposalgenerationjob.NewService(reg, newMockJobsRepo(), &mockProjectRepo{proj: validProj}, &mockBriefRepo{brief: validBrief})
		_, err := svc.CreateGeneration(context.Background(), project.Principal{}, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v1",
		})
		if !errors.Is(err, proposalgenerationjob.ErrUnauthenticated) {
			t.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})

	t.Run("missing project returns ErrProjectNotFound", func(t *testing.T) {
		svc := proposalgenerationjob.NewService(reg, newMockJobsRepo(), &mockProjectRepo{err: project.ErrNotFound}, &mockBriefRepo{brief: validBrief})
		_, err := svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v1",
		})
		if !errors.Is(err, proposalgenerationjob.ErrProjectNotFound) {
			t.Fatalf("expected ErrProjectNotFound, got %v", err)
		}
	})

	t.Run("missing brief returns ErrCreativeBriefRequired", func(t *testing.T) {
		svc := proposalgenerationjob.NewService(reg, newMockJobsRepo(), &mockProjectRepo{proj: validProj}, &mockBriefRepo{err: creativebrief.ErrNotFound})
		_, err := svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v1",
		})
		if !errors.Is(err, proposalgenerationjob.ErrCreativeBriefRequired) {
			t.Fatalf("expected ErrCreativeBriefRequired, got %v", err)
		}
	})

	t.Run("unknown provider returns ErrProviderUnavailable", func(t *testing.T) {
		svc := proposalgenerationjob.NewService(reg, newMockJobsRepo(), &mockProjectRepo{proj: validProj}, &mockBriefRepo{brief: validBrief})
		_, err := svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "unknown-provider",
			ModelID:    "lab-model-v1",
		})
		if !errors.Is(err, proposalgenerationjob.ErrProviderUnavailable) {
			t.Fatalf("expected ErrProviderUnavailable, got %v", err)
		}
	})

	t.Run("successful enqueue and idempotent replay", func(t *testing.T) {
		jobsRepo := newMockJobsRepo()
		svc := proposalgenerationjob.NewService(reg, jobsRepo, &mockProjectRepo{proj: validProj}, &mockBriefRepo{brief: validBrief})
		reqID := uuid.New()

		// 1. Initial create
		view1, err := svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  reqID,
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v1",
		})
		if err != nil {
			t.Fatalf("create generation: %v", err)
		}
		if view1.ID != reqID || view1.State != "queued" {
			t.Fatalf("unexpected view: %+v", view1)
		}

		// Verify snapshot payload
		var snap proposalgenerationjob.Payload
		if err := json.Unmarshal(jobsRepo.enqueuedInput.Payload, &snap); err != nil {
			t.Fatalf("unmarshal snapshot payload: %v", err)
		}
		if snap.Brief.Revision != 3 || snap.Brief.SourceText != "Test Brief" {
			t.Fatalf("unexpected brief snapshot: %+v", snap.Brief)
		}
		if snap.Project.ContentFormat != project.ContentFormatShort {
			t.Fatalf("unexpected project snapshot: %+v", snap.Project)
		}

		// 2. Replay same request_id -> returns same job view
		view2, err := svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  reqID,
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v1",
		})
		if err != nil {
			t.Fatalf("replay generation: %v", err)
		}
		if view2.ID != reqID || view2.State != "queued" {
			t.Fatalf("unexpected replay view: %+v", view2)
		}

		// 3. Replay succeeds even if registry is empty / provider is now removed
		emptyRegSvc := proposalgenerationjob.NewService(providers.NewRegistry(), jobsRepo, &mockProjectRepo{proj: validProj}, &mockBriefRepo{brief: validBrief})
		view3, err := emptyRegSvc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  reqID,
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v1",
		})
		if err != nil {
			t.Fatalf("replay with removed provider must succeed: %v", err)
		}
		if view3.ID != reqID || view3.State != "queued" {
			t.Fatalf("unexpected replay view with removed provider: %+v", view3)
		}

		// 4. Conflict: same request_id with different model (must return ErrGenerationRequestConflict deterministically)
		_, err = svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  reqID,
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v2",
		})
		if !errors.Is(err, proposalgenerationjob.ErrGenerationRequestConflict) {
			t.Fatalf("expected ErrGenerationRequestConflict for conflicting model, got %v", err)
		}

		// 5. Conflict: same request_id with different provider
		_, err = svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  reqID,
			ProviderID: "other-provider",
			ModelID:    "lab-model-v1",
		})
		if !errors.Is(err, proposalgenerationjob.ErrGenerationRequestConflict) {
			t.Fatalf("expected ErrGenerationRequestConflict for conflicting provider, got %v", err)
		}
	})

	t.Run("concurrent duplicate race handles matching and conflicting parameters", func(t *testing.T) {
		reqID := uuid.New()
		racePayload, _ := json.Marshal(proposalgenerationjob.Payload{
			SchemaVersion: proposalgenerationjob.SchemaVersion,
			ProviderID:    "lab-provider",
			ModelID:       "lab-model-v1",
		})
		existingRaceJob := jobs.Job{
			ID:          reqID,
			OwnerID:     ownerID,
			ProjectID:   &projectID,
			Kind:        proposalgenerationjob.JobKind,
			State:       jobs.StateQueued,
			Attempt:     0,
			MaxAttempts: 3,
			Payload:     racePayload,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}

		// 1. Matching winner: first GetByIDForProject returns ErrJobNotFound, Enqueue returns ErrDuplicateJob, second GetByIDForProject returns winner
		getCallCount := 0
		jobsRepo := newMockJobsRepo()
		jobsRepo.enqueueErr = jobs.ErrDuplicateJob
		jobsRepo.getByIDForProjectFn = func(ctx context.Context, oID uuid.UUID, pID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
			getCallCount++
			if getCallCount == 1 {
				return jobs.Job{}, jobs.ErrJobNotFound
			}
			return existingRaceJob, nil
		}

		svc := proposalgenerationjob.NewService(reg, jobsRepo, &mockProjectRepo{proj: validProj}, &mockBriefRepo{brief: validBrief})

		view, err := svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  reqID,
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v1",
		})
		if err != nil {
			t.Fatalf("expected successful race resolution, got %v", err)
		}
		if view.ID != reqID {
			t.Fatalf("expected job ID %s, got %s", reqID, view.ID)
		}
		if getCallCount != 2 {
			t.Fatalf("expected exactly 2 GetByIDForProject calls for duplicate race, got %d", getCallCount)
		}
		if jobsRepo.enqueuedInput.ID != reqID {
			t.Fatalf("expected Enqueue to be called with ID %s, got %s", reqID, jobsRepo.enqueuedInput.ID)
		}

		// 2. Conflicting winner: first GetByIDForProject returns ErrJobNotFound, Enqueue returns ErrDuplicateJob, second GetByIDForProject returns winner with different model
		getCallCount = 0
		jobsRepo = newMockJobsRepo()
		jobsRepo.enqueueErr = jobs.ErrDuplicateJob
		jobsRepo.getByIDForProjectFn = func(ctx context.Context, oID uuid.UUID, pID uuid.UUID, id uuid.UUID) (jobs.Job, error) {
			getCallCount++
			if getCallCount == 1 {
				return jobs.Job{}, jobs.ErrJobNotFound
			}
			return existingRaceJob, nil
		}
		svc = proposalgenerationjob.NewService(reg, jobsRepo, &mockProjectRepo{proj: validProj}, &mockBriefRepo{brief: validBrief})

		_, err = svc.CreateGeneration(context.Background(), principal, projectID, proposalgenerationjob.CreateProposalGenerationInput{
			RequestID:  reqID,
			ProviderID: "lab-provider",
			ModelID:    "lab-model-v2",
		})
		if !errors.Is(err, proposalgenerationjob.ErrGenerationRequestConflict) {
			t.Fatalf("expected ErrGenerationRequestConflict for conflicting race input, got %v", err)
		}
		if getCallCount != 2 {
			t.Fatalf("expected exactly 2 GetByIDForProject calls for conflicting duplicate race, got %d", getCallCount)
		}
	})
}

func TestService_GetGeneration(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	principal := project.Principal{OwnerID: ownerID}
	jobsRepo := newMockJobsRepo()
	svc := proposalgenerationjob.NewService(providers.NewRegistry(), jobsRepo, &mockProjectRepo{}, &mockBriefRepo{})

	jobID := uuid.New()
	jobsRepo.jobs[jobID] = jobs.Job{
		ID:          jobID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        proposalgenerationjob.JobKind,
		State:       jobs.StateSucceeded,
		Attempt:     1,
		MaxAttempts: 3,
		Result:      []byte(`{"proposal_version":4}`),
	}

	view, err := svc.GetGeneration(context.Background(), principal, projectID, jobID)
	if err != nil {
		t.Fatalf("get generation: %v", err)
	}
	if view.ID != jobID || view.State != "succeeded" {
		t.Fatalf("unexpected view: %+v", view)
	}
	if view.ProposalVersion == nil || *view.ProposalVersion != 4 {
		t.Fatalf("expected proposal version 4, got %v", view.ProposalVersion)
	}

	// Inaccessible project/job returns ErrJobNotFound
	_, err = svc.GetGeneration(context.Background(), principal, uuid.New(), jobID)
	if !errors.Is(err, proposalgenerationjob.ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound for foreign project, got %v", err)
	}
}
