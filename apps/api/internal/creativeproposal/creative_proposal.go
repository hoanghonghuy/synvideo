package creativeproposal

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusApproved   Status = "approved"
	StatusSuperseded Status = "superseded"
)

type StructureItem struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Purpose string `json:"purpose"`
}

type CreativeProposal struct {
	ProjectID                uuid.UUID       `json:"project_id"`
	Version                  int             `json:"version"`
	Revision                 int             `json:"revision"`
	Status                   Status          `json:"status"`
	SourceBriefRevision      int             `json:"source_brief_revision"`
	TitleOptions             []string        `json:"title_options"`
	HookOptions              []string        `json:"hook_options"`
	AudienceSummary          string          `json:"audience_summary"`
	ObjectiveSummary         string          `json:"objective_summary"`
	NarrativeAngle           string          `json:"narrative_angle"`
	EstimatedDurationSeconds *int            `json:"estimated_duration_seconds"`
	FormatRationale          string          `json:"format_rationale"`
	Structure                []StructureItem `json:"structure"`
	VisualDirection          string          `json:"visual_direction"`
	VoiceDirection           string          `json:"voice_direction"`
	MusicDirection           string          `json:"music_direction"`
	CaptionDirection         string          `json:"caption_direction"`
	CallToAction             string          `json:"call_to_action"`
	ResearchGaps             []string        `json:"research_gaps"`
	Warnings                 []string        `json:"warnings"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	ApprovedAt               *time.Time      `json:"approved_at"`
	SourceGenerationJobID    *uuid.UUID      `json:"source_generation_job_id,omitempty"`
}

type Content struct {
	TitleOptions             []string        `json:"title_options"`
	HookOptions              []string        `json:"hook_options"`
	AudienceSummary          string          `json:"audience_summary"`
	ObjectiveSummary         string          `json:"objective_summary"`
	NarrativeAngle           string          `json:"narrative_angle"`
	EstimatedDurationSeconds *int            `json:"estimated_duration_seconds"`
	FormatRationale          string          `json:"format_rationale"`
	Structure                []StructureItem `json:"structure"`
	VisualDirection          string          `json:"visual_direction"`
	VoiceDirection           string          `json:"voice_direction"`
	MusicDirection           string          `json:"music_direction"`
	CaptionDirection         string          `json:"caption_direction"`
	CallToAction             string          `json:"call_to_action"`
	ResearchGaps             []string        `json:"research_gaps"`
	Warnings                 []string        `json:"warnings"`
}

type PutInput struct {
	Revision *int
	Content
}

type CreateDraftInput struct {
	SourceBriefRevision   int
	SourceGenerationJobID *uuid.UUID
	Content
}

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	return "creative proposal validation failed"
}

func (e ValidationError) HasFields() bool {
	return len(e.Fields) > 0
}

func (input *PutInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if input.Revision == nil {
		fields["revision"] = "required"
	} else if *input.Revision < 1 {
		fields["revision"] = "positive"
	}

	input.Content.normalizeAndValidateFields(fields)

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (input *CreateDraftInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if input.SourceBriefRevision < 1 {
		fields["source_brief_revision"] = "positive"
	}

	input.Content.normalizeAndValidateFields(fields)

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (c *Content) NormalizeAndValidate() error {
	fields := map[string]string{}
	c.normalizeAndValidateFields(fields)
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (c *Content) normalizeAndValidateFields(fields map[string]string) {
	c.TitleOptions = normalizeRequiredStringArray(fields, "title_options", c.TitleOptions, 5, 300)
	c.HookOptions = normalizeRequiredStringArray(fields, "hook_options", c.HookOptions, 5, 1000)

	c.AudienceSummary = strings.TrimSpace(c.AudienceSummary)
	validateRequiredString(fields, "audience_summary", c.AudienceSummary, 2000)

	c.ObjectiveSummary = strings.TrimSpace(c.ObjectiveSummary)
	validateRequiredString(fields, "objective_summary", c.ObjectiveSummary, 2000)

	c.NarrativeAngle = strings.TrimSpace(c.NarrativeAngle)
	validateRequiredString(fields, "narrative_angle", c.NarrativeAngle, 4000)

	if c.EstimatedDurationSeconds != nil {
		if *c.EstimatedDurationSeconds < 1 || *c.EstimatedDurationSeconds > 43200 {
			fields["estimated_duration_seconds"] = "range_1_43200"
		}
	}

	c.FormatRationale = trimAndValidateOptionalString(fields, "format_rationale", c.FormatRationale, 2000)
	c.Structure = normalizeStructure(fields, c.Structure)
	c.VisualDirection = trimAndValidateOptionalString(fields, "visual_direction", c.VisualDirection, 5000)
	c.VoiceDirection = trimAndValidateOptionalString(fields, "voice_direction", c.VoiceDirection, 3000)
	c.MusicDirection = trimAndValidateOptionalString(fields, "music_direction", c.MusicDirection, 3000)
	c.CaptionDirection = trimAndValidateOptionalString(fields, "caption_direction", c.CaptionDirection, 3000)
	c.CallToAction = trimAndValidateOptionalString(fields, "call_to_action", c.CallToAction, 2000)

	c.ResearchGaps = normalizeOptionalStringArray(fields, "research_gaps", c.ResearchGaps, 20, 1000)
	c.Warnings = normalizeOptionalStringArray(fields, "warnings", c.Warnings, 20, 1000)
}

func validateRequiredString(fields map[string]string, field string, value string, maxLength int) {
	switch {
	case value == "":
		fields[field] = "required"
	case len([]rune(value)) > maxLength:
		fields[field] = maxCode(maxLength)
	}
}

func trimAndValidateOptionalString(fields map[string]string, field string, value string, maxLength int) string {
	trimmed := strings.TrimSpace(value)
	if len([]rune(trimmed)) > maxLength {
		fields[field] = maxCode(maxLength)
	}
	return trimmed
}

func normalizeRequiredStringArray(fields map[string]string, field string, values []string, maxItems int, maxLength int) []string {
	if len(values) == 0 {
		fields[field] = "required"
		return values
	}
	if len(values) > maxItems {
		fields[field] = maxCode(maxItems)
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		switch {
		case trimmed == "":
			fields[field] = "items_required"
		case len([]rune(trimmed)) > maxLength:
			fields[field] = "items_" + maxCode(maxLength)
		default:
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func normalizeOptionalStringArray(fields map[string]string, field string, values []string, maxItems int, maxLength int) []string {
	if len(values) > maxItems {
		fields[field] = maxCode(maxItems)
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		switch {
		case trimmed == "":
			fields[field] = "items_required"
		case len([]rune(trimmed)) > maxLength:
			fields[field] = "items_" + maxCode(maxLength)
		default:
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func normalizeStructure(fields map[string]string, items []StructureItem) []StructureItem {
	if len(items) == 0 {
		fields["structure"] = "required"
		return items
	}
	if len(items) > 50 {
		fields["structure"] = "max_50"
	}

	seenKeys := make(map[string]struct{}, len(items))
	normalized := make([]StructureItem, 0, len(items))

	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		title := strings.TrimSpace(item.Title)
		purpose := strings.TrimSpace(item.Purpose)

		if key == "" || title == "" || purpose == "" {
			fields["structure"] = "items_required"
		}
		if len([]rune(key)) > 64 {
			fields["structure"] = "items_key_max_64"
		} else if key != "" && !isValidSlug(key) {
			fields["structure"] = "items_key_invalid"
		}
		if _, seen := seenKeys[key]; seen && key != "" {
			fields["structure"] = "keys_unique"
		}
		if key != "" {
			seenKeys[key] = struct{}{}
		}

		if len([]rune(title)) > 300 {
			fields["structure"] = "items_title_max_300"
		}
		if len([]rune(purpose)) > 2000 {
			fields["structure"] = "items_purpose_max_2000"
		}

		normalized = append(normalized, StructureItem{
			Key:     key,
			Title:   title,
			Purpose: purpose,
		})
	}

	return normalized
}

func isValidSlug(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func maxCode(limit int) string {
	return "max_" + strconv.Itoa(limit)
}
