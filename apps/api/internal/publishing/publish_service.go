package publishing

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PublishService struct {
	connections ConnectionRepository
	attempts    AttemptRepository
	now         func() time.Time
}

func NewPublishService(connections ConnectionRepository, attempts AttemptRepository, now func() time.Time) (*PublishService, error) {
	if connections == nil || attempts == nil {
		return nil, ErrInvalidModel
	}
	if now == nil {
		now = time.Now
	}
	return &PublishService{connections: connections, attempts: attempts, now: now}, nil
}

func (s *PublishService) ListConnections(ctx context.Context, ownerID uuid.UUID) ([]ChannelConnection, error) {
	if ownerID == uuid.Nil {
		return nil, ErrInvalidModel
	}
	return s.connections.ListConnections(ctx, ownerID)
}

func (s *PublishService) ListAttempts(ctx context.Context, ownerID, projectID uuid.UUID) ([]PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil {
		return nil, ErrInvalidModel
	}
	repo, ok := s.attempts.(AttemptHistoryRepository)
	if !ok {
		return nil, ErrInvalidModel
	}
	return repo.ListAttempts(ctx, ownerID, projectID)
}

func (s *PublishService) ListPublishArtifacts(ctx context.Context, ownerID, projectID uuid.UUID) ([]PublishArtifactSummary, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil {
		return nil, ErrInvalidModel
	}
	repo, ok := s.attempts.(PublishArtifactRepository)
	if !ok {
		return nil, ErrInvalidModel
	}
	return repo.ListPublishArtifacts(ctx, ownerID, projectID)
}

func (s *PublishService) GetAttempt(ctx context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error) {
	if ownerID == uuid.Nil || projectID == uuid.Nil || attemptID == uuid.Nil {
		return PublishAttempt{}, ErrInvalidModel
	}
	return s.attempts.GetAttempt(ctx, ownerID, projectID, attemptID)
}

func (s *PublishService) CreateAttempt(ctx context.Context, ownerID, projectID, connectionID, renderArtifactID, requestID uuid.UUID, title, description string) (PublishAttempt, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if ownerID == uuid.Nil || projectID == uuid.Nil || connectionID == uuid.Nil || renderArtifactID == uuid.Nil || requestID == uuid.Nil || title == "" {
		return PublishAttempt{}, ErrInvalidModel
	}

	connection, err := s.connections.GetConnection(ctx, ownerID, connectionID)
	if err != nil {
		return PublishAttempt{}, err
	}
	if connection.State != ConnectionConnected || !connection.Capabilities.CanUpload {
		return PublishAttempt{}, ErrConnectionNotFound
	}

	now := s.now().UTC()
	attempt := PublishAttempt{
		ID:               uuid.New(),
		OwnerID:          ownerID,
		ProjectID:        projectID,
		ConnectionID:     connectionID,
		RenderArtifactID: renderArtifactID,
		RequestID:        requestID,
		Provider:         connection.Provider,
		State:            PublishQueued,
		Title:            title,
		Description:      description,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return s.attempts.CreateAttempt(ctx, attempt)
}
