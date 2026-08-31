package creativebrief

import (
	"strings"
	"testing"
)

func TestPutInputNormalizeAndValidate(t *testing.T) {
	input := PutInput{
		SourceText:          "  creator intent  ",
		TargetAudience:      "  founders  ",
		Objective:           "  launch  ",
		DesiredStyle:        "  documentary  ",
		Tone:                "  confident  ",
		DistributionTargets: []DistributionTarget{"youtube", "tiktok"},
		CallToAction:        "  sign up  ",
		MustInclude:         []string{"  product demo  ", "pricing"},
		MustAvoid:           []string{"  unsupported claims  "},
	}

	if err := input.NormalizeAndValidate(); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}

	if input.SourceText != "creator intent" {
		t.Fatalf("expected source_text to be trimmed, got %q", input.SourceText)
	}
	if input.TargetAudience != "founders" {
		t.Fatalf("expected target_audience to be trimmed, got %q", input.TargetAudience)
	}
	if input.MustInclude[0] != "product demo" {
		t.Fatalf("expected must_include item to be trimmed, got %q", input.MustInclude[0])
	}
}

func TestPutInputRejectsInvalidFields(t *testing.T) {
	input := PutInput{
		SourceText:          " ",
		TargetAudience:      strings.Repeat("a", 2001),
		Objective:           strings.Repeat("a", 2001),
		DesiredStyle:        strings.Repeat("a", 2001),
		Tone:                strings.Repeat("a", 501),
		DistributionTargets: []DistributionTarget{"youtube", "youtube", "bad"},
		CallToAction:        strings.Repeat("a", 2001),
		MustInclude:         []string{"valid", " "},
		MustAvoid:           []string{strings.Repeat("a", 501)},
	}

	err := input.NormalizeAndValidate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	for _, field := range []string{
		"source_text",
		"target_audience",
		"objective",
		"desired_style",
		"tone",
		"distribution_targets",
		"call_to_action",
		"must_include",
		"must_avoid",
	} {
		if validationErr.Fields[field] == "" {
			t.Fatalf("expected validation error for %s, got %#v", field, validationErr.Fields)
		}
	}
}

func TestPutInputRejectsTooManyArrayItems(t *testing.T) {
	input := validPutInput()
	input.DistributionTargets = []DistributionTarget{
		"youtube", "tiktok", "instagram", "other", "youtube", "tiktok", "instagram", "other", "youtube",
	}
	input.MustInclude = make([]string, 21)
	input.MustAvoid = make([]string, 21)
	for i := range input.MustInclude {
		input.MustInclude[i] = "item"
		input.MustAvoid[i] = "item"
	}

	err := input.NormalizeAndValidate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	validationErr := err.(ValidationError)
	for _, field := range []string{"distribution_targets", "must_include", "must_avoid"} {
		if validationErr.Fields[field] == "" {
			t.Fatalf("expected validation error for %s, got %#v", field, validationErr.Fields)
		}
	}
}

func validPutInput() PutInput {
	return PutInput{
		SourceText:          "Creator intent",
		TargetAudience:      "Founders",
		Objective:           "Launch",
		DesiredStyle:        "Documentary",
		Tone:                "Confident",
		DistributionTargets: []DistributionTarget{"youtube"},
		CallToAction:        "Sign up",
		MustInclude:         []string{"Product demo"},
		MustAvoid:           []string{"Unsupported claims"},
	}
}
