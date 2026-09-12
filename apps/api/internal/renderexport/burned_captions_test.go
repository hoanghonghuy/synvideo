package renderexport

import (
	"errors"
	"strings"
	"testing"
)

func TestBurnedCaptionFilterUsesVersionedServerOwnedProfile(t *testing.T) {
	filter, err := BurnedCaptionFilter(BurnedCaptionProfileV1, "/tmp/synvideo-render-123/captions.vtt")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"subtitles=filename='/tmp/synvideo-render-123/captions.vtt'",
		"FontName=DejaVu Sans",
		"FontSize=28",
		"MarginV=42",
		"Alignment=2",
		"WrapStyle=0",
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter %q does not contain %q", filter, want)
		}
	}
}

func TestBurnedCaptionFilterRejectsFilterGrammarAndUnknownProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		path    string
	}{
		{name: "unknown profile", profile: "burned_caption_v2", path: "/tmp/captions.vtt"},
		{name: "relative path", profile: BurnedCaptionProfileV1, path: "captions.vtt"},
		{name: "single quote", profile: BurnedCaptionProfileV1, path: "/tmp/cap'tions.vtt"},
		{name: "filter separator", profile: BurnedCaptionProfileV1, path: "/tmp/captions.vtt:si=1"},
		{name: "comma", profile: BurnedCaptionProfileV1, path: "/tmp/captions,vtt"},
		{name: "traversal", profile: BurnedCaptionProfileV1, path: "/tmp/render/../captions.vtt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BurnedCaptionFilter(tt.profile, tt.path)
			if !errors.Is(err, ErrInvalidBurnedCaptionInput) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidBurnedCaptionInput)
			}
		})
	}
}
