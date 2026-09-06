package audiomix

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
)

type State string

type LoopPolicy string

const (
	StateCurrent State = "CURRENT"
	StateStale   State = "STALE"
	StateBroken  State = "BROKEN"
	StateError   State = "ERROR"

	LoopNone     LoopPolicy = "NO_LOOP"
	LoopToTarget LoopPolicy = "LOOP_TO_TARGET"

	MinMusicGainDB     = -60.0
	MaxMusicGainDB     = 12.0
	MinNarrationGainDB = -24.0
	MaxNarrationGainDB = 12.0
	MinDuckReductionDB = 0.0
	MaxDuckReductionDB = 60.0
	MaxTransitionMS    = int64(10_000)
)

var (
	ErrNotFound        = errors.New("audio mix not found")
	ErrUnauthenticated = errors.New("audio mix principal is required")
	ErrInvalidInput    = errors.New("audio mix input is invalid")
	ErrConflict        = errors.New("audio mix revision conflict")
	ErrStale           = errors.New("audio mix is stale")
	ErrBroken          = errors.New("audio mix is broken")
	ErrPersistence     = errors.New("audio mix persistence failed")
)

type ValidationError struct{ Fields map[string]string }

func (e ValidationError) Error() string { return "audio mix validation failed" }

type Ducking struct {
	Enabled     bool    `json:"enabled"`
	ReductionDB float64 `json:"reduction_db"`
	AttackMS    int64   `json:"attack_ms"`
	ReleaseMS   int64   `json:"release_ms"`
}

type Config struct {
	MusicTrimStartMS int64      `json:"music_trim_start_ms"`
	StartOffsetMS    int64      `json:"start_offset_ms"`
	LoopPolicy       LoopPolicy `json:"loop_policy"`
	MusicGainDB      float64    `json:"music_gain_db"`
	NarrationGainDB  float64    `json:"narration_gain_db"`
	Ducking          Ducking    `json:"ducking"`
}

type NarrationSource struct {
	BindingID  uuid.UUID
	AssetID    uuid.UUID
	DurationMS int64
}

type MusicSource struct {
	AssetID    uuid.UUID
	ProjectID  uuid.UUID
	DurationMS int64
	Available  bool
	Audio      bool
}

