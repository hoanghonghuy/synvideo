package scriptgenerationjob_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgenerationjob"
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

type mockProposalRepo struct {
	proposals []creativeproposal.CreativeProposal
	err       error
}

func (m *mockProposalRepo) List(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.proposals, nil
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
			ID:          "fake-provider",
			DisplayName: "Fake Provider",
		},
		Models: []providers.ModelRegistration{
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "fake-provider",
					ID:                    "fake-model",
					DisplayName:           "Fake Model",
					SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
				},
				TextGenerator: fake.NewTextGenerator("{}"),
			},
			{
				Metadata: providers.ModelMetadata{
					ProviderID:            "fake-provider",
					ID:                    "fake-model-2",
					DisplayName:           "Fake Model 2",
					SupportedCapabilities: []providers.Capability{providers.CapabilityTextGeneration},
				},
				TextGenerator: fake.NewTextGenerator("{}"),
			},
		},
	})
	if err != nil {
		t.Fatalf("register fake provider: %v", err)
	}
	return reg
}

type mockTextRuntime struct {
	optionsResp providersettings.TextGenerationOptionsResponse
	generator   providers.TextGenerator
	err         error
}

func (m *mockTextRuntime) GetOwnerTextGenerationOptions(ctx context.Context, ownerID uuid.UUID) (providersettings.TextGenerationOptionsResponse, error) {
	if m.err != nil {
		return providersettings.TextGenerationOptionsResponse{}, m.err
	}
	return m.optionsResp, nil
}

func (m *mockTextRuntime) ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.generator, nil
}

func validTestProposal(version int, status creativeproposal.Status) creativeproposal.CreativeProposal {
	dur := 60
	return creativeproposal.CreativeProposal{
		Version:                  version,
		Revision:                 1,
		Status:                   status,
		SourceBriefRevision:      1,
		TitleOptions:             []string{"Title 1", "Title 2"},
		HookOptions:              []string{"Hook 1", "Hook 2"},
		AudienceSummary:          "Audience summary",
		ObjectiveSummary:         "Objective summary",
		NarrativeAngle:           "Narrative angle",
		EstimatedDurationSeconds: &dur,
		FormatRationale:          "Format rationale",
		Structure: []creativeproposal.StructureItem{
			{Key: "intro", Title: "Introduction", Purpose: "Hook viewers"},
			{Key: "body", Title: "Body", Purpose: "Deliver value"},
		},
		VisualDirection:  "Visual direction",
		VoiceDirection:   "Voice direction",
		MusicDirection:   "Music direction",
		CaptionDirection: "Caption direction",
		CallToAction:     "Call to action",
		ResearchGaps:     []string{"Gap 1"},
		Warnings:         []string{"Warning 1"},
	}
}

func TestService_CreateGeneration_Success(t *testing.T) {
	registry := registerFakeRegistry(t)
	jobsRepo := newMockJobsRepo()
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()

	targetDur := 60
	projRepo := &mockProjectRepo{
		proj: project.Project{
			ID:                    projectID,
			OwnerID:               ownerID,
			Title:                 "Test Project",
			ContentFormat:         project.ContentFormatShort,
			AspectRatio:           project.AspectRatio9x16,
			TargetDurationSeconds: &targetDur,
			Locale:                project.LocaleVI,
		},
	}

	propRepo := &mockProposalRepo{
		proposals: []creativeproposal.CreativeProposal{
			validTestProposal(2, creativeproposal.StatusDraft),
			validTestProposal(1, creativeproposal.StatusApproved),
		},
	}

	service := scriptgenerationjob.NewService(registry, jobsRepo, projRepo, propRepo)

	view, err := service.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  requestID,
		ProviderID: "fake-provider",
		ModelID:    "fake-model",
	})
	if err != nil {
		t.Fatalf("CreateGeneration failed: %v", err)
	}

	if view.ID != requestID {
		t.Fatalf("expected view.ID %s, got %s", requestID, view.ID)
	}
	if view.State != string(jobs.StateQueued) {
		t.Fatalf("expected state queued, got %s", view.State)
	}
	if view.Attempt != 0 {
		t.Fatalf("expected attempt 0, got %d", view.Attempt)
	}

	var payload scriptgenerationjob.Payload
	if err := json.Unmarshal(jobsRepo.enqueuedInput.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload.SchemaVersion != scriptgenerationjob.SchemaVersion {
		t.Fatalf("expected schema version %s, got %s", scriptgenerationjob.SchemaVersion, payload.SchemaVersion)
	}
	if payload.ProviderID != "fake-provider" || payload.ModelID != "fake-model" {
		t.Fatalf("unexpected provider/model: %s/%s", payload.ProviderID, payload.ModelID)
	}
	if payload.Project.ID != projectID || payload.Project.Locale != project.LocaleVI {
		t.Fatalf("unexpected project snapshot: %+v", payload.Project)
	}
	if payload.Proposal.Version != 1 {
		t.Fatalf("expected snapshotted proposal version 1 (the approved one), got %d", payload.Proposal.Version)
	}
	if len(payload.Proposal.Structure) != 2 || payload.Proposal.Structure[0].Key != "intro" {
		t.Fatalf("unexpected proposal structure snapshot: %+v", payload.Proposal.Structure)
	}
}

