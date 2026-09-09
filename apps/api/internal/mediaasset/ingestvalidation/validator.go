package ingestvalidation

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
)

type Kind string

const (
	KindImage Kind = "image"
	KindVideo Kind = "video"
	KindAudio Kind = "audio"
)

type DeclaredInput struct {
	Kind     Kind
	MimeType string
}

type VerifiedContent struct {
	Kind     Kind
	MimeType string
}

type Validator struct {
	runner     CommandRunner
	probeSlots chan struct{}
}

func NewValidator(runner CommandRunner) *Validator {
	return NewValidatorWithProbeLimit(runner, DefaultMaxConcurrentProbes)
}

func NewValidatorWithProbeLimit(runner CommandRunner, limit int) *Validator {
	if limit < 1 {
		limit = DefaultMaxConcurrentProbes
	}
	return &Validator{
		runner:     runner,
		probeSlots: make(chan struct{}, limit),
	}
}

func (v *Validator) ValidateFile(ctx context.Context, path string, declared DeclaredInput) (VerifiedContent, error) {
	if err := ctx.Err(); err != nil {
		return VerifiedContent{}, err
	}
	declared = normalizeDeclared(declared)
	if declared.Kind == "" {
		return VerifiedContent{}, ErrMalformed
	}

	switch declared.Kind {
	case KindImage:
		return v.validateImageFile(path, declared)
	case KindVideo, KindAudio:
		return v.validateProbeFile(ctx, path, declared)
	default:
		return VerifiedContent{}, ErrUnsupported
	}
}

func (v *Validator) ValidateReader(ctx context.Context, reader io.Reader, declared DeclaredInput, maxBytes int64) (VerifiedContent, []byte, error) {
	path, cleanup, err := materializeToTemp(ctx, reader, maxBytes)
	if err != nil {
		return VerifiedContent{}, nil, err
	}
	defer cleanup()

	verified, validateErr := v.ValidateFile(ctx, path, declared)
	if validateErr != nil {
		return VerifiedContent{}, nil, validateErr
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return VerifiedContent{}, nil, ErrInfrastructure
	}
	return verified, data, nil
}

func (v *Validator) validateImageFile(path string, declared DeclaredInput) (VerifiedContent, error) {
	file, err := os.Open(path)
	if err != nil {
		return VerifiedContent{}, ErrInfrastructure
	}
	defer file.Close()

	header, err := readBoundedHeader(file)
	if err != nil {
		return VerifiedContent{}, ErrMalformed
	}
	facts, err := detectImage(header)
	if err != nil {
		return errResult(err)
	}
	verified := VerifiedContent{Kind: KindImage, MimeType: facts.mimeType}
	return reconcileDeclared(verified, declared)
}

func (v *Validator) validateProbeFile(ctx context.Context, path string, declared DeclaredInput) (VerifiedContent, error) {
	if err := acquireProbeSlot(ctx, v.probeSlots); err != nil {
		return VerifiedContent{}, err
	}
	defer releaseProbeSlot(v.probeSlots)

	result, err := probeMediaFile(ctx, v.runner, path)
	if err != nil {
		return errResult(err)
	}
	verified, err := factsFromProbe(result, declared.Kind)
	if err != nil {
		return errResult(err)
	}
	return reconcileDeclared(verified, declared)
}

func factsFromProbe(result ffprobeResult, declaredKind Kind) (VerifiedContent, error) {
	formatNames := splitFormatNames(result.Format.FormatName)
	hasVideo := false
	hasAudio := false
	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			hasVideo = true
		case "audio":
			hasAudio = true
		}
	}

	switch declaredKind {
	case KindVideo:
		if !hasVideo {
			return VerifiedContent{}, ErrMalformed
		}
		mimeType, ok := matchVideoFormat(formatNames)
		if !ok {
			return VerifiedContent{}, ErrUnsupported
		}
		return VerifiedContent{Kind: KindVideo, MimeType: mimeType}, nil
	case KindAudio:
		if !hasAudio {
			return VerifiedContent{}, ErrMalformed
		}
		mimeType, ok := matchAudioFormat(formatNames, hasVideo)
		if !ok {
			return VerifiedContent{}, ErrUnsupported
		}
		return VerifiedContent{Kind: KindAudio, MimeType: mimeType}, nil
	default:
		return VerifiedContent{}, ErrUnsupported
	}
}

