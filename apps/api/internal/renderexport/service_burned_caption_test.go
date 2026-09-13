package renderexport

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestEnqueueCaptionSnapshotBindsBurnedCaptionProfileToPayloadAndDedupe(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()
	digest := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	store := &snapshotStoreStub{snapshot: sceneeditor.Snapshot{
		SchemaVersion: sceneeditor.SnapshotSchemaVersion,
		ProjectID:     projectID,
		Digest:        digest,
		Scenes: []sceneeditor.Scene{{
			Caption: &sceneeditor.CaptionRef{DocumentID: uuid.New(), Revision: 7, LineageID: uuid.New(), LastEndMS: 900},
		}},
	}}
	queue := &jobQueueStub{job: jobs.Job{ID: jobID}}
	service := NewService(store, queue, func() uuid.UUID { return jobID })

	if _, err := service.Enqueue(context.Background(), ownerID, projectID, digest); err != nil {
		t.Fatal(err)
	}
	wantDedupe := "render:" + projectID.String() + ":" + digest + ":" + LocalProfileID + ":" + BurnedCaptionProfileV1
	if queue.input.DedupeKey == nil || *queue.input.DedupeKey != wantDedupe {
		t.Fatalf("dedupe key = %v, want %q", queue.input.DedupeKey, wantDedupe)
	}
	var payload RenderPayload
	if err := json.Unmarshal(queue.input.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.BurnedCaptionProfileID != BurnedCaptionProfileV1 || payload.SubtitleMode != SubtitleModeOff {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDecodeRenderPayloadRejectsUnknownBurnedCaptionProfile(t *testing.T) {
	payload, err := json.Marshal(RenderPayload{
		SnapshotDigest:         "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		SnapshotSchema:         sceneeditor.SnapshotSchemaVersion,
		ProfileID:              LocalProfileID,
		BurnedCaptionProfileID: "burned_caption_v999",
		SubtitleMode:           SubtitleModeOff,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeRenderPayload(payload); err == nil {
		t.Fatal("decodeRenderPayload accepted an unknown burned-caption profile")
	}
}
