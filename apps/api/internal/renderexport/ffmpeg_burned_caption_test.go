package renderexport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestRenderSingleVisualMP4AddsBurnedCaptionsAsServerOwnedFilter(t *testing.T) {
	dir := t.TempDir()
	visual := filepath.Join(dir, "visual.ppm")
	captions := filepath.Join(dir, "captions.vtt")
	output := filepath.Join(dir, "out.mp4")
	if err := os.WriteFile(visual, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, err := BuildWebVTT([]WebVTTCue{{
		StartMS: 0,
		EndMS:   500,
		Text:    "Unicode Việt Nam 日本語 ; -vf evil <tag>",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(captions, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &renderRunnerStub{}
	_, err = RenderSingleVisualMP4(context.Background(), runner, testLocalProfile(), PreparedLocalRenderInput{
		VisualPath:       visual,
		OutputPath:       output,
		Width:            320,
		Height:           180,
		FrameRate:        24,
		DurationMS:       500,
		Fit:              sceneeditor.FitContain,
		CaptionVTTPath:   captions,
		CaptionProfileID: BurnedCaptionProfileV1,
	})
	if err != nil {
		t.Fatalf("RenderSingleVisualMP4() error = %v", err)
	}
	if len(runner.calls) < 1 {
		t.Fatal("ffmpeg was not invoked")
	}
	call := runner.calls[0]
	var filter string
	for i, arg := range call {
		if arg == "-vf" && i+1 < len(call) {
			filter = call[i+1]
			break
		}
	}
	if filter == "" || !strings.Contains(filter, "subtitles=filename='") || !strings.Contains(filter, captions) {
		t.Fatalf("caption filter not present in %#v", call)
	}
	if strings.Contains(filter, "Unicode Việt Nam") || strings.Contains(filter, "-vf evil") {
		t.Fatalf("creator caption text leaked into filter syntax: %q", filter)
	}
	for _, arg := range call {
		if arg == "sh" || arg == "-c" {
			t.Fatalf("unexpected shell invocation in %#v", call)
		}
	}
}

func TestRenderSingleVisualMP4RejectsUnsafeCaptionFileContract(t *testing.T) {
	dir := t.TempDir()
	visual := filepath.Join(dir, "visual.ppm")
	if err := os.WriteFile(visual, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := PreparedLocalRenderInput{
		VisualPath: visual,
		OutputPath: filepath.Join(dir, "out.mp4"),
		Width:      320,
		Height:     180,
		FrameRate:  24,
		DurationMS: 500,
		Fit:        sceneeditor.FitContain,
	}

	missingProfile := base
	missingProfile.CaptionVTTPath = filepath.Join(dir, "captions.vtt")
	if err := os.WriteFile(missingProfile.CaptionVTTPath, []byte("WEBVTT\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderSingleVisualMP4(context.Background(), &renderRunnerStub{}, testLocalProfile(), missingProfile); err != ErrInvalidRenderInput {
		t.Fatalf("missing profile error = %v, want %v", err, ErrInvalidRenderInput)
	}

	unsafePath := base
	unsafePath.CaptionVTTPath = filepath.Join(dir, "captions:unsafe.vtt")
	unsafePath.CaptionProfileID = BurnedCaptionProfileV1
	if err := os.WriteFile(unsafePath.CaptionVTTPath, []byte("WEBVTT\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderSingleVisualMP4(context.Background(), &renderRunnerStub{}, testLocalProfile(), unsafePath); err != ErrInvalidRenderInput {
		t.Fatalf("unsafe path error = %v, want %v", err, ErrInvalidRenderInput)
	}
}
