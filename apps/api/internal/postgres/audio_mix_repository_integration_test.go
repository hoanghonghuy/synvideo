package postgres

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/audiomix"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

func TestAudioMixRepositoryPostgresIntegration(t *testing.T) {
	pool := integrationPool(t)
	storage := integrationStorage(t)
	ctx := context.Background()

	projectRepo := NewProjectRepository(pool)
	assetRepo := NewMediaAssetRepository(pool)
	assetService := mediaasset.NewService(projectRepo, assetRepo, storage)
	mixRepo := NewAudioMixRepository(pool)

	ownerID := uuid.New()
	proj, err := projectRepo.Create(ctx, ownerID, validIntegrationCreateInput("Audio Mix Test Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	audioBytes := makeIntegrationWAV(16000, 1, make([]byte, 32000))
	music, err := assetService.Store(ctx, project.Principal{OwnerID: ownerID}, proj.ID, mediaasset.CreateInput{
		Kind:     mediaasset.KindAudio,
		Origin:   mediaasset.OriginUpload,
		MimeType: "audio/wav",
		Reader:   bytes.NewReader(audioBytes),
		MaxBytes: 10 << 20,
		Metadata: []byte(`{"duration_seconds":90}`),
	})
	if err != nil {
		t.Fatalf("store music asset: %v", err)
	}

	nonAudio, err := assetService.Store(ctx, project.Principal{OwnerID: ownerID}, proj.ID, mediaasset.CreateInput{
		Kind:     mediaasset.KindImage,
		Origin:   mediaasset.OriginUpload,
		MimeType: "image/png",
		Reader:   bytes.NewReader([]byte("not-a-real-image-but-valid-storage-bytes")),
		MaxBytes: 10 << 20,
	})
	if err != nil {
		t.Fatalf("store non-audio asset: %v", err)
	}

	config := audiomix.Config{
		LoopPolicy:      audiomix.LoopToTarget,
		MusicGainDB:     -12,
		NarrationGainDB: 0,
		Ducking: audiomix.Ducking{
			Enabled:     true,
			ReductionDB: 9,
			AttackMS:    120,
			ReleaseMS:   350,
		},
	}
	now := time.Now().UTC()
	doc, err := audiomix.NewDocument(
		uuid.New(),
		ownerID,
		proj.ID,
		audiomix.MusicSource{AssetID: music.ID, ProjectID: proj.ID, DurationMS: 90_000, Available: true, Audio: true},
		audiomix.NarrationSource{LineageID: uuid.New(), ScenePlanVersion: 1, DurationMS: 60_000},
		config,
		now,
	)
	if err != nil {
		t.Fatalf("new document: %v", err)
	}

	created, err := mixRepo.CreateInitial(ctx, doc)
	if err != nil {
		t.Fatalf("create initial: %v", err)
	}
	if created.Revision != 1 || created.ID != doc.ID || created.MusicAssetID != music.ID {
		t.Fatalf("unexpected initial document: %+v", created)
	}

	revision2 := created
	revision2.Revision = 2
	revision2.Config.MusicGainDB = -15
	revision2.UpdatedAt = now.Add(time.Second)
	updated, err := mixRepo.CreateRevision(ctx, revision2, 1)
	if err != nil {
		t.Fatalf("create revision 2: %v", err)
	}
	if updated.Revision != 2 || updated.Config.MusicGainDB != -15 {
		t.Fatalf("unexpected revision 2: %+v", updated)
	}

	staleWrite := updated
	staleWrite.Revision = 2
	staleWrite.UpdatedAt = now.Add(2 * time.Second)
	if _, err := mixRepo.CreateRevision(ctx, staleWrite, 1); !errors.Is(err, audiomix.ErrConflict) {
		t.Fatalf("stale write error = %v, want ErrConflict", err)
	}

	wrongKind := updated
	wrongKind.Revision = 3
	wrongKind.MusicAssetID = nonAudio.ID
	wrongKind.UpdatedAt = now.Add(3 * time.Second)
	if _, err := mixRepo.CreateRevision(ctx, wrongKind, 2); !errors.Is(err, audiomix.ErrMusicMissing) {
		t.Fatalf("non-audio write error = %v, want ErrMusicMissing", err)
	}

	latest, err := mixRepo.GetLatest(ctx, ownerID, proj.ID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if latest.Revision != 2 || latest.MusicAssetID != music.ID {
		t.Fatalf("latest changed after rejected writes: %+v", latest)
	}

	history, err := mixRepo.ListHistory(ctx, ownerID, proj.ID)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(history) != 2 || history[0].Revision != 2 || history[1].Revision != 1 {
		t.Fatalf("unexpected history: %+v", history)
	}
}
