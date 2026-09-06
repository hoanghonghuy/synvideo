package sceneeditor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryRepository struct {
	latest    Document
	history   map[int]Document
	snapshots map[string]Snapshot
}

func (r *memoryRepository) GetLatest(context.Context, uuid.UUID, uuid.UUID) (Document, error) {
	if r.latest.ID == uuid.Nil {
		return Document{}, ErrNotFound
	}
	return r.latest, nil
}

func (r *memoryRepository) GetRevision(_ context.Context, _ uuid.UUID, _ uuid.UUID, revision int) (Document, error) {
	doc, ok := r.history[revision]
	if !ok {
		return Document{}, ErrNotFound
	}
	return doc, nil
}

func (r *memoryRepository) CreateInitial(_ context.Context, doc Document) (Document, error) {
	if r.latest.ID != uuid.Nil {
		return Document{}, ErrConflict
	}
	r.latest = doc
	if r.history == nil {
		r.history = map[int]Document{}
	}
	r.history[doc.Revision] = doc
	return doc, nil
}

func (r *memoryRepository) CreateRevision(_ context.Context, doc Document, expectedRevision int) (Document, error) {
	if r.latest.Revision != expectedRevision {
		return Document{}, ErrConflict
	}
	r.latest = doc
	if r.history == nil {
		r.history = map[int]Document{}
	}
	r.history[doc.Revision] = doc
	return doc, nil
}

func (r *memoryRepository) CreateSnapshot(_ context.Context, _ uuid.UUID, snapshot Snapshot) (Snapshot, error) {
	if r.snapshots == nil {
		r.snapshots = map[string]Snapshot{}
	}
	if existing, ok := r.snapshots[snapshot.Digest]; ok {
		return existing, nil
	}
	r.snapshots[snapshot.Digest] = snapshot
	return snapshot, nil
}

func (r *memoryRepository) GetSnapshot(_ context.Context, _ uuid.UUID, _ uuid.UUID, digest string) (Snapshot, error) {
	snapshot, ok := r.snapshots[digest]
	if !ok {
		return Snapshot{}, ErrNotFound
	}
	return snapshot, nil
}

type staticResolver struct {
	states []DependencyState
	err    error
}

func (r staticResolver) State(context.Context, uuid.UUID, Document) ([]DependencyState, error) {
	return r.states, r.err
}

func baseScene() Scene {
	return Scene{
		ID: uuid.New(), SceneKey: "scene-a", DurationMS: 2_000,
		VisualTreatment: VisualTreatment{Fit: FitContain, Scale: 1},
		TransitionOut:   Transition{Kind: TransitionCut},
	}
}

func TestServiceCreateSaveAndSnapshot(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	clock := time.Date(2026, 9, 6, 5, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	idIndex := 0
	repo := &memoryRepository{}
	service := NewService(repo, staticResolver{states: []DependencyState{{State: StateCurrent}}}, func() uuid.UUID {
		id := ids[idIndex]
		idIndex++
		return id
	}, func() time.Time { return clock })

	created, err := service.Create(ctx, ownerID, projectID, 3, []Scene{baseScene()}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.State != StateCurrent || created.Revision != 1 {
		t.Fatalf("created=%+v", created)
	}

	duplicated, err := service.Duplicate(ctx, ownerID, projectID, created.Scenes[0].ID, 1)
	if err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if duplicated.Revision != 2 || len(duplicated.Scenes) != 2 {
		t.Fatalf("duplicated=%+v", duplicated)
	}
	if duplicated.Scenes[1].ID != ids[1] {
		t.Fatalf("duplicate id=%s want %s", duplicated.Scenes[1].ID, ids[1])
	}

	snapshot, err := service.Snapshot(ctx, ownerID, projectID, 2)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snapshot.Digest == "" || snapshot.Revision != 2 {
		t.Fatalf("snapshot=%+v", snapshot)
	}

	again, err := service.Snapshot(ctx, ownerID, projectID, 2)
	if err != nil {
		t.Fatalf("Snapshot retry: %v", err)
	}
	if again.Digest != snapshot.Digest {
		t.Fatalf("retry digest=%s want %s", again.Digest, snapshot.Digest)
	}
}

func TestServiceRejectsStaleWriter(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	repo := &memoryRepository{}
	service := NewService(repo, staticResolver{states: []DependencyState{{State: StateCurrent}}}, uuid.New, func() time.Time { return time.Now().UTC() })

	created, err := service.Create(ctx, ownerID, projectID, 1, []Scene{baseScene()}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := service.Duplicate(ctx, ownerID, projectID, created.Scenes[0].ID, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("err=%v want conflict", err)
	}
}

func TestServiceBlocksSnapshotForStaleOrBrokenDependencies(t *testing.T) {
	for _, state := range []State{StateStale, StateBroken} {
		t.Run(string(state), func(t *testing.T) {
			ctx := context.Background()
			ownerID := uuid.New()
			projectID := uuid.New()
			repo := &memoryRepository{}
			service := NewService(repo, staticResolver{states: []DependencyState{{State: state}}}, uuid.New, func() time.Time { return time.Now().UTC() })
			created, err := service.Create(ctx, ownerID, projectID, 1, []Scene{baseScene()}, nil)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if _, err := service.Snapshot(ctx, ownerID, projectID, created.Revision); !errors.Is(err, ErrSnapshotBlocked) {
				t.Fatalf("err=%v want snapshot blocked", err)
			}
		})
	}
}

func TestServicePreservesCompositionIdentityOnSave(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	projectID := uuid.New()
	repo := &memoryRepository{}
	service := NewService(repo, staticResolver{states: []DependencyState{{State: StateCurrent}}}, uuid.New, func() time.Time { return time.Now().UTC() })
	created, err := service.Create(ctx, ownerID, projectID, 1, []Scene{baseScene()}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = service.Save(ctx, ownerID, projectID, 1, func(doc Document) (Document, error) {
		doc.ProjectID = uuid.New()
		return doc, nil
	})
	validation, ok := err.(ValidationError)
	if !ok || validation.Fields["composition_identity"] != "immutable" {
		t.Fatalf("err=%T %v", err, err)
	}
}
