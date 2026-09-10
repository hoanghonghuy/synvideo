package paidgeneration

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Operation string

const (
	OperationImage     Operation = "image"
	OperationNarration Operation = "narration"
	OperationVideo     Operation = "video"
)

var (
	ErrQuotaExceeded       = errors.New("paid generation quota exceeded")
	ErrConcurrencyExceeded = errors.New("paid generation concurrency exceeded")
	ErrGuardUnavailable    = errors.New("paid generation guard unavailable")
	ErrLeaseLost           = errors.New("paid generation reservation lease lost")
)

// RetryableLimitError preserves the stable sentinel used by handlers while
// carrying a deterministic backoff hint to the generic job executor. The hint
// is intentionally conservative: callers may retry later, but never earlier
// than the guard's own policy window/lease budget suggests.
type RetryableLimitError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *RetryableLimitError) Error() string {
	if e == nil || e.Err == nil {
		return "paid generation limit exceeded"
	}
	return e.Err.Error()
}

func (e *RetryableLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *RetryableLimitError) RetryAfterDuration() time.Duration {
	if e == nil {
		return 0
	}
	return e.RetryAfter
}

func withPolicyRetryHint(err error, policy Policy) error {
	switch {
	case errors.Is(err, ErrQuotaExceeded):
		return &RetryableLimitError{Err: err, RetryAfter: policy.Window}
	case errors.Is(err, ErrConcurrencyExceeded):
		return &RetryableLimitError{Err: err, RetryAfter: policy.LeaseDuration}
	default:
		return err
	}
}

type Policy struct {
	MaxRequests   int
	Window        time.Duration
	MaxInFlight   int
	LeaseDuration time.Duration
}

func (p Policy) Valid() bool {
	return p.MaxRequests > 0 && p.Window > 0 && p.MaxInFlight > 0 && p.LeaseDuration > 0
}

type Reservation struct {
	OwnerID        uuid.UUID
	ProjectID      uuid.UUID
	Operation      Operation
	RequestID      uuid.UUID
	ReservedAt     time.Time
	LeaseToken     uuid.UUID
	LeaseExpiresAt time.Time
	Replay         bool
	Recovered      bool
}

type Guard interface {
	Reserve(context.Context, uuid.UUID, uuid.UUID, Operation, uuid.UUID, Policy) (Reservation, error)
	Renew(context.Context, Reservation, time.Duration) (Reservation, error)
	Release(context.Context, Reservation) error
}
