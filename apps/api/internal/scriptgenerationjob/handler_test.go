package scriptgenerationjob_test

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

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scriptgenerationjob"
)

type mockScriptRepo struct {
	createdDraft   script.Script
	createDraftErr error
	createdInput   script.CreateDraftInput
}

func (m *mockScriptRepo) ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]script.Script, error) {
	return nil, nil
}
func (m *mockScriptRepo) GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (script.Script, error) {
	return script.Script{}, nil
}
func (m *mockScriptRepo) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input script.CreateDraftInput) (script.Script, error) {
	m.createdInput = input
	if m.createDraftErr != nil {
		return script.Script{}, m.createDraftErr
	}
	res := m.createdDraft
	res.ProjectID = projectID
	if res.Version == 0 {
		res.Version = 1
	}
	return res, nil
}
func (m *mockScriptRepo) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input script.PutInput) (script.Script, error) {
	return script.Script{}, nil
}
func (m *mockScriptRepo) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (script.Script, error) {
	return script.Script{}, nil
}

func validScriptJSON() string {
	return `{
		"sections": [
			{"key": "intro", "heading": "Introduction", "body": "Welcome to our product tour."},
			{"key": "body", "heading": "Main Features", "body": "Here are the top three features you need to know."},
			{"key": "outro", "heading": "Call to Action", "body": "Sign up today at our website."}
		],
		"estimated_duration_seconds": 60,
		"notes": "Fast paced"
	}`
}

func sampleScriptJob(payload scriptgenerationjob.Payload) jobs.Job {
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
		Kind:      scriptgenerationjob.JobKind,
		Payload:   payloadBytes,
	}
}

func validScriptPayload() scriptgenerationjob.Payload {
	return scriptgenerationjob.Payload{
		SchemaVersion: scriptgenerationjob.SchemaVersion,
		ProviderID:    "fake-provider",
		ModelID:       "fake-model",
		Project: scriptgeneration.ProjectContext{
			ID:            uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			ContentFormat: project.ContentFormatShort,
			AspectRatio:   project.AspectRatio9x16,
			Locale:        project.LocaleVI,
		},
		Proposal: scriptgeneration.ProposalContext{
			Version:          1,
			TitleOptions:     []string{"Title 1"},
			HookOptions:      []string{"Hook 1"},
			AudienceSummary:  "Audience",
			ObjectiveSummary: "Objective",
			NarrativeAngle:   "Angle",
			Structure: []scriptgeneration.ProposalStructureItem{
				{Key: "intro", Title: "Intro", Purpose: "Hook"},
				{Key: "body", Title: "Body", Purpose: "Value"},
				{Key: "outro", Title: "Outro", Purpose: "CTA"},
			},
		},
	}
}

func TestHandler_Handle_Success(t *testing.T) {
	textGen := fake.NewTextGenerator(validScriptJSON())
	engine := scriptgeneration.NewWithGenerator(textGen)
	scriptRepo := &mockScriptRepo{
		createdDraft: script.Script{
			Version: 2,
			Status:  script.StatusDraft,
		},
	}

	handler := scriptgenerationjob.NewHandler(engine, scriptRepo)
	job := sampleScriptJob(validScriptPayload())

	resBytes, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	var res scriptgenerationjob.Result
	if err := json.Unmarshal(resBytes, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if res.ScriptVersion != 2 {
		t.Fatalf("expected script version 2, got %d", res.ScriptVersion)
	}

	if scriptRepo.createdInput.SourceProposalVersion != 1 {
		t.Fatalf("expected source proposal version 1, got %d", scriptRepo.createdInput.SourceProposalVersion)
	}
	if scriptRepo.createdInput.SourceGenerationJobID == nil || *scriptRepo.createdInput.SourceGenerationJobID != job.ID {
		t.Fatalf("expected source generation job ID %s, got %v", job.ID, scriptRepo.createdInput.SourceGenerationJobID)
	}
	if len(scriptRepo.createdInput.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(scriptRepo.createdInput.Sections))
	}
	if scriptRepo.createdInput.Sections[0].Key != "intro" {
		t.Fatalf("expected key intro, got %s", scriptRepo.createdInput.Sections[0].Key)
	}
	if scriptRepo.createdInput.ContentLocale != string(project.LocaleVI) {
		t.Fatalf("expected content locale %s, got %s", project.LocaleVI, scriptRepo.createdInput.ContentLocale)
	}
}

