package publishing

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type ConnectionService struct {
	repository ConnectionRepository
	protector  RefreshTokenProtector
}

func NewConnectionService(repository ConnectionRepository, protector RefreshTokenProtector) (*ConnectionService, error) {
	if repository == nil || protector == nil {
		return nil, ErrInvalidModel
	}
	return &ConnectionService{repository: repository, protector: protector}, nil
}

func (s *ConnectionService) SaveConnectedChannel(ctx context.Context, connection ChannelConnection, refreshToken string) (ChannelConnection, error) {
	if err := connection.Validate(); err != nil || connection.State != ConnectionConnected || strings.TrimSpace(refreshToken) == "" {
		return ChannelConnection{}, ErrInvalidModel
	}

	canonical := connection
	existing, err := s.repository.GetConnectionByRemoteChannel(ctx, connection.OwnerID, connection.Provider, connection.RemoteChannelID)
	if err == nil {
		canonical.ID = existing.ID
		canonical.CreatedAt = existing.CreatedAt
	} else if !errors.Is(err, ErrConnectionNotFound) {
		return ChannelConnection{}, err
	}

	envelope, err := s.protector.Protect(canonical.OwnerID, canonical.ID, refreshToken)
	if err != nil {
		return ChannelConnection{}, err
	}
	return s.repository.UpsertConnection(ctx, canonical, envelope.Ciphertext, envelope.Nonce, envelope.KeyID)
}

func (s *ConnectionService) RefreshToken(ctx context.Context, ownerID, connectionID uuid.UUID) (string, error) {
	if ownerID == uuid.Nil || connectionID == uuid.Nil {
		return "", ErrInvalidModel
	}
	connection, err := s.repository.GetConnection(ctx, ownerID, connectionID)
	if err != nil {
		return "", err
	}
	if connection.State != ConnectionConnected {
		return "", ErrConnectionNotFound
	}
	envelope, err := s.repository.GetRefreshTokenEnvelope(ctx, ownerID, connectionID)
	if err != nil {
		return "", err
	}
	return s.protector.Reveal(ownerID, connectionID, envelope)
}