func TestService_CreateGeneration_HighestApprovedProposalSelected(t *testing.T) {
	registry := registerFakeRegistry(t)
	jobsRepo := newMockJobsRepo()
	ownerID := uuid.New()
	projectID := uuid.New()

	projRepo := &mockProjectRepo{
		proj: project.Project{
			ID:      projectID,
			OwnerID: ownerID,
			Locale:  project.LocaleVI,
		},
	}

	// Proposals: v3 approved, v2 draft, v1 approved
	propRepo := &mockProposalRepo{
		proposals: []creativeproposal.CreativeProposal{
			validTestProposal(3, creativeproposal.StatusApproved),
			validTestProposal(2, creativeproposal.StatusDraft),
			validTestProposal(1, creativeproposal.StatusApproved),
		},
	}

	service := scriptgenerationjob.NewService(registry, jobsRepo, projRepo, propRepo)

	_, err := service.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  uuid.New(),
		ProviderID: "fake-provider",
		ModelID:    "fake-model",
	})
	if err != nil {
		t.Fatalf("CreateGeneration failed: %v", err)
	}

	var payload scriptgenerationjob.Payload
	if err := json.Unmarshal(jobsRepo.enqueuedInput.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload.Proposal.Version != 3 {
		t.Fatalf("expected highest approved proposal version 3, got %d", payload.Proposal.Version)
	}
}

func TestService_CreateGeneration_NoApprovedProposal_Fails(t *testing.T) {
	registry := registerFakeRegistry(t)
	jobsRepo := newMockJobsRepo()
	ownerID := uuid.New()
	projectID := uuid.New()

	projRepo := &mockProjectRepo{
		proj: project.Project{
			ID:      projectID,
			OwnerID: ownerID,
		},
	}

	// Only draft proposals
	propRepo := &mockProposalRepo{
		proposals: []creativeproposal.CreativeProposal{
			validTestProposal(2, creativeproposal.StatusDraft),
			validTestProposal(1, creativeproposal.StatusDraft),
		},
	}

	service := scriptgenerationjob.NewService(registry, jobsRepo, projRepo, propRepo)

	_, err := service.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  uuid.New(),
		ProviderID: "fake-provider",
		ModelID:    "fake-model",
	})
	if !errors.Is(err, scriptgenerationjob.ErrApprovedProposalRequired) {
		t.Fatalf("expected ErrApprovedProposalRequired, got %v", err)
	}
}

func TestService_CreateGeneration_UnauthenticatedAndInvalidInputs(t *testing.T) {
	registry := registerFakeRegistry(t)
	jobsRepo := newMockJobsRepo()
	ownerID := uuid.New()
	projectID := uuid.New()
	service := scriptgenerationjob.NewService(registry, jobsRepo, &mockProjectRepo{}, &mockProposalRepo{})

	// Unauthenticated
	_, err := service.CreateGeneration(context.Background(), project.Principal{}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  uuid.New(),
		ProviderID: "fake-provider",
		ModelID:    "fake-model",
	})
	if !errors.Is(err, scriptgenerationjob.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}

	// Invalid RequestID
	_, err = service.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  uuid.Nil,
		ProviderID: "fake-provider",
		ModelID:    "fake-model",
	})
	if !errors.Is(err, scriptgenerationjob.ErrInvalidRequestID) {
		t.Fatalf("expected ErrInvalidRequestID, got %v", err)
	}

	// Empty provider or model
	_, err = service.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  uuid.New(),
		ProviderID: "",
		ModelID:    "fake-model",
	})
	if !errors.Is(err, scriptgenerationjob.ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}

func TestService_CreateGeneration_IdempotentReplay(t *testing.T) {
	registry := registerFakeRegistry(t)
	jobsRepo := newMockJobsRepo()
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()

	existingPayload, _ := json.Marshal(scriptgenerationjob.Payload{
		SchemaVersion: scriptgenerationjob.SchemaVersion,
		ProviderID:    "fake-provider",
		ModelID:       "fake-model",
	})
	existingJob := jobs.Job{
		ID:          requestID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        scriptgenerationjob.JobKind,
		State:       jobs.StateRunning,
		Attempt:     1,
		MaxAttempts: 3,
		Payload:     existingPayload,
		CreatedAt:   time.Now().UTC().Add(-time.Minute),
		UpdatedAt:   time.Now().UTC(),
	}
	jobsRepo.jobs[requestID] = existingJob

	// Note: project repo returns error to prove replay returns job before project/proposal lookup
	projRepo := &mockProjectRepo{err: errors.New("should not be called")}
	propRepo := &mockProposalRepo{err: errors.New("should not be called")}

	service := scriptgenerationjob.NewService(registry, jobsRepo, projRepo, propRepo)

	view, err := service.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  requestID,
		ProviderID: "fake-provider",
		ModelID:    "fake-model",
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if view.ID != requestID || view.State != string(jobs.StateRunning) {
		t.Fatalf("unexpected replayed job view: %+v", view)
	}
}

