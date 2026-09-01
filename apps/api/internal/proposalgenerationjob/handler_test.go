package proposalgenerationjob_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
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

	t.Run("unknown fields in payload terminalizes as GENERATION_INVALID_PAYLOAD before engine work", func(t *testing.T) {
		payloadMap := map[string]any{
			"schema_version": proposalgenerationjob.SchemaVersion,
			"provider_id":    "fake-provider",
			"model_id":       "fake-model",
			"project": map[string]any{
				"id":                      "22222222-2222-4222-8222-222222222222",
				"content_format":          "short",
				"aspect_ratio":            "9:16",
				"target_duration_seconds": 60,
				"locale":                  "vi",
			},
			"brief": map[string]any{
				"revision":    2,
				"source_text": "Create an AI launch video",
			},
			"unknown_field": "injected_payload_field",
		}
		rawJSON, _ := json.Marshal(payloadMap)
		job := sampleJob(validPayload())
		job.Payload = rawJSON

		_, err := handler.Handle(context.Background(), job)
		var termErr *jobs.TerminalJobError
		if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_PAYLOAD" {
			t.Fatalf("expected terminal GENERATION_INVALID_PAYLOAD for unknown fields, got %v", err)
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

type mockResolver struct {
	resolveFn func(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error)
}

func (m *mockResolver) ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, ownerID, providerID, modelID)
	}
	return nil, providers.ErrProviderUnavailable
}

func TestHandler_WithResolver(t *testing.T) {
	t.Run("resolves owner generator dynamically and produces draft", func(t *testing.T) {
		var resolvedOwnerID uuid.UUID
		res := &mockResolver{
			resolveFn: func(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error) {
				resolvedOwnerID = ownerID
				return fake.NewTextGenerator(validProposalJSON()), nil
			},
		}
		mockRepo := &mockProposalRepo{
			createdDraft: creativeproposal.CreativeProposal{Version: 3},
		}
		handler := proposalgenerationjob.NewHandlerWithResolver(res, mockRepo)

		job := sampleJob(validPayload())
		out, err := handler.Handle(context.Background(), job)
		if err != nil {
			t.Fatalf("handle with resolver: %v", err)
		}
		if resolvedOwnerID != job.OwnerID {
			t.Fatalf("expected resolved owner %v, got %v", job.OwnerID, resolvedOwnerID)
		}

		var result proposalgenerationjob.Result
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if result.ProposalVersion != 3 {
			t.Fatalf("expected ProposalVersion 3, got %d", result.ProposalVersion)
		}
	})

	t.Run("credential unavailable at execution maps to retryable GENERATION_PROVIDER_UNAVAILABLE", func(t *testing.T) {
		res := &mockResolver{
			resolveFn: func(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error) {
				return nil, providers.ErrProviderUnavailable
			},
		}
		mockRepo := &mockProposalRepo{}
		handler := proposalgenerationjob.NewHandlerWithResolver(res, mockRepo)

		job := sampleJob(validPayload())
		_, err := handler.Handle(context.Background(), job)
		var retryErr *jobs.RetryableJobError
		if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PROVIDER_UNAVAILABLE" {
			t.Fatalf("expected retryable GENERATION_PROVIDER_UNAVAILABLE, got: %v", err)
		}
	})
}

type memorySettingsRepository struct {
	settings map[string]providersettings.Setting
}

func (m *memorySettingsRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]providersettings.Setting, error) {
	return nil, nil
}
func (m *memorySettingsRepository) GetByOwnerAndProvider(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID) (providersettings.Setting, error) {
	s, ok := m.settings[ownerID.String()+":"+string(providerID)]
	if !ok {
		return providersettings.Setting{}, providersettings.ErrSettingNotFound
	}
	return s, nil
}
func (m *memorySettingsRepository) Save(ctx context.Context, setting providersettings.Setting, expectedRevision *int) (providersettings.Setting, error) {
	if m.settings == nil {
		m.settings = make(map[string]providersettings.Setting)
	}
	setting.Revision = 1
	m.settings[setting.OwnerID.String()+":"+string(setting.ProviderID)] = setting
	return setting, nil
}
func (m *memorySettingsRepository) Delete(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, expectedRevision int) error {
	delete(m.settings, ownerID.String()+":"+string(providerID))
	return nil
}

