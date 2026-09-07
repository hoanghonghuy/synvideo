package sceneeditor

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVisualTreatmentFrozenBounds(t *testing.T) {
	tests := []struct {
		name      string
		treatment VisualTreatment
		field     string
	}{
		{name: "position x below", treatment: VisualTreatment{Fit: FitContain, PositionX: -1.01, Scale: 1}, field: "visual_treatment.position_x"},
		{name: "position x above", treatment: VisualTreatment{Fit: FitContain, PositionX: 1.01, Scale: 1}, field: "visual_treatment.position_x"},
		{name: "position y below", treatment: VisualTreatment{Fit: FitContain, PositionY: -1.01, Scale: 1}, field: "visual_treatment.position_y"},
		{name: "position y above", treatment: VisualTreatment{Fit: FitContain, PositionY: 1.01, Scale: 1}, field: "visual_treatment.position_y"},
		{name: "scale below", treatment: VisualTreatment{Fit: FitContain, Scale: 0.249}, field: "visual_treatment.scale"},
		{name: "scale above", treatment: VisualTreatment{Fit: FitContain, Scale: 4.001}, field: "visual_treatment.scale"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
				ID: uuid.New(), SceneKey: "scene-a", DurationMS: 5_000,
				VisualTreatment: tt.treatment,
				TransitionOut:   Transition{Kind: TransitionCut},
			}}, nil, time.Now().UTC())
			if err == nil {
				t.Fatalf("NewDocument unexpectedly succeeded: %#v", doc)
			}
			validation, ok := err.(ValidationError)
			if !ok {
				t.Fatalf("err=%T %v want ValidationError", err, err)
			}
			if got := validation.Fields["scenes[0]."+tt.field]; got != "out_of_range" {
				t.Fatalf("field=%q got=%q fields=%v", tt.field, got, validation.Fields)
			}
		})
	}

	for _, treatment := range []VisualTreatment{
		{Fit: FitContain, PositionX: -1, PositionY: 1, Scale: 0.25},
		{Fit: FitCover, PositionX: 1, PositionY: -1, Scale: 4},
	} {
		if _, err := NewDocument(uuid.New(), uuid.New(), uuid.New(), 1, []Scene{{
			ID: uuid.New(), SceneKey: "scene-a", DurationMS: 5_000,
			VisualTreatment: treatment,
			TransitionOut:   Transition{Kind: TransitionCut},
		}}, nil, time.Now().UTC()); err != nil {
			t.Fatalf("inclusive bounds rejected: %v", err)
		}
	}
}

func TestTransitionFrozenDurationBounds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		durationMS int64
		wantErr    bool
	}{
		{name: "below minimum", durationMS: 99, wantErr: true},
		{name: "minimum", durationMS: 100},
		{name: "maximum", durationMS: 2_000},
		{name: "above maximum", durationMS: 2_001, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTransition(Transition{Kind: TransitionFade, DurationMS: tc.durationMS}, 5_000)
			if tc.wantErr && err == nil {
				t.Fatal("expected transition duration validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected transition validation error: %v", err)
			}
		})
	}
}
