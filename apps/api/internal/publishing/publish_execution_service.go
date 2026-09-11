package publishing

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrPublishExecution = errors.New("publishing execution failed")

type ResumableChunkUploader interface {
	Initiate(ctx context.Context, accessToken string, metadata YouTubeUploadMetadata, contentType string, totalBytes int64) (ResumableUploadResult, error)
	UploadChunk(ctx context.Context, accessToken, sessionURI, contentType string, body io.Reader, start, length, totalBytes int64) (ResumableUploadResult, error)
}

type PublishExecutionService struct {
	attempts    AttemptRepository
	connections ConnectionCredentialReader
	oauth       OAuthTokenRefresher
	uploader    ResumableChunkUploader
	now         func() time.Time
}

func NewPublishExecutionService(attempts AttemptRepository, connections ConnectionCredentialReader, oauth OAuthTokenRefresher, uploader ResumableChunkUploader) (*PublishExecutionService, error) {
	if attempts == nil || connections == nil || oauth == nil || uploader == nil {
		return nil, ErrInvalidModel
	}
	return &PublishExecutionService{
		attempts:    attempts,
		connections: connections,
		oauth:       oauth,
		uploader:    uploader,
		now:         time.Now,
	}, nil
}

func (s *PublishExecutionService) ExecuteChunk(ctx context.Context, ownerID, projectID, attemptID uuid.UUID, contentType string, totalBytes, chunkBytes int64, body io.Reader) (PublishAttempt, error) {
	contentType = strings.TrimSpace(contentType)
	if ownerID == uuid.Nil || projectID == uuid.Nil || attemptID == uuid.Nil || contentType == "" || totalBytes <= 0 || chunkBytes <= 0 || body == nil {
		return PublishAttempt{}, ErrInvalidModel
	}

	attempt, err := s.attempts.GetAttempt(ctx, ownerID, projectID, attemptID)
	if err != nil {
		return PublishAttempt{}, err
	}
	if attempt.State != PublishQueued && attempt.State != PublishUploading && attempt.State != PublishRetryableFailure {
		return PublishAttempt{}, ErrPublishExecution
	}
	if attempt.UploadedBytes < 0 || attempt.UploadedBytes >= totalBytes || attempt.UploadedBytes+chunkBytes > totalBytes {
		return PublishAttempt{}, ErrPublishExecution
	}

	refreshToken, err := s.connections.RefreshToken(ctx, ownerID, attempt.ConnectionID)
	if err != nil {
		return PublishAttempt{}, err
	}
	token, err := s.oauth.Refresh(ctx, refreshToken)
	if err != nil {
		return PublishAttempt{}, err
	}

	if strings.TrimSpace(attempt.ResumableSessionURI) == "" {
		initiated, err := s.uploader.Initiate(ctx, token.AccessToken, YouTubeUploadMetadata{
			Title:       attempt.Title,
			Description: attempt.Description,
		}, contentType, totalBytes)
		if err != nil {
			return PublishAttempt{}, err
		}
		attempt, err = s.persistResult(ctx, attempt, initiated)
		if err != nil {
			return PublishAttempt{}, err
		}
		if initiated.Failure != UploadFailureNone {
			return attempt, nil
		}
		if strings.TrimSpace(attempt.ResumableSessionURI) == "" {
			return PublishAttempt{}, ErrPublishExecution
		}
	}

	result, err := s.uploader.UploadChunk(ctx, token.AccessToken, attempt.ResumableSessionURI, contentType, body, attempt.UploadedBytes, chunkBytes, totalBytes)
	if err != nil {
		return PublishAttempt{}, err
	}
	return s.persistResult(ctx, attempt, result)
}

func (s *PublishExecutionService) persistResult(ctx context.Context, attempt PublishAttempt, result ResumableUploadResult) (PublishAttempt, error) {
	sessionURI := strings.TrimSpace(result.SessionURI)
	if sessionURI == "" {
		sessionURI = attempt.ResumableSessionURI
	}
	attempt.UpdatedAt = s.now().UTC()
	attempt.LastErrorCode = ""

	switch result.Failure {
	case UploadFailureNone:
		attempt.State = PublishUploading
		attempt.ResumableSessionURI = sessionURI
		attempt.UploadedBytes = result.UploadedBytes
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
		attempt.ResumableSessionURI = ""
		sessionURI = ""
		attempt.LastErrorCode = "youtube_session_expired"
	case UploadFailureRejected:
		attempt.State = PublishRejected
		attempt.LastErrorCode = "youtube_rejected"
	default:
		return PublishAttempt{}, ErrPublishExecution
	}

	return s.attempts.SaveAttemptProgress(ctx, attempt, sessionURI, attempt.UploadedBytes, attempt.LastErrorCode)
}
