package scenevideojob

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/paidgeneration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

type fakePaidGenerationGuard struct {
	reserveErr error
	reserves   int
	renews     int
	releases   int
}

func (f *fakePaidGenerationGuard) Reserve(_ context.Context, ownerID, projectID uuid.UUID, operation paidgeneration.Operation, requestID uuid.UUID, policy paidgeneration.Policy) (paidgeneration.Reservation, error) {
	f.reserves++
	if f.reserveErr != nil {
		return paidgeneration.Reservation{}, f.reserveErr
	}
	now := time.Now().UTC()
	return paidgeneration.Reservation{
		OwnerID: ownerID, ProjectID: projectID, Operation: operation, RequestID: requestID,
		ReservedAt: now, LeaseToken: uuid.New(), LeaseExpiresAt: now.Add(policy.LeaseDuration),
	}, nil
}

func (f *fakePaidGenerationGuard) Renew(_ context.Context, reservation paidgeneration.Reservation, duration time.Duration) (paidgeneration.Reservation, error) {
	f.renews++
	reservation.LeaseExpiresAt = time.Now().UTC().Add(duration)
	return reservation, nil
}

func (f *fakePaidGenerationGuard) Release(context.Context, paidgeneration.Reservation) error {
	f.releases++
	return nil
}

func videoGuardPolicy() paidgeneration.Policy {
	return paidgeneration.Policy{MaxRequests: 10, Window: time.Hour, MaxInFlight: 2, LeaseDuration: time.Hour}
}

func TestHandleGuardDenialPreventsPaidVideoSubmit(t *testing.T) {
	gen := &fakeVideoGenerator{operation: providers.VideoOperation{ID: "op-denied", State: providers.VideoOperationRunning}}
	checkpoints := &fakeCheckpointStore{}
	guard := &fakePaidGenerationGuard{reserveErr: paidgeneration.ErrQuotaExceeded}
	h := NewGuardedHandler(fakeVideoRuntime{gen: gen}, checkpoints, &fakeAssetStore{}, fakeBinder{}, guard, videoGuardPolicy())

	_, err := h.Handle(context.Background(), makeVideoJob(t))
	var retry *jobs.RetryableJobError
	if !errors.As(err, &retry) || retry.Code != ErrorQuotaExceeded {
		t.Fatalf("expected quota retry, got %v", err)
	}
	if gen.starts != 0 || checkpoints.saves != 0 {
		t.Fatalf("guard denial reached paid submit: starts=%d saves=%d", gen.starts, checkpoints.saves)
	}
	if guard.reserves != 1 || guard.releases != 0 {
		t.Fatalf("unexpected guard lifecycle: reserves=%d releases=%d", guard.reserves, guard.releases)
	}
}

func TestHandleGuardsOnlyInitialVideoSubmitAndReusesCheckpoint(t *testing.T) {
	binary, err := providers.NewGeneratedBinary("video/mp4", []byte("video"))
	if err != nil {
		t.Fatal(err)
	}
	gen := &fakeVideoGenerator{operation: providers.VideoOperation{ID: "op-guarded", State: providers.VideoOperationRunning}, binary: binary}
	checkpoints := &fakeCheckpointStore{}
	guard := &fakePaidGenerationGuard{}
	h := NewGuardedHandler(fakeVideoRuntime{gen: gen}, checkpoints, &fakeAssetStore{}, fakeBinder{}, guard, videoGuardPolicy())
	job := makeVideoJob(t)

	_, err = h.Handle(context.Background(), job)
	var retry *jobs.RetryableJobError
	if !errors.As(err, &retry) || retry.Code != ErrorPollingPending {
		t.Fatalf("expected polling retry, got %v", err)
	}
	if gen.starts != 1 || checkpoints.saves != 1 || guard.reserves != 1 || guard.releases != 1 {
		t.Fatalf("first submit lifecycle starts=%d saves=%d reserves=%d releases=%d", gen.starts, checkpoints.saves, guard.reserves, guard.releases)
	}

	gen.operation.State = providers.VideoOperationSucceeded
	if _, err := h.Handle(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if gen.starts != 1 {
		t.Fatalf("durable checkpoint retry resubmitted paid work: starts=%d", gen.starts)
	}
	if guard.reserves != 1 || guard.releases != 1 {
		t.Fatalf("poll/result path consumed another paid reservation: reserves=%d releases=%d", guard.reserves, guard.releases)
	}
}
