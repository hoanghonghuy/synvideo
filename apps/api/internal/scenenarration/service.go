package scenenarration

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type ScenePlanRepository interface {
	GetByVersion(ctx context.Context, ownerID, projectID uuid.UUID, version int) (sceneplan.Plan, error)
}

type MediaAssetRepository interface {
	Get(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error)
}

type Service struct {
	repo   Repository
	plans  ScenePlanRepository
	assets MediaAssetRepository
}

func NewService(repo Repository, plans ScenePlanRepository, assets MediaAssetRepository) *Service {
	return &Service{
		repo:   repo,
		plans:  plans,
		assets: assets,
	}
}

func (s *Service) ListCurrent(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int) ([]CurrentSceneNarration, error) {
	if principal.OwnerID == uuid.Nil {
		return nil, ErrUnauthenticated
	}
	if projectID == uuid.Nil || planVersion < 1 {
		return nil, ErrScenePlanNotFound
	}
	if s.plans == nil || s.repo == nil {
		return nil, ErrPersistenceFailed
	}

	plan, err := s.plans.GetByVersion(ctx, principal.OwnerID, projectID, planVersion)
	if err != nil {
		if errors.Is(err, sceneplan.ErrNotFound) {
			return nil, ErrScenePlanNotFound
		}
		return nil, err
	}
	if plan.Status != sceneplan.StatusApproved {
		return nil, ErrScenePlanNotApproved
	}

	activeBindings, err := s.repo.ListActiveForPlan(ctx, principal.OwnerID, projectID, planVersion)
	if err != nil {
		return nil, err
	}

	bindingsMap := make(map[string]Binding, len(activeBindings))
	for _, b := range activeBindings {
		bindingsMap[b.SceneKey] = b
	}

	result := make([]CurrentSceneNarration, 0, len(plan.Scenes))
	for _, sc := range plan.Scenes {
		entry := CurrentSceneNarration{Scene: sc}
		if b, ok := bindingsMap[sc.Key]; ok {
			entry.Binding = &b
			if s.assets != nil {
				if asset, err := s.assets.Get(ctx, principal.OwnerID, projectID, b.AssetID); err == nil {
					entry.Asset = &asset
				}
			}
		}
		result = append(result, entry)
	}

	return result, nil
}

func (s *Service) AssignNarration(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string, assetID uuid.UUID) (Binding, error) {
	if principal.OwnerID == uuid.Nil {
		return Binding{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil || planVersion < 1 {
		return Binding{}, ErrScenePlanNotFound
	}
	if sceneKey == "" || assetID == uuid.Nil {
		return Binding{}, ErrInvalidInput
	}
	if s.plans == nil || s.repo == nil || s.assets == nil {
		return Binding{}, ErrPersistenceFailed
	}

	plan, err := s.plans.GetByVersion(ctx, principal.OwnerID, projectID, planVersion)
	if err != nil {
		if errors.Is(err, sceneplan.ErrNotFound) {
			return Binding{}, ErrScenePlanNotFound
		}
		return Binding{}, err
	}
	if plan.Status != sceneplan.StatusApproved {
		return Binding{}, ErrScenePlanNotApproved
	}

	sceneFound := false
	for _, sc := range plan.Scenes {
		if sc.Key == sceneKey {
			sceneFound = true
			break
		}
	}
	if !sceneFound {
		return Binding{}, ErrSceneKeyNotFound
	}

	asset, err := s.assets.Get(ctx, principal.OwnerID, projectID, assetID)
	if err != nil {
		if errors.Is(err, mediaasset.ErrNotFound) {
			return Binding{}, ErrMediaAssetNotFound
		}
		return Binding{}, err
	}
	if asset.Kind != mediaasset.KindAudio {
		return Binding{}, ErrMediaAssetNotAudio
	}

	return s.repo.Assign(ctx, principal.OwnerID, projectID, planVersion, sceneKey, assetID)
}

func (s *Service) ListHistory(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) ([]Binding, error) {
	if principal.OwnerID == uuid.Nil {
		return nil, ErrUnauthenticated
	}
	if projectID == uuid.Nil || planVersion < 1 {
		return nil, ErrScenePlanNotFound
	}
	if sceneKey == "" {
		return nil, ErrInvalidInput
	}
	if s.repo == nil {
		return nil, ErrPersistenceFailed
	}

	return s.repo.ListHistory(ctx, principal.OwnerID, projectID, planVersion, sceneKey)
}

func (s *Service) GetActive(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) (Binding, error) {
	if principal.OwnerID == uuid.Nil {
		return Binding{}, ErrUnauthenticated
	}
	if projectID == uuid.Nil || planVersion < 1 {
		return Binding{}, ErrScenePlanNotFound
	}
	if sceneKey == "" {
		return Binding{}, ErrInvalidInput
	}
	if s.repo == nil {
		return Binding{}, ErrPersistenceFailed
	}

	return s.repo.GetActive(ctx, principal.OwnerID, projectID, planVersion, sceneKey)
}
