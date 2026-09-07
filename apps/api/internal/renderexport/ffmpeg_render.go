package renderexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
)

const (
	LocalRenderTimeout     = 5 * time.Minute
	MaxLocalRenderDuration = 10 * time.Minute
	LocalRenderLogMax      = 128 * 1024
)

var (
	ErrInvalidRenderInput          = errors.New("render export local render input is invalid")
	ErrUnsupportedRenderSemantics = errors.New("render export semantics are unsupported by the local profile")
	ErrInvalidRenderOutput         = errors.New("render export produced invalid output")
)

type PreparedLocalRenderInput struct {
	VisualPath string
	OutputPath string
	Width      int
	Height     int
	FrameRate  int
	DurationMS int64
	Fit        sceneeditor.FitMode
}

type RenderMetadata struct {
	ByteSize         int64
	DurationMS       int64
	Width            int
	Height           int
	VideoCodec       string
	PixelFormat      string
	Container        string
	ToolchainVersion string
}

type RenderProcessRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

type ExecRenderProcessRunner struct{}

func (ExecRenderProcessRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &boundedBuffer{buf: &stdout, max: LocalRenderLogMax}
	cmd.Stderr = &boundedBuffer{buf: &stderr, max: LocalRenderLogMax}
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func RenderSingleVisualMP4(ctx context.Context, runner RenderProcessRunner, profile FFmpegProfile, input PreparedLocalRenderInput) (RenderMetadata, error) {
	if err := validatePreparedLocalRenderInput(profile, input); err != nil {
		return RenderMetadata{}, err
	}
	if runner == nil {
		runner = ExecRenderProcessRunner{}
	}

	renderCtx, cancel := context.WithTimeout(ctx, LocalRenderTimeout)
	defer cancel()

	durationSeconds := strconv.FormatFloat(float64(input.DurationMS)/1000, 'f', 3, 64)
	filter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1",
		input.Width, input.Height, input.Width, input.Height,
	)
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-loop", "1", "-framerate", strconv.Itoa(input.FrameRate), "-i", input.VisualPath,
		"-t", durationSeconds,
		"-vf", filter,
		"-r", strconv.Itoa(input.FrameRate),
		"-c:v", profile.VideoEncoder,
		"-pix_fmt", profile.PixelFormat,
		"-movflags", "+faststart",
		"-an", "-f", profile.Container,
		input.OutputPath,
	}
	_, _, err := runner.Run(renderCtx, FFmpegBinary, args...)
	if err != nil {
		if renderCtx.Err() != nil {
			return RenderMetadata{}, renderCtx.Err()
		}
		return RenderMetadata{}, fmt.Errorf("render local mp4: %w", err)
	}

	stat, err := os.Stat(input.OutputPath)
	if err != nil {
		return RenderMetadata{}, fmt.Errorf("%w: output missing: %v", ErrInvalidRenderOutput, err)
	}
	if !stat.Mode().IsRegular() || stat.Size() <= 0 {
		return RenderMetadata{}, ErrInvalidRenderOutput
	}

	metadata, err := probeRenderedMP4(renderCtx, runner, input.OutputPath)
	if err != nil {
		return RenderMetadata{}, err
	}
	metadata.ByteSize = stat.Size()
	metadata.ToolchainVersion = profile.VersionLine
	if metadata.Width != input.Width || metadata.Height != input.Height || metadata.VideoCodec != "h264" || metadata.PixelFormat != profile.PixelFormat || !strings.Contains(metadata.Container, "mp4") {
		return RenderMetadata{}, ErrInvalidRenderOutput
	}
	return metadata, nil
}

func validatePreparedLocalRenderInput(profile FFmpegProfile, input PreparedLocalRenderInput) error {
	if profile.ID != LocalProfileID || profile.VideoEncoder != "libx264" || profile.AudioEncoder != "aac" || profile.PixelFormat != "yuv420p" || profile.Container != "mp4" || strings.TrimSpace(profile.VersionLine) == "" {
		return ErrUnsupportedFFmpeg
	}
	if input.VisualPath == "" || input.OutputPath == "" || !filepath.IsAbs(input.VisualPath) || !filepath.IsAbs(input.OutputPath) || filepath.Clean(input.VisualPath) == filepath.Clean(input.OutputPath) {
		return ErrInvalidRenderInput
	}
	visualStat, err := os.Stat(input.VisualPath)
	if err != nil || !visualStat.Mode().IsRegular() || visualStat.Size() <= 0 {
		return ErrInvalidRenderInput
	}
	if input.Width < 64 || input.Width > 3840 || input.Height < 64 || input.Height > 2160 || input.Width%2 != 0 || input.Height%2 != 0 {
		return ErrInvalidRenderInput
	}
	if input.FrameRate != 24 && input.FrameRate != 30 {
		return ErrInvalidRenderInput
	}
	if input.DurationMS <= 0 || input.DurationMS > MaxLocalRenderDuration.Milliseconds() {
		return ErrInvalidRenderInput
	}
	if input.Fit != sceneeditor.FitContain {
		return ErrUnsupportedRenderSemantics
	}
	return nil
}

type ffprobeOutput struct {
	Streams []struct {
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		PixelFmt  string `json:"pix_fmt"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
}

func probeRenderedMP4(ctx context.Context, runner RenderProcessRunner, path string) (RenderMetadata, error) {
	stdout, _, err := runner.Run(ctx, FFprobeBinary,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,pix_fmt:format=format_name,duration",
		"-of", "json",
		path,
	)
	if err != nil {
		return RenderMetadata{}, fmt.Errorf("%w: ffprobe failed", ErrInvalidRenderOutput)
	}
	var probe ffprobeOutput
	if err := json.Unmarshal(stdout, &probe); err != nil || len(probe.Streams) != 1 {
		return RenderMetadata{}, ErrInvalidRenderOutput
	}
	durationSeconds, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || durationSeconds <= 0 {
		return RenderMetadata{}, ErrInvalidRenderOutput
	}
	stream := probe.Streams[0]
	return RenderMetadata{
		DurationMS: int64(durationSeconds*1000 + 0.5),
		Width:      stream.Width,
		Height:     stream.Height,
		VideoCodec: stream.CodecName,
		PixelFormat: stream.PixelFmt,
		Container:  probe.Format.FormatName,
	}, nil
}
