package scenemedia

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type ScenePlanReader interface {
	GetByVersion(ctx context.Context, ownerID, projectID uuid.UUID, version int) (sceneplan.Plan, error)
}

type MediaAssetReader interface {
	Get(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error)
}

type Service struct {
	plans  ScenePlanReader
	assets MediaAssetReader
	repo   Repository
}

func NewService(plans ScenePlanReader, assets MediaAssetReader, repo Repository) *Service {
	return &Service{plans: plans, assets: assets, repo: repo}
}

func (s *Service) AssignPrimaryVisual(ctx context.Context, principal project.Principal, projectID uuid.UUID, scenePlanVersion int, sceneKey string, assetID uuid.UUID) (Binding, error) {
	if err := validateRequest(principal, projectID, scenePlanVersion, sceneKey, assetID); err != nil {
		return Binding{}, err
	}
	if _, err := s.loadApprovedPlan(ctx, principal.OwnerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return Binding{}, err
	}
	asset, err := s.assets.Get(ctx, principal.OwnerID, projectID, assetID)
	if err != nil {
		return Binding{}, mapAssetError(err)
	}
	if asset.OwnerID != principal.OwnerID || asset.ProjectID != projectID {
		return Binding{}, ErrMediaAssetNotFound
	}
	if asset.Kind != mediaasset.KindImage && asset.Kind != mediaasset.KindVideo {
		return Binding{}, ErrMediaAssetNotVisual
	}

	binding, err := s.repo.AssignPrimaryVisual(ctx, principal.OwnerID, projectID, scenePlanVersion, sceneKey, assetID)
	if err != nil {
		return Binding{}, err
	}
	return cloneBinding(binding), nil
}

func (s *Service) GetCurrent(ctx context.Context, principal project.Principal, projectID uuid.UUID, scenePlanVersion int, sceneKey string) (Binding, error) {
	if err := validateBaseRequest(principal, projectID, scenePlanVersion); err != nil {
		return Binding{}, err
	}
	if sceneKey == "" {
		return Binding{}, ErrInvalidInput
	}
	if _, err := s.loadApprovedPlan(ctx, principal.OwnerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return Binding{}, err
	}
	binding, err := s.repo.GetCurrent(ctx, principal.OwnerID, projectID, scenePlanVersion, sceneKey)
	if err != nil {
		return Binding{}, err
	}
	return cloneBinding(binding), nil
}

func (s *Service) ListCurrent(ctx context.Context, principal project.Principal, projectID uuid.UUID, scenePlanVersion int) ([]CurrentSceneBinding, error) {
	if err := validateBaseRequest(principal, projectID, scenePlanVersion); err != nil {
		return nil, err
	}
	plan, err := s.loadApprovedPlan(ctx, principal.OwnerID, projectID, scenePlanVersion, "")
	if err != nil {
		return nil, err
	}
	bindings, err := s.repo.ListCurrent(ctx, principal.OwnerID, projectID, scenePlanVersion)
	if err != nil {
		return nil, err
	}
	byScene := make(map[string]Binding, len(bindings))
	for _, binding := range bindings {
		byScene[binding.SceneKey] = binding
	}
	result := make([]CurrentSceneBinding, 0, len(plan.Scenes))
	for _, scene := range plan.Scenes {
		item := CurrentSceneBinding{Scene: scene}
		if binding, ok := byScene[scene.Key]; ok {
			cloned := cloneBinding(binding)
			item.Binding = &cloned
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Service) ListHistory(ctx context.Context, principal project.Principal, projectID uuid.UUID, scenePlanVersion int, sceneKey string) ([]Binding, error) {
	if err := validateBaseRequest(principal, projectID, scenePlanVersion); err != nil {
		return nil, err
	}
	if sceneKey == "" {
		return nil, ErrInvalidInput
	}
	if _, err := s.loadApprovedPlan(ctx, principal.OwnerID, projectID, scenePlanVersion, sceneKey); err != nil {
		return nil, err
	}
	items, err := s.repo.ListHistory(ctx, principal.OwnerID, projectID, scenePlanVersion, sceneKey)
	if err != nil {
		return nil, err
	}
	result := make([]Binding, len(items))
	for i := range items {
		result[i] = cloneBinding(items[i])
	}
	return result, nil
}

func validateRequest(principal project.Principal, projectID uuid.UUID, scenePlanVersion int, sceneKey string, assetID uuid.UUID) error {
	if err := validateBaseRequest(principal, projectID, scenePlanVersion); err != nil {
		return err
	}
	if sceneKey == "" || assetID == uuid.Nil {
		return ErrInvalidInput
	}
	return nil
}

func validateBaseRequest(principal project.Principal, projectID uuid.UUID, scenePlanVersion int) error {
	if principal.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	if projectID == uuid.Nil || scenePlanVersion < 1 {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) loadApprovedPlan(ctx context.Context, ownerID, projectID uuid.UUID, version int, sceneKey string) (sceneplan.Plan, error) {
	plan, err := s.plans.GetByVersion(ctx, ownerID, projectID, version)
	if err != nil {
		if errors.Is(err, sceneplan.ErrNotFound) {
			return sceneplan.Plan{}, ErrScenePlanNotFound
		}
		return sceneplan.Plan{}, err
	}
	if plan.ProjectID != projectID {
		return sceneplan.Plan{}, ErrScenePlanNotFound
	}
	if plan.Status != sceneplan.StatusApproved {
		return sceneplan.Plan{}, ErrScenePlanNotApproved
	}
	if sceneKey != "" {
		for _, scene := range plan.Scenes {
			if scene.Key == sceneKey {
				return plan, nil
			}
		}
		return sceneplan.Plan{}, ErrSceneKeyNotFound
	}
	return plan, nil
}

func mapAssetError(err error) error {
	if errors.Is(err, mediaasset.ErrNotFound) {
		return ErrMediaAssetNotFound
	}
	return err
}

func bindingKey(ownerID, projectID uuid.UUID, planVersion int, sceneKey string) string {
	return ownerID.String() + ":" + projectID.String() + ":" + strconv.Itoa(planVersion) + ":" + sceneKey
}

func cloneBinding(binding Binding) Binding {
	if binding.SupersededAt != nil {
		value := *binding.SupersededAt
		binding.SupersededAt = &value
	}
	return binding
}
