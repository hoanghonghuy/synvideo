package publishing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrPublishRecovery = errors.New("publishing recovery failed")

type ConnectionCredentialReader interface {
	RefreshToken(ctx context.Context, ownerID, connectionID uuid.UUID) (string, error)
}

type OAuthTokenRefresher interface {
	Refresh(ctx context.Context, refreshToken string) (OAuthToken, error)
}

type ResumableProgressQuerier interface {
	Query(ctx context.Context, accessToken, sessionURI string, totalBytes int64) (ResumableUploadResult, error)
}

type PublishRecoveryService struct {
	attempts    AttemptRepository
	connections ConnectionCredentialReader
	oauth       OAuthTokenRefresher
	uploader    ResumableProgressQuerier
	now         func() time.Time
}

func NewPublishRecoveryService(attempts AttemptRepository, connections ConnectionCredentialReader, oauth OAuthTokenRefresher, uploader ResumableProgressQuerier) (*PublishRecoveryService, error) {
	if attempts == nil || connections == nil || oauth == nil || uploader == nil {
		return nil, ErrInvalidModel
	}
	return &PublishRecoveryService{
		attempts:     attempts,
		connections: connections,
		oauth:       oauth,
		uploader:    uploader,
		now:         time.Now,
	}, nil
}

func (s *PublishRecoveryService) Recover(ctx context.Context, ownerID, projectID, attemptID uuid.UUID, totalBytes int64) (PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || attemptID == uuid.Nil || totalBytes <= 0 {
		return PublishAttempt{}, ErrInvalidModel
	}

	attempt, err := s.attempts.GetAttempt(ctx, ownerID, projectID, attemptID)
	if err != nil {
		return PublishAttempt{}, err
	}
	if attempt.State != PublishUploading && attempt.State != PublishRetryableFailure {
		return PublishAttempt{}, ErrPublishRecovery
	}
	if strings.TrimSpace(attempt.ResumableSessionURI) == "" {
		return PublishAttempt{}, ErrPublishRecovery
	}

	refreshToken, err := s.connections.RefreshToken(ctx, ownerID, attempt.ConnectionID)
	if err != nil {
		return PublishAttempt{}, err
	}
	token, err := s.oauth.Refresh(ctx, refreshToken)
	if err != nil {
		return PublishAttempt{}, err
	}

	result, err := s.uploader.Query(ctx, token.AccessToken, attempt.ResumableSessionURI, totalBytes)
	if err != nil {
		return PublishAttempt{}, err
	}
	return s.persistProviderResult(ctx, attempt, result)
}

func (s *PublishRecoveryService) persistProviderResult(ctx context.Context, attempt PublishAttempt, result ResumableUploadResult) (PublishAttempt, error) {
	sessionURI := strings.TrimSpace(result.SessionURI)
	if sessionURI == "" {
		sessionURI = attempt.ResumableSessionURI
	}
	attempt.UpdatedAt = s.now().UTC()
	attempt.LastErrorCode = ""

	switch result.Failure {
	case UploadFailureNone:
		attempt.State = PublishUploading
		attempt.UploadedBytes = result.UploadedBytes
		attempt.ResumableSessionURI = sessionURI
		if result.Complete {
			attempt.State = PublishUploadAccepted
			attempt.RemoteVideoID = strings.TrimSpace(result.RemoteVideoID)
		}
	case UploadFailureReconnectRequired:
		attempt.State = PublishReconnectRequired
		attempt.LastErrorCode = "youtube_reconnect_required"
	case UploadFailureRetryable:
		attempt.State = PublishRetryableFailure
		attempt.LastErrorCode = "youtube_retryable"
	case UploadFailureSessionExpired:
		attempt.State = PublishRetryableFailure
		attempt.LastErrorCode = "youtube_session_expired"
	case UploadFailureRejected:
		attempt.State = PublishRejected
		attempt.LastErrorCode = "youtube_rejected"
	default:
		return PublishAttempt{}, ErrPublishRecovery
	}

	return s.attempts.SaveAttemptProgress(ctx, attempt, sessionURI, attempt.UploadedBytes, attempt.LastErrorCode)
}
