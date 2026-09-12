package renderexport

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/captions"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestBuildSingleSceneSnapshotWebVTTUsesPinnedCaptionContent(t *testing.T) {
	ownerID, projectID := uuid.New(), uuid.New()
	sceneID, documentID := uuid.New(), uuid.New()
	reader := &snapshotCaptionReaderStub{doc: captions.Document{
		ID: documentID, OwnerID: ownerID, ProjectID: projectID,
		ScenePlanVersion: 3, SceneKey: "scene-a", Revision: 2,
		Segments: []captions.Segment{{ID: uuid.New(), StartMS: 100, EndMS: 800, Text: "Pinned <caption>"}},
	}}
	snapshot := sceneeditor.Snapshot{
		ProjectID: projectID, ScenePlanVersion: 3,
		Scenes: []sceneeditor.Scene{{ID: sceneID, SceneKey: "scene-a", DurationMS: 1000, Caption: &sceneeditor.CaptionRef{DocumentID: documentID, Revision: 2, LineageID: uuid.New(), LastEndMS: 800}}},
	}

	payload, present, err := BuildSingleSceneSnapshotWebVTT(context.Background(), reader, ownerID, projectID, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !present || !strings.Contains(string(payload), "Pinned &lt;caption&gt;") {
		t.Fatalf("unexpected sidecar present=%v payload=%q", present, payload)
	}
	if len(reader.reads) != 1 || reader.reads[0].revision != 2 {
		t.Fatalf("unexpected exact revision reads: %+v", reader.reads)
	}
}

func TestBuildSingleSceneSnapshotWebVTTNoCaptionProducesNoSidecar(t *testing.T) {
	ownerID, projectID := uuid.New(), uuid.New()
	reader := &snapshotCaptionReaderStub{}
	snapshot := sceneeditor.Snapshot{ProjectID: projectID, ScenePlanVersion: 1, Scenes: []sceneeditor.Scene{{ID: uuid.New(), SceneKey: "silent", DurationMS: 1000}}}
	payload, present, err := BuildSingleSceneSnapshotWebVTT(context.Background(), reader, ownerID, projectID, snapshot)
	if err != nil || present || payload != nil || len(reader.reads) != 0 {
		t.Fatalf("unexpected no-caption result payload=%q present=%v err=%v reads=%d", payload, present, err, len(reader.reads))
	}
}

func TestBuildSingleSceneSnapshotWebVTTRejectsCuePastSceneDuration(t *testing.T) {
	ownerID, projectID := uuid.New(), uuid.New()
	documentID := uuid.New()
	reader := &snapshotCaptionReaderStub{doc: captions.Document{
		ID: documentID, OwnerID: ownerID, ProjectID: projectID,
		ScenePlanVersion: 1, SceneKey: "scene-a", Revision: 1,
		Segments: []captions.Segment{{ID: uuid.New(), StartMS: 900, EndMS: 1200, Text: "too late"}},
	}}
	snapshot := sceneeditor.Snapshot{ProjectID: projectID, ScenePlanVersion: 1, Scenes: []sceneeditor.Scene{{
		ID: uuid.New(), SceneKey: "scene-a", DurationMS: 1000,
		Caption: &sceneeditor.CaptionRef{DocumentID: documentID, Revision: 1, LineageID: uuid.New(), LastEndMS: 1200},
	}}}
	_, _, err := BuildSingleSceneSnapshotWebVTT(context.Background(), reader, ownerID, projectID, snapshot)
	if !errors.Is(err, ErrSnapshotCaptionMismatch) {
		t.Fatalf("err = %v, want snapshot caption mismatch", err)
	}
}
