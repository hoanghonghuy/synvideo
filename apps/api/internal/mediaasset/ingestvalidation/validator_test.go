package ingestvalidation_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset/ingestvalidation"
)

type commandRunnerStub struct {
	responses []struct {
		output []byte
		err    error
	}
	calls [][]string
	delay time.Duration
}

func (r *commandRunnerStub) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if len(r.responses) == 0 {
		return nil, errors.New("unexpected command")
	}
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response.output, response.err
}

func TestValidateImageFamiliesAcceptValidFixtures(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	cases := []struct {
		name string
		data []byte
		mime string
	}{
		{"png", minimalPNG, "image/png"},
		{"jpeg", minimalJPEG, "image/jpeg"},
		{"gif", minimalGIF, "image/gif"},
		{"webp", minimalWebP, "image/webp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempFile(t, tc.data)
			verified, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
				Kind: ingestvalidation.KindImage, MimeType: tc.mime,
			})
			if err != nil {
				t.Fatalf("ValidateFile() error = %v", err)
			}
			if verified.Kind != ingestvalidation.KindImage || verified.MimeType != tc.mime {
				t.Fatalf("verified = %#v", verified)
			}
		})
	}
}

func TestValidateImageRejectsSpoofedMIME(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	path := writeTempFile(t, minimalPNG)
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindImage, MimeType: "image/jpeg",
	})
	if !errors.Is(err, ingestvalidation.ErrMismatch) {
		t.Fatalf("error = %v, want ErrMismatch", err)
	}
}

func TestValidateImageRejectsMalformedAndTruncated(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	for name, data := range map[string][]byte{
		"truncated": minimalPNG[:16],
		"garbage":   []byte("not-an-image"),
	} {
		t.Run(name, func(t *testing.T) {
			path := writeTempFile(t, data)
			_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
				Kind: ingestvalidation.KindImage, MimeType: "image/png",
			})
			if err == nil || errors.Is(err, ingestvalidation.ErrMismatch) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateProbeFamiliesAcceptValidFixtures(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	cases := []struct {
		name string
		path string
		kind ingestvalidation.Kind
		mime string
	}{
		{"mp4", ffmpegFixture(t, ".mp4", "-f", "lavfi", "-i", "color=c=red:s=16x16:d=0.1", "-c:v", "libx264", "-pix_fmt", "yuv420p"), ingestvalidation.KindVideo, "video/mp4"},
		{"webm", ffmpegFixture(t, ".webm", "-f", "lavfi", "-i", "color=c=red:s=16x16:d=0.1", "-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p"), ingestvalidation.KindVideo, "video/webm"},
		{"wav", ffmpegFixture(t, ".wav", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1"), ingestvalidation.KindAudio, "audio/wav"},
		{"mp3", ffmpegFixture(t, ".mp3", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "libmp3lame"), ingestvalidation.KindAudio, "audio/mpeg"},
		{"flac", ffmpegFixture(t, ".flac", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "flac"), ingestvalidation.KindAudio, "audio/flac"},
		{"ogg", ffmpegFixture(t, ".ogg", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "libvorbis"), ingestvalidation.KindAudio, "audio/ogg"},
		{"aac", ffmpegFixture(t, ".aac", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "aac"), ingestvalidation.KindAudio, "audio/aac"},
		{"m4a", ffmpegFixture(t, ".m4a", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "aac"), ingestvalidation.KindAudio, "audio/mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verified, err := validator.ValidateFile(context.Background(), tc.path, ingestvalidation.DeclaredInput{
				Kind: tc.kind, MimeType: tc.mime,
			})
			if err != nil {
				t.Fatalf("ValidateFile() error = %v", err)
			}
			if verified.Kind != tc.kind {
				t.Fatalf("verified kind = %q", verified.Kind)
			}
		})
	}
}

func TestValidateProbeRejectsSpoofedMIME(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	path := ffmpegFixture(t, ".wav", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1")
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindAudio, MimeType: "audio/flac",
	})
	if !errors.Is(err, ingestvalidation.ErrMismatch) {
		t.Fatalf("error = %v, want ErrMismatch", err)
	}
}

