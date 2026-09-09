package ingestvalidation_test

import (
	"context"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset/ingestvalidation"
)

func TestRealFFprobeValidatesAllSupportedAudioVideoFamilies(t *testing.T) {
	requireFFprobe(t)

	validator := ingestvalidation.NewValidator(ingestvalidation.ExecCommandRunner{})
	cases := []struct {
		name         string
		path         string
		kind         ingestvalidation.Kind
		declaredMIME string
		verifiedMIME string
	}{
		{"mp4", ffmpegFixture(t, ".mp4", "-f", "lavfi", "-i", "color=c=red:s=16x16:d=0.1", "-c:v", "libx264", "-pix_fmt", "yuv420p"), ingestvalidation.KindVideo, "video/mp4", "video/mp4"},
		{"quicktime", ffmpegFixture(t, ".mov", "-f", "lavfi", "-i", "color=c=red:s=16x16:d=0.1", "-c:v", "libx264", "-pix_fmt", "yuv420p"), ingestvalidation.KindVideo, "video/quicktime", "video/mp4"},
		{"webm", ffmpegFixture(t, ".webm", "-f", "lavfi", "-i", "color=c=red:s=16x16:d=0.1", "-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p"), ingestvalidation.KindVideo, "video/webm", "video/webm"},
		{"wav", ffmpegFixture(t, ".wav", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1"), ingestvalidation.KindAudio, "audio/wav", "audio/wav"},
		{"mpeg-audio", ffmpegFixture(t, ".mp3", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "libmp3lame"), ingestvalidation.KindAudio, "audio/mpeg", "audio/mpeg"},
		{"flac", ffmpegFixture(t, ".flac", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "flac"), ingestvalidation.KindAudio, "audio/flac", "audio/flac"},
		{"ogg", ffmpegFixture(t, ".ogg", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "libvorbis"), ingestvalidation.KindAudio, "audio/ogg", "audio/ogg"},
		{"aac", ffmpegFixture(t, ".aac", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "aac"), ingestvalidation.KindAudio, "audio/aac", "audio/aac"},
		{"mp4-audio", ffmpegFixture(t, ".m4a", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.1", "-c:a", "aac"), ingestvalidation.KindAudio, "audio/mp4", "audio/mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verified, err := validator.ValidateFile(context.Background(), tc.path, ingestvalidation.DeclaredInput{
				Kind: tc.kind, MimeType: tc.declaredMIME,
			})
			if err != nil {
				t.Fatalf("ValidateFile() with real ffprobe error = %v", err)
			}
			if verified.Kind != tc.kind {
				t.Fatalf("verified kind = %q, want %q", verified.Kind, tc.kind)
			}
			if verified.MimeType != tc.verifiedMIME {
				t.Fatalf("verified mime = %q, want %q", verified.MimeType, tc.verifiedMIME)
			}
		})
	}
}
