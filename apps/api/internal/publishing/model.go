package publishing

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidModel = errors.New("invalid publishing model")

type Provider string

const (
	ProviderYouTube Provider = "youtube"
)

type ConnectionState string

const (
	ConnectionConnected         ConnectionState = "connected"
	ConnectionReconnectRequired ConnectionState = "reconnect_required"
	ConnectionRevoked           ConnectionState = "revoked"
)

type Capabilities struct {
	CanUpload   bool `json:"can_upload"`
	CanPublish  bool `json:"can_publish"`
	CanSchedule bool `json:"can_schedule"`
}

type ChannelConnection struct {
	ID              uuid.UUID       `json:"id"`
	OwnerID         uuid.UUID       `json:"-"`
	Provider        Provider        `json:"provider"`
	RemoteChannelID string          `json:"remote_channel_id"`
	DisplayName     string          `json:"display_name"`
	State           ConnectionState `json:"state"`
	Capabilities    Capabilities    `json:"capabilities"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (c ChannelConnection) Validate() error {
	if c.ID == uuid.Nil || c.OwnerID == uuid.Nil || c.Provider != ProviderYouTube {
		return ErrInvalidModel
	}
	if strings.TrimSpace(c.RemoteChannelID) == "" || strings.TrimSpace(c.DisplayName) == "" {
		return ErrInvalidModel
	}
	switch c.State {
	case ConnectionConnected, ConnectionReconnectRequired, ConnectionRevoked:
	default:
		return ErrInvalidModel
	}
	if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() || c.UpdatedAt.Before(c.CreatedAt) {
		return ErrInvalidModel
	}
	if c.State != ConnectionConnected && (c.Capabilities.CanUpload || c.Capabilities.CanPublish || c.Capabilities.CanSchedule) {
		return ErrInvalidModel
	}
	if c.Capabilities.CanSchedule && !c.Capabilities.CanPublish {
		return ErrInvalidModel
	}
	return nil
}

type PublishState string

const (
	PublishQueued            PublishState = "queued"
	PublishUploading         PublishState = "uploading"
	PublishUploadAccepted    PublishState = "upload_accepted"
	PublishProcessing        PublishState = "processing"
	PublishPrivate           PublishState = "private"
	PublishScheduled         PublishState = "scheduled"
	PublishPublic            PublishState = "public"
	PublishReconnectRequired PublishState = "reconnect_required"
	PublishRetryableFailure  PublishState = "retryable_failure"
	PublishRejected          PublishState = "rejected"
)

type PublishAttempt struct {
	ID               uuid.UUID    `json:"id"`
	OwnerID          uuid.UUID    `json:"-"`
	ProjectID        uuid.UUID    `json:"project_id"`
	ConnectionID     uuid.UUID    `json:"connection_id"`
	RenderArtifactID uuid.UUID    `json:"render_artifact_id"`
	RequestID        uuid.UUID    `json:"request_id"`
	Provider         Provider     `json:"provider"`
	State            PublishState `json:"state"`
	RemoteVideoID    string       `json:"remote_video_id,omitempty"`
	Title            string       `json:"title"`
	Description      string       `json:"description,omitempty"`
	ScheduledAt      *time.Time   `json:"scheduled_at,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

func (a PublishAttempt) Validate() error {
	if a.ID == uuid.Nil || a.OwnerID == uuid.Nil || a.ProjectID == uuid.Nil || a.ConnectionID == uuid.Nil || a.RenderArtifactID == uuid.Nil || a.RequestID == uuid.Nil {
		return ErrInvalidModel
	}
	if a.Provider != ProviderYouTube || strings.TrimSpace(a.Title) == "" || a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() || a.UpdatedAt.Before(a.CreatedAt) {
		return ErrInvalidModel
	}
	switch a.State {
	case PublishQueued, PublishUploading, PublishUploadAccepted, PublishProcessing, PublishPrivate, PublishScheduled, PublishPublic, PublishReconnectRequired, PublishRetryableFailure, PublishRejected:
	default:
		return ErrInvalidModel
	}
	if a.State == PublishScheduled {
		if a.ScheduledAt == nil || !a.ScheduledAt.After(a.CreatedAt) || strings.TrimSpace(a.RemoteVideoID) == "" {
			return ErrInvalidModel
		}
	} else if a.ScheduledAt != nil {
		return ErrInvalidModel
	}
	if requiresRemoteVideoID(a.State) && strings.TrimSpace(a.RemoteVideoID) == "" {
		return ErrInvalidModel
	}
	return nil
}

func requiresRemoteVideoID(state PublishState) bool {
	switch state {
	case PublishUploadAccepted, PublishProcessing, PublishPrivate, PublishScheduled, PublishPublic:
		return true
	default:
		return false
	}
}
