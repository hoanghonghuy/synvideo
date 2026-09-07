package sceneeditor

import "github.com/google/uuid"

func VerifySnapshotIntegrity(snapshot Snapshot) error {
	if snapshot.SchemaVersion != SnapshotSchemaVersion || snapshot.CompositionID == uuid.Nil || snapshot.ProjectID == uuid.Nil || snapshot.Revision < 1 || snapshot.ScenePlanVersion < 1 || snapshot.Digest == "" {
		return ErrInvalidInput
	}
	digest, err := snapshotDigest(snapshot)
	if err != nil || digest != snapshot.Digest {
		return ErrInvalidInput
	}
	return nil
}
