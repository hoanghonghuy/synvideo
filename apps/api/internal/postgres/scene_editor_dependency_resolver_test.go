package postgres

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestSceneEditorDurationMSFromMetadata(t *testing.T) {
	durationMS, err := sceneEditorDurationMSFromMetadata(json.RawMessage(`{"duration_seconds":1.2346}`))
	if err != nil {
		t.Fatalf("duration: %v", err)
	}
	if durationMS != 1_235 {
		t.Fatalf("duration=%d want 1235", durationMS)
	}

	for _, raw := range []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{"duration_seconds":0}`),
		json.RawMessage(`not-json`),
	} {
		if _, err := sceneEditorDurationMSFromMetadata(raw); err == nil {
			t.Fatalf("raw=%q unexpectedly accepted", raw)
		}
	}
}

func TestSceneEditorNarrationLineageIDIsDeterministicAndIdentityBound(t *testing.T) {
	bindingID := uuid.New()
	assetID := uuid.New()
	first := sceneEditorNarrationLineageID(3, "intro", bindingID, assetID, 2_500)
	second := sceneEditorNarrationLineageID(3, "intro", bindingID, assetID, 2_500)
	if first == uuid.Nil || first != second {
		t.Fatalf("lineage not deterministic: %s %s", first, second)
	}
	if changed := sceneEditorNarrationLineageID(3, "intro", bindingID, assetID, 2_501); changed == first {
		t.Fatal("duration change must change lineage")
	}
	if changed := sceneEditorNarrationLineageID(3, "other", bindingID, assetID, 2_500); changed == first {
		t.Fatal("scene change must change lineage")
	}
}

func TestSceneEditorCaptionLastEndMSUsesPersistedSegments(t *testing.T) {
	lastEndMS, err := sceneEditorCaptionLastEndMS(json.RawMessage(`[
		{"id":"a","start_ms":0,"end_ms":900},
		{"id":"b","start_ms":900,"end_ms":1750}
	]`))
	if err != nil {
		t.Fatalf("last end: %v", err)
	}
	if lastEndMS != 1_750 {
		t.Fatalf("last_end_ms=%d want 1750", lastEndMS)
	}

	for _, raw := range []json.RawMessage{
		json.RawMessage(`[]`),
		json.RawMessage(`[{"end_ms":0}]`),
		json.RawMessage(`not-json`),
	} {
		if _, err := sceneEditorCaptionLastEndMS(raw); err == nil {
			t.Fatalf("raw=%q unexpectedly accepted", raw)
		}
	}
}
