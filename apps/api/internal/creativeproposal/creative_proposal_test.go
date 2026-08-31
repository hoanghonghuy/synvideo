package creativeproposal

import (
	"strings"
	"testing"
)

func validTestContent() Content {
	duration := 90
	return Content{
		TitleOptions:             []string{"Title 1", "Title 2"},
		HookOptions:              []string{"Hook 1", "Hook 2"},
		AudienceSummary:          "Founders and creators",
		ObjectiveSummary:         "Explain product value",
		NarrativeAngle:           "From friction to flow",
		EstimatedDurationSeconds: &duration,
		FormatRationale:          "Short duration retains engagement",
		Structure: []StructureItem{
			{
				Key:     "hook",
				Title:   "The Hook",
				Purpose: "Grab attention in 3 seconds",
			},
			{
				Key:     "problem",
				Title:   "The Problem",
				Purpose: "Show friction in manual video editing",
			},
		},
		VisualDirection:  "Clean modern aesthetic",
		VoiceDirection:   "Warm and authoritative",
		MusicDirection:   "Subtle lo-fi beats",
		CaptionDirection: "Bold dynamic captions",
		CallToAction:     "Visit synvideo.com",
		ResearchGaps:     []string{"Competitor benchmarks"},
		Warnings:         []string{"Avoid overpromising render speed"},
	}
}

func TestContentNormalizeAndValidateSuccess(t *testing.T) {
	c := validTestContent()
	if err := c.NormalizeAndValidate(); err != nil {
		t.Fatalf("expected valid content, got %v", err)
	}
}

func TestContentNormalizeAndValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(c *Content)
		expectedField string
		expectedCode  string
	}{
		{
			name: "empty title options",
			mutate: func(c *Content) {
				c.TitleOptions = nil
			},
			expectedField: "title_options",
			expectedCode:  "required",
		},
		{
			name: "too many title options",
			mutate: func(c *Content) {
				c.TitleOptions = []string{"1", "2", "3", "4", "5", "6"}
			},
			expectedField: "title_options",
			expectedCode:  "max_5",
		},
		{
			name: "empty title option item",
			mutate: func(c *Content) {
				c.TitleOptions = []string{"Title 1", "   "}
			},
			expectedField: "title_options",
			expectedCode:  "items_required",
		},
		{
			name: "title option item too long",
			mutate: func(c *Content) {
				c.TitleOptions = []string{strings.Repeat("a", 301)}
			},
			expectedField: "title_options",
			expectedCode:  "items_max_300",
		},
		{
			name: "empty hook options",
			mutate: func(c *Content) {
				c.HookOptions = []string{}
			},
			expectedField: "hook_options",
			expectedCode:  "required",
		},
		{
			name: "too many hook options",
			mutate: func(c *Content) {
				c.HookOptions = []string{"1", "2", "3", "4", "5", "6"}
			},
			expectedField: "hook_options",
			expectedCode:  "max_5",
		},
		{
			name: "hook option item too long",
			mutate: func(c *Content) {
				c.HookOptions = []string{strings.Repeat("h", 1001)}
			},
			expectedField: "hook_options",
			expectedCode:  "items_max_1000",
		},
		{
			name: "empty audience summary",
			mutate: func(c *Content) {
				c.AudienceSummary = "   "
			},
			expectedField: "audience_summary",
			expectedCode:  "required",
		},
		{
			name: "audience summary too long",
			mutate: func(c *Content) {
				c.AudienceSummary = strings.Repeat("a", 2001)
			},
			expectedField: "audience_summary",
			expectedCode:  "max_2000",
		},
		{
			name: "empty objective summary",
			mutate: func(c *Content) {
				c.ObjectiveSummary = ""
			},
			expectedField: "objective_summary",
			expectedCode:  "required",
		},
		{
			name: "objective summary too long",
			mutate: func(c *Content) {
				c.ObjectiveSummary = strings.Repeat("o", 2001)
			},
			expectedField: "objective_summary",
			expectedCode:  "max_2000",
		},
		{
			name: "empty narrative angle",
			mutate: func(c *Content) {
				c.NarrativeAngle = ""
			},
			expectedField: "narrative_angle",
			expectedCode:  "required",
		},
		{
			name: "narrative angle too long",
			mutate: func(c *Content) {
				c.NarrativeAngle = strings.Repeat("n", 4001)
			},
			expectedField: "narrative_angle",
			expectedCode:  "max_4000",
		},
		{
			name: "estimated duration negative",
			mutate: func(c *Content) {
				d := 0
				c.EstimatedDurationSeconds = &d
			},
			expectedField: "estimated_duration_seconds",
			expectedCode:  "range_1_43200",
		},
		{
			name: "estimated duration over max",
			mutate: func(c *Content) {
				d := 43201
				c.EstimatedDurationSeconds = &d
			},
			expectedField: "estimated_duration_seconds",
			expectedCode:  "range_1_43200",
		},
		{
			name: "format rationale too long",
			mutate: func(c *Content) {
				c.FormatRationale = strings.Repeat("r", 2001)
			},
			expectedField: "format_rationale",
			expectedCode:  "max_2000",
		},
		{
			name: "structure empty",
			mutate: func(c *Content) {
				c.Structure = nil
			},
			expectedField: "structure",
			expectedCode:  "required",
		},
		{
			name: "structure item key empty",
			mutate: func(c *Content) {
				c.Structure = []StructureItem{{Key: "", Title: "T", Purpose: "P"}}
			},
			expectedField: "structure",
			expectedCode:  "items_required",
		},
		{
			name: "structure item key invalid slug uppercase",
			mutate: func(c *Content) {
				c.Structure = []StructureItem{{Key: "Invalid_Key!", Title: "T", Purpose: "P"}}
			},
			expectedField: "structure",
			expectedCode:  "items_key_invalid",
		},
		{
			name: "structure item key too long",
			mutate: func(c *Content) {
				c.Structure = []StructureItem{{Key: strings.Repeat("a", 65), Title: "T", Purpose: "P"}}
			},
			expectedField: "structure",
			expectedCode:  "items_key_max_64",
		},
		{
			name: "structure duplicate keys",
			mutate: func(c *Content) {
				c.Structure = []StructureItem{
					{Key: "intro", Title: "T1", Purpose: "P1"},
					{Key: "intro", Title: "T2", Purpose: "P2"},
				}
			},
			expectedField: "structure",
			expectedCode:  "keys_unique",
		},
		{
			name: "structure item title too long",
			mutate: func(c *Content) {
				c.Structure = []StructureItem{{Key: "k", Title: strings.Repeat("t", 301), Purpose: "P"}}
			},
			expectedField: "structure",
			expectedCode:  "items_title_max_300",
		},
		{
			name: "structure item purpose too long",
			mutate: func(c *Content) {
				c.Structure = []StructureItem{{Key: "k", Title: "T", Purpose: strings.Repeat("p", 2001)}}
			},
			expectedField: "structure",
			expectedCode:  "items_purpose_max_2000",
		},
		{
			name: "visual direction too long",
			mutate: func(c *Content) {
				c.VisualDirection = strings.Repeat("v", 5001)
			},
			expectedField: "visual_direction",
			expectedCode:  "max_5000",
		},
		{
			name: "voice direction too long",
			mutate: func(c *Content) {
				c.VoiceDirection = strings.Repeat("v", 3001)
			},
			expectedField: "voice_direction",
			expectedCode:  "max_3000",
		},
		{
			name: "music direction too long",
			mutate: func(c *Content) {
				c.MusicDirection = strings.Repeat("m", 3001)
			},
			expectedField: "music_direction",
			expectedCode:  "max_3000",
		},
		{
			name: "caption direction too long",
			mutate: func(c *Content) {
				c.CaptionDirection = strings.Repeat("c", 3001)
			},
			expectedField: "caption_direction",
			expectedCode:  "max_3000",
		},
		{
			name: "call to action too long",
			mutate: func(c *Content) {
				c.CallToAction = strings.Repeat("c", 2001)
			},
			expectedField: "call_to_action",
			expectedCode:  "max_2000",
		},
		{
			name: "research gaps too many",
			mutate: func(c *Content) {
				c.ResearchGaps = make([]string, 21)
				for i := range c.ResearchGaps {
					c.ResearchGaps[i] = "gap"
				}
			},
			expectedField: "research_gaps",
			expectedCode:  "max_20",
		},
		{
			name: "warnings too many",
			mutate: func(c *Content) {
				c.Warnings = make([]string, 21)
				for i := range c.Warnings {
					c.Warnings[i] = "warning"
				}
			},
			expectedField: "warnings",
			expectedCode:  "max_20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := validTestContent()
			tt.mutate(&content)
			err := content.NormalizeAndValidate()
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			valErr, ok := err.(ValidationError)
			if !ok {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			if code, exists := valErr.Fields[tt.expectedField]; !exists || code != tt.expectedCode {
				t.Fatalf("expected field %q = %q, got fields: %#v", tt.expectedField, tt.expectedCode, valErr.Fields)
			}
		})
	}
}

