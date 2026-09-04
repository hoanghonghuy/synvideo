package s3storage

import (
	"context"
	"fmt"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
)

// Ready verifies that the configured bucket is reachable without mutating it.
func (s *Storage) Ready(ctx context.Context) error {
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()

	exists, err := s.client.BucketExists(operationCtx, s.bucket)
	if err != nil {
		return mapError(err)
	}
	if !exists {
		return fmt.Errorf("%w", mediaasset.ErrObjectNotFound)
	}
	return nil
}
