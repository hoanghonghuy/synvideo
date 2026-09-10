package publishing

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrConnectionNotFound = errors.New("publishing connection not found")
	ErrAttemptNotFound    = errors.New("publishing attempt not found")
	ErrAttemptConflict    = errors.New("publishing attempt conflict")
)

type ConnectionRepository interface {
	Upsert(ctx context.Context, connection ChannelConnection, encryptedRefreshToken []byte, tokenKeyID string) (ChannelConnection, error)
	Get(ctx context.Context, ownerID, connectionID uuid.UUID) (ChannelConnection, error)
	List(ctx context.Context, ownerID uuid.UUID) ([]ChannelConnection, error)
}

type AttemptRepository interface {
	Create(ctx context.Context, attempt PublishAttempt) (PublishAttempt, error)
	Get(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error)
	GetByRequest(ctx context.Context, ownerID, projectID, connectionID, renderArtifactID, requestID uuid.UUID) (PublishAttempt, error)
	SaveProgress(ctx context.Context, attempt PublishAttempt, resumableSessionURI string, uploadedBytes int64, lastErrorCode string) (PublishAttempt, error)
}
