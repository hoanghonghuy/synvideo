package mediaasset

import (
	"context"
	"fmt"
)

func (s *Service) Ready(ctx context.Context) error {
	probe, ok := s.storage.(interface{ Ready(context.Context) error })
	if !ok {
		return fmt.Errorf("object storage readiness unavailable")
	}
	return probe.Ready(ctx)
}
