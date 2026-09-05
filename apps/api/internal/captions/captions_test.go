package captions

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestValidateSegmentsRejectsInvalidTimingAndOverlap(t *testing.T) {
	duration := int64(10_000)
	tests := []struct {
		name string
		segments []Segment
	}{
		{"negative", []Segment{{ID: uuid.New(), Text: "a", StartMS: -1, EndMS: 100}}},
		{"reversed", []Segment{{ID: uuid.New(), Text: "a", StartMS: 100, EndMS: 100}}},
		{"out_of_duration", []Segment{{ID: uuid.New(), Text: "a", StartMS: 0, EndMS: 10_001}}},
		{"overlap", []Segment{{ID: uuid.New(), Text: "a", StartMS: 0, EndMS: 6_000}, {ID: uuid.New(), Text: "b", StartMS: 5_000, EndMS: 7_000}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateSegments(tt.segments, duration); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestNormalizeSegmentsSortsDeterministicallyAndPreservesIDs(t *testing.T) {
	first := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	second := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	segments := []Segment{
		{ID: second, Text: "second", StartMS: 2_000, EndMS: 3_000},
		{ID: first, Text: "first", StartMS: 0, EndMS: 1_000},
	}
	got, err := NormalizeSegments(segments, 3_000)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != first || got[1].ID != second {
		t.Fatalf("unexpected deterministic order: %#v", got)
	}
}

func TestStateForSourceDetectsNarrationLineageChange(t *testing.T) {
	doc := Document{
		SourceBindingID: uuid.New(),
		SourceAssetID: uuid.New(),
		SourceDurationMS: 4_000,
	}
	if got := StateForSource(doc, Source{BindingID: doc.SourceBindingID, AssetID: doc.SourceAssetID, DurationMS: doc.SourceDurationMS}); got != StateCurrent {
		t.Fatalf("expected current, got %s", got)
	}
	if got := StateForSource(doc, Source{BindingID: uuid.New(), AssetID: doc.SourceAssetID, DurationMS: doc.SourceDurationMS}); got != StateStale {
		t.Fatalf("expected stale after lineage change, got %s", got)
	}
}

func TestSnapshotRejectsStaleDocumentAndCopiesContent(t *testing.T) {
	doc := Document{ID: uuid.New(), Revision: 2, SourceBindingID: uuid.New(), SourceAssetID: uuid.New(), SourceDurationMS: 2_000, Segments: []Segment{{ID: uuid.New(), Text: "hello", StartMS: 0, EndMS: 2_000}}}
	if _, err := NewSnapshot(doc, StateStale); !errors.Is(err, ErrStale) {
		t.Fatalf("expected stale error, got %v", err)
	}
	snap, err := NewSnapshot(doc, StateCurrent)
	if err != nil {
		t.Fatal(err)
	}
	doc.Segments[0].Text = "mutated"
	if snap.Segments[0].Text != "hello" {
		t.Fatalf("snapshot mutated with live document")
	}
}
