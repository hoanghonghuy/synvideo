package proposalgeneration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var structureKeyPattern = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

type providerPayload struct {
	TitleOptions             []string                `json:"title_options"`
	HookOptions              []string                `json:"hook_options"`
	AudienceSummary          string                  `json:"audience_summary"`
	ObjectiveSummary         string                  `json:"objective_summary"`
	NarrativeAngle           string                  `json:"narrative_angle"`
	EstimatedDurationSeconds *int                    `json:"estimated_duration_seconds"`
	FormatRationale          string                  `json:"format_rationale"`
	Structure                []providerStructureItem `json:"structure"`
	VisualDirection          string                  `json:"visual_direction"`
	VoiceDirection           string                  `json:"voice_direction"`
	MusicDirection           string                  `json:"music_direction"`
	CaptionDirection         string                  `json:"caption_direction"`
	CallToAction             string                  `json:"call_to_action"`
	ResearchGaps             []string                `json:"research_gaps"`
	Warnings                 []string                `json:"warnings"`
}

type providerStructureItem struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Purpose string `json:"purpose"`
}

func parseCandidate(raw string, sourceBriefRevision int) (Candidate, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Candidate{}, newInvalidOutputError(fmt.Errorf("empty provider output"))
	}

	decoder := json.NewDecoder(bytes.NewReader([]byte(trimmed)))
	decoder.DisallowUnknownFields()

	var payload providerPayload
	if err := decoder.Decode(&payload); err != nil {
		return Candidate{}, newInvalidOutputError(fmt.Errorf("decode provider output: %w", err))
	}
	if decoder.More() {
		return Candidate{}, newInvalidOutputError(fmt.Errorf("trailing json values"))
	}

	candidate, err := validateCandidate(payload, sourceBriefRevision)
	if err != nil {
		return Candidate{}, newInvalidOutputError(err)
	}
	return candidate, nil
}

func validateCandidate(payload providerPayload, sourceBriefRevision int) (Candidate, error) {
	titleOptions, err := normalizeRequiredStringArray(payload.TitleOptions, 1, 5, 300, "title_options")
	if err != nil {
		return Candidate{}, err
	}
	hookOptions, err := normalizeRequiredStringArray(payload.HookOptions, 1, 5, 1000, "hook_options")
	if err != nil {
		return Candidate{}, err
	}
	audienceSummary, err := normalizeRequiredString(payload.AudienceSummary, 2000, "audience_summary")
	if err != nil {
		return Candidate{}, err
	}
	objectiveSummary, err := normalizeRequiredString(payload.ObjectiveSummary, 2000, "objective_summary")
	if err != nil {
		return Candidate{}, err
	}
	narrativeAngle, err := normalizeRequiredString(payload.NarrativeAngle, 4000, "narrative_angle")
	if err != nil {
		return Candidate{}, err
	}

	var estimatedDuration *int
	if payload.EstimatedDurationSeconds != nil {
		if *payload.EstimatedDurationSeconds < 1 || *payload.EstimatedDurationSeconds > 43200 {
			return Candidate{}, fmt.Errorf("estimated_duration_seconds out of range")
		}
		value := *payload.EstimatedDurationSeconds
		estimatedDuration = &value
	}

	formatRationale, err := normalizeOptionalString(payload.FormatRationale, 2000, "format_rationale")
	if err != nil {
		return Candidate{}, err
	}
	structure, err := normalizeStructure(payload.Structure)
	if err != nil {
		return Candidate{}, err
	}
	visualDirection, err := normalizeOptionalString(payload.VisualDirection, 5000, "visual_direction")
	if err != nil {
		return Candidate{}, err
	}
	voiceDirection, err := normalizeOptionalString(payload.VoiceDirection, 3000, "voice_direction")
	if err != nil {
		return Candidate{}, err
	}
	musicDirection, err := normalizeOptionalString(payload.MusicDirection, 3000, "music_direction")
	if err != nil {
		return Candidate{}, err
	}
	captionDirection, err := normalizeOptionalString(payload.CaptionDirection, 3000, "caption_direction")
	if err != nil {
		return Candidate{}, err
	}
	callToAction, err := normalizeOptionalString(payload.CallToAction, 2000, "call_to_action")
	if err != nil {
		return Candidate{}, err
	}
	researchGaps, err := normalizeOptionalStringArray(payload.ResearchGaps, 20, 1000, "research_gaps")
	if err != nil {
		return Candidate{}, err
	}
	warnings, err := normalizeOptionalStringArray(payload.Warnings, 20, 1000, "warnings")
	if err != nil {
		return Candidate{}, err
	}

	return Candidate{
		SourceBriefRevision:      sourceBriefRevision,
		TitleOptions:             titleOptions,
		HookOptions:              hookOptions,
		AudienceSummary:          audienceSummary,
		ObjectiveSummary:         objectiveSummary,
		NarrativeAngle:           narrativeAngle,
		EstimatedDurationSeconds: estimatedDuration,
		FormatRationale:          formatRationale,
		Structure:                structure,
		VisualDirection:          visualDirection,
		VoiceDirection:           voiceDirection,
		MusicDirection:           musicDirection,
		CaptionDirection:         captionDirection,
		CallToAction:             callToAction,
		ResearchGaps:             researchGaps,
		Warnings:                 warnings,
	}, nil
}

func normalizeRequiredString(value string, maxLength int, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s required", field)
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return "", fmt.Errorf("%s exceeds max length %d", field, maxLength)
	}
	return trimmed, nil
}

func normalizeOptionalString(value string, maxLength int, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return "", fmt.Errorf("%s exceeds max length %d", field, maxLength)
	}
	return trimmed, nil
}

func normalizeRequiredStringArray(values []string, minItems, maxItems, maxLength int, field string) ([]string, error) {
	if len(values) < minItems || len(values) > maxItems {
		return nil, fmt.Errorf("%s item count out of range", field)
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("%s items must be non-empty", field)
		}
		if utf8.RuneCountInString(trimmed) > maxLength {
			return nil, fmt.Errorf("%s item exceeds max length %d", field, maxLength)
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

func normalizeOptionalStringArray(values []string, maxItems, maxLength int, field string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > maxItems {
		return nil, fmt.Errorf("%s item count out of range", field)
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("%s items must be non-empty", field)
		}
		if utf8.RuneCountInString(trimmed) > maxLength {
			return nil, fmt.Errorf("%s item exceeds max length %d", field, maxLength)
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

func normalizeStructure(items []providerStructureItem) ([]StructureItem, error) {
	if len(items) < 1 || len(items) > 50 {
		return nil, fmt.Errorf("structure item count out of range")
	}

	normalized := make([]StructureItem, 0, len(items))
	seenKeys := map[string]struct{}{}
	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" || utf8.RuneCountInString(key) > 64 || !structureKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("structure key invalid")
		}
		if _, exists := seenKeys[key]; exists {
			return nil, fmt.Errorf("structure key duplicate")
		}
		seenKeys[key] = struct{}{}

		title, err := normalizeRequiredString(item.Title, 300, "structure.title")
		if err != nil {
			return nil, err
		}
		purpose, err := normalizeRequiredString(item.Purpose, 2000, "structure.purpose")
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, StructureItem{Key: key, Title: title, Purpose: purpose})
	}
	return normalized, nil
}
