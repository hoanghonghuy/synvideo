package scenenarrationjob_test

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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type fakeProjectRepo struct {
	projects map[uuid.UUID]project.Project
}

func (r *fakeProjectRepo) Get(_ context.Context, ownerID, projectID uuid.UUID) (project.Project, error) {
	p, ok := r.projects[projectID]
	if !ok || p.OwnerID != ownerID {
		return project.Project{}, project.ErrNotFound
	}
	return p, nil
}

type fakePlanRepo struct {
	plans map[int]sceneplan.Plan
}

func (r *fakePlanRepo) GetByVersion(_ context.Context, _, projectID uuid.UUID, version int) (sceneplan.Plan, error) {
	p, ok := r.plans[version]
	if !ok || p.ProjectID != projectID {
		return sceneplan.Plan{}, sceneplan.ErrNotFound
	}
	return p, nil
}

type fakeTTSRuntime struct {
	synthesizer providers.SpeechSynthesizer
	err         error
}

func (r *fakeTTSRuntime) ResolveSpeechSynthesizer(_ context.Context, _ uuid.UUID, _ providers.ProviderID, _ providers.ModelID) (providers.SpeechSynthesizer, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.synthesizer, nil
}

type fakeJobsRepo struct {
	jobMap map[uuid.UUID]jobs.Job
}

