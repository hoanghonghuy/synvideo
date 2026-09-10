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
	UpsertConnection(ctx context.Context, connection ChannelConnection, encryptedRefreshToken, tokenNonce []byte, tokenKeyID string) (ChannelConnection, error)
	GetConnection(ctx context.Context, ownerID, connectionID uuid.UUID) (ChannelConnection, error)
	ListConnections(ctx context.Context, ownerID uuid.UUID) ([]ChannelConnection, error)
}

type AttemptRepository interface {
	CreateAttempt(ctx context.Context, attempt PublishAttempt) (PublishAttempt, error)
	GetAttempt(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error)
	GetAttemptByRequest(ctx context.Context, ownerID, projectID, connectionID, renderArtifactID, requestID uuid.UUID) (PublishAttempt, error)
	SaveAttemptProgress(ctx context.Context, attempt PublishAttempt, resumableSessionURI string, uploadedBytes int64, lastErrorCode string) (PublishAttempt, error)
}
