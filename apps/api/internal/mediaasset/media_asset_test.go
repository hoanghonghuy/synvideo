package mediaasset_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

var (
	ownerID   = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectID = uuid.MustParse("22222222-2222-4222-8222-222222222222")
)

func validCreateInput() mediaasset.CreateInput {
	return mediaasset.CreateInput{
		Kind:             mediaasset.KindImage,
		Origin:           mediaasset.OriginUpload,
		MimeType:         "image/png",
		OriginalFilename: "cover.png",
		Metadata:         []byte(`{"width":1920,"height":1080}`),
		Reader:           strings.NewReader("media bytes"),
	}
}

func validProject() project.Project {
	return project.Project{ID: projectID, OwnerID: ownerID, Status: project.StatusActive}
}

func TestCreateInputValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*mediaasset.CreateInput)
	}{
		{"missing kind", func(in *mediaasset.CreateInput) { in.Kind = "" }},
		{"invalid kind", func(in *mediaasset.CreateInput) { in.Kind = "spreadsheet" }},
		{"missing origin", func(in *mediaasset.CreateInput) { in.Origin = "" }},
		{"invalid origin", func(in *mediaasset.CreateInput) { in.Origin = "provider" }},
		{"missing mime type", func(in *mediaasset.CreateInput) { in.MimeType = "  " }},
		{"mime type too long", func(in *mediaasset.CreateInput) { in.MimeType = strings.Repeat("x", 256) }},
		{"filename too long", func(in *mediaasset.CreateInput) { in.OriginalFilename = strings.Repeat("ế", 501) }},
		{"reader required", func(in *mediaasset.CreateInput) { in.Reader = nil }},
		{"metadata must be object", func(in *mediaasset.CreateInput) { in.Metadata = []byte(`[]`) }},
		{"metadata malformed", func(in *mediaasset.CreateInput) { in.Metadata = []byte(`{`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := validCreateInput()
			tc.edit(&input)
			if err := input.NormalizeAndValidate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCreateInputValidationNormalizesWithoutLeakingMutableMetadata(t *testing.T) {
	input := validCreateInput()
	if err := input.NormalizeAndValidate(); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if input.MimeType != "image/png" || string(input.Metadata) != `{"width":1920,"height":1080}` {
		t.Fatalf("unexpected normalized input: %+v", input)
	}
}

func TestObjectKeyValidationRejectsTraversalAndExternalSelectors(t *testing.T) {
	valid := "projects/22222222-2222-4222-8222-222222222222/assets/33333333-3333-4333-8333-333333333333"
	if err := mediaasset.ValidateObjectKey(valid); err != nil {
		t.Fatalf("expected canonical key to pass: %v", err)
	}
	for _, key := range []string{"", "../secret", "/absolute", "https://example.test/object", "projects/x/../secret", "projects/x/assets/"} {
		if err := mediaasset.ValidateObjectKey(key); err == nil {
			t.Errorf("expected key %q to fail", key)
		}
	}
}

func TestAssetValidationRejectsObjectKeyIdentityMismatch(t *testing.T) {
	asset := validAssetForValidation()

	for name, key := range map[string]string{
		"project": "projects/33333333-3333-4333-8333-333333333333/assets/" + asset.ID.String(),
		"asset":   "projects/" + asset.ProjectID.String() + "/assets/44444444-4444-4444-8444-444444444444",
	} {
		t.Run(name, func(t *testing.T) {
			asset.ObjectKey = key
			if err := asset.Validate(); err == nil {
				t.Fatal("expected object key identity validation error")
			}
		})
	}
}

func validAssetForValidation() mediaasset.MediaAsset {
	assetID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	return mediaasset.MediaAsset{
		ID: assetID, OwnerID: ownerID, ProjectID: projectID,
		Kind: mediaasset.KindImage, Origin: mediaasset.OriginUpload,
		ObjectKey: "projects/" + projectID.String() + "/assets/" + assetID.String(),
		MimeType:  "image/png", ByteSize: 1, SHA256: strings.Repeat("a", 64), Metadata: []byte(`{}`),
	}
}

type fakeProjectRepository struct {
	item project.Project
	err  error
}

func (f fakeProjectRepository) Create(context.Context, uuid.UUID, project.CreateInput) (project.Project, error) {
	panic("unused")
}
func (f fakeProjectRepository) List(context.Context, uuid.UUID, project.ListOptions) (project.ListResult, error) {
	panic("unused")
}
func (f fakeProjectRepository) Get(context.Context, uuid.UUID, uuid.UUID) (project.Project, error) {
	if f.err != nil {
		return project.Project{}, f.err
	}
	return f.item, nil
}
func (f fakeProjectRepository) Update(context.Context, uuid.UUID, uuid.UUID, project.UpdateInput) (project.Project, error) {
	panic("unused")
}

type fakeMetadataRepository struct {
	created       mediaasset.MediaAsset
	createErr     error
	assets        []mediaasset.MediaAsset
	getErr        error
	deleteErr     error
	hasReferences bool
	deleteCalls   int
}

func (f *fakeMetadataRepository) Create(_ context.Context, asset mediaasset.MediaAsset) (mediaasset.MediaAsset, error) {
	if f.createErr != nil {
		return mediaasset.MediaAsset{}, f.createErr
	}
	f.created = asset
	return asset, nil
}
func (f *fakeMetadataRepository) Get(_ context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	if f.getErr != nil {
		return mediaasset.MediaAsset{}, f.getErr
	}
	if f.created.ID == uuid.Nil || f.created.ID != assetID || f.created.OwnerID != ownerID || f.created.ProjectID != projectID {
		return mediaasset.MediaAsset{}, mediaasset.ErrNotFound
	}
	return f.created, nil
}
func (f *fakeMetadataRepository) List(context.Context, uuid.UUID, uuid.UUID, mediaasset.ListOptions) (mediaasset.ListResult, error) {
	return mediaasset.ListResult{Assets: append([]mediaasset.MediaAsset(nil), f.assets...)}, nil
}
func (f *fakeMetadataRepository) Delete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	f.deleteCalls++
	return f.deleteErr
}
func (f *fakeMetadataRepository) HasReferences(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return f.hasReferences, nil
}

type fakeObjectStorage struct {
	putErr      error
	deleteErr   error
	putCalls    int
	openCalls   int
	deleteCalls int
	lastPut     mediaasset.PutObjectInput
}

type fakeDeletionRepository struct {
	base          *fakeMetadataRepository
	beginErr      error
	finalizeErr   error
	beginCalls    int
	finalizeCalls int
}

func (f *fakeDeletionRepository) Create(ctx context.Context, asset mediaasset.MediaAsset) (mediaasset.MediaAsset, error) {
	return f.base.Create(ctx, asset)
}
func (f *fakeDeletionRepository) Get(ctx context.Context, ownerID, projectID, assetID uuid.UUID) (mediaasset.MediaAsset, error) {
	return f.base.Get(ctx, ownerID, projectID, assetID)
}
func (f *fakeDeletionRepository) List(ctx context.Context, ownerID, projectID uuid.UUID, options mediaasset.ListOptions) (mediaasset.ListResult, error) {
	return f.base.List(ctx, ownerID, projectID, options)
}
func (f *fakeDeletionRepository) Delete(ctx context.Context, ownerID, projectID, assetID uuid.UUID) error {
	return f.base.Delete(ctx, ownerID, projectID, assetID)
}
func (f *fakeDeletionRepository) BeginDeletion(_ context.Context, _, _, _ uuid.UUID) (mediaasset.MediaAsset, error) {
	f.beginCalls++
	if f.beginErr != nil {
		return mediaasset.MediaAsset{}, f.beginErr
	}
	return f.base.created, nil
}
func (f *fakeDeletionRepository) FinalizeDeletion(_ context.Context, _, _, _ uuid.UUID) error {
	f.finalizeCalls++
	return f.finalizeErr
}

func (f *fakeObjectStorage) Put(_ context.Context, input mediaasset.PutObjectInput) (mediaasset.ObjectInfo, error) {
	f.putCalls++
	f.lastPut = input
	if f.putErr != nil {
		return mediaasset.ObjectInfo{}, f.putErr
	}
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return mediaasset.ObjectInfo{}, err
	}
	return mediaasset.ObjectInfo{Key: input.Key, Size: int64(len(data))}, nil
}
func (f *fakeObjectStorage) Stat(context.Context, string) (mediaasset.ObjectInfo, error) {
	panic("unused")
}
func (f *fakeObjectStorage) Open(context.Context, string) (io.ReadCloser, error) {
	f.openCalls++
	return io.NopCloser(strings.NewReader("opened media")), nil
}
func (f *fakeObjectStorage) OpenRange(context.Context, string, int64, int64) (io.ReadCloser, error) {
	f.openCalls++
	return io.NopCloser(strings.NewReader("opened range")), nil
}
func (f *fakeObjectStorage) Delete(context.Context, string) error {
	f.deleteCalls++
	return f.deleteErr
}

func TestServiceStoreCalculatesStreamingMetadataAndUsesScopedKey(t *testing.T) {
	metadata := []byte(`{"width":1920}`)
	input := validCreateInput()
	input.Metadata = metadata
	input.Reader = strings.NewReader("media bytes")
	repository := &fakeMetadataRepository{}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	asset, err := service.Store(context.Background(), project.Principal{OwnerID: ownerID}, projectID, input)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	wantHash := sha256.Sum256([]byte("media bytes"))
	if asset.ByteSize != int64(len("media bytes")) || asset.SHA256 != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("unexpected integrity metadata: %+v", asset)
	}
	if !strings.HasPrefix(asset.ObjectKey, "projects/"+projectID.String()+"/assets/") || strings.Contains(asset.ObjectKey, "..") {
		t.Fatalf("unexpected object key: %q", asset.ObjectKey)
	}
	if repository.created.ID != asset.ID || storage.lastPut.Key != asset.ObjectKey {
		t.Fatalf("storage/repository mismatch: asset=%+v stored=%+v", asset, repository.created)
	}
	if string(input.Metadata) != string(metadata) {
		t.Fatalf("input metadata mutated: %s", input.Metadata)
	}
}

func TestServiceStoreRejectsOversizedReaderWithoutPersistingMetadata(t *testing.T) {
	input := validCreateInput()
	input.MaxBytes = int64(len("media bytes") - 1)
	input.Reader = strings.NewReader("media bytes")
	repository := &fakeMetadataRepository{}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	_, err := service.Store(context.Background(), project.Principal{OwnerID: ownerID}, projectID, input)
	if !errors.Is(err, mediaasset.ErrTooLarge) {
		t.Fatalf("expected oversized upload error, got %v", err)
	}
	if repository.created.ID != uuid.Nil {
		t.Fatal("oversized upload persisted metadata")
	}
	if storage.putCalls != 1 || storage.deleteCalls != 1 {
		t.Fatalf("expected bounded storage attempt and compensation, puts=%d deletes=%d", storage.putCalls, storage.deleteCalls)
	}
}

func TestServiceStoreCompensatesWhenMetadataPersistenceFails(t *testing.T) {
	dbErr := errors.New("database unavailable")
	repository := &fakeMetadataRepository{createErr: dbErr}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	_, err := service.Store(context.Background(), project.Principal{OwnerID: ownerID}, projectID, validCreateInput())
	if err == nil || storage.deleteCalls != 1 {
		t.Fatalf("expected safe failure and one compensation delete, err=%v deletes=%d", err, storage.deleteCalls)
	}
	if strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("raw persistence error leaked: %v", err)
	}
}

