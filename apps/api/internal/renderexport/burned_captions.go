package renderexport

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	// BurnedCaptionProfileV1 is the immutable presentation contract for V1
	// in-video captions. Changes to this visual contract require a new profile
	// identifier so render dedupe/provenance remains truthful.
	BurnedCaptionProfileV1 = "burned_caption_v1"

	burnedCaptionFontName = "DejaVu Sans"
	burnedCaptionFontSize = 28
	burnedCaptionMarginV  = 42
)

var ErrInvalidBurnedCaptionInput = errors.New("render export burned caption input is invalid")

// BurnedCaptionFilter returns a server-owned FFmpeg subtitles filter. Creator
// caption text never enters this filter string: text lives only in the bounded
// WebVTT file materialized from the accepted immutable snapshot.
func BurnedCaptionFilter(profileID, subtitlePath string) (string, error) {
	if profileID != BurnedCaptionProfileV1 || !safeBurnedCaptionPath(subtitlePath) {
		return "", ErrInvalidBurnedCaptionInput
	}
	style := fmt.Sprintf(
		"FontName=%s,FontSize=%d,PrimaryColour=&H00FFFFFF,OutlineColour=&H00000000,BorderStyle=3,BackColour=&H80000000,Outline=1,Shadow=0,MarginV=%d,Alignment=2,WrapStyle=0",
		burnedCaptionFontName,
		burnedCaptionFontSize,
		burnedCaptionMarginV,
	)
	return fmt.Sprintf("subtitles=filename='%s':force_style='%s'", subtitlePath, style), nil
}

// safeBurnedCaptionPath deliberately accepts only absolute server-created paths
// made from a conservative portable alphabet. This keeps the path out of
// FFmpeg filter grammar even though the filter itself is passed as one argv
// element (never through a shell).
func safeBurnedCaptionPath(path string) bool {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	for _, r := range path {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("/_-.", r) {
			continue
		}
		return false
	}
	return true
}
