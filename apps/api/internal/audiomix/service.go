package audiomix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type ScenePlanRepository interface {
	ListVersions(ctx context.Context, ownerID, projectID uuid.UUID) ([]sceneplan.Plan, error)
}

type NarrationRepository interface {
	ListActiveForPlan(ctx context.Context, ownerID, projectID uuid.UUID, planVersion int) ([]scenenarration.Binding, error)
}

type MediaAssetRepository interface {
	Get(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error)
}

type Service struct {
	repo       Repository
	plans      ScenePlanRepository
	narrations NarrationRepository
	assets     MediaAssetRepository
	now        func() time.Time
}

func NewService(repo Repository, plans ScenePlanRepository, narrations NarrationRepository, assets MediaAssetRepository) *Service {
	return &Service{repo: repo, plans: plans, narrations: narrations, assets: assets, now: time.Now}
}

type CreateInput struct {
	MusicAssetID uuid.UUID `json:"music_asset_id"`
	Config       Config    `json:"config"`
}

type UpdateInput struct {
	ExpectedRevision int       `json:"expected_revision"`
	MusicAssetID     uuid.UUID `json:"music_asset_id"`
	Config           Config    `json:"config"`
}

func (s *Service) Create(ctx context.Context, principal project.Principal, projectID uuid.UUID, input CreateInput) (View, error) {
	if err := validateIdentity(principal, projectID); err != nil {
		return View{}, err
	}
	if input.MusicAssetID == uuid.Nil {
		return View{}, ErrInvalidInput
	}
	if s.repo == nil {
		return View{}, ErrPersistence
	}
	if _, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID); err == nil {
		return View{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return View{}, err
	}
	music, err := s.resolveMusic(ctx, principal, projectID, input.MusicAssetID)
	if err != nil {
		return View{}, err
	}
	narration, err := s.resolveNarration(ctx, principal, projectID)
	if err != nil {
		return View{}, err
	}
	doc, err := NewDocument(uuid.New(), principal.OwnerID, projectID, music, narration, input.Config, s.now().UTC())
	if err != nil {
		return View{}, err
	}
	created, err := s.repo.CreateInitial(ctx, doc)
	if err != nil {
		return View{}, err
	}
	return View{Document: created, State: StateCurrent}, nil
}

func (s *Service) Get(ctx context.Context, principal project.Principal, projectID uuid.UUID) (View, error) {
	if err := validateIdentity(principal, projectID); err != nil {
		return View{}, err
	}
	if s.repo == nil {
		return View{}, ErrPersistence
	}
	doc, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID)
	if err != nil {
		return View{}, err
	}
	music, musicErr := s.resolveMusic(ctx, principal, projectID, doc.MusicAssetID)
	if musicErr != nil && !errors.Is(musicErr, ErrMusicMissing) {
		return View{}, musicErr
	}
	if errors.Is(musicErr, ErrMusicMissing) {
		music = MusicSource{AssetID: doc.MusicAssetID, ProjectID: projectID, DurationMS: doc.MusicDurationMS}
	}
	narration, narrationErr := s.resolveNarration(ctx, principal, projectID)
	if narrationErr != nil && !errors.Is(narrationErr, ErrNarrationMissing) {
		return View{}, narrationErr
	}
	return View{Document: doc, State: StateForSources(doc, music, narration)}, nil
}

func (s *Service) Update(ctx context.Context, principal project.Principal, projectID uuid.UUID, input UpdateInput) (View, error) {
	if err := validateIdentity(principal, projectID); err != nil {
		return View{}, err
	}
	if input.ExpectedRevision < 1 || input.MusicAssetID == uuid.Nil {
		return View{}, ErrInvalidInput
	}
	if s.repo == nil {
		return View{}, ErrPersistence
	}
	latest, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID)
	if err != nil {
		return View{}, err
	}
	if latest.Revision != input.ExpectedRevision {
		return View{}, ErrConflict
	}
	music, err := s.resolveMusic(ctx, principal, projectID, input.MusicAssetID)
	if err != nil {
		return View{}, err
	}
	boundNarration := NarrationSource{LineageID: latest.NarrationLineageID, ScenePlanVersion: latest.ScenePlanVersion, DurationMS: latest.NarrationDurationMS}
	if err := ValidateBinding(projectID, music, boundNarration, input.Config); err != nil {
		return View{}, err
	}
	latest.Revision++
	latest.MusicAssetID = music.AssetID
	latest.MusicDurationMS = music.DurationMS
	latest.Config = input.Config
	latest.UpdatedAt = s.now().UTC()
	updated, err := s.repo.CreateRevision(ctx, latest, input.ExpectedRevision)
	if err != nil {
		return View{}, err
	}
	currentNarration, narrationErr := s.resolveNarration(ctx, principal, projectID)
	if narrationErr != nil && !errors.Is(narrationErr, ErrNarrationMissing) {
		return View{}, narrationErr
	}
	return View{Document: updated, State: StateForSources(updated, music, currentNarration)}, nil
}

func (s *Service) RebindNarration(ctx context.Context, principal project.Principal, projectID uuid.UUID, expectedRevision int) (View, error) {
	if err := validateIdentity(principal, projectID); err != nil {
		return View{}, err
	}
	if expectedRevision < 1 {
		return View{}, ErrInvalidInput
	}
	if s.repo == nil {
		return View{}, ErrPersistence
	}
	latest, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID)
	if err != nil {
		return View{}, err
	}
	if latest.Revision != expectedRevision {
		return View{}, ErrConflict
	}
	music, err := s.resolveMusic(ctx, principal, projectID, latest.MusicAssetID)
	if err != nil {
		return View{}, err
	}
	narration, err := s.resolveNarration(ctx, principal, projectID)
	if err != nil {
		return View{}, err
	}
	if err := ValidateBinding(projectID, music, narration, latest.Config); err != nil {
		return View{}, err
	}
	latest.Revision++
	latest.ScenePlanVersion = narration.ScenePlanVersion
	latest.MusicDurationMS = music.DurationMS
	latest.NarrationLineageID = narration.LineageID
	latest.NarrationDurationMS = narration.DurationMS
	latest.UpdatedAt = s.now().UTC()
	updated, err := s.repo.CreateRevision(ctx, latest, expectedRevision)
	if err != nil {
		return View{}, err
	}
	return View{Document: updated, State: StateCurrent}, nil
}