func TestServiceStoreDoesNotPersistWhenObjectWriteFails(t *testing.T) {
	storageErr := errors.New("storage credential secret")
	repository := &fakeMetadataRepository{}
	storage := &fakeObjectStorage{putErr: storageErr}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	_, err := service.Store(context.Background(), project.Principal{OwnerID: ownerID}, projectID, validCreateInput())
	if err == nil || repository.created.ID != uuid.Nil {
		t.Fatalf("expected object failure without metadata, err=%v asset=%+v", err, repository.created)
	}
	if strings.Contains(err.Error(), "credential") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("raw storage error leaked: %v", err)
	}
}

func TestServiceStoreRejectsCrossOwnerProjectWithoutWritingObject(t *testing.T) {
	storage := &fakeObjectStorage{}
	repository := &fakeMetadataRepository{}
	service := mediaasset.NewService(fakeProjectRepository{err: project.ErrNotFound}, repository, storage)
	_, err := service.Store(context.Background(), project.Principal{OwnerID: ownerID}, projectID, validCreateInput())
	if !errors.Is(err, mediaasset.ErrNotFound) || storage.putCalls != 0 {
		t.Fatalf("expected scoped project not found before storage write, err=%v puts=%d", err, storage.putCalls)
	}
}

type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) { return 0, io.ErrNoProgress }

func TestServiceStorePropagatesCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, &fakeMetadataRepository{}, storage)
	_, err := service.Store(ctx, project.Principal{OwnerID: ownerID}, projectID, validCreateInput())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestServiceAuthorizesMetadataBeforeOpeningAndDeletingObject(t *testing.T) {
	asset := mediaasset.MediaAsset{
		ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID,
		Kind: mediaasset.KindImage, Origin: mediaasset.OriginUpload,
		ObjectKey: "projects/" + projectID.String() + "/assets/" + uuid.NewString(),
		MimeType:  "image/png", ByteSize: 1,
		SHA256: strings.Repeat("a", 64), Metadata: []byte(`{}`),
	}
	repository := &fakeMetadataRepository{created: asset}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	reader, err := service.Open(context.Background(), project.Principal{OwnerID: ownerID}, projectID, asset.ID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reader.Close()
	if got, _ := io.ReadAll(reader); string(got) != "opened media" {
		t.Fatalf("unexpected opened content: %q", got)
	}
	if err := service.Delete(context.Background(), project.Principal{OwnerID: ownerID}, projectID, asset.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if storage.deleteCalls != 1 || repository.deleteCalls != 1 {
		t.Fatalf("expected object-first deletion, storage=%d repository=%d", storage.deleteCalls, repository.deleteCalls)
	}
}

func TestServiceDoesNotDiscloseCrossOwnerOrProjectContentOrDelete(t *testing.T) {
	asset := validAssetForValidation()
	repository := &fakeMetadataRepository{created: asset}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	cases := map[string]struct {
		ownerID   uuid.UUID
		projectID uuid.UUID
	}{
		"foreign owner":   {ownerID: uuid.MustParse("44444444-4444-4444-8444-444444444444"), projectID: asset.ProjectID},
		"foreign project": {ownerID: asset.OwnerID, projectID: uuid.MustParse("55555555-5555-4555-8555-555555555555")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			principal := project.Principal{OwnerID: tc.ownerID}
			if _, err := service.Open(context.Background(), principal, tc.projectID, asset.ID); !errors.Is(err, mediaasset.ErrNotFound) {
				t.Fatalf("open should be non-disclosing, got %v", err)
			}
			if err := service.Delete(context.Background(), principal, tc.projectID, asset.ID); !errors.Is(err, mediaasset.ErrNotFound) {
				t.Fatalf("delete should be non-disclosing, got %v", err)
			}
		})
	}
	if storage.openCalls != 0 || storage.deleteCalls != 0 {
		t.Fatalf("foreign access reached object storage: opens=%d deletes=%d", storage.openCalls, storage.deleteCalls)
	}
}

func TestServiceDeleteStopsBeforeMetadataDeletionOnStorageFailure(t *testing.T) {
	asset := mediaasset.MediaAsset{
		ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID,
		Kind: mediaasset.KindImage, Origin: mediaasset.OriginUpload,
		ObjectKey: "projects/" + projectID.String() + "/assets/" + uuid.NewString(),
		MimeType:  "image/png", ByteSize: 1,
		SHA256: strings.Repeat("a", 64), Metadata: []byte(`{}`),
	}
	repository := &fakeMetadataRepository{created: asset}
	storage := &fakeObjectStorage{deleteErr: errors.New("storage secret")}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)
	if err := service.Delete(context.Background(), project.Principal{OwnerID: ownerID}, projectID, asset.ID); err == nil {
		t.Fatal("expected storage failure")
	}
	if repository.deleteCalls != 0 {
		t.Fatal("metadata was deleted after object deletion failure")
	}
}

