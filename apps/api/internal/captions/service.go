package captions

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenenarration"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type ScenePlanRepository interface {
	GetByVersion(ctx context.Context, ownerID, projectID uuid.UUID, version int) (sceneplan.Plan, error)
}

type NarrationRepository interface {
	GetActive(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, sceneKey string) (scenenarration.Binding, error)
}

type MediaAssetRepository interface {
	Get(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error)
}

type Service struct {
	repo       Repository
	plans      ScenePlanRepository
	narrations NarrationRepository
	assets     MediaAssetRepository
}

func NewService(repo Repository, plans ScenePlanRepository, narrations NarrationRepository, assets MediaAssetRepository) *Service {
	return &Service{repo: repo, plans: plans, narrations: narrations, assets: assets}
}

type UpdateInput struct {
	ExpectedRevision int       `json:"expected_revision"`
	Segments         []Segment `json:"segments"`
	Style            Style     `json:"style"`
}

func (s *Service) Derive(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) (View, error) {
	if err := validateIdentity(principal, projectID, planVersion, sceneKey); err != nil { return View{}, err }
	if s.repo == nil { return View{}, ErrPersistence }
	if _, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID, planVersion, sceneKey); err == nil {
		return View{}, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return View{}, err
	}
	source, err := s.resolveSource(ctx, principal, projectID, planVersion, sceneKey)
	if err != nil { return View{}, err }
	segments, err := InitialSegments(source.Text, source.DurationMS)
	if err != nil { return View{}, err }
	doc, err := s.repo.CreateInitial(ctx, Document{
		ID: uuid.New(), OwnerID: principal.OwnerID, ProjectID: projectID,
		ScenePlanVersion: planVersion, SceneKey: sceneKey, Revision: 1,
		SourceBindingID: source.BindingID, SourceAssetID: source.AssetID,
		SourceDurationMS: source.DurationMS, Segments: segments, Style: DefaultStyle(),
	})
	if err != nil { return View{}, err }
	return View{Document: doc, State: StateCurrent}, nil
}

func (s *Service) Get(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) (View, error) {
	if err := validateIdentity(principal, projectID, planVersion, sceneKey); err != nil { return View{}, err }
	if s.repo == nil { return View{}, ErrPersistence }
	doc, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID, planVersion, sceneKey)
	if err != nil { return View{}, err }
	state := StateStale
	if source, sourceErr := s.resolveSource(ctx, principal, projectID, planVersion, sceneKey); sourceErr == nil {
		state = StateForSource(doc, source)
	} else if !errors.Is(sourceErr, ErrSourceMissing) {
		return View{}, sourceErr
	}
	return View{Document: doc, State: state}, nil
}

func (s *Service) Update(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string, input UpdateInput) (View, error) {
	if err := validateIdentity(principal, projectID, planVersion, sceneKey); err != nil { return View{}, err }
	if input.ExpectedRevision < 1 { return View{}, ErrInvalidInput }
	if s.repo == nil { return View{}, ErrPersistence }
	latest, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID, planVersion, sceneKey)
	if err != nil { return View{}, err }
	if latest.Revision != input.ExpectedRevision { return View{}, ErrConflict }
	segments, err := NormalizeSegments(input.Segments, latest.SourceDurationMS)
	if err != nil { return View{}, err }
	style, err := NormalizeStyle(input.Style)
	if err != nil { return View{}, err }
	latest.ID = uuid.New()
	latest.Revision++
	latest.Segments = segments
	latest.Style = style
	latest.CreatedAt = latest.CreatedAt.UTC()
	updated, err := s.repo.CreateRevision(ctx, latest, input.ExpectedRevision)
	if err != nil { return View{}, err }
	state := StateStale
	if source, sourceErr := s.resolveSource(ctx, principal, projectID, planVersion, sceneKey); sourceErr == nil {
		state = StateForSource(updated, source)
	} else if !errors.Is(sourceErr, ErrSourceMissing) { return View{}, sourceErr }
	return View{Document: updated, State: state}, nil
}

