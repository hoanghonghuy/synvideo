package renderexport

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/captions"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

var ErrSnapshotCaptionMismatch = errors.New("render export snapshot caption mismatch")

type SnapshotCaptionRevisionReader interface {
	GetRevision(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string, revision int) (captions.Document, error)
}

type SnapshotCaption struct {
	SceneID  uuid.UUID
	SceneKey string
	Document captions.Document
}

// ResolveSnapshotCaptions hydrates only the caption revisions pinned by an
// immutable composition snapshot. It deliberately has no latest-revision read
// path: edits made after enqueue must not mutate queued/running/retried output.
func ResolveSnapshotCaptions(ctx context.Context, reader SnapshotCaptionRevisionReader, ownerID, projectID uuid.UUID, snapshot sceneeditor.Snapshot) ([]SnapshotCaption, error) {
	if reader == nil || ownerID == uuid.Nil || projectID == uuid.Nil || snapshot.ProjectID != projectID || snapshot.ScenePlanVersion < 1 {
		return nil, ErrSnapshotCaptionMismatch
	}

	resolved := make([]SnapshotCaption, 0, len(snapshot.Scenes))
	for _, scene := range snapshot.Scenes {
		if scene.Caption == nil {
			continue
		}
		ref := scene.Caption
		if scene.ID == uuid.Nil || scene.SceneKey == "" || ref.DocumentID == uuid.Nil || ref.Revision < 1 {
			return nil, ErrSnapshotCaptionMismatch
		}

		doc, err := reader.GetRevision(ctx, ownerID, projectID, snapshot.ScenePlanVersion, scene.SceneKey, ref.Revision)
		if err != nil {
			return nil, err
		}
		if doc.ID != ref.DocumentID || doc.OwnerID != ownerID || doc.ProjectID != projectID || doc.ScenePlanVersion != snapshot.ScenePlanVersion || doc.SceneKey != scene.SceneKey || doc.Revision != ref.Revision {
			return nil, ErrSnapshotCaptionMismatch
		}

		resolved = append(resolved, SnapshotCaption{
			SceneID:  scene.ID,
			SceneKey: scene.SceneKey,
			Document: doc,
		})
	}
	return resolved, nil
}
