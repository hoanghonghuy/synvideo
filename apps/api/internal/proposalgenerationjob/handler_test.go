package proposalgenerationjob_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/proposalgenerationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
)

type mockProposalRepo struct {
	createdDraft   creativeproposal.CreativeProposal
	createDraftErr error
	createdInput   creativeproposal.CreateDraftInput
}

func (m *mockProposalRepo) List(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]creativeproposal.CreativeProposal, error) {
	return nil, nil
}
func (m *mockProposalRepo) Get(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (creativeproposal.CreativeProposal, error) {
	return creativeproposal.CreativeProposal{}, nil
}
func (m *mockProposalRepo) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input creativeproposal.CreateDraftInput) (creativeproposal.CreativeProposal, error) {
	m.createdInput = input
	if m.createDraftErr != nil {
		return creativeproposal.CreativeProposal{}, m.createDraftErr
	}
	res := m.createdDraft
	res.ProjectID = projectID
	if res.Version == 0 {
		res.Version = 1
	}
	return res, nil
}
func (m *mockProposalRepo) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input creativeproposal.PutInput) (creativeproposal.CreativeProposal, error) {
	return creativeproposal.CreativeProposal{}, nil
}
func (m *mockProposalRepo) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (creativeproposal.CreativeProposal, error) {
	return creativeproposal.CreativeProposal{}, nil
}

func validProposalJSON() string {
	return `{
		"title_options": ["Launch video"],
		"hook_options": ["What if video creation took seconds?"],
		"audience_summary": "Creators",
		"objective_summary": "Brand awareness",
		"narrative_angle": "Creative partnership",
		"structure": [
			{"key": "opening", "title": "Opening", "purpose": "Hook attention"}
		]
	}`
}

func sampleJob(payload proposalgenerationjob.Payload) jobs.Job {
	projectID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	jobID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	return jobs.Job{
		ID:        jobID,
		OwnerID:   ownerID,
		ProjectID: &projectID,
		Kind:      proposalgenerationjob.JobKind,
		Payload:   payloadBytes,
	}
}

func validPayload() proposalgenerationjob.Payload {
	return proposalgenerationjob.Payload{
		SchemaVersion: proposalgenerationjob.SchemaVersion,
		ProviderID:    "fake-provider",
		ModelID:       "fake-model",
		Project: proposalgeneration.ProjectContext{
			ID:            uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			ContentFormat: project.ContentFormatShort,
			AspectRatio:   project.AspectRatio9x16,
			Locale:        project.LocaleVI,
		},
		Brief: proposalgeneration.BriefContext{
			Revision:            2,
			SourceText:          "Create an AI launch video",
			TargetAudience:      "Creators",
			Objective:           "Awareness",
			DesiredStyle:        "Dynamic",
			Tone:                "Energetic",
			DistributionTargets: []creativebrief.DistributionTarget{creativebrief.DistributionTargetYouTube},
			CallToAction:        "Subscribe",
		},
	}
}

func TestHandler_Success(t *testing.T) {
	textGen := fake.NewTextGenerator(validProposalJSON())
	engine := proposalgeneration.NewWithGenerator(textGen)
	mockRepo := &mockProposalRepo{
		createdDraft: creativeproposal.CreativeProposal{
			Version: 3,
			Status:  creativeproposal.StatusDraft,
		},
	}

	handler := proposalgenerationjob.NewHandler(engine, mockRepo)
	job := sampleJob(validPayload())

	rawResult, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	var res proposalgenerationjob.Result
	if err := json.Unmarshal(rawResult, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.ProposalVersion != 3 {
		t.Fatalf("expected ProposalVersion 3, got %d", res.ProposalVersion)
	}
	if mockRepo.createdInput.SourceGenerationJobID == nil || *mockRepo.createdInput.SourceGenerationJobID != job.ID {
		t.Fatalf("expected SourceGenerationJobID %v, got %v", job.ID, mockRepo.createdInput.SourceGenerationJobID)
	}
	if mockRepo.createdInput.SourceBriefRevision != 2 {
		t.Fatalf("expected SourceBriefRevision 2, got %d", mockRepo.createdInput.SourceBriefRevision)
	}
}

func TestHandler_InvalidPayload(t *testing.T) {
	textGen := fake.NewTextGenerator(validProposalJSON())
	engine := proposalgeneration.NewWithGenerator(textGen)
	mockRepo := &mockProposalRepo{}
	handler := proposalgenerationjob.NewHandler(engine, mockRepo)

	t.Run("nil project ID", func(t *testing.T) {
		job := sampleJob(validPayload())
		job.ProjectID = nil
		_, err := handler.Handle(context.Background(), job)
		var termErr *jobs.TerminalJobError
		if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_PAYLOAD" {
			t.Fatalf("expected terminal GENERATION_INVALID_PAYLOAD, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		job := sampleJob(validPayload())
		job.Payload = []byte(`invalid json`)
		_, err := handler.Handle(context.Background(), job)
		var termErr *jobs.TerminalJobError
		if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_PAYLOAD" {
			t.Fatalf("expected terminal GENERATION_INVALID_PAYLOAD, got %v", err)
		}
	})

	t.Run("wrong schema version", func(t *testing.T) {
		payload := validPayload()
		payload.SchemaVersion = "wrong_version"
		job := sampleJob(payload)
		_, err := handler.Handle(context.Background(), job)
		var termErr *jobs.TerminalJobError
		if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_PAYLOAD" {
			t.Fatalf("expected terminal GENERATION_INVALID_PAYLOAD, got %v", err)
		}
	})
}

