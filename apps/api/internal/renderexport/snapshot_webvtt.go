package renderexport

import (
	"context"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

// BuildSingleSceneSnapshotWebVTT derives cues only from the caption revision
// pinned by the immutable composition snapshot. Multi-scene timing remains
// unsupported by the local renderer, so this helper intentionally does not
// invent cross-scene offset semantics.
func BuildSingleSceneSnapshotWebVTT(ctx context.Context, reader SnapshotCaptionRevisionReader, ownerID, projectID uuid.UUID, snapshot sceneeditor.Snapshot) ([]byte, bool, error) {
	if len(snapshot.Scenes) != 1 {
		return nil, false, ErrSnapshotCaptionMismatch
	}
	scene := snapshot.Scenes[0]
	if scene.Caption == nil {
		return nil, false, nil
	}
	resolved, err := ResolveSnapshotCaptions(ctx, reader, ownerID, projectID, snapshot)
	if err != nil {
		return nil, false, err
	}
	if len(resolved) != 1 || resolved[0].SceneID != scene.ID {
		return nil, false, ErrSnapshotCaptionMismatch
	}

	segments := resolved[0].Document.Segments
	cues := make([]WebVTTCue, 0, len(segments))
	for _, segment := range segments {
		if segment.StartMS < 0 || segment.EndMS > scene.DurationMS {
			return nil, false, ErrSnapshotCaptionMismatch
		}
		cues = append(cues, WebVTTCue{StartMS: segment.StartMS, EndMS: segment.EndMS, Text: segment.Text})
	}
	payload, err := BuildWebVTT(cues)
	if err != nil {
		return nil, false, err
	}
	return payload, true, nil
}
