package scenenarrationjob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

type ChunkStore interface {
	GetChunk(ctx context.Context, projectID, jobID uuid.UUID, chunkIndex int) ([]byte, error)
	PutChunk(ctx context.Context, projectID, jobID uuid.UUID, chunkIndex int, data []byte) error
	DeleteChunks(ctx context.Context, projectID, jobID uuid.UUID, totalChunks int) error
}

type ObjectStorageChunkStore struct {
	storage mediaasset.ObjectStorage
}

func NewObjectStorageChunkStore(storage mediaasset.ObjectStorage) *ObjectStorageChunkStore {
	return &ObjectStorageChunkStore{storage: storage}
}

func (s *ObjectStorageChunkStore) chunkKey(projectID, jobID uuid.UUID, chunkIndex int) string {
	return fmt.Sprintf("projects/%s/internal_chunks/%s/%d", projectID, jobID, chunkIndex)
}

func (s *ObjectStorageChunkStore) GetChunk(ctx context.Context, projectID, jobID uuid.UUID, chunkIndex int) ([]byte, error) {
	if s.storage == nil {
		return nil, nil
	}
	key := s.chunkKey(projectID, jobID, chunkIndex)
	reader, err := s.storage.Open(ctx, key)
	if err != nil {
		if errors.Is(err, mediaasset.ErrObjectNotFound) {
			return nil, nil
		}
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (s *ObjectStorageChunkStore) PutChunk(ctx context.Context, projectID, jobID uuid.UUID, chunkIndex int, data []byte) error {
	if s.storage == nil {
		return nil
	}
	key := s.chunkKey(projectID, jobID, chunkIndex)
	_, err := s.storage.Put(ctx, mediaasset.PutObjectInput{
		Key:         key,
		Body:        bytes.NewReader(data),
		ContentType: "application/octet-stream",
	})
	return err
}

func (s *ObjectStorageChunkStore) DeleteChunks(ctx context.Context, projectID, jobID uuid.UUID, totalChunks int) error {
	if s.storage == nil {
		return nil
	}
	for i := 0; i < totalChunks; i++ {
		key := s.chunkKey(projectID, jobID, i)
		_ = s.storage.Delete(ctx, key)
	}
	return nil
}
