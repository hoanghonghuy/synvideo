package renderexport

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

func TestRenderSingleVisualMP4RealFFmpegWithBurnedCaptions(t *testing.T) {
	if os.Getenv("SYNVIDEO_TEST_FFMPEG") != "1" {
		t.Skip("set SYNVIDEO_TEST_FFMPEG=1 to run real FFmpeg burned-caption smoke")
	}
	ctx := context.Background()
	profile, err := ProbeLocalFFmpegProfile(ctx, nil)
	if err != nil {
		t.Fatalf("ProbeLocalFFmpegProfile() error = %v", err)
	}
	dir := t.TempDir()
	visual := filepath.Join(dir, "visual.ppm")
	captions := filepath.Join(dir, "captions.vtt")
	output := filepath.Join(dir, "render.mp4")
	ppm := []byte("P3\n2 2\n255\n20 20 20  20 20 20\n20 20 20  20 20 20\n")
	if err := os.WriteFile(visual, ppm, 0o600); err != nil {
		t.Fatal(err)
	}
	vtt, err := BuildWebVTT([]WebVTTCue{
		{StartMS: 100, EndMS: 900, Text: "Xin chào Việt Nam — 日本語"},
		{StartMS: 900, EndMS: 1700, Text: "A deliberately long caption line that exercises deterministic libass wrapping near the bottom safe margin."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(captions, vtt, 0o600); err != nil {
		t.Fatal(err)
	}

	metadata, err := RenderSingleVisualMP4(ctx, nil, profile, PreparedLocalRenderInput{
		VisualPath:       visual,
		OutputPath:       output,
		Width:            320,
		Height:           180,
		FrameRate:        24,
		DurationMS:       2000,
		Fit:              sceneeditor.FitContain,
		CaptionVTTPath:   captions,
		CaptionProfileID: BurnedCaptionProfileV1,
	})
	if err != nil {
		t.Fatalf("RenderSingleVisualMP4(real captions) error = %v", err)
	}
	if metadata.ByteSize <= 0 || metadata.Width != 320 || metadata.Height != 180 || metadata.VideoCodec != "h264" || metadata.PixelFormat != "yuv420p" {
		t.Fatalf("real caption metadata = %#v", metadata)
	}
	if metadata.DurationMS < 1900 || metadata.DurationMS > 2200 {
		t.Fatalf("duration = %dms, expected near 2000ms", metadata.DurationMS)
	}
}