func TestServiceDeleteRejectsReferencedAssetBeforeRemovingObject(t *testing.T) {
	asset := validAssetForValidation()
	repository := &fakeMetadataRepository{created: asset, hasReferences: true}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	err := service.Delete(context.Background(), project.Principal{OwnerID: ownerID}, projectID, asset.ID)
	if !errors.Is(err, mediaasset.ErrInUse) {
		t.Fatalf("expected in-use error, got %v", err)
	}
	if storage.deleteCalls != 0 || repository.deleteCalls != 0 {
		t.Fatalf("referenced asset was mutated: storage=%d repository=%d", storage.deleteCalls, repository.deleteCalls)
	}
}

func TestServiceDeleteUsesTombstoneRepositoryBeforeObjectDeletion(t *testing.T) {
	asset := validAssetForValidation()
	repository := &fakeDeletionRepository{base: &fakeMetadataRepository{created: asset}, finalizeErr: errors.New("database commit failed")}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	err := service.Delete(context.Background(), project.Principal{OwnerID: ownerID}, projectID, asset.ID)
	if !errors.Is(err, mediaasset.ErrPersistenceFailed) {
		t.Fatalf("expected finalize persistence error, got %v", err)
	}
	if repository.beginCalls != 1 || repository.finalizeCalls != 1 || storage.deleteCalls != 1 {
		t.Fatalf("unexpected tombstone sequence: begin=%d finalize=%d storage_delete=%d", repository.beginCalls, repository.finalizeCalls, storage.deleteCalls)
	}
}

