package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/httpserver"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/postgres"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/renderexport"
)

func configureLocalRender(
	ctx context.Context,
	pool *pgxpool.Pool,
	snapshots renderexport.SnapshotStore,
	queue renderexport.JobQueue,
	reader renderexport.JobReader,
	mediaService *mediaasset.Service,
	captionReader renderexport.SnapshotCaptionRevisionReader,
	registry *jobs.Registry,
) (httpserver.RenderExportService, error) {
	if pool == nil || snapshots == nil || queue == nil || reader == nil || mediaService == nil || captionReader == nil || registry == nil {
		return nil, fmt.Errorf("render runtime dependencies are incomplete")
	}
	profile, err := renderexport.ProbeLocalFFmpegProfile(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("probe local render profile: %w", err)
	}
	artifactRepo := postgres.NewRenderArtifactRepository(pool)
	mediaRepo := postgres.NewMediaAssetRepository(pool)
	assetStore := renderexport.NewAssetStore(mediaService, mediaRepo)
	handler := renderexport.NewHandler(snapshots, assetStore, artifactRepo, profile, captionReader)
	if err := registry.Register(renderexport.JobKind, handler); err != nil {
		return nil, fmt.Errorf("register render export handler: %w", err)
	}
	return renderexport.NewServiceWithRuntime(snapshots, queue, reader, artifactRepo, nil), nil
}
