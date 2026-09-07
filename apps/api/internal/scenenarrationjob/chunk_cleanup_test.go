package scenenarrationjob_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarrationjob"
)

type failingCleanupChunkStore struct {
	deleteErr   error
	deleteCalls int
}

func (s *failingCleanupChunkStore) GetChunk(context.Context, uuid.UUID, uuid.UUID, int) ([]byte, error) {
	return nil, nil
}

func (s *failingCleanupChunkStore) PutChunk(context.Context, uuid.UUID, uuid.UUID, int, []byte) error {
	return nil
}

func (s *failingCleanupChunkStore) DeleteChunks(context.Context, uuid.UUID, uuid.UUID, int) error {
	s.deleteCalls++
	return s.deleteErr
}

func TestSceneNarrationHandler_RetriesCheckpointCleanupForCommittedAsset(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	assetStore := newFakeAssetStore()
	assetStore.byJob[jobID] = mediaasset.MediaAsset{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		ProjectID: projectID,
		Metadata:  json.RawMessage(`{"duration_seconds":1.25}`),
	}
	chunkStore := &failingCleanupChunkStore{deleteErr: errors.New("object storage delete failed")}
	handler := scenenarrationjob.NewHandler(nil, assetStore, nil, chunkStore)

	payload, err := json.Marshal(scenenarrationjob.Payload{
		SchemaVersion:    scenenarrationjob.SchemaVersion,
		ProviderID:       "openai",
		ModelID:          "tts-1",
		VoiceID:          "voice-nova",
		Format:           "wav",
		ScenePlanVersion: 1,
		SceneKey:         "sc-1",
		NarrationText:    "Committed generation must not strand durable checkpoints.",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	job := jobs.Job{ID: jobID, OwnerID: ownerID, ProjectID: &projectID, Kind: scenenarrationjob.JobKind, Payload: payload}

	if _, err := handler.Handle(ctx, job); err == nil {
		t.Fatal("expected checkpoint cleanup failure to keep the job retryable")
	} else {
		var retryErr *jobs.RetryableJobError
		if !errors.As(err, &retryErr) || retryErr.Code != scenenarrationjob.ErrorStorageFailed {
			t.Fatalf("expected retryable storage error, got %T %v", err, err)
		}
	}
	if chunkStore.deleteCalls != 1 {
		t.Fatalf("expected one cleanup attempt, got %d", chunkStore.deleteCalls)
	}

	chunkStore.deleteErr = nil
	result, err := handler.Handle(ctx, job)
	if err != nil {
		t.Fatalf("expected retry to converge after storage recovery: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected completed job result after cleanup converged")
	}
	if chunkStore.deleteCalls != 2 {
		t.Fatalf("expected cleanup to be retried, got %d attempts", chunkStore.deleteCalls)
	}
}
