package publishing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeConnectionRepository struct {
	connection ChannelConnection
	envelope   RefreshTokenEnvelope
}

func (f *fakeConnectionRepository) UpsertConnection(_ context.Context, connection ChannelConnection, ciphertext, nonce []byte, keyID string) (ChannelConnection, error) {
	f.connection = connection
	f.envelope = RefreshTokenEnvelope{Ciphertext: append([]byte(nil), ciphertext...), Nonce: append([]byte(nil), nonce...), KeyID: keyID}
	return connection, nil
}

func (f *fakeConnectionRepository) GetConnection(_ context.Context, ownerID, connectionID uuid.UUID) (ChannelConnection, error) {
	if f.connection.OwnerID != ownerID || f.connection.ID != connectionID {
		return ChannelConnection{}, ErrConnectionNotFound
	}
	return f.connection, nil
}

func (f *fakeConnectionRepository) GetConnectionByRemoteChannel(_ context.Context, ownerID uuid.UUID, provider Provider, remoteChannelID string) (ChannelConnection, error) {
	if f.connection.OwnerID != ownerID || f.connection.Provider != provider || f.connection.RemoteChannelID != remoteChannelID {
		return ChannelConnection{}, ErrConnectionNotFound
	}
	return f.connection, nil
}

func (f *fakeConnectionRepository) GetRefreshTokenEnvelope(_ context.Context, ownerID, connectionID uuid.UUID) (RefreshTokenEnvelope, error) {
	if f.connection.OwnerID != ownerID || f.connection.ID != connectionID {
		return RefreshTokenEnvelope{}, ErrConnectionNotFound
	}
	return f.envelope, nil
}

func (f *fakeConnectionRepository) ListConnections(_ context.Context, ownerID uuid.UUID) ([]ChannelConnection, error) {
	if f.connection.OwnerID != ownerID {
		return []ChannelConnection{}, nil
	}
	return []ChannelConnection{f.connection}, nil
}

func TestConnectionServiceStoresOnlyProtectedRefreshTokenAndRevealsIt(t *testing.T) {
	repository := &fakeConnectionRepository{}
	protector, err := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewConnectionService(repository, protector)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	connection := ChannelConnection{
		ID: uuid.New(), OwnerID: uuid.New(), Provider: ProviderYouTube,
		RemoteChannelID: "UC-test", DisplayName: "Test channel", State: ConnectionConnected,
		Capabilities: Capabilities{CanUpload: true, CanPublish: true}, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := service.SaveConnectedChannel(context.Background(), connection, "provider-refresh-secret"); err != nil {
		t.Fatalf("save connected channel: %v", err)
	}
	if string(repository.envelope.Ciphertext) == "provider-refresh-secret" || repository.envelope.KeyID != "v1" {
		t.Fatalf("credential was not safely protected: %+v", repository.envelope)
	}
	token, err := service.RefreshToken(context.Background(), connection.OwnerID, connection.ID)
	if err != nil {
		t.Fatalf("refresh token: %v", err)
	}
	if token != "provider-refresh-secret" {
		t.Fatalf("refresh token = %q", token)
	}
	if _, err := service.RefreshToken(context.Background(), uuid.New(), connection.ID); err != ErrConnectionNotFound {
		t.Fatalf("cross-owner lookup error = %v", err)
	}
}

func TestConnectionServiceReconnectKeepsCanonicalIDAndRotatesCredential(t *testing.T) {
	protector, err := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	serviceRepository := &fakeConnectionRepository{}
	service, err := NewConnectionService(serviceRepository, protector)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	ownerID := uuid.New()
	canonicalID := uuid.New()
	initial := ChannelConnection{
		ID: canonicalID, OwnerID: ownerID, Provider: ProviderYouTube,
		RemoteChannelID: "UC-reconnect", DisplayName: "Channel", State: ConnectionConnected,
		Capabilities: Capabilities{CanUpload: true, CanPublish: true}, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := service.SaveConnectedChannel(context.Background(), initial, "refresh-v1"); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	reconnect := initial
	reconnect.ID = uuid.New()
	reconnect.DisplayName = "Channel renamed"
	reconnect.UpdatedAt = now.Add(time.Minute)
	saved, err := service.SaveConnectedChannel(context.Background(), reconnect, "refresh-v2")
	if err != nil {
		t.Fatalf("reconnect save: %v", err)
	}
	if saved.ID != canonicalID {
		t.Fatalf("canonical id changed: got %s want %s", saved.ID, canonicalID)
	}
	if !saved.CreatedAt.Equal(now) {
		t.Fatalf("created at changed: got %s want %s", saved.CreatedAt, now)
	}
	token, err := service.RefreshToken(context.Background(), ownerID, canonicalID)
	if err != nil {
		t.Fatalf("refresh rotated token: %v", err)
	}
	if token != "refresh-v2" {
		t.Fatalf("rotated token = %q", token)
	}
	if _, err := service.RefreshToken(context.Background(), ownerID, reconnect.ID); err != ErrConnectionNotFound {
		t.Fatalf("new transient id lookup error = %v", err)
	}
}

func TestConnectionServiceDoesNotRevealCredentialForDisconnectedConnection(t *testing.T) {
	now := time.Now().UTC()
	repository := &fakeConnectionRepository{connection: ChannelConnection{
		ID: uuid.New(), OwnerID: uuid.New(), Provider: ProviderYouTube,
		RemoteChannelID: "UC-test", DisplayName: "Test channel", State: ConnectionReconnectRequired,
		CreatedAt: now, UpdatedAt: now,
	}}
	protector, err := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewConnectionService(repository, protector)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RefreshToken(context.Background(), repository.connection.OwnerID, repository.connection.ID); err != ErrConnectionNotFound {
		t.Fatalf("disconnected credential error = %v", err)
	}
}
