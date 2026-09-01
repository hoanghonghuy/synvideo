package sceneplangeneration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

var sceneKeyPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

type providerPayload struct {
	Scenes []providerScene `json:"scenes"`
}

type providerScene struct {
	Key                     string     `json:"key"`
	ScriptSectionKey        string     `json:"script_section_key"`
	Narration               string     `json:"narration"`
	VisualInstruction       string     `json:"visual_instruction"`
	PlannedSourceType       SourceType `json:"planned_source_type"`
	ExpectedDurationSeconds int        `json:"expected_duration_seconds"`
	CaptionIntent           string     `json:"caption_intent"`
	TransitionNotes         string     `json:"transition_notes"`
}

func parseCandidate(raw string, script ScriptContext, sourceProposalVersion int) (Candidate, error) {
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
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Candidate{}, newInvalidOutputError(fmt.Errorf("trailing json values"))
		}
		return Candidate{}, newInvalidOutputError(fmt.Errorf("trailing provider output: %w", err))
	}

	candidate, err := validateCandidate(payload, script, sourceProposalVersion)
	if err != nil {
		return Candidate{}, newInvalidOutputError(err)
	}
	return candidate, nil
}

func validateCandidate(payload providerPayload, script ScriptContext, sourceProposalVersion int) (Candidate, error) {
	if len(payload.Scenes) < 1 || len(payload.Scenes) > 500 {
		return Candidate{}, fmt.Errorf("scenes count must be between 1 and 500")
	}
	if len(script.Sections) == 0 {
		return Candidate{}, fmt.Errorf("script must contain sections")
	}

	sectionBodies := make(map[string]string, len(script.Sections))
	sectionOrder := make([]string, len(script.Sections))
	for index, section := range script.Sections {
		key := strings.TrimSpace(section.Key)
		if key == "" {
			return Candidate{}, fmt.Errorf("script section key is required")
		}
		if _, exists := sectionBodies[key]; exists {
			return Candidate{}, fmt.Errorf("duplicate script section key %q", key)
		}
		sectionOrder[index] = key
		sectionBodies[key] = canonicalWhitespace(section.Body)
	}

	scenes := make([]Scene, 0, len(payload.Scenes))
	seenSceneKeys := make(map[string]struct{}, len(payload.Scenes))
	sectionSceneText := make(map[string][]string, len(script.Sections))
	sectionIndexes := make(map[string]int, len(script.Sections))
	for index, key := range sectionOrder {
		sectionIndexes[key] = index
	}
	currentSectionIndex := -1
	for _, item := range payload.Scenes {
		scene, err := normalizeScene(item)
		if err != nil {
			return Candidate{}, err
		}
		if _, exists := seenSceneKeys[scene.Key]; exists {
			return Candidate{}, fmt.Errorf("duplicate scene key %q", scene.Key)
		}
		seenSceneKeys[scene.Key] = struct{}{}

		sectionIndex, exists := sectionIndexes[scene.ScriptSectionKey]
		if !exists {
			return Candidate{}, fmt.Errorf("unknown script section key %q", scene.ScriptSectionKey)
		}
		if currentSectionIndex == -1 {
			currentSectionIndex = sectionIndex
		} else if sectionIndex < currentSectionIndex || sectionIndex > currentSectionIndex+1 {
			return Candidate{}, fmt.Errorf("script sections are reordered or non-contiguous")
		} else if sectionIndex == currentSectionIndex+1 {
			currentSectionIndex = sectionIndex
		}
		sectionSceneText[scene.ScriptSectionKey] = append(sectionSceneText[scene.ScriptSectionKey], scene.Narration)
		scenes = append(scenes, scene)
	}

	for _, key := range sectionOrder {
		segments, exists := sectionSceneText[key]
		if !exists || len(segments) == 0 {
			return Candidate{}, fmt.Errorf("script section %q is missing scene coverage", key)
		}
		if canonicalWhitespace(strings.Join(segments, "")) != sectionBodies[key] {
			return Candidate{}, fmt.Errorf("narration coverage does not match approved script section %q", key)
		}
	}

	return Candidate{
		SourceScriptVersion:   script.Version,
		SourceProposalVersion: sourceProposalVersion,
		Scenes:                scenes,
	}, nil
}

func normalizeScene(item providerScene) (Scene, error) {
	key := strings.TrimSpace(item.Key)
	if key == "" || utf8.RuneCountInString(key) > 64 || !sceneKeyPattern.MatchString(key) {
		return Scene{}, fmt.Errorf("invalid scene key %q", key)
	}
	sectionKey := strings.TrimSpace(item.ScriptSectionKey)
	if sectionKey == "" {
		return Scene{}, fmt.Errorf("script section key is required")
	}
	narration, err := normalizeNarration(item.Narration, 20000)
	if err != nil {
		return Scene{}, err
	}
	visual, err := normalizeRequiredString(item.VisualInstruction, 5000, "visual instruction")
	if err != nil {
		return Scene{}, err
	}
	if !validSourceType(item.PlannedSourceType) {
		return Scene{}, fmt.Errorf("invalid planned source type %q", item.PlannedSourceType)
	}
	if item.ExpectedDurationSeconds < 1 || item.ExpectedDurationSeconds > 3600 {
		return Scene{}, fmt.Errorf("expected duration is out of range")
	}
	caption, err := normalizeOptionalString(item.CaptionIntent, 3000, "caption intent")
	if err != nil {
		return Scene{}, err
	}
	transition, err := normalizeOptionalString(item.TransitionNotes, 2000, "transition notes")
	if err != nil {
		return Scene{}, err
	}
	return Scene{
		Key:                     key,
		ScriptSectionKey:        sectionKey,
		Narration:               narration,
		VisualInstruction:       visual,
		PlannedSourceType:       item.PlannedSourceType,
		ExpectedDurationSeconds: item.ExpectedDurationSeconds,
		CaptionIntent:           caption,
		TransitionNotes:         transition,
	}, nil
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

func normalizeRequiredString(value string, maxLength int, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return "", fmt.Errorf("%s exceeds max length %d", field, maxLength)
	}
	return trimmed, nil
}

func normalizeNarration(value string, maxLength int) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("narration is required")
	}
	if utf8.RuneCountInString(value) > maxLength {
		return "", fmt.Errorf("narration exceeds max length %d", maxLength)
	}
	return value, nil
}

func normalizeOptionalString(value string, maxLength int, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) > maxLength {
		return "", fmt.Errorf("%s exceeds max length %d", field, maxLength)
	}
	return trimmed, nil
}
