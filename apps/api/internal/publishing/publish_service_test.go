package publishing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type publishServiceRepo struct {
	connection ChannelConnection
	created    PublishAttempt
	createErr  error
}

func (r *publishServiceRepo) UpsertConnection(context.Context, ChannelConnection, []byte, []byte, string) (ChannelConnection, error) {
	return ChannelConnection{}, errors.New("not implemented")
}

func (r *publishServiceRepo) GetConnection(_ context.Context, ownerID, connectionID uuid.UUID) (ChannelConnection, error) {
	if r.connection.OwnerID != ownerID || r.connection.ID != connectionID {
		return ChannelConnection{}, ErrConnectionNotFound
	}
	return r.connection, nil
}

func (r *publishServiceRepo) GetConnectionByRemoteChannel(context.Context, uuid.UUID, Provider, string) (ChannelConnection, error) {
	return ChannelConnection{}, errors.New("not implemented")
}

func (r *publishServiceRepo) GetRefreshTokenEnvelope(context.Context, uuid.UUID, uuid.UUID) (RefreshTokenEnvelope, error) {
	return RefreshTokenEnvelope{}, errors.New("not implemented")
}

func (r *publishServiceRepo) ListConnections(_ context.Context, ownerID uuid.UUID) ([]ChannelConnection, error) {
	if r.connection.OwnerID != ownerID {
		return []ChannelConnection{}, nil
	}
	return []ChannelConnection{r.connection}, nil
}

func (r *publishServiceRepo) CreateAttempt(_ context.Context, attempt PublishAttempt) (PublishAttempt, error) {
	r.created = attempt
	if r.createErr != nil {
		return PublishAttempt{}, r.createErr
	}
	return attempt, nil
}

func (r *publishServiceRepo) GetAttempt(_ context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error) {
	if r.created.OwnerID == ownerID && r.created.ProjectID == projectID && r.created.ID == attemptID {
		return r.created, nil
	}
	return PublishAttempt{}, ErrAttemptNotFound
}

func (r *publishServiceRepo) GetAttemptByRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (PublishAttempt, error) {
	return PublishAttempt{}, errors.New("not implemented")
}

func (r *publishServiceRepo) SaveAttemptProgress(context.Context, PublishAttempt, string, int64, string) (PublishAttempt, error) {
	return PublishAttempt{}, errors.New("not implemented")
}

func TestPublishServiceCreateAttemptUsesOwnerScopedConnectedChannel(t *testing.T) {
	now := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	ownerID := uuid.New()
	projectID := uuid.New()
	connectionID := uuid.New()
	artifactID := uuid.New()
	requestID := uuid.New()
	repo := &publishServiceRepo{connection: ChannelConnection{
		ID:              connectionID,
		OwnerID:         ownerID,
		Provider:        ProviderYouTube,
		RemoteChannelID: "channel-1",
		DisplayName:     "Creator channel",
		State:           ConnectionConnected,
		Capabilities:    Capabilities{CanUpload: true},
		CreatedAt:       now.Add(-time.Hour),
		UpdatedAt:       now,
	}}
	service, err := NewPublishService(repo, repo, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	attempt, err := service.CreateAttempt(context.Background(), ownerID, projectID, connectionID, artifactID, requestID, "  Launch video  ", "  description  ")
	if err != nil {
		t.Fatal(err)
	}
	if attempt.OwnerID != ownerID || attempt.ProjectID != projectID || attempt.ConnectionID != connectionID || attempt.RenderArtifactID != artifactID || attempt.RequestID != requestID {
		t.Fatalf("unexpected identity fields: %+v", attempt)
	}
	if attempt.Provider != ProviderYouTube || attempt.State != PublishQueued || attempt.Title != "Launch video" || attempt.Description != "description" {
		t.Fatalf("unexpected attempt payload: %+v", attempt)
	}
	if attempt.CreatedAt != now || attempt.UpdatedAt != now || attempt.ID == uuid.Nil {
		t.Fatalf("unexpected timestamps/id: %+v", attempt)
	}
}

func TestPublishServiceCreateAttemptRejectsUnavailableConnection(t *testing.T) {
	now := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	ownerID := uuid.New()
	connectionID := uuid.New()
	repo := &publishServiceRepo{connection: ChannelConnection{
		ID:              connectionID,
		OwnerID:         ownerID,
		Provider:        ProviderYouTube,
		RemoteChannelID: "channel-1",
		DisplayName:     "Creator channel",
		State:           ConnectionReconnectRequired,
		CreatedAt:       now.Add(-time.Hour),
		UpdatedAt:       now,
	}}
	service, err := NewPublishService(repo, repo, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.CreateAttempt(context.Background(), ownerID, uuid.New(), connectionID, uuid.New(), uuid.New(), "Title", "")
	if !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("expected unavailable connection, got %v", err)
	}
}

func TestPublishServiceCreateAttemptPreservesRepositoryIdempotencyConflict(t *testing.T) {
	now := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)
	ownerID := uuid.New()
	connectionID := uuid.New()
	repo := &publishServiceRepo{
		connection: ChannelConnection{
			ID: connectionID, OwnerID: ownerID, Provider: ProviderYouTube,
			RemoteChannelID: "channel-1", DisplayName: "Creator channel",
			State: ConnectionConnected, Capabilities: Capabilities{CanUpload: true},
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
		},
		createErr: ErrAttemptConflict,
	}
	service, err := NewPublishService(repo, repo, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.CreateAttempt(context.Background(), ownerID, uuid.New(), connectionID, uuid.New(), uuid.New(), "Title", "")
	if !errors.Is(err, ErrAttemptConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}
