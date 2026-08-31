package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

func TestJobValidation(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	dedupe := "dedupe-1"

	t.Run("valid enqueue input", func(t *testing.T) {
		input := jobs.EnqueueInput{
			OwnerID:     ownerID,
			ProjectID:   &projectID,
			Kind:        "ai_proposal_generation",
			DedupeKey:   &dedupe,
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{"prompt":"test"}`),
		}
		if err := input.Validate(); err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
	})

	t.Run("missing owner_id", func(t *testing.T) {
		input := jobs.EnqueueInput{
			Kind:        "ai_proposal_generation",
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{}`),
		}
		if err := input.Validate(); !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("empty or invalid kind", func(t *testing.T) {
		input := jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "",
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{}`),
		}
		if err := input.Validate(); !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}

		longKind := string(make([]byte, 101))
		input.Kind = longKind
		if err := input.Validate(); !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput for long kind, got %v", err)
		}
	})

	t.Run("invalid max attempts", func(t *testing.T) {
		input := jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "ai_proposal_generation",
			MaxAttempts: 0,
			Payload:     json.RawMessage(`{}`),
		}
		if err := input.Validate(); !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("invalid payload json", func(t *testing.T) {
		input := jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "ai_proposal_generation",
			MaxAttempts: 3,
			Payload:     json.RawMessage(`not valid json`),
		}
		if err := input.Validate(); !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput for invalid json, got %v", err)
		}
	})

	t.Run("payload must be a json object", func(t *testing.T) {
		for _, payload := range []string{`[]`, `null`, `"text"`, `42`} {
			input := jobs.EnqueueInput{
				OwnerID:     ownerID,
				Kind:        "ai_proposal_generation",
				MaxAttempts: 3,
				Payload:     json.RawMessage(payload),
			}
			if err := input.Validate(); !errors.Is(err, jobs.ErrInvalidInput) {
				t.Errorf("payload %s: expected ErrInvalidInput, got %v", payload, err)
			}
		}
	})

	t.Run("dedupe key length validation", func(t *testing.T) {
		longKey := string(make([]byte, 201))
		input := jobs.EnqueueInput{
			OwnerID:     ownerID,
			Kind:        "ai_proposal_generation",
			DedupeKey:   &longKey,
			MaxAttempts: 3,
			Payload:     json.RawMessage(`{}`),
		}
		if err := input.Validate(); !errors.Is(err, jobs.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput for long dedupe key, got %v", err)
		}
	})
}

func TestRegistry(t *testing.T) {
	registry := jobs.NewRegistry()

	handler := jobs.HandlerFunc(func(ctx context.Context, job jobs.Job) (json.RawMessage, error) {
		return json.RawMessage(`{"status":"ok"}`), nil
	})

	if err := registry.Register("test_kind", handler); err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Registering same kind again should error
	if err := registry.Register("test_kind", handler); !errors.Is(err, jobs.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput registering duplicate kind, got %v", err)
	}

	h, ok := registry.Get("test_kind")
	if !ok || h == nil {
		t.Fatalf("expected handler for test_kind")
	}

	_, ok = registry.Get("unknown_kind")
	if ok {
		t.Fatalf("expected not ok for unknown_kind")
	}

	kinds := registry.Kinds()
	if len(kinds) != 1 || kinds[0] != "test_kind" {
		t.Fatalf("unexpected kinds: %v", kinds)
	}
}
