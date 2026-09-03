package scenenarration

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type Role string

const RoleNarration Role = "narration"

type Status string

const (
	StatusActive     Status = "active"
	StatusSuperseded Status = "superseded"
)

var (
	ErrNotFound             = errors.New("scene narration binding not found")
	ErrScenePlanNotFound    = errors.New("scene plan not found")
	ErrScenePlanNotApproved = errors.New("scene plan is not approved")
	ErrSceneKeyNotFound     = errors.New("scene key not found")
	ErrMediaAssetNotFound   = errors.New("media asset not found")
	ErrMediaAssetNotAudio   = errors.New("media asset is not an audio asset")
	ErrUnauthenticated      = errors.New("scene narration principal is required")
	ErrInvalidInput         = errors.New("scene narration input is invalid")
	ErrPersistenceFailed    = errors.New("scene narration persistence failed")
)

type Binding struct {
	ID               uuid.UUID  `json:"id"`
	OwnerID          uuid.UUID  `json:"-"`
	ProjectID        uuid.UUID  `json:"project_id"`
	ScenePlanVersion int        `json:"scene_plan_version"`
	SceneKey         string     `json:"scene_key"`
	Role             Role       `json:"role"`
	BindingVersion   int        `json:"binding_version"`
	AssetID          uuid.UUID  `json:"asset_id"`
	Status           Status     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	SupersededAt     *time.Time `json:"superseded_at,omitempty"`
}

type CurrentSceneNarration struct {
	Scene   sceneplan.Scene        `json:"scene"`
	Binding *Binding               `json:"binding,omitempty"`
	Asset   *mediaasset.MediaAsset `json:"asset,omitempty"`
}
