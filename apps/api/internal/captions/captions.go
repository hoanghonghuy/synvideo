package captions

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type State string

const (
	StateCurrent    State = "CURRENT"
	StateStale      State = "STALE"
	StateRebuilding State = "REBUILDING"
	StateError      State = "ERROR"
)

var (
	ErrNotFound        = errors.New("caption document not found")
	ErrUnauthenticated = errors.New("caption principal is required")
	ErrInvalidInput    = errors.New("caption input is invalid")
	ErrConflict        = errors.New("caption revision conflict")
	ErrStale           = errors.New("caption document is stale")
	ErrSourceMissing   = errors.New("current narration source is missing")
	ErrPersistence     = errors.New("caption persistence failed")
)

type ValidationError struct{ Fields map[string]string }

func (e ValidationError) Error() string { return "caption validation failed" }

type Segment struct {
	ID      uuid.UUID `json:"id"`
	Text    string    `json:"text"`
	StartMS int64     `json:"start_ms"`
	EndMS   int64     `json:"end_ms"`
}

type Style struct {
	Alignment       string `json:"alignment"`
	Position        string `json:"position"`
	FontFamilyToken string `json:"font_family_token,omitempty"`
	Size            string `json:"size"`
	Weight          string `json:"weight"`
}

func DefaultStyle() Style {
	return Style{Alignment: "center", Position: "bottom", Size: "medium", Weight: "normal"}
}

type Source struct {
	BindingID  uuid.UUID
	AssetID    uuid.UUID
	DurationMS int64
	Text       string
}

type Document struct {
	ID               uuid.UUID `json:"id"`
	OwnerID          uuid.UUID `json:"-"`
	ProjectID        uuid.UUID `json:"project_id"`
	ScenePlanVersion int       `json:"scene_plan_version"`
	SceneKey         string    `json:"scene_key"`
	Revision         int       `json:"revision"`
	SourceBindingID  uuid.UUID `json:"source_binding_id"`
	SourceAssetID    uuid.UUID `json:"source_asset_id"`
	SourceDurationMS int64     `json:"source_duration_ms"`
	Segments         []Segment `json:"segments"`
	Style            Style     `json:"style"`
	CreatedAt        time.Time `json:"created_at"`
}

type View struct {
	Document
	State State `json:"state"`
}

type Snapshot struct {
	DocumentID       uuid.UUID `json:"document_id"`
	Revision         int       `json:"revision"`
	SourceBindingID  uuid.UUID `json:"source_binding_id"`
	SourceAssetID    uuid.UUID `json:"source_asset_id"`
	SourceDurationMS int64     `json:"source_duration_ms"`
	Segments         []Segment `json:"segments"`
	Style            Style     `json:"style"`
}

func NormalizeSegments(input []Segment, sourceDurationMS int64) ([]Segment, error) {
	segments := append([]Segment(nil), input...)
	for i := range segments {
		segments[i].Text = strings.TrimSpace(segments[i].Text)
	}
	sort.SliceStable(segments, func(i, j int) bool {
		if segments[i].StartMS == segments[j].StartMS {
			return segments[i].ID.String() < segments[j].ID.String()
		}
		return segments[i].StartMS < segments[j].StartMS
	})
	if err := ValidateSegments(segments, sourceDurationMS); err != nil {
		return nil, err
	}
	return segments, nil
}

func ValidateSegments(segments []Segment, sourceDurationMS int64) error {
	fields := map[string]string{}
	if sourceDurationMS <= 0 {
		fields["source_duration_ms"] = "positive"
	}
	if len(segments) < 1 {
		fields["segments"] = "required"
	}
	seen := map[uuid.UUID]struct{}{}
	var previousEnd int64 = -1
	for i, segment := range segments {
		prefix := "segments[" + itoa(i) + "]"
		if segment.ID == uuid.Nil {
			fields[prefix+".id"] = "required"
		} else if _, ok := seen[segment.ID]; ok {
			fields[prefix+".id"] = "duplicate"
		}
		seen[segment.ID] = struct{}{}
		if strings.TrimSpace(segment.Text) == "" {
			fields[prefix+".text"] = "required"
		} else if utf8.RuneCountInString(segment.Text) > 2_000 {
			fields[prefix+".text"] = "max_length"
		}
		if segment.StartMS < 0 {
			fields[prefix+".start_ms"] = "non_negative"
		}
		if segment.EndMS <= segment.StartMS {
			fields[prefix+".end_ms"] = "after_start"
		}
		if sourceDurationMS > 0 && segment.EndMS > sourceDurationMS {
			fields[prefix+".end_ms"] = "within_source_duration"
		}
		if previousEnd >= 0 && segment.StartMS < previousEnd {
			fields[prefix+".start_ms"] = "overlap_not_allowed"
		}
		if segment.EndMS > previousEnd {
			previousEnd = segment.EndMS
		}
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func NormalizeStyle(style Style) (Style, error) {
	style.Alignment = strings.ToLower(strings.TrimSpace(style.Alignment))
	style.Position = strings.ToLower(strings.TrimSpace(style.Position))
	style.FontFamilyToken = strings.TrimSpace(style.FontFamilyToken)
	style.Size = strings.ToLower(strings.TrimSpace(style.Size))
	style.Weight = strings.ToLower(strings.TrimSpace(style.Weight))
	if style.Alignment == "" && style.Position == "" && style.Size == "" && style.Weight == "" && style.FontFamilyToken == "" {
		style = DefaultStyle()
	}
	fields := map[string]string{}
	if !oneOf(style.Alignment, "left", "center", "right") {
		fields["style.alignment"] = "invalid"
	}
	if !oneOf(style.Position, "top", "middle", "bottom") {
		fields["style.position"] = "invalid"
	}
	if !oneOf(style.Size, "small", "medium", "large") {
		fields["style.size"] = "invalid"
	}
	if !oneOf(style.Weight, "normal", "semibold", "bold") {
		fields["style.weight"] = "invalid"
	}
	if utf8.RuneCountInString(style.FontFamilyToken) > 100 {
		fields["style.font_family_token"] = "max_length"
	}
	if len(fields) > 0 {
		return Style{}, ValidationError{Fields: fields}
	}
	return style, nil
}

func StateForSource(doc Document, current Source) State {
	if current.BindingID == uuid.Nil || current.AssetID == uuid.Nil || current.DurationMS <= 0 {
		return StateStale
	}
	if doc.SourceBindingID != current.BindingID || doc.SourceAssetID != current.AssetID || doc.SourceDurationMS != current.DurationMS {
		return StateStale
	}
	return StateCurrent
}

func NewSnapshot(doc Document, state State) (Snapshot, error) {
	if state != StateCurrent {
		return Snapshot{}, ErrStale
	}
	return Snapshot{
		DocumentID:       doc.ID,
		Revision:         doc.Revision,
		SourceBindingID:  doc.SourceBindingID,
		SourceAssetID:    doc.SourceAssetID,
		SourceDurationMS: doc.SourceDurationMS,
		Segments:         append([]Segment(nil), doc.Segments...),
		Style:            doc.Style,
	}, nil
}

func InitialSegments(text string, durationMS int64) ([]Segment, error) {
	text = strings.TrimSpace(text)
	segments := []Segment{{ID: uuid.New(), Text: text, StartMS: 0, EndMS: durationMS}}
	return NormalizeSegments(segments, durationMS)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