func matchVideoFormat(formatNames []string) (string, bool) {
	set := make(map[string]bool, len(formatNames))
	for _, name := range formatNames {
		set[name] = true
	}
	if set["webm"] || set["matroska"] {
		return "video/webm", true
	}
	if set["mp4"] || set["m4v"] || set["isom"] {
		return "video/mp4", true
	}
	if set["mov"] || set["qt"] {
		return "video/quicktime", true
	}
	return "", false
}

func matchAudioFormat(formatNames []string, hasVideo bool) (string, bool) {
	if hasVideo {
		return "", false
	}
	for _, name := range formatNames {
		switch name {
		case "aac":
			return "audio/aac", true
		case "flac":
			return "audio/flac", true
		case "mp3", "mp2", "mpeg":
			return "audio/mpeg", true
		case "ogg":
			return "audio/ogg", true
		case "wav":
			return "audio/wav", true
		case "mp4", "m4a", "isom", "mov":
			return "audio/mp4", true
		}
	}
	return "", false
}

func splitFormatNames(raw string) []string {
	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			names = append(names, part)
		}
	}
	return names
}

func reconcileDeclared(verified VerifiedContent, declared DeclaredInput) (VerifiedContent, error) {
	if declared.Kind != "" && declared.Kind != verified.Kind {
		return VerifiedContent{}, ErrMismatch
	}
	if declared.MimeType != "" && !mimeCompatible(declared.MimeType, verified.MimeType) {
		return VerifiedContent{}, ErrMismatch
	}
	return verified, nil
}

func mimeCompatible(declared, verified string) bool {
	declared = strings.ToLower(strings.TrimSpace(declared))
	verified = strings.ToLower(strings.TrimSpace(verified))
	if declared == verified {
		return true
	}
	if declared == "audio/x-wav" && verified == "audio/wav" {
		return true
	}
	if (declared == "video/mp4" && verified == "video/quicktime") || (declared == "video/quicktime" && verified == "video/mp4") {
		return true
	}
	return false
}

func normalizeDeclared(declared DeclaredInput) DeclaredInput {
	declared.MimeType = strings.ToLower(strings.TrimSpace(declared.MimeType))
	return declared
}

func errResult(err error) (VerifiedContent, error) {
	switch {
	case errors.Is(err, ErrUnsupported), errors.Is(err, ErrMalformed), errors.Is(err, ErrMismatch),
		errors.Is(err, ErrTimeout), errors.Is(err, ErrInfrastructure):
		return VerifiedContent{}, err
	default:
		return VerifiedContent{}, ErrInfrastructure
	}
}

func acquireProbeSlot(ctx context.Context, slots chan struct{}) error {
	select {
	case slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseProbeSlot(slots chan struct{}) {
	select {
	case <-slots:
	default:
	}
}

func materializeToTemp(ctx context.Context, reader io.Reader, maxBytes int64) (string, func(), error) {
	file, err := os.CreateTemp("", "synvideo-ingest-*")
	if err != nil {
		return "", nil, ErrInfrastructure
	}
	cleanup := func() { _ = os.Remove(file.Name()); _ = file.Close() }

	var limited io.Reader = reader
	if maxBytes > 0 {
		limited = &maxBytesReader{reader: reader, max: maxBytes}
	}
	written, err := copyWithContext(ctx, file, limited)
	if err != nil {
		cleanup()
		if errors.Is(err, ErrTooLarge) {
			return "", nil, ErrTooLarge
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", nil, err
		}
		return "", nil, ErrMalformed
	}
	if written == 0 {
		cleanup()
		return "", nil, ErrMalformed
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, ErrInfrastructure
	}
	return file.Name(), cleanup, nil
}

type maxBytesReader struct {
	reader   io.Reader
	max      int64
	read     int64
	exceeded bool
}

func (r *maxBytesReader) Read(p []byte) (int, error) {
	if r.read > r.max {
		r.exceeded = true
		return 0, ErrTooLarge
	}
	remaining := r.max - r.read
	if remaining == 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			return 0, ErrTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := r.reader.Read(p)
	r.read += int64(n)
	if r.read > r.max {
		r.exceeded = true
		return n, ErrTooLarge
	}
	return n, err
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			m, writeErr := dst.Write(buf[:n])
			written += int64(m)
			if writeErr != nil {
				return written, writeErr
			}
			if m != n {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}
