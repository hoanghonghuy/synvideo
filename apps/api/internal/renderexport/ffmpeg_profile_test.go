package renderexport

import (
	"context"
	"errors"
	"testing"
)

type commandResponse struct {
	output []byte
	err    error
}

type commandRunnerStub struct {
	responses []commandResponse
	calls     [][]string
}

func (r *commandRunnerStub) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(r.responses) == 0 {
		return nil, errors.New("unexpected command")
	}
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response.output, response.err
}

func TestProbeLocalFFmpegProfileFreezesSupportedSoftwareProfile(t *testing.T) {
	runner := &commandRunnerStub{responses: []commandResponse{
		{output: []byte("ffmpeg version 7.1.1 Copyright\nconfiguration: --enable-gpl --enable-libx264\n")},
		{output: []byte("Encoders:\n V....D libx264              H.264 / AVC\n A..... aac                  AAC\n")},
	}}

	profile, err := ProbeLocalFFmpegProfile(context.Background(), runner)
	if err != nil {
		t.Fatalf("ProbeLocalFFmpegProfile() error = %v", err)
	}
	if profile.ID != FFmpegLocalProfileID || profile.VideoEncoder != "libx264" || profile.AudioEncoder != "aac" || profile.PixelFormat != "yuv420p" || profile.Container != "mp4" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.VersionLine != "ffmpeg version 7.1.1 Copyright" {
		t.Fatalf("version line = %q", profile.VersionLine)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(runner.calls))
	}
}

func TestProbeLocalFFmpegProfileFailsClosedWithoutRequiredEncoder(t *testing.T) {
	runner := &commandRunnerStub{responses: []commandResponse{
		{output: []byte("ffmpeg version 7.1.1\n")},
		{output: []byte("Encoders:\n V....D mpeg4 MPEG-4\n A..... aac AAC\n")},
	}}

	_, err := ProbeLocalFFmpegProfile(context.Background(), runner)
	if !errors.Is(err, ErrUnsupportedFFmpeg) {
		t.Fatalf("error = %v, want ErrUnsupportedFFmpeg", err)
	}
}

func TestProbeLocalFFmpegProfileDoesNotExposeClientControlledArguments(t *testing.T) {
	runner := &commandRunnerStub{responses: []commandResponse{
		{output: []byte("ffmpeg version 7.1.1\n")},
		{output: []byte(" V....D libx264 H.264\n A..... aac AAC\n")},
	}}

	_, err := ProbeLocalFFmpegProfile(context.Background(), runner)
	if err != nil {
		t.Fatalf("ProbeLocalFFmpegProfile() error = %v", err)
	}
	want := [][]string{
		{FFmpegBinary, "-hide_banner", "-version"},
		{FFmpegBinary, "-hide_banner", "-encoders"},
	}
	if len(runner.calls) != len(want) {
		t.Fatalf("calls = %#v", runner.calls)
	}
	for i := range want {
		if len(runner.calls[i]) != len(want[i]) {
			t.Fatalf("call %d = %#v", i, runner.calls[i])
		}
		for j := range want[i] {
			if runner.calls[i][j] != want[i][j] {
				t.Fatalf("call %d = %#v, want %#v", i, runner.calls[i], want[i])
			}
		}
	}
}