func (s *Service) Rebuild(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string, expectedRevision int) (View, error) {
	if err := validateIdentity(principal, projectID, planVersion, sceneKey); err != nil { return View{}, err }
	if expectedRevision < 1 || s.repo == nil { return View{}, ErrInvalidInput }
	latest, err := s.repo.GetLatest(ctx, principal.OwnerID, projectID, planVersion, sceneKey)
	if err != nil { return View{}, err }
	if latest.Revision != expectedRevision { return View{}, ErrConflict }
	source, err := s.resolveSource(ctx, principal, projectID, planVersion, sceneKey)
	if err != nil { return View{}, err }
	segments, err := InitialSegments(source.Text, source.DurationMS)
	if err != nil { return View{}, err }
	doc, err := s.repo.CreateRevision(ctx, Document{
		ID: uuid.New(), OwnerID: principal.OwnerID, ProjectID: projectID,
		ScenePlanVersion: planVersion, SceneKey: sceneKey, Revision: expectedRevision + 1,
		SourceBindingID: source.BindingID, SourceAssetID: source.AssetID, SourceDurationMS: source.DurationMS,
		Segments: segments, Style: latest.Style,
	}, expectedRevision)
	if err != nil { return View{}, err }
	return View{Document: doc, State: StateCurrent}, nil
}

func (s *Service) Snapshot(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) (Snapshot, error) {
	view, err := s.Get(ctx, principal, projectID, planVersion, sceneKey)
	if err != nil { return Snapshot{}, err }
	return NewSnapshot(view.Document, view.State)
}

func (s *Service) History(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) ([]Document, error) {
	if err := validateIdentity(principal, projectID, planVersion, sceneKey); err != nil { return nil, err }
	if s.repo == nil { return nil, ErrPersistence }
	return s.repo.ListHistory(ctx, principal.OwnerID, projectID, planVersion, sceneKey)
}

func (s *Service) resolveSource(ctx context.Context, principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) (Source, error) {
	if s.plans == nil || s.narrations == nil || s.assets == nil { return Source{}, ErrSourceMissing }
	plan, err := s.plans.GetByVersion(ctx, principal.OwnerID, projectID, planVersion)
	if err != nil { return Source{}, err }
	if plan.Status != sceneplan.StatusApproved { return Source{}, ErrSourceMissing }
	var text string
	for _, scene := range plan.Scenes { if scene.Key == sceneKey { text = strings.TrimSpace(scene.Narration); break } }
	if text == "" { return Source{}, ErrSourceMissing }
	binding, err := s.narrations.GetActive(ctx, principal.OwnerID, projectID, planVersion, sceneKey)
	if err != nil {
		if errors.Is(err, scenenarration.ErrNotFound) { return Source{}, ErrSourceMissing }
		return Source{}, err
	}
	asset, err := s.assets.Get(ctx, principal.OwnerID, projectID, binding.AssetID)
	if err != nil {
		if errors.Is(err, mediaasset.ErrNotFound) { return Source{}, ErrSourceMissing }
		return Source{}, err
	}
	if asset.Kind != mediaasset.KindAudio { return Source{}, ErrSourceMissing }
	var metadata struct { DurationSeconds float64 `json:"duration_seconds"` }
	if json.Unmarshal(asset.Metadata, &metadata) != nil || metadata.DurationSeconds <= 0 || math.IsNaN(metadata.DurationSeconds) || math.IsInf(metadata.DurationSeconds, 0) {
		return Source{}, ErrSourceMissing
	}
	durationMS := int64(math.Round(metadata.DurationSeconds * 1000))
	if durationMS <= 0 { return Source{}, ErrSourceMissing }
	return Source{BindingID: binding.ID, AssetID: asset.ID, DurationMS: durationMS, Text: text}, nil
}

func validateIdentity(principal project.Principal, projectID uuid.UUID, planVersion int, sceneKey string) error {
	if principal.OwnerID == uuid.Nil { return ErrUnauthenticated }
	if projectID == uuid.Nil || planVersion < 1 || strings.TrimSpace(sceneKey) == "" { return ErrInvalidInput }
	return nil
}
