package sceneplan

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusApproved   Status = "approved"
	StatusSuperseded Status = "superseded"
)

type SourceType string

const (
	SourceTypeStock          SourceType = "stock"
	SourceTypeUpload         SourceType = "upload"
	SourceTypeCreatorMedia   SourceType = "creator_media"
	SourceTypeGeneratedImage SourceType = "generated_image"
	SourceTypeGeneratedVideo SourceType = "generated_video"
)

var (
	ErrNotFound            = errors.New("scene plan not found")
	ErrStaleRevision       = errors.New("scene plan revision is stale")
	ErrScenePlanImmutable  = errors.New("scene plan is immutable")
	ErrScriptNotApproved   = errors.New("source script is not approved")
	ErrScriptSourceInvalid = errors.New("source script is invalid")
	ErrUnauthenticated     = errors.New("unauthenticated")
	ErrInvalidInput        = errors.New("invalid scene plan input")

	sceneKeyRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)
)

type Scene struct {
	Key                     string     `json:"key"`
	ScriptSectionKey        string     `json:"script_section_key"`
	Narration               string     `json:"narration"`
	VisualInstruction       string     `json:"visual_instruction"`
	PlannedSourceType       SourceType `json:"planned_source_type"`
	ExpectedDurationSeconds int        `json:"expected_duration_seconds"`
	CaptionIntent           string     `json:"caption_intent,omitempty"`
	TransitionNotes         string     `json:"transition_notes,omitempty"`
}

