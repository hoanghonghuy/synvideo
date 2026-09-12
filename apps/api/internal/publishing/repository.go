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

type RefreshTokenEnvelope struct {
	Ciphertext []byte
	Nonce      []byte
	KeyID      string
}

type ConnectionRepository interface {
	UpsertConnection(ctx context.Context, connection ChannelConnection, encryptedRefreshToken, tokenNonce []byte, tokenKeyID string) (ChannelConnection, error)
	GetConnection(ctx context.Context, ownerID, connectionID uuid.UUID) (ChannelConnection, error)
	GetConnectionByRemoteChannel(ctx context.Context, ownerID uuid.UUID, provider Provider, remoteChannelID string) (ChannelConnection, error)
	GetRefreshTokenEnvelope(ctx context.Context, ownerID, connectionID uuid.UUID) (RefreshTokenEnvelope, error)
	ListConnections(ctx context.Context, ownerID uuid.UUID) ([]ChannelConnection, error)
}

type AttemptRepository interface {
	CreateAttempt(ctx context.Context, attempt PublishAttempt) (PublishAttempt, error)
	GetAttempt(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error)
	GetAttemptByRequest(ctx context.Context, ownerID, projectID, connectionID, renderArtifactID, requestID uuid.UUID) (PublishAttempt, error)
	SaveAttemptProgress(ctx context.Context, attempt PublishAttempt, resumableSessionURI string, uploadedBytes int64, lastErrorCode string) (PublishAttempt, error)
}

// AttemptHistoryRepository is an optional read contract used by Channel Hub.
// It deliberately remains separate from AttemptRepository so existing lifecycle
// test doubles do not need to grow merely to support a read-only UI concern.
type AttemptHistoryRepository interface {
	ListAttempts(ctx context.Context, ownerID, projectID uuid.UUID) ([]PublishAttempt, error)
}

// PublishArtifactRepository exposes only safe immutable render metadata required
// for selecting an owned project artifact. Storage identity and integrity details
// remain server-side.
type PublishArtifactRepository interface {
	ListPublishArtifacts(ctx context.Context, ownerID, projectID uuid.UUID) ([]PublishArtifactSummary, error)
}