func TestHandler_EndToEndOwnerCredentialResolutionAndIsolation(t *testing.T) {
	ownerA := uuid.New()
	ownerB := uuid.New()
	keyA := "sk-owner-A-secret-key-11111"
	keyB := "sk-owner-B-secret-key-22222"

	var receivedAuthHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if receivedAuthHeader != "Bearer "+keyA && receivedAuthHeader != "Bearer "+keyB {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": validProposalJSON(),
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer upstream.Close()

	cat, err := providersettings.NewCatalog([]providersettings.ProviderDefinition{
		{
			ProviderID:  "openai",
			DisplayName: "OpenAI",
			BaseURL:     upstream.URL,
			Models: []providersettings.ModelDefinition{
				{
					ModelID:         "gpt-5-mini",
					DisplayName:     "GPT-5 mini",
					ExternalModelID: "gpt-5-mini",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}

	masterKey := make([]byte, 32)
	_, _ = rand.Read(masterKey)
	cipher, err := providersettings.NewAESGCMCipher(hex.EncodeToString(masterKey), "v1")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}

	repo := &memorySettingsRepository{settings: make(map[string]providersettings.Setting)}
	svc := providersettings.NewService(cat, repo, cipher, upstream.Client())

	ctx := context.Background()
	_, err = svc.PutSetting(ctx, ownerA, "openai", providersettings.PutSettingInput{
		Enabled:         true,
		EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
		APIKey:          &keyA,
	})
	if err != nil {
		t.Fatalf("put owner A setting: %v", err)
	}

	_, err = svc.PutSetting(ctx, ownerB, "openai", providersettings.PutSettingInput{
		Enabled:         true,
		EnabledModelIDs: []providers.ModelID{"gpt-5-mini"},
		APIKey:          &keyB,
	})
	if err != nil {
		t.Fatalf("put owner B setting: %v", err)
	}

	mockRepo := &mockProposalRepo{
		createdDraft: creativeproposal.CreativeProposal{Version: 1},
	}
	handler := proposalgenerationjob.NewHandlerWithResolver(svc, mockRepo)

	// Execute Job for Owner A
	payloadA := validPayload()
	payloadA.ProviderID = "openai"
	payloadA.ModelID = "gpt-5-mini"

	jobA := sampleJob(payloadA)
	jobA.OwnerID = ownerA

	outA, err := handler.Handle(ctx, jobA)
	if err != nil {
		t.Fatalf("handle job for owner A: %v", err)
	}

	// Verify Owner A's key was sent upstream and NOT Owner B's key
	if receivedAuthHeader != "Bearer "+keyA {
		t.Fatalf("expected Authorization 'Bearer %s', got %q", keyA, receivedAuthHeader)
	}
	// Verify durable job payload and result bytes are 100% secret-free
	if bytes.Contains(jobA.Payload, []byte(keyA)) || bytes.Contains(jobA.Payload, []byte(keyB)) {
		t.Fatalf("job payload contains sensitive API key")
	}
	if bytes.Contains(outA, []byte(keyA)) || bytes.Contains(outA, []byte(keyB)) {
		t.Fatalf("job result contains sensitive API key")
	}

	// Execute Job for Owner B
	jobB := sampleJob(payloadA)
	jobB.OwnerID = ownerB

	outB, err := handler.Handle(ctx, jobB)
	if err != nil {
		t.Fatalf("handle job for owner B: %v", err)
	}

	// Verify Owner B's key was sent upstream and NOT Owner A's key
	if receivedAuthHeader != "Bearer "+keyB {
		t.Fatalf("expected Authorization 'Bearer %s', got %q", keyB, receivedAuthHeader)
	}
	if bytes.Contains(jobB.Payload, []byte(keyA)) || bytes.Contains(jobB.Payload, []byte(keyB)) {
		t.Fatalf("job payload contains sensitive API key")
	}
	if bytes.Contains(outB, []byte(keyA)) || bytes.Contains(outB, []byte(keyB)) {
		t.Fatalf("job result contains sensitive API key")
	}
}
