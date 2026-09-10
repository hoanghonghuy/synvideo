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
)

type Policy struct {
	MaxRequests  int
	Window       time.Duration
	MaxInFlight  int
}

func (p Policy) Valid() bool {
	return p.MaxRequests > 0 && p.Window > 0 && p.MaxInFlight > 0
}

type Reservation struct {
	OwnerID    uuid.UUID
	ProjectID  uuid.UUID
	Operation  Operation
	RequestID  uuid.UUID
	ReservedAt time.Time
	Replay     bool
}

type Guard interface {
	Reserve(context.Context, uuid.UUID, uuid.UUID, Operation, uuid.UUID, Policy) (Reservation, error)
	Release(context.Context, Reservation) error
}