func (s *Service) Snapshot(ctx context.Context, principal project.Principal, projectID uuid.UUID) (Snapshot, error) {
	view, err := s.Get(ctx, principal, projectID)
	if err != nil {
		return Snapshot{}, err
	}
	return NewSnapshot(view.Document, view.State)
}

func (s *Service) History(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]Document, error) {
	if err := validateIdentity(principal, projectID); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, ErrPersistence
	}
	return s.repo.ListHistory(ctx, principal.OwnerID, projectID)
}

func (s *Service) resolveMusic(ctx context.Context, principal project.Principal, projectID, assetID uuid.UUID) (MusicSource, error) {
	if s.assets == nil {
		return MusicSource{}, ErrMusicMissing
	}
	asset, err := s.assets.Get(ctx, principal.OwnerID, projectID, assetID)
	if err != nil {
		if errors.Is(err, mediaasset.ErrNotFound) {
			return MusicSource{}, ErrMusicMissing
		}
		return MusicSource{}, err
	}
	if asset.Kind != mediaasset.KindAudio || asset.DeletionRequestedAt != nil {
		return MusicSource{}, ErrMusicMissing
	}
	durationMS, err := durationMSFromMetadata(asset.Metadata)
	if err != nil {
		return MusicSource{}, ErrMusicMissing
	}
	return MusicSource{AssetID: asset.ID, ProjectID: asset.ProjectID, DurationMS: durationMS, Available: true, Audio: true}, nil
}

func (s *Service) resolveNarration(ctx context.Context, principal project.Principal, projectID uuid.UUID) (NarrationSource, error) {
	if s.plans == nil || s.narrations == nil || s.assets == nil {
		return NarrationSource{}, ErrNarrationMissing
	}
	plans, err := s.plans.ListVersions(ctx, principal.OwnerID, projectID)
	if err != nil {
		return NarrationSource{}, err
	}
	var selected *sceneplan.Plan
	for i := range plans {
		if plans[i].Status != sceneplan.StatusApproved {
			continue
		}
		if selected == nil || plans[i].Version > selected.Version {
			selected = &plans[i]
		}
	}
	if selected == nil || len(selected.Scenes) == 0 {
		return NarrationSource{}, ErrNarrationMissing
	}
	bindings, err := s.narrations.ListActiveForPlan(ctx, principal.OwnerID, projectID, selected.Version)
	if err != nil {
		return NarrationSource{}, err
	}
	byScene := make(map[string]scenenarration.Binding, len(bindings))
	for _, binding := range bindings {
		if binding.Status == scenenarration.StatusActive {
			byScene[binding.SceneKey] = binding
		}
	}
	parts := make([]string, 0, len(selected.Scenes)+1)
	parts = append(parts, fmt.Sprintf("plan:%d", selected.Version))
	var totalDurationMS int64
	for _, scene := range selected.Scenes {
		binding, ok := byScene[scene.Key]
		if !ok || binding.ID == uuid.Nil || binding.AssetID == uuid.Nil {
			return NarrationSource{}, ErrNarrationMissing
		}
		asset, err := s.assets.Get(ctx, principal.OwnerID, projectID, binding.AssetID)
		if err != nil {
			if errors.Is(err, mediaasset.ErrNotFound) {
				return NarrationSource{}, ErrNarrationMissing
			}
			return NarrationSource{}, err
		}
		if asset.Kind != mediaasset.KindAudio || asset.DeletionRequestedAt != nil {
			return NarrationSource{}, ErrNarrationMissing
		}
		durationMS, err := durationMSFromMetadata(asset.Metadata)
		if err != nil {
			return NarrationSource{}, ErrNarrationMissing
		}
		if totalDurationMS > math.MaxInt64-durationMS {
			return NarrationSource{}, ErrNarrationMissing
		}
		totalDurationMS += durationMS
		parts = append(parts, strings.Join([]string{scene.Key, binding.ID.String(), asset.ID.String(), fmt.Sprintf("%d", durationMS)}, ":"))
	}
	lineageID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join(parts, "|")))
	if lineageID == uuid.Nil || totalDurationMS <= 0 {
		return NarrationSource{}, ErrNarrationMissing
	}
	return NarrationSource{LineageID: lineageID, ScenePlanVersion: selected.Version, DurationMS: totalDurationMS}, nil
}

func durationMSFromMetadata(raw json.RawMessage) (int64, error) {
	var metadata struct {
		DurationSeconds float64 `json:"duration_seconds"`
	}
	if json.Unmarshal(raw, &metadata) != nil || metadata.DurationSeconds <= 0 || math.IsNaN(metadata.DurationSeconds) || math.IsInf(metadata.DurationSeconds, 0) {
		return 0, ErrInvalidInput
	}
	durationMS := int64(math.Round(metadata.DurationSeconds * 1000))
	if durationMS <= 0 {
		return 0, ErrInvalidInput
	}
	return durationMS, nil
}

func validateIdentity(principal project.Principal, projectID uuid.UUID) error {
	if principal.OwnerID == uuid.Nil {
		return ErrUnauthenticated
	}
	if projectID == uuid.Nil {
		return ErrInvalidInput
	}
	return nil
}
