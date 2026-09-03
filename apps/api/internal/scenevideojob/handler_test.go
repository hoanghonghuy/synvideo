package scenevideojob

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
)

type fakeVideoRuntime struct{ gen providers.VideoGenerator }

func (f fakeVideoRuntime) ResolveVideoGenerator(context.Context, uuid.UUID, providers.ProviderID, providers.ModelID) (providers.VideoGenerator, error) {
	return f.gen, nil
}

type fakeVideoGenerator struct {
	starts    int
	retrieves int
	opens     int
	startErr  error
	operation providers.VideoOperation
	binary    providers.GeneratedBinary
}

func (f *fakeVideoGenerator) StartVideo(context.Context, providers.VideoGenerationRequest) (providers.VideoOperation, error) {
	f.starts++
	if f.startErr != nil {
		return providers.VideoOperation{}, f.startErr
	}
	return f.operation, nil
}

func (f *fakeVideoGenerator) GetVideoOperation(context.Context, string) (providers.VideoOperation, error) {
	f.retrieves++
	return f.operation, nil
}

func (f *fakeVideoGenerator) OpenVideoResult(context.Context, string) (providers.GeneratedBinary, error) {
	f.opens++
	return f.binary, nil
}

type fakeCheckpointStore struct {
	checkpoint OperationCheckpoint
	found      bool
	saves      int
	ambiguous  int
}

func (f *fakeCheckpointStore) Get(context.Context, project.Principal, uuid.UUID, uuid.UUID) (OperationCheckpoint, error) {
	if !f.found {
		return OperationCheckpoint{}, ErrCheckpointNotFound
	}
	return f.checkpoint, nil
}

func (f *fakeCheckpointStore) SaveSubmitted(_ context.Context, _ project.Principal, projectID, jobID uuid.UUID, externalID string) (OperationCheckpoint, error) {
	f.saves++
	f.found = true
	f.checkpoint = OperationCheckpoint{JobID: jobID, ProjectID: projectID, ExternalOperationID: externalID, State: OperationStateSubmitted}
	return f.checkpoint, nil
}

func (f *fakeCheckpointStore) SaveAmbiguous(_ context.Context, _ project.Principal, projectID, jobID uuid.UUID) (OperationCheckpoint, error) {
	f.ambiguous++
	f.found = true
	f.checkpoint = OperationCheckpoint{JobID: jobID, ProjectID: projectID, State: OperationStateAmbiguous}
	return f.checkpoint, nil
}

type fakeAssetStore struct{ asset mediaasset.MediaAsset }