func TestHandler_Handle_StrictPayloadDecoding(t *testing.T) {
	textGen := fake.NewTextGenerator(validScriptJSON())
	engine := scriptgeneration.NewWithGenerator(textGen)
	scriptRepo := &mockScriptRepo{}
	handler := scriptgenerationjob.NewHandler(engine, scriptRepo)

	testCases := []struct {
		name      string
		modifyJob func(j *jobs.Job)
	}{
		{
			name: "unknown field in payload",
			modifyJob: func(j *jobs.Job) {
				var rawMap map[string]interface{}
				_ = json.Unmarshal(j.Payload, &rawMap)
				rawMap["unknown_field"] = "malicious"
				j.Payload, _ = json.Marshal(rawMap)
			},
		},
		{
			name: "trailing content after json payload",
			modifyJob: func(j *jobs.Job) {
				j.Payload = append(j.Payload, []byte(` {"extra":"trailing"}`)...)
			},
		},
		{
			name: "wrong schema version",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.SchemaVersion = "wrong_version_v2"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "missing project ID on job",
			modifyJob: func(j *jobs.Job) {
				j.ProjectID = nil
			},
		},
		{
			name: "nil project ID in payload",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Project.ID = uuid.Nil
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "mismatched project ID between job and payload",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Project.ID = uuid.New()
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "empty provider ID",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.ProviderID = ""
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "empty model ID",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.ModelID = "   "
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project content format",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Project.ContentFormat = "invalid_format"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project aspect ratio",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Project.AspectRatio = "3:4"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project target duration out of range",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				dur := 0
				p.Project.TargetDurationSeconds = &dur
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project locale",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Project.Locale = "fr"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid proposal version",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Proposal.Version = 0
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "empty proposal structure",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Proposal.Structure = nil
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "proposal structure item with empty key",
			modifyJob: func(j *jobs.Job) {
				p := validScriptPayload()
				p.Proposal.Structure = []scriptgeneration.ProposalStructureItem{
					{Key: "   ", Title: "Title", Purpose: "Purpose"},
				}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := sampleScriptJob(validScriptPayload())
			tc.modifyJob(&job)

			_, err := handler.Handle(context.Background(), job)
			if err == nil {
				t.Fatalf("expected error for case %q, got nil", tc.name)
			}
			var termErr *jobs.TerminalJobError
			if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_PAYLOAD" {
				t.Fatalf("expected terminal GENERATION_INVALID_PAYLOAD for %q, got %v", tc.name, err)
			}
		})
	}
}