func (r *fakeJobsRepo) Enqueue(_ context.Context, input jobs.EnqueueInput) (jobs.Job, error) {
	if _, exists := r.jobMap[input.ID]; exists {
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
	r.jobMap[input.ID] = j
	return j, nil
}

func (r *fakeJobsRepo) GetByIDForProject(_ context.Context, ownerID, projectID, jobID uuid.UUID) (jobs.Job, error) {
	j, ok := r.jobMap[jobID]
	if !ok || j.OwnerID != ownerID || j.ProjectID == nil || *j.ProjectID != projectID {
		return jobs.Job{}, jobs.ErrJobNotFound
	}
	return j, nil
}

func TestSceneNarrationJobService_CreateGeneration(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	voiceID := providers.VoiceID("voice-nova")

	projRepo := &fakeProjectRepo{
		projects: map[uuid.UUID]project.Project{
			projectID: {ID: projectID, OwnerID: ownerID},
		},
	}
	planRepo := &fakePlanRepo{
		plans: map[int]sceneplan.Plan{
			1: {
				ProjectID: projectID,
				Version:   1,
				Status:    sceneplan.StatusApproved,
				Scenes: []sceneplan.Scene{
					{Key: "sc-1", Narration: "Lời dẫn truyện của phân cảnh một."},
					{Key: "sc-empty", Narration: ""},
				},
			},
			2: {
				ProjectID: projectID,
				Version:   2,
				Status:    sceneplan.StatusDraft,
				Scenes: []sceneplan.Scene{
					{Key: "sc-1", Narration: "Bản nháp."},
				},
			},
		},
	}

	synth := fake.NewSpeechSynthesizer([]byte("audio-bytes")).WithVoice(providers.VoiceMetadata{
		ID:          voiceID,
		DisplayName: "Nova",
	})
	ttsRuntime := &fakeTTSRuntime{synthesizer: synth}
	jobsRepo := &fakeJobsRepo{jobMap: make(map[uuid.UUID]jobs.Job)}

	svc := scenenarrationjob.NewService(ttsRuntime, jobsRepo, projRepo, planRepo)

	t.Run("Fails if unauthenticated", func(t *testing.T) {
		_, err := svc.CreateGeneration(ctx, project.Principal{}, projectID, 1, "sc-1", scenenarrationjob.CreateGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "openai",
			ModelID:    "tts-1",
			VoiceID:    string(voiceID),
		})
		if !errors.Is(err, scenenarrationjob.ErrUnauthenticated) {
			t.Fatalf("expected ErrUnauthenticated, got %v", err)
		}
	})

	t.Run("Fails if request ID is nil", func(t *testing.T) {
		_, err := svc.CreateGeneration(ctx, principal, projectID, 1, "sc-1", scenenarrationjob.CreateGenerationInput{
			RequestID:  uuid.Nil,
			ProviderID: "openai",
			ModelID:    "tts-1",
			VoiceID:    string(voiceID),
		})
		if !errors.Is(err, scenenarrationjob.ErrInvalidRequestID) {
			t.Fatalf("expected ErrInvalidRequestID, got %v", err)
		}
	})

	t.Run("Fails if plan is not approved", func(t *testing.T) {
		_, err := svc.CreateGeneration(ctx, principal, projectID, 2, "sc-1", scenenarrationjob.CreateGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "openai",
			ModelID:    "tts-1",
			VoiceID:    string(voiceID),
		})
		if !errors.Is(err, scenenarrationjob.ErrScenePlanNotApproved) {
			t.Fatalf("expected ErrScenePlanNotApproved, got %v", err)
		}
	})

	t.Run("Fails if scene has empty narration", func(t *testing.T) {
		_, err := svc.CreateGeneration(ctx, principal, projectID, 1, "sc-empty", scenenarrationjob.CreateGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "openai",
			ModelID:    "tts-1",
			VoiceID:    string(voiceID),
		})
		if !errors.Is(err, scenenarrationjob.ErrEmptyNarration) {
			t.Fatalf("expected ErrEmptyNarration, got %v", err)
		}
	})

	t.Run("Fails if provider runtime is unavailable", func(t *testing.T) {
		badRuntime := &fakeTTSRuntime{err: providers.ErrProviderUnavailable}
		badSvc := scenenarrationjob.NewService(badRuntime, jobsRepo, projRepo, planRepo)
		_, err := badSvc.CreateGeneration(ctx, principal, projectID, 1, "sc-1", scenenarrationjob.CreateGenerationInput{
			RequestID:  uuid.New(),
			ProviderID: "openai",
			ModelID:    "tts-1",
			VoiceID:    string(voiceID),
		})
		if !errors.Is(err, scenenarrationjob.ErrProviderUnavailable) {
			t.Fatalf("expected ErrProviderUnavailable, got %v", err)
		}
	})

	t.Run("Successfully enqueues job with exact source snapshot", func(t *testing.T) {
		reqID := uuid.New()
		view, err := svc.CreateGeneration(ctx, principal, projectID, 1, "sc-1", scenenarrationjob.CreateGenerationInput{
			RequestID:     reqID,
			ProviderID:    "openai",
			ModelID:       "tts-1",
			VoiceID:       string(voiceID),
			Format:        "mp3",
			AssignCurrent: true,
		})
		if err != nil {
			t.Fatalf("unexpected create generation error: %v", err)
		}
		if view.ID != reqID || view.State != string(jobs.StateQueued) {
			t.Fatalf("unexpected job view: %+v", view)
		}

		enqueuedJob := jobsRepo.jobMap[reqID]
		var payload scenenarrationjob.Payload
		if err := json.Unmarshal(enqueuedJob.Payload, &payload); err != nil {
			t.Fatalf("unmarshal payload error: %v", err)
		}
		if payload.NarrationText != "Lời dẫn truyện của phân cảnh một." {
			t.Fatalf("narration text snapshot mismatch: %q", payload.NarrationText)
		}
		if payload.SceneKey != "sc-1" || payload.ScenePlanVersion != 1 || payload.VoiceID != string(voiceID) {
			t.Fatalf("payload fields mismatch: %+v", payload)
		}
	})

	t.Run("Idempotent replay returns same job view", func(t *testing.T) {
		reqID := uuid.New()
		input := scenenarrationjob.CreateGenerationInput{
			RequestID:     reqID,
			ProviderID:    "openai",
			ModelID:       "tts-1",
			VoiceID:       string(voiceID),
			Format:        "mp3",
			AssignCurrent: true,
		}
		v1, err := svc.CreateGeneration(ctx, principal, projectID, 1, "sc-1", input)
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		v2, err := svc.CreateGeneration(ctx, principal, projectID, 1, "sc-1", input)
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if v1.ID != v2.ID || v1.State != v2.State {
			t.Fatalf("idempotent replay returned different views: v1=%+v v2=%+v", v1, v2)
		}
	})

	t.Run("Conflicting parameters with same request ID returns conflict error", func(t *testing.T) {
		reqID := uuid.New()
		input := scenenarrationjob.CreateGenerationInput{
			RequestID:     reqID,
			ProviderID:    "openai",
			ModelID:       "tts-1",
			VoiceID:       string(voiceID),
			Format:        "mp3",
			AssignCurrent: true,
		}
		_, err := svc.CreateGeneration(ctx, principal, projectID, 1, "sc-1", input)
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		conflictingInput := input
		conflictingInput.VoiceID = "voice-alloy"
		_, err = svc.CreateGeneration(ctx, principal, projectID, 1, "sc-1", conflictingInput)
		if !errors.Is(err, scenenarrationjob.ErrGenerationRequestConflict) {
			t.Fatalf("expected ErrGenerationRequestConflict, got %v", err)
		}
	})
}
