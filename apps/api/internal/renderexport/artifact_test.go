package renderexport

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRenderArtifactValidateRequiresDurablePlayableProvenance(t *testing.T) {
	valid := RenderArtifact{
		ID:               uuid.New(),
		OwnerID:          uuid.New(),
		ProjectID:        uuid.New(),
		JobID:            uuid.New(),
		SnapshotDigest:   strings.Repeat("a", 64),
		ProfileID:        LocalProfileID,
		MediaAssetID:     uuid.New(),
		ByteSize:         1024,
		SHA256:           strings.Repeat("b", 64),
		MimeType:         "video/mp4",
		DurationMS:       1000,
		Width:            320,
		Height:           180,
		ToolchainVersion: "ffmpeg version test",
		CreatedAt:        time.Now().UTC(),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*RenderArtifact)
	}{
		{name: "missing media", mutate: func(a *RenderArtifact) { a.MediaAssetID = uuid.Nil }},
		{name: "wrong profile", mutate: func(a *RenderArtifact) { a.ProfileID = "client-profile" }},
		{name: "invalid digest", mutate: func(a *RenderArtifact) { a.SnapshotDigest = "bad" }},
		{name: "empty bytes", mutate: func(a *RenderArtifact) { a.ByteSize = 0 }},
		{name: "invalid sha", mutate: func(a *RenderArtifact) { a.SHA256 = "bad" }},
		{name: "wrong mime", mutate: func(a *RenderArtifact) { a.MimeType = "application/octet-stream" }},
		{name: "missing metadata", mutate: func(a *RenderArtifact) { a.DurationMS = 0 }},
		{name: "missing toolchain", mutate: func(a *RenderArtifact) { a.ToolchainVersion = " " }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			tt.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
