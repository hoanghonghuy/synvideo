package postgres

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers/fake"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

type integrationTTSRuntime struct {
	synthesizer providers.SpeechSynthesizer
}

func (r integrationTTSRuntime) ResolveSpeechSynthesizer(_ context.Context, _ uuid.UUID, _ providers.ProviderID, _ providers.ModelID) (providers.SpeechSynthesizer, error) {
	return r.synthesizer, nil
}

func makeIntegrationWAV(sampleRate uint32, numChannels uint16, pcmData []byte) []byte {
	buf := new(bytes.Buffer)
	bitsPerSample := uint16(16)
	byteRate := sampleRate * uint32(numChannels) * uint32(bitsPerSample/8)
	blockAlign := numChannels * (bitsPerSample / 8)
	dataLen := uint32(len(pcmData))
	riffLen := uint32(36 + dataLen)

	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, riffLen)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, numChannels)
	_ = binary.Write(buf, binary.LittleEndian, sampleRate)
	_ = binary.Write(buf, binary.LittleEndian, byteRate)
	_ = binary.Write(buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(buf, binary.LittleEndian, bitsPerSample)
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, dataLen)
	buf.Write(pcmData)

	return buf.Bytes()
}

func TestSceneNarrationPostgresIntegration(t *testing.T) {
	pool := integrationPool(t)
	storage := integrationStorage(t)
	ctx := context.Background()

	projectRepo := NewProjectRepository(pool)
	proposalRepo := NewCreativeProposalRepository(pool)
	scriptRepo := NewScriptRepository(pool)
	scenePlanRepo := NewScenePlanRepository(pool)
	assetRepo := NewMediaAssetRepository(pool)
	bindingRepo := NewSceneNarrationBindingRepository(pool)
	assetService := mediaasset.NewService(projectRepo, assetRepo, storage)

	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	proj, err := projectRepo.Create(ctx, ownerID, validIntegrationCreateInput("Narration Test Project"))
	if err != nil {
		t.Fatalf("create project error: %v", err)
	}

	proposal, err := proposalRepo.CreateDraft(ctx, ownerID, proj.ID, creativeproposalCreateDraftInput("Narration Proposal"))
	if err != nil {
		t.Fatalf("create proposal error: %v", err)
	}
	proposal, err = proposalRepo.Approve(ctx, ownerID, proj.ID, proposal.Version, proposal.Revision)
	if err != nil {
		t.Fatalf("approve proposal error: %v", err)
	}

	scriptVersion, err := scriptRepo.CreateDraft(ctx, ownerID, proj.ID, script.CreateDraftInput{
		SourceProposalVersion: proposal.Version,
		Content:               validScriptContent("Narration Source"),
	})
	if err != nil {
		t.Fatalf("create script error: %v", err)
	}
	scriptVersion, err = scriptRepo.Approve(ctx, ownerID, proj.ID, scriptVersion.Version, scriptVersion.Revision)
	if err != nil {
		t.Fatalf("approve script error: %v", err)
	}

	plan, err := scenePlanRepo.CreateDraft(ctx, ownerID, proj.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptVersion.Version,
		Content:             validScenePlanContent("Narration Source"),
	})
	if err != nil {
		t.Fatalf("create plan draft error: %v", err)
	}

	approvedPlan, err := scenePlanRepo.Approve(ctx, ownerID, proj.ID, plan.Version, plan.Revision)
	if err != nil {
		t.Fatalf("approve plan error: %v", err)
	}

	sampleAudio := makeIntegrationWAV(16000, 1, make([]byte, 32000)) // 1.0s WAV

	// Ingest audio asset 1
	asset1, err := assetService.Store(ctx, project.Principal{OwnerID: ownerID}, proj.ID, mediaasset.CreateInput{
		Kind:     mediaasset.KindAudio,
		Origin:   mediaasset.OriginGeneratedAudio,
		MimeType: "audio/wav",
		Reader:   bytes.NewReader(sampleAudio),
		MaxBytes: 10 << 20,
	})
	if err != nil {
		t.Fatalf("store audio asset 1 error: %v", err)
	}

	// Ingest audio asset 2
	asset2, err := assetService.Store(ctx, project.Principal{OwnerID: ownerID}, proj.ID, mediaasset.CreateInput{
		Kind:     mediaasset.KindAudio,
		Origin:   mediaasset.OriginGeneratedAudio,
		MimeType: "audio/wav",
		Reader:   bytes.NewReader(sampleAudio),
		MaxBytes: 10 << 20,
	})
	if err != nil {
		t.Fatalf("store audio asset 2 error: %v", err)
	}

	firstSceneKey := approvedPlan.Scenes[0].Key
	secondSceneKey := approvedPlan.Scenes[1].Key

	// 1. Assign asset 1 to first scene
	b1, err := bindingRepo.Assign(ctx, ownerID, proj.ID, approvedPlan.Version, firstSceneKey, asset1.ID)
	if err != nil {
		t.Fatalf("assign b1 error: %v", err)
	}
	if b1.BindingVersion != 1 || b1.Status != scenenarration.StatusActive || b1.AssetID != asset1.ID {
		t.Fatalf("unexpected b1: %+v", b1)
	}

	// 2. Idempotent assign
	b1Replay, err := bindingRepo.Assign(ctx, ownerID, proj.ID, approvedPlan.Version, firstSceneKey, asset1.ID)
	if err != nil {
		t.Fatalf("assign b1 replay error: %v", err)
	}
	if b1Replay.ID != b1.ID || b1Replay.BindingVersion != 1 {
		t.Fatalf("idempotent replay mismatch: %+v", b1Replay)
	}

	// 3. Assign asset 2 to first scene (replaces asset 1)
	b2, err := bindingRepo.Assign(ctx, ownerID, proj.ID, approvedPlan.Version, firstSceneKey, asset2.ID)
	if err != nil {
		t.Fatalf("assign b2 error: %v", err)
	}
	if b2.BindingVersion != 2 || b2.Status != scenenarration.StatusActive || b2.AssetID != asset2.ID {
		t.Fatalf("unexpected b2: %+v", b2)
	}

	// 4. Verify Active
	active, err := bindingRepo.GetActive(ctx, ownerID, proj.ID, approvedPlan.Version, firstSceneKey)
	if err != nil {
		t.Fatalf("get active error: %v", err)
	}
	if active.ID != b2.ID || active.AssetID != asset2.ID {
		t.Fatalf("active binding mismatch: %+v", active)
	}

	// 5. Verify History
	history, err := bindingRepo.ListHistory(ctx, ownerID, proj.ID, approvedPlan.Version, firstSceneKey)
	if err != nil {
		t.Fatalf("list history error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].ID != b2.ID || history[0].Status != scenenarration.StatusActive {
		t.Fatalf("expected b2 first in history: %+v", history[0])
	}
	if history[1].ID != b1.ID || history[1].Status != scenenarration.StatusSuperseded || history[1].SupersededAt == nil {
		t.Fatalf("expected b1 superseded in history: %+v", history[1])
	}

	// 6. Test full End-to-End durable job execution with chunkStore and Postgres
	voiceID := providers.VoiceID("voice-nova")
	synth := fake.NewSpeechSynthesizer(sampleAudio).WithVoice(providers.VoiceMetadata{
		ID:          voiceID,
		DisplayName: "Nova",
	}).WithMIMEType("audio/wav")
	runtime := integrationTTSRuntime{synthesizer: synth}
	assetStore := scenenarrationjob.NewAssetStore(assetService, assetRepo)
	narrationService := scenenarration.NewService(bindingRepo, scenePlanRepo, assetRepo)
	chunkStore := scenenarrationjob.NewObjectStorageChunkStore(storage)

	handler := scenenarrationjob.NewHandler(runtime, assetStore, narrationService, chunkStore)

	jobID := uuid.New()
	jobPayload, _ := json.Marshal(scenenarrationjob.Payload{
		SchemaVersion:    scenenarrationjob.SchemaVersion,
		ProviderID:       "openai",
		ModelID:          "tts-1",
		VoiceID:          string(voiceID),
		Format:           "wav",
		ScenePlanVersion: approvedPlan.Version,
		SceneKey:         secondSceneKey,
		NarrationText:    approvedPlan.Scenes[1].Narration,
		AssignCurrent:    true,
	})

	job := jobs.Job{
		ID:        jobID,
		OwnerID:   ownerID,
		ProjectID: &proj.ID,
		Kind:      scenenarrationjob.JobKind,
		Payload:   jobPayload,
	}

	resBytes, err := handler.Handle(ctx, job)
	if err != nil {
		t.Fatalf("handler execution error: %v", err)
	}
	var result scenenarrationjob.Result
	if err := json.Unmarshal(resBytes, &result); err != nil {
		t.Fatalf("unmarshal result error: %v", err)
	}
	if result.MediaAssetID == uuid.Nil || !result.AssignedNarration || result.DurationSeconds <= 0 {
		t.Fatalf("unexpected handler result: %+v", result)
	}

	// Verify secondSceneKey now has active narration binding in DB
	activeSc2, err := bindingRepo.GetActive(ctx, ownerID, proj.ID, approvedPlan.Version, secondSceneKey)
	if err != nil {
		t.Fatalf("get sc-2 active error: %v", err)
	}
	if activeSc2.AssetID != result.MediaAssetID {
		t.Fatalf("active sc-2 asset mismatch: got %s, want %s", activeSc2.AssetID, result.MediaAssetID)
	}
}
