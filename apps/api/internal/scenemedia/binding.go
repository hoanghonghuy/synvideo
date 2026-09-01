package scenemedia

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type Role string

const RolePrimaryVisual Role = "primary_visual"

type Status string

const (
	StatusActive     Status = "active"
	StatusSuperseded Status = "superseded"
)

var (
	ErrNotFound             = errors.New("scene media binding not found")
	ErrScenePlanNotFound    = errors.New("scene plan not found")
	ErrScenePlanNotApproved = errors.New("scene plan is not approved")
	ErrSceneKeyNotFound     = errors.New("scene key not found")
	ErrMediaAssetNotFound   = errors.New("media asset not found")
	ErrMediaAssetNotVisual  = errors.New("media asset is not a visual")
	ErrUnauthenticated      = errors.New("scene media principal is required")
	ErrInvalidInput         = errors.New("scene media input is invalid")
	ErrPersistenceFailed    = errors.New("scene media persistence failed")
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

// CurrentSceneBinding keeps every scene in the approved plan order. Binding is
// nil when the scene has not received a primary visual yet.
type CurrentSceneBinding struct {
	Scene   sceneplan.Scene `json:"scene"`
	Binding *Binding        `json:"binding,omitempty"`
}