func TestValidateProbeHonorsCancellationAndTimeout(t *testing.T) {
	runner := &commandRunnerStub{delay: 200 * time.Millisecond}
	validator := ingestvalidation.NewValidator(runner)
	path := writeTempFile(t, []byte("not media"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := validator.ValidateFile(ctx, path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindVideo, MimeType: "video/mp4",
	})
	if !errors.Is(err, ingestvalidation.ErrTimeout) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateProbeUsesServerOwnedArgumentsOnly(t *testing.T) {
	runner := &commandRunnerStub{responses: []struct {
		output []byte
		err    error
	}{{output: []byte(`{"format":{"format_name":"mp4"},"streams":[{"codec_type":"video"}]}`)}}}
	validator := ingestvalidation.NewValidator(runner)
	path := writeTempFile(t, []byte("fixture"))
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindVideo, MimeType: "video/mp4",
	})
	if err != nil {
		t.Fatalf("ValidateFile() error = %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	call := runner.calls[0]
	if call[0] != ingestvalidation.FFprobeBinary || call[len(call)-1] != path {
		t.Fatalf("unexpected ffprobe invocation: %#v", call)
	}
	for _, arg := range call {
		if strings.Contains(arg, ";") || strings.Contains(arg, "|") {
			t.Fatalf("shell metacharacter in args: %#v", call)
		}
	}
}

func TestValidateReaderEnforcesByteBudget(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	_, _, err := validator.ValidateReader(context.Background(), bytes.NewReader(minimalPNG), ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindImage, MimeType: "image/png",
	}, int64(len(minimalPNG)-1))
	if err == nil {
		t.Fatal("expected byte budget failure")
	}
}

func TestProbeOutputBudgetIsBounded(t *testing.T) {
	runner := &commandRunnerStub{responses: []struct {
		output []byte
		err    error
	}{{output: bytes.Repeat([]byte("x"), ingestvalidation.ProbeLogMaxBytes+1024)}}}
	validator := ingestvalidation.NewValidator(runner)
	path := writeTempFile(t, []byte("fixture"))
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindVideo, MimeType: "video/mp4",
	})
	if !errors.Is(err, ingestvalidation.ErrMalformed) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateReaderReturnsMaterializedBytes(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	verified, data, err := validator.ValidateReader(context.Background(), bytes.NewReader(minimalPNG), ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindImage, MimeType: "image/png",
	}, int64(len(minimalPNG)+1))
	if err != nil {
		t.Fatalf("ValidateReader() error = %v", err)
	}
	if !bytes.Equal(data, minimalPNG) || verified.MimeType != "image/png" {
		t.Fatalf("verified=%#v data=%d bytes", verified, len(data))
	}
}

func TestValidateFileRejectsEmptyPayload(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	path := writeTempFile(t, nil)
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindImage, MimeType: "image/png",
	})
	if err == nil {
		t.Fatal("expected empty payload rejection")
	}
}

func TestValidateReaderPropagatesContextCancellationDuringMaterialize(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := validator.ValidateReader(ctx, blockingReader{}, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindImage, MimeType: "image/png",
	}, 1024)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) {
	time.Sleep(50 * time.Millisecond)
	return 0, io.ErrNoProgress
}

func TestValidateProbeInfrastructureFailureDoesNotLeakDetails(t *testing.T) {
	runner := &commandRunnerStub{responses: []struct {
		output []byte
		err    error
	}{{err: errors.New("secret ffprobe path /tmp/leak")}}}
	validator := ingestvalidation.NewValidator(runner)
	path := writeTempFile(t, []byte("fixture"))
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindAudio, MimeType: "audio/wav",
	})
	if !errors.Is(err, ingestvalidation.ErrInfrastructure) {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "/tmp/leak") {
		t.Fatalf("leaked infrastructure detail: %v", err)
	}
}

func TestConcurrentProbeAdmissionIsBounded(t *testing.T) {
	runner := &commandRunnerStub{delay: 100 * time.Millisecond, responses: []struct {
		output []byte
		err    error
	}{
		{output: []byte(`{"format":{"format_name":"wav"},"streams":[{"codec_type":"audio"}]}`)},
		{output: []byte(`{"format":{"format_name":"wav"},"streams":[{"codec_type":"audio"}]}`)},
		{output: []byte(`{"format":{"format_name":"wav"},"streams":[{"codec_type":"audio"}]}`)},
	}}
	validator := ingestvalidation.NewValidator(runner)
	path := writeTempFile(t, []byte("fixture"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := validator.ValidateFile(ctx, path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindAudio, MimeType: "audio/wav",
	})
	if err == nil {
		t.Fatal("expected timeout or cancellation under saturated probe slots")
	}
}

func TestAVIFRequiresBrand(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	// ftyp box with non-avif brand only
	data := []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p',
		'j', 'p', 'e', 'g', 0x00, 0x00, 0x00, 0x00,
		'j', 'p', 'e', 'g', 'i', 's', 'o', 'm',
	}
	path := writeTempFile(t, data)
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindImage, MimeType: "image/avif",
	})
	if !errors.Is(err, ingestvalidation.ErrMalformed) && !errors.Is(err, ingestvalidation.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateProbeMalformedJSON(t *testing.T) {
	runner := &commandRunnerStub{responses: []struct {
		output []byte
		err    error
	}{{output: []byte("not-json")}}}
	validator := ingestvalidation.NewValidator(runner)
	path := writeTempFile(t, []byte("fixture"))
	_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindVideo, MimeType: "video/mp4",
	})
	if !errors.Is(err, ingestvalidation.ErrMalformed) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateFileUsesExistingTempPath(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	path := writeTempFile(t, minimalPNG)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty fixture")
	}
	_, err = validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
		Kind: ingestvalidation.KindImage, MimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("ValidateFile() error = %v", err)
	}
}
