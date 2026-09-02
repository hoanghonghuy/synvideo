package sceneplangenerationjob_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplangenerationjob"
)

type mockScenePlanRepo struct {
	createdDraft   sceneplan.Plan
	createDraftErr error
	createdInput   sceneplan.CreateDraftInput
}

func (m *mockScenePlanRepo) ListVersions(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID) ([]sceneplan.Plan, error) {
	return nil, nil
}
func (m *mockScenePlanRepo) GetByVersion(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int) (sceneplan.Plan, error) {
	return sceneplan.Plan{}, nil
}
func (m *mockScenePlanRepo) CreateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, input sceneplan.CreateDraftInput) (sceneplan.Plan, error) {
	m.createdInput = input
	if m.createDraftErr != nil {
		return sceneplan.Plan{}, m.createDraftErr
	}
	res := m.createdDraft
	res.ProjectID = projectID
	if res.Version == 0 {
		res.Version = 1
	}
	return res, nil
}
func (m *mockScenePlanRepo) UpdateDraft(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, input sceneplan.PutInput) (sceneplan.Plan, error) {
	return sceneplan.Plan{}, nil
}
func (m *mockScenePlanRepo) Approve(ctx context.Context, ownerID uuid.UUID, projectID uuid.UUID, version int, revision int) (sceneplan.Plan, error) {
	return sceneplan.Plan{}, nil
}

func validScenePlanJSON() string {
	return `{
		"scenes": [
			{
				"key": "intro-scene",
				"script_section_key": "intro",
				"narration": "Welcome to our product tour.",
				"visual_instruction": "Show hero banner with product logo.",
				"planned_source_type": "stock",
				"expected_duration_seconds": 10,
				"caption_intent": "Highlight title",
				"transition_notes": "Fade in"
			},
			{
				"key": "body-scene",
				"script_section_key": "body",
				"narration": "Here are the top three features you need to know.",
				"visual_instruction": "Demonstrate the app UI in action.",
				"planned_source_type": "upload",
				"expected_duration_seconds": 20,
				"caption_intent": "Feature bullet points",
				"transition_notes": "Cut"
			},
			{
				"key": "outro-scene",
				"script_section_key": "outro",
				"narration": "Sign up today at our website.",
				"visual_instruction": "Call to action splash screen.",
				"planned_source_type": "creator_media",
				"expected_duration_seconds": 10,
				"caption_intent": "Website URL and CTA",
				"transition_notes": "Fade to black"
			}
		]
	}`
}

func sampleScenePlanJob(payload sceneplangenerationjob.Payload) jobs.Job {
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
		Kind:      sceneplangenerationjob.JobKind,
		Payload:   payloadBytes,
	}
}

func validScenePlanPayload() sceneplangenerationjob.Payload {
	return sceneplangenerationjob.Payload{
		SchemaVersion: sceneplangenerationjob.SchemaVersion,
		ProviderID:    "fake-provider",
		ModelID:       "fake-model",
		Project: sceneplangeneration.ProjectContext{
			ID:            uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			ContentFormat: project.ContentFormatShort,
			AspectRatio:   project.AspectRatio9x16,
			Locale:        project.LocaleVI,
		},
		Script: sceneplangeneration.ScriptContext{
			Version:               1,
			SourceProposalVersion: 1,
			Sections: []sceneplangeneration.ScriptSection{
				{Key: "intro", Heading: "Introduction", Body: "Welcome to our product tour."},
				{Key: "body", Heading: "Main Features", Body: "Here are the top three features you need to know."},
				{Key: "outro", Heading: "Call to Action", Body: "Sign up today at our website."},
			},
		},
		Proposal: sceneplangeneration.ProposalContext{
			Version:          1,
			VisualDirection:  "Visual direction",
			VoiceDirection:   "Voice direction",
			MusicDirection:   "Music direction",
			CaptionDirection: "Caption direction",
		},
	}
}

