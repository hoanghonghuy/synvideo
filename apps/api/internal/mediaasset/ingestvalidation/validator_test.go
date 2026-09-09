package ingestvalidation_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
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
		{"avif", minimalAVIF, "image/avif"},
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
		"truncated-png": minimalPNG[:16],
		"garbage":       []byte("not-an-image"),
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

func TestValidateImageRejectsHeaderOnlyTruncatedPayloads(t *testing.T) {
	validator := ingestvalidation.NewValidator(nil)
	cases := map[string]struct {
		data []byte
		mime string
	}{
		"jpeg-app-only":      {data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x02}, mime: "image/jpeg"},
		"jpeg-missing-eoi":   {data: minimalJPEG[:len(minimalJPEG)-2], mime: "image/jpeg"},
		"webp-fourcc-only":   {data: minimalWebP[:20], mime: "image/webp"},
		"webp-truncated-vp8": {data: minimalWebP[:len(minimalWebP)-10], mime: "image/webp"},
		"avif-ftyp-only":     {data: minimalAVIF[:32], mime: "image/avif"},
		"png-missing-iend":   {data: minimalPNG[:len(minimalPNG)-12], mime: "image/png"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeTempFile(t, tc.data)
			_, err := validator.ValidateFile(context.Background(), path, ingestvalidation.DeclaredInput{
				Kind: ingestvalidation.KindImage, MimeType: tc.mime,
			})
			if err == nil || errors.Is(err, ingestvalidation.ErrMismatch) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateProbeRejectsSpoofedMIME(t *testing.T) {
	requireFFprobe(t)
	validator := ingestvalidation.NewValidator(ingestvalidation.ExecCommandRunner{})
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

type saturatedProbeRunner struct {
	mu          sync.Mutex
	active      int
	maxActive   int
	invocations int
	release     chan struct{}
	probeOutput []byte
}

func (r *saturatedProbeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	r.active++
	r.invocations++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()

	select {
	case <-r.release:
	case <-ctx.Done():
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
		return nil, ctx.Err()
	}

	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return r.probeOutput, nil
}

func TestConcurrentProbeAdmissionIsBounded(t *testing.T) {
	const limit = 3
	runner := &saturatedProbeRunner{
		release:     make(chan struct{}),
		probeOutput: []byte(`{"format":{"format_name":"wav"},"streams":[{"codec_type":"audio"}]}`),
	}
	validator := ingestvalidation.NewValidatorWithProbeLimit(runner, limit)
	path := writeTempFile(t, []byte("fixture"))
	declared := ingestvalidation.DeclaredInput{Kind: ingestvalidation.KindAudio, MimeType: "audio/wav"}

	var wg sync.WaitGroup
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := validator.ValidateFile(context.Background(), path, declared)
			if err != nil {
				t.Errorf("saturating probe failed: %v", err)
			}
		}()
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		runner.mu.Lock()
		active := runner.active
		runner.mu.Unlock()
		if active == limit {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	runner.mu.Lock()
	if runner.maxActive != limit || runner.active != limit {
		t.Fatalf("probe slots not saturated: active=%d maxActive=%d limit=%d", runner.active, runner.maxActive, limit)
	}
	invocationsBeforeWaiter := runner.invocations
	runner.mu.Unlock()

	waiterCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := validator.ValidateFile(waiterCtx, path, declared)
	if err == nil {
		t.Fatal("expected blocked waiter to fail without starting another probe")
	}

	runner.mu.Lock()
	if runner.maxActive > limit {
		t.Fatalf("admission exceeded limit: maxActive=%d limit=%d", runner.maxActive, limit)
	}
	if runner.invocations != invocationsBeforeWaiter {
		t.Fatalf("waiter spawned extra probe: invocations=%d before=%d", runner.invocations, invocationsBeforeWaiter)
	}
	runner.mu.Unlock()

	close(runner.release)
	wg.Wait()
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
