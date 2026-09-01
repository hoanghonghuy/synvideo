package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

var mediaAssetOwnerID = uuid.MustParse("11111111-1111-4111-8111-111111111111")

func TestMediaAssetRepositoryIntegrationScopeOrderingAndConstraints(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	assetRepository := NewMediaAssetRepository(pool)
	otherOwner := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	item, err := projectRepository.Create(context.Background(), mediaAssetOwnerID, validIntegrationCreateInput("Media assets"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	first := integrationAsset(item.ID, mediaAssetOwnerID, 1)
	created, err := assetRepository.Create(context.Background(), first)
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	var createdMetadata, firstMetadata map[string]any
	_ = json.Unmarshal(created.Metadata, &createdMetadata)
	_ = json.Unmarshal(first.Metadata, &firstMetadata)
	if created.ID != first.ID || created.OwnerID != first.OwnerID || created.ProjectID != first.ProjectID ||
		created.Kind != first.Kind || created.Origin != first.Origin || created.ObjectKey != first.ObjectKey ||
		created.MimeType != first.MimeType || created.ByteSize != first.ByteSize || created.SHA256 != first.SHA256 ||
		created.OriginalFilename != first.OriginalFilename || !reflect.DeepEqual(createdMetadata, firstMetadata) {
		t.Fatalf("created asset mismatch: got=%+v want=%+v", created, first)
	}

	fetched, err := assetRepository.Get(context.Background(), mediaAssetOwnerID, item.ID, first.ID)
	if err != nil || fetched.ID != first.ID {
		t.Fatalf("get asset: asset=%+v err=%v", fetched, err)
	}
	if _, err := assetRepository.Get(context.Background(), otherOwner, item.ID, first.ID); !errors.Is(err, mediaasset.ErrNotFound) {
		t.Fatalf("expected cross-owner get to be not found, got %v", err)
	}

	second := integrationAsset(item.ID, mediaAssetOwnerID, 2)
	second.CreatedAt = first.CreatedAt.Add(time.Second)
	second.UpdatedAt = second.CreatedAt
	if _, err := assetRepository.Create(context.Background(), second); err != nil {
		t.Fatalf("create second asset: %v", err)
	}
	list, err := assetRepository.List(context.Background(), mediaAssetOwnerID, item.ID, mediaasset.ListOptions{Limit: 1})
	if err != nil || len(list.Assets) != 1 || list.Assets[0].ID != second.ID {
		t.Fatalf("expected newest bounded list, got=%+v err=%v", list, err)
	}
	otherList, err := assetRepository.List(context.Background(), otherOwner, item.ID, mediaasset.ListOptions{Limit: 10})
	if err != nil || len(otherList.Assets) != 0 {
		t.Fatalf("expected cross-owner empty list, got=%+v err=%v", otherList, err)
	}

	if err := assetRepository.Delete(context.Background(), mediaAssetOwnerID, item.ID, first.ID); err != nil {
		t.Fatalf("delete asset: %v", err)
	}
	if _, err := assetRepository.Get(context.Background(), mediaAssetOwnerID, item.ID, first.ID); !errors.Is(err, mediaasset.ErrNotFound) {
		t.Fatalf("expected deleted asset not found, got %v", err)
	}

	_, err = pool.Exec(context.Background(), `
		INSERT INTO media_assets (owner_id, project_id, kind, origin, object_key, mime_type, byte_size, sha256, metadata)
		VALUES ($1, $2, 'invalid', 'upload', $3, 'image/png', 1, $4, '{}'::jsonb)
	`, mediaAssetOwnerID, item.ID, "projects/"+item.ID.String()+"/assets/"+uuid.NewString(), strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("expected invalid kind constraint failure")
	}
}

func integrationAsset(projectID, ownerID uuid.UUID, suffix int) mediaasset.MediaAsset {
	return mediaasset.MediaAsset{
		ID: uuid.New(), OwnerID: ownerID, ProjectID: projectID,
		Kind: mediaasset.KindImage, Origin: mediaasset.OriginUpload,
		ObjectKey: "projects/" + projectID.String() + "/assets/" + uuid.NewString(),
		MimeType:  "image/png", ByteSize: int64(suffix), SHA256: strings.Repeat("a", 64),
		OriginalFilename: "asset.png", Metadata: json.RawMessage(`{"source":"test"}`),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
}