func TestHandler_Handle_Success(t *testing.T) {
	textGen := fake.NewTextGenerator(validScenePlanJSON())
	engine := sceneplangeneration.NewWithGenerator(textGen)
	scenePlanRepo := &mockScenePlanRepo{
		createdDraft: sceneplan.Plan{
			Version: 2,
			Status:  sceneplan.StatusDraft,
		},
	}

	handler := sceneplangenerationjob.NewHandler(engine, scenePlanRepo)
	job := sampleScenePlanJob(validScenePlanPayload())

	resBytes, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	var res sceneplangenerationjob.Result
	if err := json.Unmarshal(resBytes, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if res.ScenePlanVersion != 2 {
		t.Fatalf("expected scene plan version 2, got %d", res.ScenePlanVersion)
	}

	if scenePlanRepo.createdInput.SourceScriptVersion != 1 {
		t.Fatalf("expected source script version 1, got %d", scenePlanRepo.createdInput.SourceScriptVersion)
	}
	if scenePlanRepo.createdInput.SourceGenerationJobID == nil || *scenePlanRepo.createdInput.SourceGenerationJobID != job.ID {
		t.Fatalf("expected source generation job ID %s, got %v", job.ID, scenePlanRepo.createdInput.SourceGenerationJobID)
	}
	if scenePlanRepo.createdInput.ContentLocale != string(project.LocaleVI) {
		t.Fatalf("expected content locale %s, got %s", project.LocaleVI, scenePlanRepo.createdInput.ContentLocale)
	}
	if len(scenePlanRepo.createdInput.Scenes) != 3 {
		t.Fatalf("expected 3 scenes, got %d", len(scenePlanRepo.createdInput.Scenes))
	}
	if scenePlanRepo.createdInput.Scenes[0].Key != "intro-scene" {
		t.Fatalf("expected key intro-scene, got %s", scenePlanRepo.createdInput.Scenes[0].Key)
	}
}

func TestHandler_Handle_StrictPayloadDecoding(t *testing.T) {
	textGen := fake.NewTextGenerator(validScenePlanJSON())
	engine := sceneplangeneration.NewWithGenerator(textGen)
	scenePlanRepo := &mockScenePlanRepo{}
	handler := sceneplangenerationjob.NewHandler(engine, scenePlanRepo)

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
				p := validScenePlanPayload()
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
				p := validScenePlanPayload()
				p.Project.ID = uuid.Nil
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "mismatched project ID between job and payload",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Project.ID = uuid.New()
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "empty provider ID",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.ProviderID = ""
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "empty model ID",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.ModelID = "   "
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project content format",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Project.ContentFormat = "invalid_format"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project aspect ratio",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Project.AspectRatio = "3:4"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project target duration out of range",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				dur := 0
				p.Project.TargetDurationSeconds = &dur
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid project locale",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Project.Locale = "fr"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid script version",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Version = 0
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid script source proposal version",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.SourceProposalVersion = 0
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "empty script sections",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections = nil
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "script section item with empty key",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections = []sceneplangeneration.ScriptSection{
					{Key: "   ", Heading: "Heading", Body: "Body"},
				}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "script section item with invalid key format",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections[0].Key = "INVALID KEY WITH SPACES"
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "script section item with key exceeding max length",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections[0].Key = strings.Repeat("a", 65)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "duplicate script section keys",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections = []sceneplangeneration.ScriptSection{
					{Key: "intro", Heading: "Heading 1", Body: "Body 1"},
					{Key: "intro", Heading: "Heading 2", Body: "Body 2"},
				}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "script section with blank body",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections[0].Body = "   "
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "script section with oversized body",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections[0].Body = strings.Repeat("a", 20001)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "script section with oversized heading",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections[0].Heading = strings.Repeat("a", 301)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "script sections exceeding max count",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Sections = make([]sceneplangeneration.ScriptSection, 201)
				for i := range p.Script.Sections {
					p.Script.Sections[i] = sceneplangeneration.ScriptSection{
						Key:  fmt.Sprintf("sec-%d", i),
						Body: "Body",
					}
				}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "invalid script estimated duration",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				dur := 0
				p.Script.EstimatedDurationSeconds = &dur
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "oversized script notes",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Script.Notes = strings.Repeat("a", 10001)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "mismatched proposal version for script source proposal version",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.Version = 99
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "oversized proposal visual direction",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.VisualDirection = strings.Repeat("a", 5001)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "oversized proposal voice direction (3001 runes)",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.VoiceDirection = strings.Repeat("a", 3001)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "oversized proposal music direction (3001 runes)",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.MusicDirection = strings.Repeat("a", 3001)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "oversized proposal caption direction (3001 runes)",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.CaptionDirection = strings.Repeat("a", 3001)
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "proposal warnings exceeding max items (21 items)",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.Warnings = make([]string, 21)
				for i := range p.Proposal.Warnings {
					p.Proposal.Warnings[i] = "warning"
				}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "proposal warnings with blank item",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.Warnings = []string{"valid warning", "   "}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "proposal warnings with item exceeding max length (1001 runes)",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.Warnings = []string{strings.Repeat("a", 1001)}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "proposal research gaps exceeding max items (21 items)",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.ResearchGaps = make([]string, 21)
				for i := range p.Proposal.ResearchGaps {
					p.Proposal.ResearchGaps[i] = "gap"
				}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "proposal research gaps with blank item",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.ResearchGaps = []string{"valid gap", "   "}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
		{
			name: "proposal research gaps with item exceeding max length (1001 runes)",
			modifyJob: func(j *jobs.Job) {
				p := validScenePlanPayload()
				p.Proposal.ResearchGaps = []string{strings.Repeat("a", 1001)}
				b, _ := json.Marshal(p)
				j.Payload = b
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := sampleScenePlanJob(validScenePlanPayload())
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

type trackingResolver struct {
	called bool
}

func (r *trackingResolver) ResolveTextGenerator(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, modelID providers.ModelID) (providers.TextGenerator, error) {
	r.called = true
	return fake.NewTextGenerator(validScenePlanJSON()), nil
}

func TestHandler_Handle_ResolverNotCalledOnInvalidPayload(t *testing.T) {
	cases := []struct {
		name          string
		modifyPayload func(p *sceneplangenerationjob.Payload)
	}{
		{
			name: "invalid script: duplicate section keys",
			modifyPayload: func(p *sceneplangenerationjob.Payload) {
				p.Script.Sections = []sceneplangeneration.ScriptSection{
					{Key: "dup-key", Heading: "H1", Body: "B1"},
					{Key: "dup-key", Heading: "H2", Body: "B2"},
				}
			},
		},
		{
			name: "invalid proposal: oversized voice direction (3001 runes)",
			modifyPayload: func(p *sceneplangenerationjob.Payload) {
				p.Proposal.VoiceDirection = strings.Repeat("a", 3001)
			},
		},
		{
			name: "invalid proposal: blank warning item",
			modifyPayload: func(p *sceneplangenerationjob.Payload) {
				p.Proposal.Warnings = []string{"   "}
			},
		},
		{
			name: "invalid proposal: 21 warning items",
			modifyPayload: func(p *sceneplangenerationjob.Payload) {
				p.Proposal.Warnings = make([]string, 21)
				for i := range p.Proposal.Warnings {
					p.Proposal.Warnings[i] = "warning"
				}
			},
		},
		{
			name: "invalid proposal: 1001 runes research gap item",
			modifyPayload: func(p *sceneplangenerationjob.Payload) {
				p.Proposal.ResearchGaps = []string{strings.Repeat("g", 1001)}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &trackingResolver{}
			scenePlanRepo := &mockScenePlanRepo{}
			handler := sceneplangenerationjob.NewHandlerWithResolver(resolver, scenePlanRepo)

			job := sampleScenePlanJob(validScenePlanPayload())
			p := validScenePlanPayload()
			tc.modifyPayload(&p)
			job.Payload, _ = json.Marshal(p)

			_, err := handler.Handle(context.Background(), job)
			if err == nil {
				t.Fatal("expected error on invalid payload, got nil")
			}
			if resolver.called {
				t.Fatal("expected resolver to NOT be called for invalid payload before validation")
			}
		})
	}
}

func TestHandler_Handle_ProviderErrorMappings(t *testing.T) {
	scenePlanRepo := &mockScenePlanRepo{}

	// Invalid JSON output -> terminal GENERATION_INVALID_OUTPUT
	genBadOutput := fake.NewTextGenerator("not json")
	h1 := sceneplangenerationjob.NewHandler(sceneplangeneration.NewWithGenerator(genBadOutput), scenePlanRepo)
	_, err := h1.Handle(context.Background(), sampleScenePlanJob(validScenePlanPayload()))
	var termErr *jobs.TerminalJobError
	if !errors.As(err, &termErr) || termErr.Code != "GENERATION_INVALID_OUTPUT" {
		t.Fatalf("expected terminal GENERATION_INVALID_OUTPUT, got %v", err)
	}

	// Provider execution failure -> retryable GENERATION_PROVIDER_FAILED
	genFailed := &sceneplangeneration.FailingGenerator{Err: providers.ErrProviderExecution}
	h2 := sceneplangenerationjob.NewHandler(sceneplangeneration.NewWithGenerator(genFailed), scenePlanRepo)
	_, err = h2.Handle(context.Background(), sampleScenePlanJob(validScenePlanPayload()))
	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PROVIDER_FAILED" {
		t.Fatalf("expected retryable GENERATION_PROVIDER_FAILED, got %v", err)
	}

	// Provider unavailable -> retryable GENERATION_PROVIDER_UNAVAILABLE
	genUnavail := &sceneplangeneration.FailingGenerator{Err: providers.ErrProviderUnavailable}
	h3 := sceneplangenerationjob.NewHandler(sceneplangeneration.NewWithGenerator(genUnavail), scenePlanRepo)
	_, err = h3.Handle(context.Background(), sampleScenePlanJob(validScenePlanPayload()))
	if !errors.As(err, &retryErr) || retryErr.Code != "GENERATION_PROVIDER_UNAVAILABLE" {
		t.Fatalf("expected retryable GENERATION_PROVIDER_UNAVAILABLE, got %v", err)
	}

	// Context cancellation preserved
	genCanceled := &sceneplangeneration.FailingGenerator{Err: context.Canceled}
	h4 := sceneplangenerationjob.NewHandler(sceneplangeneration.NewWithGenerator(genCanceled), scenePlanRepo)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = h4.Handle(canceledCtx, sampleScenePlanJob(validScenePlanPayload()))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestHandler_Handle_PersistenceErrorMapped(t *testing.T) {
	textGen := fake.NewTextGenerator(validScenePlanJSON())
	engine := sceneplangeneration.NewWithGenerator(textGen)
	scenePlanRepo := &mockScenePlanRepo{
		createDraftErr: errors.New("db connection lost"),
	}

	handler := sceneplangenerationjob.NewHandler(engine, scenePlanRepo)
	_, err := handler.Handle(context.Background(), sampleScenePlanJob(validScenePlanPayload()))

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
						"content": validScenePlanJSON(),
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
		Enabled:             true,
		EnabledTextModelIDs: []providers.ModelID{"gpt-4o"},
		APIKey:              &secretKey,
	})
	if err != nil {
		t.Fatalf("put owner setting: %v", err)
	}

	scenePlanRepo := &mockScenePlanRepo{
		createdDraft: sceneplan.Plan{
			Version: 1,
			Status:  sceneplan.StatusDraft,
		},
	}

	handler := sceneplangenerationjob.NewHandlerWithResolver(runtimeService, scenePlanRepo)

	payload := validScenePlanPayload()
	payload.ProviderID = "byok-openai"
	payload.ModelID = "gpt-4o"
	job := sampleScenePlanJob(payload)

	resBytes, err := handler.Handle(context.Background(), job)
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	var res sceneplangenerationjob.Result
	if err := json.Unmarshal(resBytes, &res); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if res.ScenePlanVersion != 1 {
		t.Fatalf("expected scene plan version 1, got %d", res.ScenePlanVersion)
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