func TestPutInputValidation(t *testing.T) {
	rev := 1
	put := PutInput{
		Revision: &rev,
		Content:  validTestContent(),
	}
	if err := put.NormalizeAndValidate(); err != nil {
		t.Fatalf("expected valid put input, got %v", err)
	}

	// Missing revision
	putNoRev := PutInput{
		Content: validTestContent(),
	}
	err := putNoRev.NormalizeAndValidate()
	if err == nil {
		t.Fatal("expected error for missing revision")
	}
	valErr := err.(ValidationError)
	if valErr.Fields["revision"] != "required" {
		t.Fatalf("expected revision=required, got %#v", valErr.Fields)
	}

	// Non-positive revision
	zero := 0
	putZeroRev := PutInput{
		Revision: &zero,
		Content:  validTestContent(),
	}
	err = putZeroRev.NormalizeAndValidate()
	if err == nil {
		t.Fatal("expected error for zero revision")
	}
	valErr = err.(ValidationError)
	if valErr.Fields["revision"] != "positive" {
		t.Fatalf("expected revision=positive, got %#v", valErr.Fields)
	}
}

func TestCreateDraftInputValidation(t *testing.T) {
	create := CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validTestContent(),
	}
	if err := create.NormalizeAndValidate(); err != nil {
		t.Fatalf("expected valid create draft input, got %v", err)
	}

	createZeroBriefRev := CreateDraftInput{
		SourceBriefRevision: 0,
		Content:             validTestContent(),
	}
	err := createZeroBriefRev.NormalizeAndValidate()
	if err == nil {
		t.Fatal("expected error for zero source_brief_revision")
	}
	valErr := err.(ValidationError)
	if valErr.Fields["source_brief_revision"] != "positive" {
		t.Fatalf("expected source_brief_revision=positive, got %#v", valErr.Fields)
	}
}