func TestService_CreateGeneration_ConflictOnDifferentParams(t *testing.T) {
	registry := registerFakeRegistry(t)
	jobsRepo := newMockJobsRepo()
	ownerID := uuid.New()
	projectID := uuid.New()
	requestID := uuid.New()

	existingPayload, _ := json.Marshal(scriptgenerationjob.Payload{
		SchemaVersion: scriptgenerationjob.SchemaVersion,
		ProviderID:    "fake-provider",
		ModelID:       "fake-model-A",
	})
	jobsRepo.jobs[requestID] = jobs.Job{
		ID:          requestID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        scriptgenerationjob.JobKind,
		State:       jobs.StateQueued,
		Attempt:     0,
		MaxAttempts: 3,
		Payload:     existingPayload,
	}

	service := scriptgenerationjob.NewService(registry, jobsRepo, &mockProjectRepo{}, &mockProposalRepo{})

	// Same request_id but requesting fake-model-B
	_, err := service.CreateGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  requestID,
		ProviderID: "fake-provider",
		ModelID:    "fake-model-B",
	})
	if !errors.Is(err, scriptgenerationjob.ErrGenerationRequestConflict) {
		t.Fatalf("expected ErrGenerationRequestConflict, got %v", err)
	}
}

func TestService_GetGeneration_SuccessAndNotFound(t *testing.T) {
	registry := registerFakeRegistry(t)
	jobsRepo := newMockJobsRepo()
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()

	resBytes, _ := json.Marshal(scriptgenerationjob.Result{ScriptVersion: 4})
	jobsRepo.jobs[jobID] = jobs.Job{
		ID:          jobID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        scriptgenerationjob.JobKind,
		State:       jobs.StateSucceeded,
		Attempt:     1,
		MaxAttempts: 3,
		Result:      resBytes,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	service := scriptgenerationjob.NewService(registry, jobsRepo, &mockProjectRepo{}, &mockProposalRepo{})

	view, err := service.GetGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, jobID)
	if err != nil {
		t.Fatalf("GetGeneration failed: %v", err)
	}

	if view.ScriptVersion == nil || *view.ScriptVersion != 4 {
		t.Fatalf("expected script version 4, got %v", view.ScriptVersion)
	}

	// Missing job
	_, err = service.GetGeneration(context.Background(), project.Principal{OwnerID: ownerID}, projectID, uuid.New())
	if !errors.Is(err, scriptgenerationjob.ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestService_CreateGeneration_DuplicateEnqueueRace(t *testing.T) {
	registry := registerFakeRegistry(t)
	ownerID := uuid.New()
	projectID := uuid.New()
	reqID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}

	validProj := project.Project{
		ID:      projectID,
		OwnerID: ownerID,
		Locale:  project.LocaleVI,
	}
	validProp := validTestProposal(1, creativeproposal.StatusApproved)

	racePayload, _ := json.Marshal(scriptgenerationjob.Payload{
		SchemaVersion: scriptgenerationjob.SchemaVersion,
		ProviderID:    "fake-provider",
		ModelID:       "fake-model",
	})
	existingRaceJob := jobs.Job{
		ID:          reqID,
		OwnerID:     ownerID,
		ProjectID:   &projectID,
		Kind:        scriptgenerationjob.JobKind,
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

	svc := scriptgenerationjob.NewService(registry, jobsRepo, &mockProjectRepo{proj: validProj}, &mockProposalRepo{proposals: []creativeproposal.CreativeProposal{validProp}})

	view, err := svc.CreateGeneration(context.Background(), principal, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  reqID,
		ProviderID: "fake-provider",
		ModelID:    "fake-model",
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
	svc = scriptgenerationjob.NewService(registry, jobsRepo, &mockProjectRepo{proj: validProj}, &mockProposalRepo{proposals: []creativeproposal.CreativeProposal{validProp}})

	_, err = svc.CreateGeneration(context.Background(), principal, projectID, scriptgenerationjob.CreateScriptGenerationInput{
		RequestID:  reqID,
		ProviderID: "fake-provider",
		ModelID:    "fake-model-2",
	})
	if !errors.Is(err, scriptgenerationjob.ErrGenerationRequestConflict) {
		t.Fatalf("expected ErrGenerationRequestConflict for conflicting race input, got %v", err)
	}
	if getCallCount != 2 {
		t.Fatalf("expected exactly 2 GetByIDForProject calls for conflicting duplicate race, got %d", getCallCount)
	}
}
