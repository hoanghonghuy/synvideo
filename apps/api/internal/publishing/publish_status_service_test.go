package publishing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type task057AttemptRepo struct {
	attempt PublishAttempt
	saved   PublishAttempt
}

func (r *task057AttemptRepo) CreateAttempt(context.Context, PublishAttempt) (PublishAttempt, error) {
	return PublishAttempt{}, errors.New("not implemented")
}
func (r *task057AttemptRepo) GetAttempt(_ context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error) {
	if r.attempt.OwnerID != ownerID || r.attempt.ProjectID != projectID || r.attempt.ID != attemptID {
		return PublishAttempt{}, ErrAttemptNotFound
	}
	return r.attempt, nil
}
func (r *task057AttemptRepo) GetAttemptByRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (PublishAttempt, error) {
	return PublishAttempt{}, errors.New("not implemented")
}
func (r *task057AttemptRepo) SaveAttemptProgress(_ context.Context, attempt PublishAttempt, sessionURI string, uploadedBytes int64, lastErrorCode string) (PublishAttempt, error) {
	attempt.ResumableSessionURI = sessionURI
	attempt.UploadedBytes = uploadedBytes
	attempt.LastErrorCode = lastErrorCode
	r.saved = attempt
	return attempt, nil
}

func task057Attempt(state PublishState) PublishAttempt {
	now := time.Date(2026, 9, 12, 1, 0, 0, 0, time.UTC)
	return PublishAttempt{
		ID:               uuid.New(),
		OwnerID:          uuid.New(),
		ProjectID:        uuid.New(),
		ConnectionID:     uuid.New(),
		RenderArtifactID: uuid.New(),
		RequestID:        uuid.New(),
		Provider:         ProviderYouTube,
		State:            state,
		RemoteVideoID:    "video-123",
		UploadedBytes:    100,
		Title:            "Video title",
		CreatedAt:        now.Add(-time.Hour),
		UpdatedAt:        now,
	}
}

func TestPublishStatusServiceMapsProcessingAndPublicationState(t *testing.T) {
	tests := []struct {
		name       string
		status     YouTubeRemoteStatus
		wantState  PublishState
		wantError  string
		wantFuture bool
	}{
		{name: "processing", status: YouTubeRemoteStatus{ProcessingStatus: "processing", PrivacyStatus: "private"}, wantState: PublishProcessing},
		{name: "private", status: YouTubeRemoteStatus{ProcessingStatus: "succeeded", PrivacyStatus: "private"}, wantState: PublishPrivate},
		{name: "public", status: YouTubeRemoteStatus{ProcessingStatus: "succeeded", PrivacyStatus: "public"}, wantState: PublishPublic},
		{name: "failed", status: YouTubeRemoteStatus{ProcessingStatus: "failed", PrivacyStatus: "private"}, wantState: PublishRejected, wantError: "youtube_processing_failed"},
		{name: "retryable query", status: YouTubeRemoteStatus{Failure: RemoteStatusFailureRetryable}, wantState: PublishUploadAccepted, wantError: "youtube_status_retryable"},
		{name: "reconnect", status: YouTubeRemoteStatus{Failure: RemoteStatusFailureReconnectRequired}, wantState: PublishReconnectRequired, wantError: "youtube_reconnect_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := task057Attempt(PublishUploadAccepted)
			repo := &task057AttemptRepo{attempt: attempt}
			service := &PublishStatusService{attempts: repo, now: func() time.Time { return time.Date(2026, 9, 12, 2, 0, 0, 0, time.UTC) }}
			got, err := service.persistRemoteStatus(context.Background(), attempt, tt.status)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tt.wantState || got.LastErrorCode != tt.wantError {
				t.Fatalf("got state=%q error=%q, want state=%q error=%q", got.State, got.LastErrorCode, tt.wantState, tt.wantError)
			}
		})
	}
}

func TestPublishStatusServiceMapsFuturePrivatePublishToScheduled(t *testing.T) {
	attempt := task057Attempt(PublishUploadAccepted)
	repo := &task057AttemptRepo{attempt: attempt}
	now := time.Date(2026, 9, 12, 2, 0, 0, 0, time.UTC)
	publishAt := now.Add(2 * time.Hour)
	service := &PublishStatusService{attempts: repo, now: func() time.Time { return now }}
	got, err := service.persistRemoteStatus(context.Background(), attempt, YouTubeRemoteStatus{
		ProcessingStatus: "succeeded",
		PrivacyStatus:    "private",
		PublishAt:        &publishAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != PublishScheduled || got.ScheduledAt == nil || !got.ScheduledAt.Equal(publishAt) {
		t.Fatalf("unexpected scheduled result: %+v", got)
	}
}

func TestPublishStatusServiceRejectsPreUploadState(t *testing.T) {
	attempt := task057Attempt(PublishQueued)
	repo := &task057AttemptRepo{attempt: attempt}
	service := &PublishStatusService{attempts: repo}
	_, err := service.Reconcile(context.Background(), attempt.OwnerID, attempt.ProjectID, attempt.ID)
	if !errors.Is(err, ErrPublishStatusReconcile) {
		t.Fatalf("err = %v", err)
	}
}
