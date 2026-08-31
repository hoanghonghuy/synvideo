package creativebrief

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DistributionTarget string

const (
	DistributionTargetYouTube   DistributionTarget = "youtube"
	DistributionTargetTikTok    DistributionTarget = "tiktok"
	DistributionTargetInstagram DistributionTarget = "instagram"
	DistributionTargetOther     DistributionTarget = "other"
)

type CreativeBrief struct {
	ProjectID           uuid.UUID
	Revision            int
	SourceText          string
	TargetAudience      string
	Objective           string
	DesiredStyle        string
	Tone                string
	DistributionTargets []DistributionTarget
	CallToAction        string
	MustInclude         []string
	MustAvoid           []string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type PutInput struct {
	Revision            *int
	SourceText          string
	TargetAudience      string
	Objective           string
	DesiredStyle        string
	Tone                string
	DistributionTargets []DistributionTarget
	CallToAction        string
	MustInclude         []string
	MustAvoid           []string
}

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	return "creative brief validation failed"
}

func (e ValidationError) HasFields() bool {
	return len(e.Fields) > 0
}

func (input *PutInput) NormalizeAndValidate() error {
	fields := map[string]string{}

	input.SourceText = strings.TrimSpace(input.SourceText)
	validateRequiredString(fields, "source_text", input.SourceText, 20000)
	input.TargetAudience = trimAndValidateOptionalString(fields, "target_audience", input.TargetAudience, 2000)
	input.Objective = trimAndValidateOptionalString(fields, "objective", input.Objective, 2000)
	input.DesiredStyle = trimAndValidateOptionalString(fields, "desired_style", input.DesiredStyle, 2000)
	input.Tone = trimAndValidateOptionalString(fields, "tone", input.Tone, 500)
	input.CallToAction = trimAndValidateOptionalString(fields, "call_to_action", input.CallToAction, 2000)
	input.DistributionTargets = normalizeDistributionTargets(fields, input.DistributionTargets)
	input.MustInclude = normalizeStringArray(fields, "must_include", input.MustInclude, 20, 500)
	input.MustAvoid = normalizeStringArray(fields, "must_avoid", input.MustAvoid, 20, 500)

	if input.Revision != nil && *input.Revision < 1 {
		fields["revision"] = "positive"
	}

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
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

func normalizeDistributionTargets(fields map[string]string, values []DistributionTarget) []DistributionTarget {
	if len(values) > 8 {
		fields["distribution_targets"] = "max_8"
	}

	seen := map[DistributionTarget]struct{}{}
	normalized := make([]DistributionTarget, 0, len(values))
	for _, value := range values {
		if !validDistributionTarget(value) {
			fields["distribution_targets"] = "invalid"
			continue
		}
		if _, ok := seen[value]; ok {
			fields["distribution_targets"] = "unique"
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func normalizeStringArray(fields map[string]string, field string, values []string, maxItems int, maxLength int) []string {
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

func validDistributionTarget(value DistributionTarget) bool {
	switch value {
	case DistributionTargetYouTube, DistributionTargetTikTok, DistributionTargetInstagram, DistributionTargetOther:
		return true
	default:
		return false
	}
}

func maxCode(limit int) string {
	return "max_" + strconv.Itoa(limit)
}
