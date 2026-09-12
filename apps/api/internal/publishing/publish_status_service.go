package publishing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrPublishStatusReconcile = errors.New("publishing status reconciliation failed")

type RemoteStatusReader interface {
	Get(ctx context.Context, accessToken, remoteVideoID string) (YouTubeRemoteStatus, error)
}

type PublishStatusService struct {
	attempts    AttemptRepository
	connections ConnectionCredentialReader
	oauth       OAuthTokenRefresher
	remote      RemoteStatusReader
	now         func() time.Time
}

func NewPublishStatusService(attempts AttemptRepository, connections ConnectionCredentialReader, oauth OAuthTokenRefresher, remote RemoteStatusReader) (*PublishStatusService, error) {
	if attempts == nil || connections == nil || oauth == nil || remote == nil {
		return nil, ErrInvalidModel
	}
	return &PublishStatusService{
		attempts:    attempts,
		connections: connections,
		oauth:       oauth,
		remote:      remote,
		now:         time.Now,
	}, nil
}

func (s *PublishStatusService) Reconcile(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || attemptID == uuid.Nil {
		return PublishAttempt{}, ErrInvalidModel
	}
	attempt, err := s.attempts.GetAttempt(ctx, ownerID, projectID, attemptID)
	if err != nil {
		return PublishAttempt{}, err
	}
	if !canReconcileRemoteStatus(attempt.State) || strings.TrimSpace(attempt.RemoteVideoID) == "" {
		return PublishAttempt{}, ErrPublishStatusReconcile
	}

	refreshToken, err := s.connections.RefreshToken(ctx, ownerID, attempt.ConnectionID)
	if err != nil {
		return PublishAttempt{}, err
	}
	token, err := s.oauth.Refresh(ctx, refreshToken)
	if err != nil {
		attempt.UpdatedAt = s.now().UTC()
		if errors.Is(err, ErrOAuthRefreshReconnect) {
			attempt.State = PublishReconnectRequired
			attempt.ScheduledAt = nil
			attempt.LastErrorCode = "youtube_reconnect_required"
			return s.save(ctx, attempt)
		}
		// Refresh transport/provider failures occur after the remote video already
		// exists. Preserve its last truthful publication state and expose only a
		// retryable status-check signal; never fall back to upload retry.
		attempt.LastErrorCode = "youtube_status_retryable"
		return s.save(ctx, attempt)
	}
	status, err := s.remote.Get(ctx, token.AccessToken, attempt.RemoteVideoID)
	if err != nil {
		// A transport/provider read failure happens after the remote video already
		// exists. Persist a status-check retry signal while preserving the last
		// truthful publication state; never send the attempt back to upload retry,
		// which could duplicate a YouTube video.
		attempt.UpdatedAt = s.now().UTC()
		attempt.LastErrorCode = "youtube_status_retryable"
		return s.save(ctx, attempt)
	}
	return s.persistRemoteStatus(ctx, attempt, status)
}

func canReconcileRemoteStatus(state PublishState) bool {
	switch state {
	case PublishUploadAccepted, PublishProcessing, PublishPrivate, PublishScheduled, PublishPublic:
		return true
	default:
		return false
	}
}

func (s *PublishStatusService) persistRemoteStatus(ctx context.Context, attempt PublishAttempt, status YouTubeRemoteStatus) (PublishAttempt, error) {
	attempt.UpdatedAt = s.now().UTC()

	switch status.Failure {
	case RemoteStatusFailureReconnectRequired:
		attempt.State = PublishReconnectRequired
		attempt.ScheduledAt = nil
		attempt.LastErrorCode = "youtube_reconnect_required"
	case RemoteStatusFailureRetryable:
		// Keep the last truthful remote publication state. This is a status-query
		// retry, not an upload retry; moving the attempt back to the upload retry
		// state could cause the creator to resume/re-initiate media unnecessarily.
		attempt.LastErrorCode = "youtube_status_retryable"
	case RemoteStatusFailureRejected:
		attempt.State = PublishRejected
		attempt.ScheduledAt = nil
		attempt.LastErrorCode = "youtube_remote_rejected"
	case RemoteStatusFailureNone:
		attempt.LastErrorCode = ""
		processing := strings.ToLower(strings.TrimSpace(status.ProcessingStatus))
		privacy := strings.ToLower(strings.TrimSpace(status.PrivacyStatus))

		switch processing {
		case "processing":
			attempt.State = PublishProcessing
			attempt.ScheduledAt = nil
			return s.save(ctx, attempt)
		case "failed":
			attempt.State = PublishRejected
			attempt.ScheduledAt = nil
			attempt.LastErrorCode = "youtube_processing_failed"
			return s.save(ctx, attempt)
		case "succeeded", "terminated":
			// Privacy/publication state below is authoritative after processing is no
			// longer active. "terminated" only means processing details are no longer
			// available, so do not invent a processing failure from it.
		default:
			return PublishAttempt{}, ErrPublishStatusReconcile
		}

		switch privacy {
		case "public":
			attempt.State = PublishPublic
			attempt.ScheduledAt = nil
		case "private":
			if status.PublishAt != nil && status.PublishAt.After(attempt.UpdatedAt) {
				publishAt := status.PublishAt.UTC()
				attempt.State = PublishScheduled
				attempt.ScheduledAt = &publishAt
			} else {
				attempt.State = PublishPrivate
				attempt.ScheduledAt = nil
			}
		default:
			attempt.LastErrorCode = "youtube_privacy_unsupported"
		}
	default:
		return PublishAttempt{}, ErrPublishStatusReconcile
	}

	return s.save(ctx, attempt)
}

func (s *PublishStatusService) save(ctx context.Context, attempt PublishAttempt) (PublishAttempt, error) {
	return s.attempts.SaveAttemptProgress(ctx, attempt, attempt.ResumableSessionURI, attempt.UploadedBytes, attempt.LastErrorCode)
}