type Plan struct {
	ProjectID             uuid.UUID  `json:"project_id"`
	Version               int        `json:"version"`
	Revision              int        `json:"revision"`
	Status                Status     `json:"status"`
	SourceScriptVersion   int        `json:"source_script_version"`
	SourceProposalVersion int        `json:"source_proposal_version"`
	ContentLocale         string     `json:"content_locale"`
	Scenes                []Scene    `json:"scenes"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	ApprovedAt            *time.Time `json:"approved_at,omitempty"`
	SourceGenerationJobID *uuid.UUID `json:"-"`
}

type Content struct {
	Scenes []Scene `json:"scenes"`
}

type CreateDraftInput struct {
	SourceScriptVersion   int
	SourceGenerationJobID *uuid.UUID
	ContentLocale         string
	Content
}

type PutInput struct {
	Revision *int
	Content
}

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string { return "scene plan validation failed" }

func (e ValidationError) HasFields() bool { return len(e.Fields) > 0 }

func (c *Content) NormalizeAndValidate() error {
	fields := map[string]string{}
	if len(c.Scenes) < 1 {
		fields["scenes"] = "required"
	} else if len(c.Scenes) > 500 {
		fields["scenes"] = "max_items"
	} else {
		seenKeys := make(map[string]bool, len(c.Scenes))
		for index := range c.Scenes {
			normalizeScene(&c.Scenes[index], index, seenKeys, fields)
		}
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (in *CreateDraftInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if in.SourceScriptVersion < 1 {
		fields["source_script_version"] = "positive"
	}
	if err := in.Content.NormalizeAndValidate(); err != nil {
		for field, value := range validationFields(err) {
			fields[field] = value
		}
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (in *PutInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if in.Revision == nil {
		fields["revision"] = "required"
	} else if *in.Revision < 1 {
		fields["revision"] = "positive"
	}
	if err := in.Content.NormalizeAndValidate(); err != nil {
		for field, value := range validationFields(err) {
			fields[field] = value
		}
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

// ValidateContentAgainstScript verifies the approved narration preservation invariant.
func ValidateContentAgainstScript(content *Content, source script.Script) error {
	if source.Status != script.StatusApproved {
		return ErrScriptNotApproved
	}
	if source.Version < 1 || source.SourceProposalVersion < 1 || len(source.Sections) == 0 {
		return ErrScriptSourceInvalid
	}
	if err := content.NormalizeAndValidate(); err != nil {
		return err
	}

	sectionBodies := make(map[string]string, len(source.Sections))
	sectionOrder := make([]string, len(source.Sections))
	sectionIndexes := make(map[string]int, len(source.Sections))
	for index, section := range source.Sections {
		key := strings.TrimSpace(section.Key)
		if key == "" {
			return ErrScriptSourceInvalid
		}
		if _, exists := sectionBodies[key]; exists {
			return ErrScriptSourceInvalid
		}
		sectionOrder[index] = key
		sectionIndexes[key] = index
		sectionBodies[key] = canonicalWhitespace(section.Body)
	}

	sectionSceneText := make(map[string][]string, len(sectionOrder))
	currentSectionIndex := -1
	for _, scene := range content.Scenes {
		sectionIndex, exists := sectionIndexes[scene.ScriptSectionKey]
		if !exists {
			return validationError("scenes", "unknown_script_section")
		}
		if currentSectionIndex == -1 {
			currentSectionIndex = sectionIndex
		} else if sectionIndex < currentSectionIndex || sectionIndex > currentSectionIndex+1 {
			return validationError("scenes", "section_order")
		} else if sectionIndex == currentSectionIndex+1 {
			currentSectionIndex = sectionIndex
		}
		sectionSceneText[scene.ScriptSectionKey] = append(sectionSceneText[scene.ScriptSectionKey], scene.Narration)
	}

	for _, key := range sectionOrder {
		segments := sectionSceneText[key]
		if len(segments) == 0 {
			return validationError("scenes", "narration_coverage")
		}
		if canonicalWhitespace(strings.Join(segments, "")) != sectionBodies[key] {
			return validationError("scenes", "narration_coverage")
		}
	}
	return nil
}

func normalizeScene(scene *Scene, index int, seenKeys map[string]bool, fields map[string]string) {
	prefix := "scenes[" + strconv.Itoa(index) + "]"
	scene.Key = strings.TrimSpace(scene.Key)
	if scene.Key == "" {
		fields[prefix+".key"] = "required"
	} else if utf8.RuneCountInString(scene.Key) > 64 || !sceneKeyRegex.MatchString(scene.Key) {
		fields[prefix+".key"] = "invalid_format"
	} else if seenKeys[scene.Key] {
		fields[prefix+".key"] = "duplicate"
	} else {
		seenKeys[scene.Key] = true
	}

	scene.ScriptSectionKey = strings.TrimSpace(scene.ScriptSectionKey)
	if scene.ScriptSectionKey == "" {
		fields[prefix+".script_section_key"] = "required"
	}

	if strings.TrimSpace(scene.Narration) == "" {
		fields[prefix+".narration"] = "required"
	} else if utf8.RuneCountInString(scene.Narration) > 20000 {
		fields[prefix+".narration"] = "max_length"
	}

	scene.VisualInstruction = strings.TrimSpace(scene.VisualInstruction)
	if scene.VisualInstruction == "" {
		fields[prefix+".visual_instruction"] = "required"
	} else if utf8.RuneCountInString(scene.VisualInstruction) > 5000 {
		fields[prefix+".visual_instruction"] = "max_length"
	}

	if !validSourceType(scene.PlannedSourceType) {
		fields[prefix+".planned_source_type"] = "invalid"
	}
	if scene.ExpectedDurationSeconds < 1 || scene.ExpectedDurationSeconds > 3600 {
		fields[prefix+".expected_duration_seconds"] = "range"
	}

	scene.CaptionIntent = strings.TrimSpace(scene.CaptionIntent)
	if utf8.RuneCountInString(scene.CaptionIntent) > 3000 {
		fields[prefix+".caption_intent"] = "max_length"
	}
	scene.TransitionNotes = strings.TrimSpace(scene.TransitionNotes)
	if utf8.RuneCountInString(scene.TransitionNotes) > 2000 {
		fields[prefix+".transition_notes"] = "max_length"
	}
}

func validationError(field, code string) error {
	return ValidationError{Fields: map[string]string{field: code}}
}

func validationFields(err error) map[string]string {
	var validation ValidationError
	if errors.As(err, &validation) {
		return validation.Fields
	}
	return map[string]string{"content": "invalid"}
}

func validSourceType(value SourceType) bool {
	switch value {
	case SourceTypeStock, SourceTypeUpload, SourceTypeCreatorMedia, SourceTypeGeneratedImage, SourceTypeGeneratedVideo:
		return true
	default:
		return false
	}
}

func canonicalWhitespace(value string) string { return strings.Join(strings.Fields(value), " ") }
