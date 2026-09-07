package renderexport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	FFmpegBinary       = "ffmpeg"
	FFprobeBinary      = "ffprobe"
	FFmpegProbeTimeout = 5 * time.Second
	FFmpegProbeLogMax  = 64 * 1024
)

var ErrUnsupportedFFmpeg = errors.New("render export FFmpeg build does not support the local profile")

type FFmpegProfile struct {
	ID           string
	VideoEncoder string
	AudioEncoder string
	PixelFormat  string
	Container    string
	VersionLine  string
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var output bytes.Buffer
	cmd.Stdout = &boundedBuffer{buf: &output, max: FFmpegProbeLogMax}
	cmd.Stderr = &boundedBuffer{buf: &output, max: FFmpegProbeLogMax}
	err := cmd.Run()
	return output.Bytes(), err
}

type boundedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.max - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return original, nil
}

func ProbeLocalFFmpegProfile(ctx context.Context, runner CommandRunner) (FFmpegProfile, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	probeCtx, cancel := context.WithTimeout(ctx, FFmpegProbeTimeout)
	defer cancel()

	versionOutput, err := runner.Run(probeCtx, FFmpegBinary, "-hide_banner", "-version")
	if err != nil {
		return FFmpegProfile{}, fmt.Errorf("%w: ffmpeg version probe: %v", ErrUnsupportedFFmpeg, err)
	}
	encodersOutput, err := runner.Run(probeCtx, FFmpegBinary, "-hide_banner", "-encoders")
	if err != nil {
		return FFmpegProfile{}, fmt.Errorf("%w: ffmpeg encoder probe: %v", ErrUnsupportedFFmpeg, err)
	}
	if !hasEncoder(encodersOutput, "libx264") || !hasEncoder(encodersOutput, "aac") {
		return FFmpegProfile{}, ErrUnsupportedFFmpeg
	}

	versionLine := firstNonEmptyLine(string(versionOutput))
	if versionLine == "" {
		return FFmpegProfile{}, ErrUnsupportedFFmpeg
	}

	return FFmpegProfile{
		ID:           LocalProfileID,
		VideoEncoder: "libx264",
		AudioEncoder: "aac",
		PixelFormat:  "yuv420p",
		Container:    "mp4",
		VersionLine:  versionLine,
	}, nil
}

func hasEncoder(output []byte, encoder string) bool {
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == encoder {
			return true
		}
	}
	return false
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
