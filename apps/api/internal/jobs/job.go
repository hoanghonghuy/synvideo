package jobs

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
)

var (
	ErrInvalidInput   = errors.New("jobs: invalid input")
	ErrJobNotFound    = errors.New("jobs: job not found")
	ErrDuplicateJob   = errors.New("jobs: duplicate job with same dedupe key")
	ErrStaleLease     = errors.New("jobs: lease expired or owned by another worker attempt")
	ErrJobNotRunning  = errors.New("jobs: job is not in running state")
	ErrJobTerminal    = errors.New("jobs: job is in terminal state")
	ErrNoJobAvailable = errors.New("jobs: no eligible job available")
	ErrUnknownJobKind = errors.New("jobs: unknown job kind")
)

type Job struct {
	ID          uuid.UUID       `json:"id"`
	OwnerID     uuid.UUID       `json:"owner_id"`
	ProjectID   *uuid.UUID      `json:"project_id,omitempty"`
	Kind        string          `json:"kind"`
	DedupeKey   *string         `json:"dedupe_key,omitempty"`
	State       State           `json:"state"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	AvailableAt time.Time       `json:"available_at"`
	LeaseToken  *uuid.UUID      `json:"lease_token,omitempty"`
	LeaseUntil  *time.Time      `json:"lease_until,omitempty"`
	Payload     json.RawMessage `json:"payload"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   *string         `json:"error_code,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}

type EnqueueInput struct {
	ID          uuid.UUID       `json:"id"`
	OwnerID     uuid.UUID       `json:"owner_id"`
	ProjectID   *uuid.UUID      `json:"project_id,omitempty"`
	Kind        string          `json:"kind"`
	DedupeKey   *string         `json:"dedupe_key,omitempty"`
	MaxAttempts int             `json:"max_attempts"`
	AvailableAt *time.Time      `json:"available_at,omitempty"`
	Payload     json.RawMessage `json:"payload"`
}

func (in EnqueueInput) Validate() error {
	if in.OwnerID == uuid.Nil {
		return errors.Join(ErrInvalidInput, errors.New("owner_id is required"))
	}
	trimmedKind := strings.TrimSpace(in.Kind)
	if trimmedKind == "" || len(trimmedKind) > 100 {
		return errors.Join(ErrInvalidInput, errors.New("kind must be between 1 and 100 characters"))
	}
	if in.DedupeKey != nil {
		trimmedDedupe := strings.TrimSpace(*in.DedupeKey)
		if trimmedDedupe == "" || len(trimmedDedupe) > 200 {
			return errors.Join(ErrInvalidInput, errors.New("dedupe_key must be between 1 and 200 characters"))
		}
	}
	if in.MaxAttempts < 0 {
		return errors.Join(ErrInvalidInput, errors.New("max_attempts cannot be negative"))
	}
	if in.MaxAttempts == 0 {
		// EnqueueInput with MaxAttempts=0 is either uninitialized or explicitly invalid
		// If Caller provides 0, Validate checks if it's invalid unless defaulted before validation.
		return errors.Join(ErrInvalidInput, errors.New("max_attempts must be >= 1"))
	}
	if len(in.Payload) > 0 && !json.Valid(in.Payload) {
		return errors.Join(ErrInvalidInput, errors.New("payload must be valid json"))
	}
	return nil
}

type ClaimOptions struct {
	Kinds         []string
	LeaseDuration time.Duration
}

type RetryableJobError struct {
	Code       string
	Err        error
	RetryAfter *time.Duration
}

func (e *RetryableJobError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *RetryableJobError) Unwrap() error {
	return e.Err
}

func NewRetryableError(code string, err error, retryAfter *time.Duration) *RetryableJobError {
	return &RetryableJobError{
		Code:       code,
		Err:        err,
		RetryAfter: retryAfter,
	}
}

type TerminalJobError struct {
	Code string
	Err  error
}

func (e *TerminalJobError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *TerminalJobError) Unwrap() error {
	return e.Err
}

func NewTerminalError(code string, err error) *TerminalJobError {
	return &TerminalJobError{
		Code: code,
		Err:  err,
	}
}
