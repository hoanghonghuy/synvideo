package scriptgeneration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var sectionKeyPattern = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

type providerPayload struct {
	Sections                 []providerSection `json:"sections"`
	EstimatedDurationSeconds *int              `json:"estimated_duration_seconds"`
	Notes                    string            `json:"notes"`
}

type providerSection struct {
	Key     string `json:"key"`
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

func parseCandidate(raw string, sourceProposalVersion int) (Candidate, error) {
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

	candidate, err := validateCandidate(payload, sourceProposalVersion)
	if err != nil {
		return Candidate{}, newInvalidOutputError(err)
	}
	return candidate, nil
}

func validateCandidate(payload providerPayload, sourceProposalVersion int) (Candidate, error) {
	sections, err := normalizeSections(payload.Sections)
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

	notes, err := normalizeOptionalString(payload.Notes, 10000, "notes")
	if err != nil {
		return Candidate{}, err
	}

	return Candidate{
		SourceProposalVersion:    sourceProposalVersion,
		Sections:                 sections,
		EstimatedDurationSeconds: estimatedDuration,
		Notes:                    notes,
	}, nil
}

func normalizeSections(items []providerSection) ([]Section, error) {
	if len(items) < 1 || len(items) > 200 {
		return nil, fmt.Errorf("sections count must be between 1 and 200")
	}

	normalized := make([]Section, 0, len(items))
	seenKeys := map[string]struct{}{}

	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" || utf8.RuneCountInString(key) > 64 || !sectionKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid section key %q", key)
		}
		if _, exists := seenKeys[key]; exists {
			return nil, fmt.Errorf("duplicate section key %q", key)
		}
		seenKeys[key] = struct{}{}

		heading, err := normalizeOptionalString(item.Heading, 300, "section heading")
		if err != nil {
			return nil, err
		}

		body, err := normalizeRequiredString(item.Body, 20000, "section body")
		if err != nil {
			return nil, err
		}

		normalized = append(normalized, Section{
			Key:     key,
			Heading: heading,
			Body:    body,
		})
	}

	return normalized, nil
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