func TestHandler_GenerationErrors(t *testing.T) {
	t.Run("invalid output is terminal", func(t *testing.T) {
		textGen := fake.NewTextGenerator(`{"not_matching_schema": true}`)
		engine := proposalgeneration.NewWithGenerator(textGen)
		mockRepo := &mockProposalRepo{}
		handler := proposalgenerationjob.NewHandler(engine, mockRepo)

		_, err := handler.Handle(context.Background(), sampleJob(validPayload()))
		var termErr *jobs.TerminalJobError
		if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_OUTPUT" {
			t.Fatalf("expected terminal GENERATION_INVALID_OUTPUT, got: %v", err)
		}
	})

	t.Run("provider unavailable is retryable", func(t *testing.T) {
		engine := proposalgeneration.NewWithGenerator(proposalgeneration.FailingGenerator{
			Err: providers.ErrProviderUnavailable,
		})
		mockRepo := &mockProposalRepo{}
		handler := proposalgenerationjob.NewHandler(engine, mockRepo)

		_, err := handler.Handle(context.Background(), sampleJob(validPayload()))
		var retryErr *jobs.RetryableJobError
		if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PROVIDER_UNAVAILABLE" {
			t.Fatalf("expected retryable GENERATION_PROVIDER_UNAVAILABLE, got: %v", err)
		}
	})

	t.Run("provider failed is retryable", func(t *testing.T) {
		engine := proposalgeneration.NewWithGenerator(proposalgeneration.FailingGenerator{
			Err: providers.ErrProviderExecution,
		})
		mockRepo := &mockProposalRepo{}
		handler := proposalgenerationjob.NewHandler(engine, mockRepo)

		_, err := handler.Handle(context.Background(), sampleJob(validPayload()))
		var retryErr *jobs.RetryableJobError
		if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PROVIDER_FAILED" {
			t.Fatalf("expected retryable GENERATION_PROVIDER_FAILED, got: %v", err)
		}
	})

	t.Run("context canceled propagates directly", func(t *testing.T) {
		textGen := fake.NewTextGenerator(validProposalJSON()).WithDelay(100 * time.Millisecond)
		engine := proposalgeneration.NewWithGenerator(textGen)
		mockRepo := &mockProposalRepo{}
		handler := proposalgenerationjob.NewHandler(engine, mockRepo)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := handler.Handle(ctx, sampleJob(validPayload()))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected raw context.Canceled, got: %v", err)
		}
	})

	t.Run("persistence error is retryable", func(t *testing.T) {
		textGen := fake.NewTextGenerator(validProposalJSON())
		engine := proposalgeneration.NewWithGenerator(textGen)
		mockRepo := &mockProposalRepo{
			createDraftErr: errors.New("db connection failure"),
		}
		handler := proposalgenerationjob.NewHandler(engine, mockRepo)

		_, err := handler.Handle(context.Background(), sampleJob(validPayload()))
		var retryErr *jobs.RetryableJobError
		if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PERSISTENCE_FAILED" {
			t.Fatalf("expected retryable GENERATION_PERSISTENCE_FAILED, got: %v", err)
		}
	})
}
