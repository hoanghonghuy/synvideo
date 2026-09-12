package renderexport

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/captions"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type snapshotCaptionRead struct {
	ownerID          uuid.UUID
	projectID        uuid.UUID
	scenePlanVersion int
	sceneKey         string
	revision         int
}

type snapshotCaptionReaderStub struct {
	doc   captions.Document
	err   error
	reads []snapshotCaptionRead
}

func (r *snapshotCaptionReaderStub) GetRevision(_ context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string, revision int) (captions.Document, error) {
	r.reads = append(r.reads, snapshotCaptionRead{ownerID: ownerID, projectID: projectID, scenePlanVersion: scenePlanVersion, sceneKey: sceneKey, revision: revision})
	if r.err != nil {
		return captions.Document{}, r.err
	}
	return r.doc, nil
}

func TestResolveSnapshotCaptionsReadsPinnedRevision(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	sceneID := uuid.New()
	documentID := uuid.New()
	reader := &snapshotCaptionReaderStub{doc: captions.Document{
		ID:               documentID,
		OwnerID:          ownerID,
		ProjectID:        projectID,
		ScenePlanVersion: 7,
		SceneKey:         "intro",
		Revision:         3,
		Segments: []captions.Segment{{
			ID: uuid.New(), Text: "Pinned caption", StartMS: 100, EndMS: 900,
		}},
	}}
	snapshot := sceneeditor.Snapshot{
		ProjectID:        projectID,
		ScenePlanVersion: 7,
		Scenes: []sceneeditor.Scene{{
			ID: sceneID, SceneKey: "intro", Caption: &sceneeditor.CaptionRef{DocumentID: documentID, Revision: 3, LineageID: uuid.New(), LastEndMS: 900},
		}},
	}

	got, err := ResolveSnapshotCaptions(context.Background(), reader, ownerID, projectID, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.reads) != 1 {
		t.Fatalf("reads = %d, want 1", len(reader.reads))
	}
	read := reader.reads[0]
	if read.ownerID != ownerID || read.projectID != projectID || read.scenePlanVersion != 7 || read.sceneKey != "intro" || read.revision != 3 {
		t.Fatalf("unexpected exact-revision read: %+v", read)
	}
	if len(got) != 1 || got[0].SceneID != sceneID || got[0].Document.Revision != 3 || got[0].Document.Segments[0].Text != "Pinned caption" {
		t.Fatalf("unexpected resolved captions: %+v", got)
	}
}

func TestResolveSnapshotCaptionsRejectsRevisionIdentityMismatch(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	pinnedDocumentID := uuid.New()
	reader := &snapshotCaptionReaderStub{doc: captions.Document{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		ProjectID:        projectID,
		ScenePlanVersion: 2,
		SceneKey:         "scene-a",
		Revision:         4,
	}}
	snapshot := sceneeditor.Snapshot{
		ProjectID:        projectID,
		ScenePlanVersion: 2,
		Scenes: []sceneeditor.Scene{{
			ID: uuid.New(), SceneKey: "scene-a", Caption: &sceneeditor.CaptionRef{DocumentID: pinnedDocumentID, Revision: 4, LineageID: uuid.New()},
		}},
	}

	_, err := ResolveSnapshotCaptions(context.Background(), reader, ownerID, projectID, snapshot)
	if !errors.Is(err, ErrSnapshotCaptionMismatch) {
		t.Fatalf("err = %v, want snapshot caption mismatch", err)
	}
}

func TestResolveSnapshotCaptionsSkipsScenesWithoutPinnedCaptions(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	reader := &snapshotCaptionReaderStub{}
	snapshot := sceneeditor.Snapshot{
		ProjectID:        projectID,
		ScenePlanVersion: 1,
		Scenes:           []sceneeditor.Scene{{ID: uuid.New(), SceneKey: "silent"}},
	}

	got, err := ResolveSnapshotCaptions(context.Background(), reader, ownerID, projectID, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || len(reader.reads) != 0 {
		t.Fatalf("unexpected caption reads/results: reads=%d resolved=%d", len(reader.reads), len(got))
	}
}
