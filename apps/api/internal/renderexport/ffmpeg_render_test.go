package renderexport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

type renderRunnerStub struct {
	calls [][]string
}

func (r *renderRunnerStub) Run(_ context.Context, name string, args ...string) ([]byte, []byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if name == FFmpegBinary {
		if len(args) == 0 {
			return nil, nil, errors.New("missing args")
		}
		output := args[len(args)-1]
		if err := os.WriteFile(output, []byte("fake-mp4"), 0o600); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}
	if name == FFprobeBinary {
		return []byte(`{"streams":[{"codec_name":"h264","width":320,"height":180,"pix_fmt":"yuv420p"}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"0.500000"}}`), nil, nil
	}
	return nil, nil, errors.New("unexpected binary")
}

func testLocalProfile() FFmpegProfile {
	return FFmpegProfile{
		ID:           LocalProfileID,
		VideoEncoder: "libx264",
		AudioEncoder: "aac",
		PixelFormat:  "yuv420p",
		Container:    "mp4",
		VersionLine:  "ffmpeg version test",
	}
}

func TestRenderSingleVisualMP4UsesFixedServerOwnedArgumentsAndVerifiesOutput(t *testing.T) {
	dir := t.TempDir()
	visual := filepath.Join(dir, "visual.ppm")
	output := filepath.Join(dir, "out.mp4")
	if err := os.WriteFile(visual, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &renderRunnerStub{}
	metadata, err := RenderSingleVisualMP4(context.Background(), runner, testLocalProfile(), PreparedLocalRenderInput{
		VisualPath: visual,
		OutputPath: output,
		Width:      320,
		Height:     180,
		FrameRate:  24,
		DurationMS: 500,
		Fit:        sceneeditor.FitContain,
	})
	if err != nil {
		t.Fatalf("RenderSingleVisualMP4() error = %v", err)
	}
	if metadata.ByteSize != int64(len("fake-mp4")) || metadata.DurationMS != 500 || metadata.Width != 320 || metadata.Height != 180 || metadata.VideoCodec != "h264" || metadata.PixelFormat != "yuv420p" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	wantPrefix := []string{FFmpegBinary, "-hide_banner", "-nostdin", "-loglevel", "error", "-y", "-loop", "1", "-framerate", "24", "-i", visual}
	if !reflect.DeepEqual(runner.calls[0][:len(wantPrefix)], wantPrefix) {
		t.Fatalf("ffmpeg prefix = %#v, want %#v", runner.calls[0][:len(wantPrefix)], wantPrefix)
	}
	for _, arg := range runner.calls[0] {
		if arg == "sh" || arg == "-c" {
			t.Fatalf("unexpected shell argument in %#v", runner.calls[0])
		}
	}
	if got := runner.calls[0][len(runner.calls[0])-1]; got != output {
		t.Fatalf("output arg = %q, want %q", got, output)
	}
}

func TestRenderSingleVisualMP4FailsClosedOnUnsupportedOrUnsafeInput(t *testing.T) {
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
	tests := []struct {
		name  string
		input PreparedLocalRenderInput
		want  error
	}{
		{name: "relative visual", input: func() PreparedLocalRenderInput { v := base; v.VisualPath = "visual.ppm"; return v }(), want: ErrInvalidRenderInput},
		{name: "same path", input: func() PreparedLocalRenderInput { v := base; v.OutputPath = v.VisualPath; return v }(), want: ErrInvalidRenderInput},
		{name: "odd canvas", input: func() PreparedLocalRenderInput { v := base; v.Width = 321; return v }(), want: ErrInvalidRenderInput},
		{name: "unapproved fps", input: func() PreparedLocalRenderInput { v := base; v.FrameRate = 60; return v }(), want: ErrInvalidRenderInput},
		{name: "duration over budget", input: func() PreparedLocalRenderInput { v := base; v.DurationMS = MaxLocalRenderDuration.Milliseconds() + 1; return v }(), want: ErrInvalidRenderInput},
		{name: "unsupported fit", input: func() PreparedLocalRenderInput { v := base; v.Fit = sceneeditor.FitCover; return v }(), want: ErrUnsupportedRenderSemantics},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RenderSingleVisualMP4(context.Background(), &renderRunnerStub{}, testLocalProfile(), tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRenderSingleVisualMP4RealFFmpegSmoke(t *testing.T) {
	if os.Getenv("SYNVIDEO_TEST_FFMPEG") != "1" {
		t.Skip("set SYNVIDEO_TEST_FFMPEG=1 to run real FFmpeg smoke")
	}
	ctx := context.Background()
	profile, err := ProbeLocalFFmpegProfile(ctx, nil)
	if err != nil {
		t.Fatalf("ProbeLocalFFmpegProfile() error = %v", err)
	}
	dir := t.TempDir()
	visual := filepath.Join(dir, "visual.ppm")
	output := filepath.Join(dir, "render.mp4")
	ppm := []byte("P3\n2 2\n255\n255 0 0  0 255 0\n0 0 255  255 255 255\n")
	if err := os.WriteFile(visual, ppm, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := RenderSingleVisualMP4(ctx, nil, profile, PreparedLocalRenderInput{
		VisualPath: visual,
		OutputPath: output,
		Width:      320,
		Height:     180,
		FrameRate:  24,
		DurationMS: 500,
		Fit:        sceneeditor.FitContain,
	})
	if err != nil {
		t.Fatalf("RenderSingleVisualMP4(real) error = %v", err)
	}
	if metadata.ByteSize <= 0 || metadata.Width != 320 || metadata.Height != 180 || metadata.VideoCodec != "h264" || metadata.PixelFormat != "yuv420p" {
		t.Fatalf("real metadata = %#v", metadata)
	}
	if metadata.DurationMS < 400 || metadata.DurationMS > 800 {
		t.Fatalf("duration = %dms, expected near 500ms", metadata.DurationMS)
	}
}