type Document struct {
	ID                  uuid.UUID `json:"id"`
	OwnerID             uuid.UUID `json:"-"`
	ProjectID           uuid.UUID `json:"project_id"`
	Revision            int       `json:"revision"`
	MusicAssetID        uuid.UUID `json:"music_asset_id"`
	MusicDurationMS     int64     `json:"music_duration_ms"`
	NarrationBindingID  uuid.UUID `json:"narration_binding_id"`
	NarrationAssetID    uuid.UUID `json:"narration_asset_id"`
	NarrationDurationMS int64     `json:"narration_duration_ms"`
	Config              Config    `json:"config"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type View struct {
	Document
	State State `json:"state"`
}

type Snapshot struct {
	DocumentID          uuid.UUID `json:"document_id"`
	Revision            int       `json:"revision"`
	ProjectID           uuid.UUID `json:"project_id"`
	MusicAssetID        uuid.UUID `json:"music_asset_id"`
	MusicDurationMS     int64     `json:"music_duration_ms"`
	NarrationBindingID  uuid.UUID `json:"narration_binding_id"`
	NarrationAssetID    uuid.UUID `json:"narration_asset_id"`
	NarrationDurationMS int64     `json:"narration_duration_ms"`
	Config              Config    `json:"config"`
}

func ValidateConfig(config Config, musicDurationMS, narrationDurationMS int64) error {
	fields := map[string]string{}
	if musicDurationMS <= 0 {
		fields["music_duration_ms"] = "positive"
	}
	if narrationDurationMS <= 0 {
		fields["narration_duration_ms"] = "positive"
	}
	if config.MusicTrimStartMS < 0 {
		fields["config.music_trim_start_ms"] = "non_negative"
	} else if musicDurationMS > 0 && config.MusicTrimStartMS >= musicDurationMS {
		fields["config.music_trim_start_ms"] = "within_music_duration"
	}
	if config.StartOffsetMS < 0 {
		fields["config.start_offset_ms"] = "non_negative"
	} else if narrationDurationMS > 0 && config.StartOffsetMS > narrationDurationMS {
		fields["config.start_offset_ms"] = "within_narration_duration"
	}
	if config.LoopPolicy != LoopNone && config.LoopPolicy != LoopToTarget {
		fields["config.loop_policy"] = "invalid"
	}
	if !finiteInRange(config.MusicGainDB, MinMusicGainDB, MaxMusicGainDB) {
		fields["config.music_gain_db"] = "out_of_range"
	}
	if !finiteInRange(config.NarrationGainDB, MinNarrationGainDB, MaxNarrationGainDB) {
		fields["config.narration_gain_db"] = "out_of_range"
	}
	if !finiteInRange(config.Ducking.ReductionDB, MinDuckReductionDB, MaxDuckReductionDB) {
		fields["config.ducking.reduction_db"] = "out_of_range"
	}
	if config.Ducking.AttackMS < 0 || config.Ducking.AttackMS > MaxTransitionMS {
		fields["config.ducking.attack_ms"] = "out_of_range"
	}
	if config.Ducking.ReleaseMS < 0 || config.Ducking.ReleaseMS > MaxTransitionMS {
		fields["config.ducking.release_ms"] = "out_of_range"
	}
	if !config.Ducking.Enabled && (config.Ducking.ReductionDB != 0 || config.Ducking.AttackMS != 0 || config.Ducking.ReleaseMS != 0) {
		fields["config.ducking"] = "disabled_requires_zero_parameters"
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func ValidateBinding(projectID uuid.UUID, music MusicSource, narration NarrationSource, config Config) error {
	fields := map[string]string{}
	if projectID == uuid.Nil {
		fields["project_id"] = "required"
	}
	if music.AssetID == uuid.Nil {
		fields["music_asset_id"] = "required"
	}
	if !music.Available {
		fields["music_asset_id"] = "unavailable"
	}
	if !music.Audio {
		fields["music_asset_id"] = "audio_required"
	}
	if music.ProjectID != projectID {
		fields["music_asset_id"] = "same_project_required"
	}
	if narration.BindingID == uuid.Nil {
		fields["narration_binding_id"] = "required"
	}
	if narration.AssetID == uuid.Nil {
		fields["narration_asset_id"] = "required"
	}
	if narration.DurationMS <= 0 {
		fields["narration_duration_ms"] = "positive"
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return ValidateConfig(config, music.DurationMS, narration.DurationMS)
}

func NewDocument(id, ownerID, projectID uuid.UUID, music MusicSource, narration NarrationSource, config Config, now time.Time) (Document, error) {
	if id == uuid.Nil || ownerID == uuid.Nil {
		fields := map[string]string{}
		if id == uuid.Nil {
			fields["id"] = "required"
		}
		if ownerID == uuid.Nil {
			fields["owner_id"] = "required"
		}
		return Document{}, ValidationError{Fields: fields}
	}
	if err := ValidateBinding(projectID, music, narration, config); err != nil {
		return Document{}, err
	}
	return Document{
		ID:                  id,
		OwnerID:             ownerID,
		ProjectID:           projectID,
		Revision:            1,
		MusicAssetID:        music.AssetID,
		MusicDurationMS:     music.DurationMS,
		NarrationBindingID:  narration.BindingID,
		NarrationAssetID:    narration.AssetID,
		NarrationDurationMS: narration.DurationMS,
		Config:              config,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

func StateForSources(doc Document, music MusicSource, narration NarrationSource) State {
	if !music.Available || !music.Audio || music.AssetID != doc.MusicAssetID || music.ProjectID != doc.ProjectID || music.DurationMS != doc.MusicDurationMS {
		return StateBroken
	}
	if narration.BindingID == uuid.Nil || narration.AssetID == uuid.Nil || narration.DurationMS <= 0 {
		return StateStale
	}
	if narration.BindingID != doc.NarrationBindingID || narration.AssetID != doc.NarrationAssetID || narration.DurationMS != doc.NarrationDurationMS {
		return StateStale
	}
	return StateCurrent
}

func NewSnapshot(doc Document, state State) (Snapshot, error) {
	if state == StateBroken {
		return Snapshot{}, ErrBroken
	}
	if state != StateCurrent {
		return Snapshot{}, ErrStale
	}
	return Snapshot{
		DocumentID:          doc.ID,
		Revision:            doc.Revision,
		ProjectID:           doc.ProjectID,
		MusicAssetID:        doc.MusicAssetID,
		MusicDurationMS:     doc.MusicDurationMS,
		NarrationBindingID:  doc.NarrationBindingID,
		NarrationAssetID:    doc.NarrationAssetID,
		NarrationDurationMS: doc.NarrationDurationMS,
		Config:              doc.Config,
	}, nil
}

func finiteInRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}
