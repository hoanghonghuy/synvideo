package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/paidgeneration"
)

type PaidGenerationGuard struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewPaidGenerationGuard(pool *pgxpool.Pool) *PaidGenerationGuard {
	return &PaidGenerationGuard{pool: pool, now: time.Now}
}

func (g *PaidGenerationGuard) Reserve(ctx context.Context, ownerID, projectID uuid.UUID, operation paidgeneration.Operation, requestID uuid.UUID, policy paidgeneration.Policy) (paidgeneration.Reservation, error) {
	if g == nil || g.pool == nil || ownerID == uuid.Nil || projectID == uuid.Nil || requestID == uuid.Nil || !validOperation(operation) || !policy.Valid() {
		return paidgeneration.Reservation{}, paidgeneration.ErrGuardUnavailable
	}

	tx, err := g.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return paidgeneration.Reservation{}, fmt.Errorf("begin paid generation reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockKey := ownerID.String() + ":" + projectID.String() + ":" + string(operation)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return paidgeneration.Reservation{}, fmt.Errorf("lock paid generation budget: %w", err)
	}

	now := g.now().UTC()
	leaseExpiresAt := now.Add(policy.LeaseDuration)
	var reservedAt, existingLeaseExpiresAt time.Time
	var state string
	var existingLeaseToken uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT reserved_at, state, lease_token, lease_expires_at
		FROM paid_generation_reservations
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
	`, ownerID, projectID, string(operation), requestID).Scan(&reservedAt, &state, &existingLeaseToken, &existingLeaseExpiresAt)
	if err == nil {
		releasedReplay := state == "released"
		expiredRecovery := state == "reserved" && !existingLeaseExpiresAt.After(now)
		if releasedReplay || expiredRecovery {
			var inFlight int
			if err := tx.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM paid_generation_reservations
				WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3
					AND request_id <> $4 AND state = 'reserved' AND lease_expires_at > $5
			`, ownerID, projectID, string(operation), requestID, now).Scan(&inFlight); err != nil {
				return paidgeneration.Reservation{}, fmt.Errorf("count paid generation replay concurrency: %w", err)
			}
			if inFlight >= policy.MaxInFlight {
				return paidgeneration.Reservation{}, paidgeneration.ErrConcurrencyExceeded
			}

			leaseToken := uuid.New()
			if _, err := tx.Exec(ctx, `
				UPDATE paid_generation_reservations
				SET state = 'reserved', lease_token = $5, lease_expires_at = $6, released_at = NULL
				WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
			`, ownerID, projectID, string(operation), requestID, leaseToken, leaseExpiresAt); err != nil {
				return paidgeneration.Reservation{}, fmt.Errorf("reacquire paid generation reservation: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return paidgeneration.Reservation{}, fmt.Errorf("commit paid generation replay recovery: %w", err)
			}
			return paidgeneration.Reservation{
				OwnerID: ownerID, ProjectID: projectID, Operation: operation, RequestID: requestID,
				ReservedAt: reservedAt, LeaseToken: leaseToken, LeaseExpiresAt: leaseExpiresAt,
				Replay: releasedReplay, Recovered: expiredRecovery,
			}, nil
		}

		if err := tx.Commit(ctx); err != nil {
			return paidgeneration.Reservation{}, fmt.Errorf("commit paid generation replay: %w", err)
		}
		return paidgeneration.Reservation{
			OwnerID: ownerID, ProjectID: projectID, Operation: operation, RequestID: requestID,
			ReservedAt: reservedAt, LeaseToken: existingLeaseToken, LeaseExpiresAt: existingLeaseExpiresAt, Replay: true,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return paidgeneration.Reservation{}, fmt.Errorf("load paid generation reservation: %w", err)
	}

	windowStart := now.Add(-policy.Window)
	var requestsInWindow, inFlight int
	if err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE reserved_at >= $4),
			COUNT(*) FILTER (WHERE state = 'reserved' AND lease_expires_at > $5)
		FROM paid_generation_reservations
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3
	`, ownerID, projectID, string(operation), windowStart, now).Scan(&requestsInWindow, &inFlight); err != nil {
		return paidgeneration.Reservation{}, fmt.Errorf("count paid generation usage: %w", err)
	}
	if inFlight >= policy.MaxInFlight {
		return paidgeneration.Reservation{}, paidgeneration.ErrConcurrencyExceeded
	}
	if requestsInWindow >= policy.MaxRequests {
		return paidgeneration.Reservation{}, paidgeneration.ErrQuotaExceeded
	}

	leaseToken := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO paid_generation_reservations
			(owner_id, project_id, operation_kind, request_id, state, reserved_at, lease_token, lease_expires_at)
		VALUES ($1, $2, $3, $4, 'reserved', $5, $6, $7)
	`, ownerID, projectID, string(operation), requestID, now, leaseToken, leaseExpiresAt); err != nil {
		return paidgeneration.Reservation{}, fmt.Errorf("insert paid generation reservation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return paidgeneration.Reservation{}, fmt.Errorf("commit paid generation reservation: %w", err)
	}
	return paidgeneration.Reservation{
		OwnerID: ownerID, ProjectID: projectID, Operation: operation, RequestID: requestID,
		ReservedAt: now, LeaseToken: leaseToken, LeaseExpiresAt: leaseExpiresAt,
	}, nil
}

