package scenenarrationjob_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
)

type recordingTTS struct {
	mu        sync.Mutex
	requests  []providers.SpeechSynthesisRequest
	sampleWAV []byte
}

func (r *recordingTTS) SynthesizeSpeech(_ context.Context, req providers.SpeechSynthesisRequest) (providers.SpeechSynthesisResponse, error) {
	r.mu.Lock()
	r.requests = append(r.requests, req)
	r.mu.Unlock()

	audio, err := providers.NewGeneratedAudio("audio/wav", r.sampleWAV)
	if err != nil {
		return providers.SpeechSynthesisResponse{}, err
	}
	return providers.SpeechSynthesisResponse{
		Audio:   audio,
		ModelID: "tts-1",
		Voice: providers.VoiceMetadata{
			ID:          req.VoiceID,
			DisplayName: "Nova",
		},
	}, nil
}

type fakeAssetStore struct {
	mu     sync.Mutex
	assets map[uuid.UUID]mediaasset.MediaAsset
	byJob  map[uuid.UUID]mediaasset.MediaAsset
}

func newFakeAssetStore() *fakeAssetStore {
	return &fakeAssetStore{
		assets: make(map[uuid.UUID]mediaasset.MediaAsset),
		byJob:  make(map[uuid.UUID]mediaasset.MediaAsset),
	}
}