func TestServiceDeleteDoesNotTouchObjectWhenDeletionCannotCommitTombstone(t *testing.T) {
	asset := validAssetForValidation()
	repository := &fakeDeletionRepository{base: &fakeMetadataRepository{created: asset}, beginErr: errors.New("database unavailable")}
	storage := &fakeObjectStorage{}
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, repository, storage)

	err := service.Delete(context.Background(), project.Principal{OwnerID: ownerID}, projectID, asset.ID)
	if !errors.Is(err, mediaasset.ErrPersistenceFailed) {
		t.Fatalf("expected tombstone persistence error, got %v", err)
	}
	if storage.deleteCalls != 0 {
		t.Fatal("object was deleted before tombstone commit")
	}
}

func TestServiceListIsBounded(t *testing.T) {
	service := mediaasset.NewService(fakeProjectRepository{item: validProject()}, &fakeMetadataRepository{}, &fakeObjectStorage{})
	if _, err := service.List(context.Background(), project.Principal{OwnerID: ownerID}, projectID, 101); err == nil {
		t.Fatal("expected list limit validation")
	}
}

func TestAssetValidationRejectsInvalidIntegrityMetadata(t *testing.T) {
	asset := mediaasset.MediaAsset{
		ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID,
		Kind: mediaasset.KindImage, Origin: mediaasset.OriginUpload,
		ObjectKey: "projects/" + projectID.String() + "/assets/" + uuid.NewString(),
		MimeType:  "image/png", ByteSize: -1, SHA256: "BAD",
	}
	if err := asset.Validate(); err == nil {
		t.Fatal("expected invalid asset metadata")
	}
}
