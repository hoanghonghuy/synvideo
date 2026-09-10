package publishing

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validConnection(now time.Time) ChannelConnection {
	return ChannelConnection{
		ID:              uuid.New(),
		OwnerID:         uuid.New(),
		Provider:        ProviderYouTube,
		RemoteChannelID: "UC-test",
		DisplayName:     "Creator channel",
		State:           ConnectionConnected,
		Capabilities: Capabilities{
			CanUpload:   true,
			CanPublish:  true,
			CanSchedule: true,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func validAttempt(now time.Time) PublishAttempt {
	return PublishAttempt{
		ID:               uuid.New(),
		OwnerID:          uuid.New(),
		ProjectID:        uuid.New(),
		ConnectionID:     uuid.New(),
		RenderArtifactID: uuid.New(),
		RequestID:        uuid.New(),
		Provider:         ProviderYouTube,
		State:            PublishQueued,
		Title:            "Ready to publish",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestChannelConnectionValidateRequiresConnectedCapabilities(t *testing.T) {
	now := time.Now().UTC()
	connection := validConnection(now)
	connection.State = ConnectionReconnectRequired

	if err := connection.Validate(); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected disconnected capabilities to be rejected, got %v", err)
	}
}

func TestChannelConnectionValidateScheduleRequiresPublishCapability(t *testing.T) {
	now := time.Now().UTC()
	connection := validConnection(now)
	connection.Capabilities.CanPublish = false

	if err := connection.Validate(); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected schedule without publish capability to be rejected, got %v", err)
	}
}

func TestPublishAttemptValidateKeepsUploadAndProcessingDistinct(t *testing.T) {
	now := time.Now().UTC()
	attempt := validAttempt(now)
	attempt.State = PublishUploadAccepted
	attempt.RemoteVideoID = "video-123"

	if err := attempt.Validate(); err != nil {
		t.Fatalf("expected accepted upload to validate: %v", err)
	}
	attempt.State = PublishProcessing
	if err := attempt.Validate(); err != nil {
		t.Fatalf("expected processing state to validate independently: %v", err)
	}
}

func TestPublishAttemptValidateRequiresRemoteIdentityAfterAcceptance(t *testing.T) {
	now := time.Now().UTC()
	attempt := validAttempt(now)
	attempt.State = PublishUploadAccepted

	if err := attempt.Validate(); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected accepted upload without remote id to fail, got %v", err)
	}
}

func TestPublishAttemptValidateSchedulingRequiresFutureTimeAndRemoteIdentity(t *testing.T) {
	now := time.Now().UTC()
	attempt := validAttempt(now)
	attempt.State = PublishScheduled
	attempt.RemoteVideoID = "video-123"
	future := now.Add(time.Hour)
	attempt.ScheduledAt = &future

	if err := attempt.Validate(); err != nil {
		t.Fatalf("expected valid schedule: %v", err)
	}

	past := now.Add(-time.Minute)
	attempt.ScheduledAt = &past
	if err := attempt.Validate(); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected past schedule to fail, got %v", err)
	}
}

func TestPublishAttemptValidateRejectsScheduleMetadataOnUnscheduledState(t *testing.T) {
	now := time.Now().UTC()
	attempt := validAttempt(now)
	future := now.Add(time.Hour)
	attempt.ScheduledAt = &future

	if err := attempt.Validate(); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected queued attempt with schedule metadata to fail, got %v", err)
	}
}