func (s *fakeAssetStore) FindGeneratedByJob(_ context.Context, _ project.Principal, _ uuid.UUID, jobID uuid.UUID) (mediaasset.MediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.byJob[jobID]
	if !ok {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return a, nil
}

func (s *fakeAssetStore) Store(_ context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := io.ReadAll(input.Reader)
	if err != nil {
		return mediaasset.MediaAsset{}, err
	}
	id := uuid.New()
	asset := mediaasset.MediaAsset{
		ID:        id,
		OwnerID:   principal.OwnerID,
		ProjectID: projectID,
		Kind:      input.Kind,
		Origin:    input.Origin,
		MimeType:  input.MimeType,
		ByteSize:  int64(len(data)),
		SHA256:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Metadata:  input.Metadata,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	s.assets[id] = asset

	var meta struct {
		JobID string `json:"job_id"`
	}
	if json.Unmarshal(input.Metadata, &meta) == nil && meta.JobID != "" {
		if jobUUID, err := uuid.Parse(meta.JobID); err == nil {
			s.byJob[jobUUID] = asset
		}
	}

	return asset, nil
}

type fakeBinder struct {
	mu       sync.Mutex
	bindings map[string]scenenarration.Binding
}

func newFakeBinder() *fakeBinder {
	return &fakeBinder{bindings: make(map[string]scenenarration.Binding)}
}

func (b *fakeBinder) GetActive(_ context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) (scenenarration.Binding, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := sceneKey
	binding, ok := b.bindings[k]
	if !ok || binding.OwnerID != principal.OwnerID || binding.ProjectID != projectID || binding.ScenePlanVersion != planVersion {
		return scenenarration.Binding{}, scenenarration.ErrNotFound
	}
	return binding, nil
}

func (b *fakeBinder) AssignNarration(_ context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string, assetID uuid.UUID) (scenenarration.Binding, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := sceneKey
	binding := scenenarration.Binding{
		ID:               uuid.New(),
		OwnerID:          principal.OwnerID,
		ProjectID:        projectID,
		ScenePlanVersion: planVersion,
		SceneKey:         sceneKey,
		Role:             scenenarration.RoleNarration,
		BindingVersion:   1,
		AssetID:          assetID,
		Status:           scenenarration.StatusActive,
		CreatedAt:        time.Now().UTC(),
	}
	b.bindings[k] = binding
	return binding, nil
}

type inMemoryChunkStore struct {
	mu          sync.Mutex
	chunks      map[string][]byte
	getErr      error
	putErr      error
	getCalls    int
	putCalls    int
}

func newInMemoryChunkStore() *inMemoryChunkStore {
	return &inMemoryChunkStore{chunks: make(map[string][]byte)}
}

func (s *inMemoryChunkStore) GetChunk(_ context.Context, _ uuid.UUID, jobID uuid.UUID, chunkIndex int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	key := jobID.String() + ":" + string(rune('0'+chunkIndex))
	data, ok := s.chunks[key]
	if !ok {
		return nil, nil
	}
	return data, nil
}

func (s *inMemoryChunkStore) PutChunk(_ context.Context, _ uuid.UUID, jobID uuid.UUID, chunkIndex int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putCalls++
	if s.putErr != nil {
		return s.putErr
	}
	key := jobID.String() + ":" + string(rune('0'+chunkIndex))
	s.chunks[key] = append([]byte(nil), data...)
	return nil
}

func (s *inMemoryChunkStore) DeleteChunks(_ context.Context, _ uuid.UUID, jobID uuid.UUID, totalChunks int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < totalChunks; i++ {
		key := jobID.String() + ":" + string(rune('0'+i))
		delete(s.chunks, key)
	}
	return nil
}

func TestSceneNarrationHandler_ChunkStoreReadFailureDoesNotSynthesize(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	sampleWAV := makeSampleWAV(1.0)
	tts := &recordingTTS{sampleWAV: sampleWAV}
	chunkStore := newInMemoryChunkStore()
	chunkStore.getErr = errors.New("object storage unavailable")
	assetStore := newFakeAssetStore()
	handler := scenenarrationjob.NewHandler(
		&fakeTTSRuntime{synthesizer: tts},
		assetStore,
		newFakeBinder(),
		chunkStore,
	)

	payloadBytes, err := json.Marshal(scenenarrationjob.Payload{
		SchemaVersion:    scenenarrationjob.SchemaVersion,
		ProviderID:       "openai",
		ModelID:          "tts-1",
		VoiceID:          "voice-nova",
		Format:           "wav",
		ScenePlanVersion: 1,
		SceneKey:         "sc-1",
		NarrationText:    "Storage recovery must be safe.",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	projectRef := projectID
	job := jobs.Job{ID: jobID, OwnerID: ownerID, ProjectID: &projectRef, Kind: scenenarrationjob.JobKind, Payload: payloadBytes}

	got, err := handler.Handle(ctx, job)
	if err == nil {
		t.Fatalf("expected storage error, got result %s", got)
	}
	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != scenenarrationjob.ErrorStorageFailed {
		t.Fatalf("expected retryable storage error, got %T %v", err, err)
	}
	if len(tts.requests) != 0 {
		t.Fatalf("expected no paid synthesis after chunk read failure, got %d calls", len(tts.requests))
	}
	if len(assetStore.assets) != 0 {
		t.Fatalf("expected no final asset after chunk read failure, got %d assets", len(assetStore.assets))
	}
}

func TestSceneNarrationHandler_ChunkStoreWriteFailureStopsPaidWork(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	sampleWAV := makeSampleWAV(1.0)
	tts := &recordingTTS{sampleWAV: sampleWAV}
	chunkStore := newInMemoryChunkStore()
	chunkStore.putErr = errors.New("object storage write failed")
	assetStore := newFakeAssetStore()
	handler := scenenarrationjob.NewHandler(
		&fakeTTSRuntime{synthesizer: tts},
		assetStore,
		newFakeBinder(),
		chunkStore,
	)

	payloadBytes, err := json.Marshal(scenenarrationjob.Payload{
		SchemaVersion:    scenenarrationjob.SchemaVersion,
		ProviderID:       "openai",
		ModelID:          "tts-1",
		VoiceID:          "voice-nova",
		Format:           "wav",
		ScenePlanVersion: 1,
		SceneKey:         "sc-1",
		NarrationText:    strings.Repeat("Paid narration chunk. ", 250),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	projectRef := projectID
	job := jobs.Job{ID: jobID, OwnerID: ownerID, ProjectID: &projectRef, Kind: scenenarrationjob.JobKind, Payload: payloadBytes}

	got, err := handler.Handle(ctx, job)
	if err == nil {
		t.Fatalf("expected storage error, got result %s", got)
	}
	var retryErr *jobs.RetryableJobError
	if !errors.As(err, &retryErr) || retryErr.Code != scenenarrationjob.ErrorStorageFailed {
		t.Fatalf("expected retryable storage error, got %T %v", err, err)
	}
	if len(tts.requests) != 1 {
		t.Fatalf("expected exactly one paid synthesis before checkpoint failure, got %d calls", len(tts.requests))
	}
	if chunkStore.putCalls != 1 {
		t.Fatalf("expected one failed checkpoint write, got %d calls", chunkStore.putCalls)
	}
	if len(assetStore.assets) != 0 {
		t.Fatalf("expected no final asset after checkpoint failure, got %d assets", len(assetStore.assets))
	}
}

func makeSampleWAV(durationSec float64) []byte {
	sampleRate := uint32(16000)
	numChannels := uint16(1)
	bitsPerSample := uint16(16)
	byteRate := sampleRate * uint32(numChannels) * uint32(bitsPerSample/8)
	blockAlign := numChannels * (bitsPerSample / 8)
	dataLen := uint32(float64(byteRate) * durationSec)
	riffLen := uint32(36 + dataLen)

	buf := new(bytes.Buffer)
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
	buf.Write(make([]byte, dataLen))
	return buf.Bytes()
}

func TestSceneNarrationHandler_Handle(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	voiceID := "voice-nova"

	sampleWAV := makeSampleWAV(1.0)
	tts := &recordingTTS{sampleWAV: sampleWAV}
	ttsRuntime := &fakeTTSRuntime{synthesizer: tts}
	assetStore := newFakeAssetStore()
	binder := newFakeBinder()
	chunkStore := newInMemoryChunkStore()

	handler := scenenarrationjob.NewHandler(ttsRuntime, assetStore, binder, chunkStore)

	payload := scenenarrationjob.Payload{
		SchemaVersion:    scenenarrationjob.SchemaVersion,
		ProviderID:       "openai",
		ModelID:          "tts-1",
		VoiceID:          voiceID,
		Format:           "wav",
		ScenePlanVersion: 1,
		SceneKey:         "sc-1",
		NarrationText:    "Đây là đoạn văn bản thuyết minh hoàn chỉnh cho phân cảnh thứ nhất.",
		AssignCurrent:    true,
	}
	payloadBytes, _ := json.Marshal(payload)

	job := jobs.Job{
		ID:        jobID,
		OwnerID:   ownerID,
		ProjectID: &projectID,
		Kind:      scenenarrationjob.JobKind,
		Payload:   payloadBytes,
	}

	t.Run("Executes synthesis, stores asset, assigns binding, and returns result", func(t *testing.T) {
		resBytes, err := handler.Handle(ctx, job)
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
		var result scenenarrationjob.Result
		if err := json.Unmarshal(resBytes, &result); err != nil {
			t.Fatalf("unmarshal result error: %v", err)
		}
		if result.MediaAssetID == uuid.Nil || !result.AssignedNarration || result.DurationSeconds <= 0 {
			t.Fatalf("unexpected result: %+v", result)
		}

		// Verify asset properties
		asset, err := assetStore.FindGeneratedByJob(ctx, project.Principal{OwnerID: ownerID}, projectID, jobID)
		if err != nil {
			t.Fatalf("asset was not found: %v", err)
		}
		if asset.Kind != mediaasset.KindAudio || asset.Origin != mediaasset.OriginGeneratedAudio {
			t.Fatalf("unexpected asset kind/origin: %+v", asset)
		}

		// Verify scene narration binding
		binding, err := binder.GetActive(ctx, project.Principal{OwnerID: ownerID}, projectID, 1, "sc-1")
		if err != nil {
			t.Fatalf("binding was not found: %v", err)
		}
		if binding.AssetID != asset.ID {
			t.Fatalf("binding asset mismatch: got %s, want %s", binding.AssetID, asset.ID)
		}
	})

	t.Run("Duplicate worker execution reuses existing asset and skips synthesis", func(t *testing.T) {
		tts.requests = nil // reset recorded requests
		resBytes, err := handler.Handle(ctx, job)
		if err != nil {
			t.Fatalf("duplicate execution error: %v", err)
		}
		var result scenenarrationjob.Result
		if err := json.Unmarshal(resBytes, &result); err != nil {
			t.Fatalf("unmarshal duplicate result error: %v", err)
		}
		if len(tts.requests) != 0 {
			t.Fatalf("expected 0 synthesis calls on duplicate execution, got %d", len(tts.requests))
		}
	})
}

func TestSceneNarrationHandler_DurableChunkRecovery(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	voiceID := "voice-nova"

	sampleWAV := makeSampleWAV(1.0)
	tts := &recordingTTS{sampleWAV: sampleWAV}
	ttsRuntime := &fakeTTSRuntime{synthesizer: tts}
	assetStore := newFakeAssetStore()
	binder := newFakeBinder()
	chunkStore := newInMemoryChunkStore()

	// Pre-populate chunk 0 in chunkStore simulating previous partial synthesis
	_ = chunkStore.PutChunk(ctx, projectID, jobID, 0, sampleWAV)

	handler := scenenarrationjob.NewHandler(ttsRuntime, assetStore, binder, chunkStore)

	payload := scenenarrationjob.Payload{
		SchemaVersion:    scenenarrationjob.SchemaVersion,
		ProviderID:       "openai",
		ModelID:          "tts-1",
		VoiceID:          voiceID,
		Format:           "wav",
		ScenePlanVersion: 1,
		SceneKey:         "sc-1",
		NarrationText:    "Câu thứ nhất dài vừa đủ. Câu thứ hai tiếp tục mạch văn.",
		AssignCurrent:    true,
	}
	payloadBytes, _ := json.Marshal(payload)

	job := jobs.Job{
		ID:        jobID,
		OwnerID:   ownerID,
		ProjectID: &projectID,
		Kind:      scenenarrationjob.JobKind,
		Payload:   payloadBytes,
	}

	resBytes, err := handler.Handle(ctx, job)
	if err != nil {
		t.Fatalf("handler handle error: %v", err)
	}
	var result scenenarrationjob.Result
	if err := json.Unmarshal(resBytes, &result); err != nil {
		t.Fatalf("unmarshal result error: %v", err)
	}
	if result.MediaAssetID == uuid.Nil {
		t.Fatalf("expected generated media asset ID")
	}
}
