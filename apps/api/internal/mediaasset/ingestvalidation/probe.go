package ingestvalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

const (
	FFprobeBinary              = "ffprobe"
	ProbeTimeout               = 5 * time.Second
	ProbeLogMaxBytes           = 64 * 1024
	DefaultMaxConcurrentProbes = 8
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var output bytes.Buffer
	cmd.Stdout = &boundedBuffer{buf: &output, max: ProbeLogMaxBytes}
	cmd.Stderr = &boundedBuffer{buf: &output, max: ProbeLogMaxBytes}
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

type ffprobeResult struct {
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
}

type ffprobeStream struct {
	CodecType string `json:"codec_type"`
}

func probeMediaFile(ctx context.Context, runner CommandRunner, path string) (ffprobeResult, error) {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	probeCtx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	output, err := runner.Run(probeCtx, FFprobeBinary,
		"-hide_banner",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	if err != nil {
		if probeCtx.Err() != nil {
			return ffprobeResult{}, ErrTimeout
		}
		return ffprobeResult{}, ErrInfrastructure
	}

	var result ffprobeResult
	if err := json.Unmarshal(output, &result); err != nil || strings.TrimSpace(result.Format.FormatName) == "" {
		return ffprobeResult{}, ErrMalformed
	}
	return result, nil
}
