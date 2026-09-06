package audiomix

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validFixture(t *testing.T) (uuid.UUID, uuid.UUID, MusicSource, NarrationSource, Config) {
	t.Helper()
	ownerID := uuid.New()
	projectID := uuid.New()
	music := MusicSource{AssetID: uuid.New(), ProjectID: projectID, DurationMS: 90_000, Available: true, Audio: true}
	narration := NarrationSource{BindingID: uuid.New(), AssetID: uuid.New(), DurationMS: 60_000}
	config := Config{LoopPolicy: LoopToTarget, MusicGainDB: -12, NarrationGainDB: 0, Ducking: Ducking{Enabled: true, ReductionDB: 9, AttackMS: 120, ReleaseMS: 350}}
	return ownerID, projectID, music, narration, config
}

func TestNewDocumentBindsAuthoritativeSources(t *testing.T) {
	ownerID, projectID, music, narration, config := validFixture(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	doc, err := NewDocument(uuid.New(), ownerID, projectID, music, narration, config, now)
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	if doc.Revision != 1 || doc.MusicAssetID != music.AssetID || doc.MusicDurationMS != music.DurationMS {
		t.Fatalf("music binding not frozen: %+v", doc)
	}
	if doc.NarrationBindingID != narration.BindingID || doc.NarrationAssetID != narration.AssetID || doc.NarrationDurationMS != narration.DurationMS {
		t.Fatalf("narration lineage not frozen: %+v", doc)
	}
}

func TestValidateBindingRejectsWrongProjectAndNonAudio(t *testing.T) {
	_, projectID, music, narration, config := validFixture(t)
	music.ProjectID = uuid.New()
	music.Audio = false
	err := ValidateBinding(projectID, music, narration, config)
	var validation ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if validation.Fields["music_asset_id"] == "" {
		t.Fatalf("expected music asset validation error: %+v", validation.Fields)
	}
}

func TestValidateConfigRejectsOutOfBoundsAndForgedTiming(t *testing.T) {
	config := Config{
		MusicTrimStartMS: 30_000,
		StartOffsetMS:    61_000,
		LoopPolicy:       LoopPolicy("MAGIC"),
		MusicGainDB:      99,
		NarrationGainDB:  -99,
		Ducking:          Ducking{Enabled: true, ReductionDB: 61, AttackMS: MaxTransitionMS + 1},
	}
	err := ValidateConfig(config, 30_000, 60_000)
	var validation ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	for _, field := range []string{"config.music_trim_start_ms", "config.start_offset_ms", "config.loop_policy", "config.music_gain_db", "config.narration_gain_db", "config.ducking.reduction_db", "config.ducking.attack_ms"} {
		if validation.Fields[field] == "" {
			t.Fatalf("expected %s validation error: %+v", field, validation.Fields)
		}
	}
}

func TestDisabledDuckingCannotHideActiveParameters(t *testing.T) {
	_, _, music, narration, config := validFixture(t)
	config.Ducking.Enabled = false
	err := ValidateConfig(config, music.DurationMS, narration.DurationMS)
	var validation ValidationError
	if !errors.As(err, &validation) || validation.Fields["config.ducking"] == "" {
		t.Fatalf("expected disabled ducking validation, got %v", err)
	}
}

func TestNarrationReplacementMarksMixStaleWithoutRebinding(t *testing.T) {
	ownerID, projectID, music, narration, config := validFixture(t)
	doc, err := NewDocument(uuid.New(), ownerID, projectID, music, narration, config, time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	replacement := narration
	replacement.BindingID = uuid.New()
	replacement.AssetID = uuid.New()
	if state := StateForSources(doc, music, replacement); state != StateStale {
		t.Fatalf("state = %s, want STALE", state)
	}
	if doc.NarrationBindingID == replacement.BindingID || doc.NarrationAssetID == replacement.AssetID {
		t.Fatal("document was silently rebound")
	}
}

func TestMissingMusicMarksMixBroken(t *testing.T) {
	ownerID, projectID, music, narration, config := validFixture(t)
	doc, err := NewDocument(uuid.New(), ownerID, projectID, music, narration, config, time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	music.Available = false
	if state := StateForSources(doc, music, narration); state != StateBroken {
		t.Fatalf("state = %s, want BROKEN", state)
	}
}

func TestSnapshotRequiresCurrentAndCopiesFrozenLineage(t *testing.T) {
	ownerID, projectID, music, narration, config := validFixture(t)
	doc, err := NewDocument(uuid.New(), ownerID, projectID, music, narration, config, time.Now())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	snapshot, err := NewSnapshot(doc, StateCurrent)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	if snapshot.DocumentID != doc.ID || snapshot.Revision != doc.Revision || snapshot.MusicAssetID != doc.MusicAssetID || snapshot.NarrationBindingID != doc.NarrationBindingID {
		t.Fatalf("snapshot does not preserve frozen identity: %+v", snapshot)
	}
	if _, err := NewSnapshot(doc, StateStale); !errors.Is(err, ErrStale) {
		t.Fatalf("stale snapshot error = %v, want ErrStale", err)
	}
	if _, err := NewSnapshot(doc, StateBroken); !errors.Is(err, ErrBroken) {
		t.Fatalf("broken snapshot error = %v, want ErrBroken", err)
	}
}