func TestHandler_Handle_ProviderErrorMappings(t *testing.T) {
	scriptRepo := &mockScriptRepo{}

	// Invalid JSON output -> terminal GENERATION_INVALID_OUTPUT
	genBadOutput := fake.NewTextGenerator("not json")
	h1 := scriptgenerationjob.NewHandler(scriptgeneration.NewWithGenerator(genBadOutput), scriptRepo)
	_, err := h1.Handle(context.Background(), sampleScriptJob(validScriptPayload()))
	var termErr *jobs.TerminalJobError
	if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_OUTPUT" {
		t.Fatalf("expected terminal GENERATION_INVALID_OUTPUT, got %v", err)
	}

	// Provider execution failure -> retryable GENERATION_PROVIDER_FAILED
	genFailed := &scriptgeneration.FailingGenerator{Err: providers.ErrProviderExecution}
	h2 := scriptgenerationjob.NewHandler(scriptgeneration.NewWithGenerator(genFailed), scriptRepo)
	_, err = h2.Handle(context.Background(), sampleScriptJob(validScriptPayload()))
	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PROVIDER_FAILED" {
		t.Fatalf("expected retryable GENERATION_PROVIDER_FAILED, got %v", err)
	}

	// Provider unavailable -> retryable GENERATION_PROVIDER_UNAVAILABLE
	genUnavail := &scriptgeneration.FailingGenerator{Err: providers.ErrProviderUnavailable}
	h3 := scriptgenerationjob.NewHandler(scriptgeneration.NewWithGenerator(genUnavail), scriptRepo)
	_, err = h3.Handle(context.Background(), sampleScriptJob(validScriptPayload()))
	if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PROVIDER_UNAVAILABLE" {
		t.Fatalf("expected retryable GENERATION_PROVIDER_UNAVAILABLE, got %v", err)
	}

	// Context cancellation preserved
	genCanceled := &scriptgeneration.FailingGenerator{Err: context.Canceled}
	h4 := scriptgenerationjob.NewHandler(scriptgeneration.NewWithGenerator(genCanceled), scriptRepo)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = h4.Handle(canceledCtx, sampleScriptJob(validScriptPayload()))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestHandler_Handle_PersistenceErrorMapped(t *testing.T) {
	textGen := fake.NewTextGenerator(validScriptJSON())
	engine := scriptgeneration.NewWithGenerator(textGen)
	scriptRepo := &mockScriptRepo{
		createDraftErr: errors.New("db connection lost"),
	}

	handler := scriptgenerationjob.NewHandler(engine, scriptRepo)
	_, err := handler.Handle(context.Background(), sampleScriptJob(validScriptPayload()))

	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PERSISTENCE_FAILED" {
		t.Fatalf("expected retryable GENERATION_PERSISTENCE_FAILED, got %v", err)
	}
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

func TestHandler_Handle_OwnerCredentialResolution_LiveHttptest(t *testing.T) {
	secretKey := "secret-owner-api-key-xyz"
	var receivedAuthHeader string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if receivedAuthHeader != "Bearer "+secretKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "chatcmpl-test",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": validScriptJSON(),
					},
				},
			},
		})
	}))
	defer upstream.Close()

	catalog, err := providersettings.NewCatalog([]providersettings.ProviderDefinition{
		{
			ProviderID:  "byok-openai",
			DisplayName: "OpenAI BYOK",
			BaseURL:     upstream.URL,
			Models: []providersettings.ModelDefinition{
				{ModelID: "gpt-4o", DisplayName: "GPT-4o", ExternalModelID: "gpt-4o", Capabilities: []providersettings.Capability{providersettings.CapabilityText}},
			},
		},
	})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}

	masterKey := make([]byte, 32)
	_, _ = rand.Read(masterKey)
	keyHex := hex.EncodeToString(masterKey)
	cipher, err := providersettings.NewAESGCMCipher(keyHex, "v1")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}

	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	settingsRepo := &memorySettingsRepository{settings: make(map[string]providersettings.Setting)}
	runtimeService := providersettings.NewService(catalog, settingsRepo, cipher, upstream.Client())

	ctx := context.Background()
	_, err = runtimeService.PutSetting(ctx, ownerID, "byok-openai", providersettings.PutSettingInput{
		Enabled:         true,
		EnabledTextModelIDs: []providers.ModelID{"gpt-4o"},
		APIKey:          &secretKey,
	})
	if err != nil {
		t.Fatalf("put owner setting: %v", err)
	}

	scriptRepo := &mockScriptRepo{
		createdDraft: script.Script{
			Version: 1,
			Status:  script.StatusDraft,
		},
	}

	handler := scriptgenerationjob.NewHandlerWithResolver(runtimeService, scriptRepo)

	payload := validScriptPayload()
	payload.ProviderID = "byok-openai"
	payload.ModelID = "gpt-4o"
	job := sampleScriptJob(payload)

	resBytes, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	var res scriptgenerationjob.Result
	if err := json.Unmarshal(resBytes, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.ScriptVersion != 1 {
		t.Fatalf("expected script version 1, got %d", res.ScriptVersion)
	}

	expectedAuth := "Bearer " + secretKey
	if receivedAuthHeader != expectedAuth {
		t.Fatalf("expected auth header %q, got %q", expectedAuth, receivedAuthHeader)
	}

	// Verify no secret in result bytes
	if bytes.Contains(resBytes, []byte(secretKey)) {
		t.Fatalf("secret key leaked in result bytes: %s", string(resBytes))
	}
}