func (g *PaidGenerationGuard) Renew(ctx context.Context, reservation paidgeneration.Reservation, leaseDuration time.Duration) (paidgeneration.Reservation, error) {
	if g == nil || g.pool == nil || leaseDuration <= 0 || reservation.OwnerID == uuid.Nil || reservation.ProjectID == uuid.Nil || reservation.RequestID == uuid.Nil || reservation.LeaseToken == uuid.Nil || !validOperation(reservation.Operation) {
		return paidgeneration.Reservation{}, paidgeneration.ErrGuardUnavailable
	}

	now := g.now().UTC()
	leaseExpiresAt := now.Add(leaseDuration)
	commandTag, err := g.pool.Exec(ctx, `
		UPDATE paid_generation_reservations
		SET lease_expires_at = $6
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
			AND state = 'reserved' AND lease_token = $5 AND lease_expires_at > $7
	`, reservation.OwnerID, reservation.ProjectID, string(reservation.Operation), reservation.RequestID, reservation.LeaseToken, leaseExpiresAt, now)
	if err != nil {
		return paidgeneration.Reservation{}, fmt.Errorf("renew paid generation reservation: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return paidgeneration.Reservation{}, paidgeneration.ErrLeaseLost
	}
	reservation.LeaseExpiresAt = leaseExpiresAt
	return reservation, nil
}

func (g *PaidGenerationGuard) Release(ctx context.Context, reservation paidgeneration.Reservation) error {
	if g == nil || g.pool == nil {
		return paidgeneration.ErrGuardUnavailable
	}
	if reservation.OwnerID == uuid.Nil || reservation.ProjectID == uuid.Nil || reservation.RequestID == uuid.Nil || reservation.LeaseToken == uuid.Nil || !validOperation(reservation.Operation) {
		return paidgeneration.ErrGuardUnavailable
	}
	_, err := g.pool.Exec(ctx, `
		UPDATE paid_generation_reservations
		SET state = 'released', released_at = COALESCE(released_at, NOW())
		WHERE owner_id = $1 AND project_id = $2 AND operation_kind = $3 AND request_id = $4
			AND state = 'reserved' AND lease_token = $5
	`, reservation.OwnerID, reservation.ProjectID, string(reservation.Operation), reservation.RequestID, reservation.LeaseToken)
	if err != nil {
		return fmt.Errorf("release paid generation reservation: %w", err)
	}
	return nil
}

func validOperation(operation paidgeneration.Operation) bool {
	switch operation {
	case paidgeneration.OperationImage, paidgeneration.OperationNarration, paidgeneration.OperationVideo:
		return true
	default:
		return false
	}
}
