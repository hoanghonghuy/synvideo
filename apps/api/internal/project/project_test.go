package project

import "testing"

func TestCreateInputNormalizesAndValidates(t *testing.T) {
	input := CreateInput{
		Title:         "  Du an moi  ",
		Description:   "  Mo ta  ",
		ContentFormat: ContentFormatShort,
		AspectRatio:   AspectRatio9x16,
	}

	if err := input.NormalizeAndValidate(); err != nil {
		t.Fatalf("expected valid input: %v", err)
	}
	if input.Title != "Du an moi" {
		t.Fatalf("expected trimmed title, got %q", input.Title)
	}
	if input.Description != "Mo ta" {
		t.Fatalf("expected trimmed description, got %q", input.Description)
	}
	if input.Locale != LocaleVI {
		t.Fatalf("expected default locale vi, got %q", input.Locale)
	}
}

func TestCreateInputRejectsInvalidMetadata(t *testing.T) {
	duration := 0
	input := CreateInput{
		Title:                 " ",
		Description:           "ok",
		ContentFormat:         "clip",
		AspectRatio:           "21:9",
		TargetDurationSeconds: &duration,
		Locale:                "fr",
	}

	err := input.NormalizeAndValidate()
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected validation error, got %T", err)
	}
	for _, field := range []string{"title", "content_format", "aspect_ratio", "target_duration_seconds", "locale"} {
		if _, exists := validationErr.Fields[field]; !exists {
			t.Fatalf("expected field %q validation error, got %#v", field, validationErr.Fields)
		}
	}
}

func TestUpdateInputDistinguishesClearDuration(t *testing.T) {
	var cleared *int
	input := UpdateInput{TargetDurationSeconds: &cleared}

	if err := input.NormalizeAndValidate(); err != nil {
		t.Fatalf("expected clearing duration to be valid: %v", err)
	}
}

func TestUpdateInputRejectsInvalidStatus(t *testing.T) {
	status := Status("done")
	input := UpdateInput{Status: &status}

	err := input.NormalizeAndValidate()
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected validation error, got %T", err)
	}
	if validationErr.Fields["status"] != "invalid" {
		t.Fatalf("expected status invalid error, got %#v", validationErr.Fields)
	}
}

func TestUpdateInputRejectsEmpty(t *testing.T) {
	input := UpdateInput{}

	err := input.NormalizeAndValidate()
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected validation error for empty update, got %T", err)
	}
	if validationErr.Fields["body"] != "empty" {
		t.Fatalf("expected body empty error, got %#v", validationErr.Fields)
	}
}
