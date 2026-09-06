package sceneeditor

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("scene editor composition not found")
	ErrUnauthenticated = errors.New("scene editor principal is required")
	ErrPersistence     = errors.New("scene editor persistence failed")
)

type IDGenerator func() uuid.UUID

type Clock func() time.Time

type Service struct {
	repo     Repository
	resolver DependencyResolver
	newID    IDGenerator
	now      Clock
}

func NewService(repo Repository, resolver DependencyResolver, newID IDGenerator, now Clock) *Service {
	if newID == nil {
		newID = uuid.New
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, resolver: resolver, newID: newID, now: now}
}

func (s *Service) Create(ctx context.Context, ownerID, projectID uuid.UUID, scenePlanVersion int, scenes []Scene, audioMix *AudioMixRef) (View, error) {
	if ownerID == uuid.Nil {
		return View{}, ErrUnauthenticated
	}
	doc, err := NewDocument(s.newID(), ownerID, projectID, scenePlanVersion, scenes, audioMix, s.now().UTC())
	if err != nil {
		return View{}, err
	}
	created, err := s.repo.CreateInitial(ctx, doc)
	if err != nil {
		return View{}, normalizeRepoError(err)
	}
	return s.view(ctx, ownerID, created)
}

func (s *Service) Get(ctx context.Context, ownerID, projectID uuid.UUID) (View, error) {
	if ownerID == uuid.Nil {
		return View{}, ErrUnauthenticated
	}
	doc, err := s.repo.GetLatest(ctx, ownerID, projectID)
	if err != nil {
		return View{}, normalizeRepoError(err)
	}
	return s.view(ctx, ownerID, doc)
}

func (s *Service) Save(ctx context.Context, ownerID, projectID uuid.UUID, expectedRevision int, mutate func(Document) (Document, error)) (View, error) {
	if ownerID == uuid.Nil {
		return View{}, ErrUnauthenticated
	}
	doc, err := s.repo.GetLatest(ctx, ownerID, projectID)
	if err != nil {
		return View{}, normalizeRepoError(err)
	}
	if doc.Revision != expectedRevision {
		return View{}, ErrConflict
	}

	updated, err := mutate(doc)
	if err != nil {
		return View{}, err
	}
	if updated.ID != doc.ID || updated.OwnerID != doc.OwnerID || updated.ProjectID != doc.ProjectID || updated.ScenePlanVersion != doc.ScenePlanVersion {
		return View{}, ValidationError{Fields: map[string]string{"composition_identity": "immutable"}}
	}
	updated.Revision = doc.Revision + 1
	updated.CreatedAt = doc.CreatedAt
	updated.UpdatedAt = s.now().UTC()
	if err := ValidateDocument(updated); err != nil {
		return View{}, err
	}

	saved, err := s.repo.CreateRevision(ctx, updated, expectedRevision)
	if err != nil {
		return View{}, normalizeRepoError(err)
	}
	return s.view(ctx, ownerID, saved)
}

func (s *Service) Reorder(ctx context.Context, ownerID, projectID, sceneID uuid.UUID, to, expectedRevision int) (View, error) {
	return s.Save(ctx, ownerID, projectID, expectedRevision, func(doc Document) (Document, error) {
		if to < 0 || to >= len(doc.Scenes) {
			return Document{}, ValidationError{Fields: map[string]string{"to": "out_of_range"}}
		}
		idx := sceneIndex(doc.Scenes, sceneID)
		if idx < 0 {
			return Document{}, ErrSceneNotFound
		}
		scenes := cloneScenes(doc.Scenes)
		moved := scenes[idx]
		scenes = append(scenes[:idx], scenes[idx+1:]...)
		scenes = append(scenes, Scene{})
		copy(scenes[to+1:], scenes[to:])
		scenes[to] = moved
		doc.Scenes = scenes
		return doc, nil
	})
}

func (s *Service) Duplicate(ctx context.Context, ownerID, projectID, sceneID uuid.UUID, expectedRevision int) (View, error) {
	newSceneID := s.newID()
	return s.Save(ctx, ownerID, projectID, expectedRevision, func(doc Document) (Document, error) {
		idx := sceneIndex(doc.Scenes, sceneID)
		if idx < 0 {
			return Document{}, ErrSceneNotFound
		}
		if newSceneID == uuid.Nil || sceneIndex(doc.Scenes, newSceneID) >= 0 {
			return Document{}, ValidationError{Fields: map[string]string{"new_scene_id": "invalid"}}
		}
		scenes := cloneScenes(doc.Scenes)
		copyScene := scenes[idx]
		copyScene.ID = newSceneID
		scenes = append(scenes, Scene{})
		copy(scenes[idx+2:], scenes[idx+1:])
		scenes[idx+1] = copyScene
		doc.Scenes = scenes
		return doc, nil
	})
}

func (s *Service) Remove(ctx context.Context, ownerID, projectID, sceneID uuid.UUID, expectedRevision int) (View, error) {
	return s.Save(ctx, ownerID, projectID, expectedRevision, func(doc Document) (Document, error) {
		if len(doc.Scenes) == 1 {
			return Document{}, ErrLastScene
		}
		idx := sceneIndex(doc.Scenes, sceneID)
		if idx < 0 {
			return Document{}, ErrSceneNotFound
		}
		scenes := cloneScenes(doc.Scenes)
		doc.Scenes = append(scenes[:idx], scenes[idx+1:]...)
		return doc, nil
	})
}

func (s *Service) Snapshot(ctx context.Context, ownerID, projectID uuid.UUID, expectedRevision int) (Snapshot, error) {
	if ownerID == uuid.Nil {
		return Snapshot{}, ErrUnauthenticated
	}
	doc, err := s.repo.GetLatest(ctx, ownerID, projectID)
	if err != nil {
		return Snapshot{}, normalizeRepoError(err)
	}
	if doc.Revision != expectedRevision {
		return Snapshot{}, ErrConflict
	}
	view, err := s.view(ctx, ownerID, doc)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, err := NewSnapshot(doc, view.State)
	if err != nil {
		return Snapshot{}, err
	}
	persisted, err := s.repo.CreateSnapshot(ctx, ownerID, snapshot)
	if err != nil {
		return Snapshot{}, normalizeRepoError(err)
	}
	return persisted, nil
}

func (s *Service) view(ctx context.Context, ownerID uuid.UUID, doc Document) (View, error) {
	states, err := s.resolver.State(ctx, ownerID, doc)
	if err != nil {
		return View{}, err
	}
	return View{Document: doc, State: StateForDependencies(states...)}, nil
}

func normalizeRepoError(err error) error {
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
		return err
	}
	return errors.Join(ErrPersistence, err)
}