func (f *fakeAssetStore) FindGeneratedByJob(context.Context, project.Principal, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error) {
	if f.asset.ID == uuid.Nil {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return f.asset, nil
}

func (f *fakeAssetStore) Store(_ context.Context, principal project.Principal, projectID uuid.UUID, input mediaasset.CreateInput) (mediaasset.MediaAsset, error) {
	_, _ = io.ReadAll(input.Reader)
	f.asset = mediaasset.MediaAsset{ID: uuid.New(), OwnerID: principal.OwnerID, ProjectID: projectID, Kind: input.Kind, Origin: input.Origin, MimeType: input.MimeType, Metadata: input.Metadata}
	return f.asset, nil
}

type fakeBinder struct{}

func (fakeBinder) GetCurrent(context.Context, project.Principal, uuid.UUID, int, string) (scenemedia.Binding, error) {
	return scenemedia.Binding{}, scenemedia.ErrNotFound
}

func (fakeBinder) AssignPrimaryVisual(context.Context, project.Principal, uuid.UUID, int, string, uuid.UUID) (scenemedia.Binding, error) {
	return scenemedia.Binding{}, nil
}

func makeVideoJob(t *testing.T) jobs.Job {
	t.Helper()
	projectID := uuid.New()
	payload, err := json.Marshal(Payload{SchemaVersion: SchemaVersion, ProviderID: "runway", ModelID: "gen4", ScenePlanVersion: 1, SceneKey: "scene-1", Prompt: "camera pushes in", AspectRatio: "16:9"})
	if err != nil {
		t.Fatal(err)
	}
	return jobs.Job{ID: uuid.New(), OwnerID: uuid.New(), ProjectID: &projectID, Kind: JobKind, Payload: payload}
}

func TestHandlePersistsExternalOperationBeforePollingAndReusesIt(t *testing.T) {
	binary, err := providers.NewGeneratedBinary("video/mp4", []byte("video"))
	if err != nil {
		t.Fatal(err)
	}
	gen := &fakeVideoGenerator{operation: providers.VideoOperation{ID: "op-123", State: providers.VideoOperationRunning}, binary: binary}
	checkpoints := &fakeCheckpointStore{}
	h := NewHandler(fakeVideoRuntime{gen: gen}, checkpoints, &fakeAssetStore{}, fakeBinder{})
	job := makeVideoJob(t)

	_, err = h.Handle(context.Background(), job)
	var retry *jobs.RetryableJobError
	if !errors.As(err, &retry) {
		t.Fatalf("expected polling retry, got %v", err)
	}
	if gen.starts != 1 || checkpoints.saves != 1 {
		t.Fatalf("start=%d saves=%d", gen.starts, checkpoints.saves)
	}
	if gen.retrieves != 0 {
		t.Fatalf("must not poll before durable checkpoint, retrieves=%d", gen.retrieves)
	}

	gen.operation.State = providers.VideoOperationSucceeded
	resultJSON, err := h.Handle(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	if gen.starts != 1 {
		t.Fatalf("retry submitted paid generation again: starts=%d", gen.starts)
	}
	if gen.retrieves != 1 || gen.opens != 1 {
		t.Fatalf("retrieves=%d opens=%d", gen.retrieves, gen.opens)
	}
	var result Result
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.MediaAssetID == uuid.Nil {
		t.Fatal("missing generated video asset")
	}
}

func TestHandleAmbiguousSubmitNeverAutomaticallyResubmits(t *testing.T) {
	gen := &fakeVideoGenerator{startErr: providers.NewAmbiguousSubmitError(errors.New("connection closed after request write"))}
	checkpoints := &fakeCheckpointStore{}
	h := NewHandler(fakeVideoRuntime{gen: gen}, checkpoints, &fakeAssetStore{}, fakeBinder{})
	job := makeVideoJob(t)

	_, err := h.Handle(context.Background(), job)
	var terminal *jobs.TerminalJobError
	if !errors.As(err, &terminal) || terminal.Code != ErrorAmbiguousSubmit {
		t.Fatalf("expected ambiguous terminal state, got %v", err)
	}
	if gen.starts != 1 || checkpoints.ambiguous != 1 {
		t.Fatalf("starts=%d ambiguous=%d", gen.starts, checkpoints.ambiguous)
	}

	_, err = h.Handle(context.Background(), job)
	if !errors.As(err, &terminal) || terminal.Code != ErrorAmbiguousSubmit {
		t.Fatalf("expected persisted ambiguous state, got %v", err)
	}
	if gen.starts != 1 {
		t.Fatalf("ambiguous retry submitted again: starts=%d", gen.starts)
	}
}

func TestHandleSucceededOperationStoresGeneratedVideoProvenance(t *testing.T) {
	binary, _ := providers.NewGeneratedBinary("video/mp4", []byte("video"))
	gen := &fakeVideoGenerator{operation: providers.VideoOperation{ID: "op-456", State: providers.VideoOperationSucceeded}, binary: binary}
	job := makeVideoJob(t)
	checkpoints := &fakeCheckpointStore{found: true, checkpoint: OperationCheckpoint{JobID: job.ID, ProjectID: *job.ProjectID, ExternalOperationID: "op-456", State: OperationStateSubmitted}}
	assets := &fakeAssetStore{}
	h := NewHandler(fakeVideoRuntime{gen: gen}, checkpoints, assets, fakeBinder{})

	_, err := h.Handle(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	if assets.asset.Kind != mediaasset.KindVideo || assets.asset.Origin != mediaasset.OriginGeneratedVideo {
		t.Fatalf("wrong asset type: %#v", assets.asset)
	}
	metadata := string(assets.asset.Metadata)
	for _, want := range []string{"op-456", "runway", "gen4", job.ID.String()} {
		if !strings.Contains(metadata, want) {
			t.Fatalf("metadata missing %q: %s", want, metadata)
		}
	}
}
