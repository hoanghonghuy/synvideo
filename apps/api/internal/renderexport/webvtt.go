package renderexport

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxWebVTTCues      = 10_000
	maxWebVTTCueRunes  = 2_000
	maxWebVTTTimestamp = int64(99*60*60*1000 + 59*60*1000 + 59*1000 + 999)
)

var ErrInvalidWebVTT = errors.New("render export webvtt input is invalid")

// WebVTTCue is an artifact-local subtitle cue. It intentionally carries no
// mutable document identifiers so serialized sidecars cannot leak internal
// lineage metadata.
type WebVTTCue struct {
	StartMS int64
	EndMS   int64
	Text    string
}

// BuildWebVTT serializes validated cues deterministically. Callers are
// responsible for deriving cues from the render job's immutable caption
// revisions before invoking this function.
func BuildWebVTT(input []WebVTTCue) ([]byte, error) {
	if len(input) > maxWebVTTCues {
		return nil, ErrInvalidWebVTT
	}

	cues := append([]WebVTTCue(nil), input...)
	for i := range cues {
		cue := &cues[i]
		cue.Text = normalizeWebVTTText(cue.Text)
		if cue.StartMS < 0 || cue.EndMS <= cue.StartMS || cue.EndMS > maxWebVTTTimestamp {
			return nil, ErrInvalidWebVTT
		}
		if cue.Text == "" || !utf8.ValidString(cue.Text) || utf8.RuneCountInString(cue.Text) > maxWebVTTCueRunes {
			return nil, ErrInvalidWebVTT
		}
	}

	sort.SliceStable(cues, func(i, j int) bool {
		if cues[i].StartMS != cues[j].StartMS {
			return cues[i].StartMS < cues[j].StartMS
		}
		if cues[i].EndMS != cues[j].EndMS {
			return cues[i].EndMS < cues[j].EndMS
		}
		return cues[i].Text < cues[j].Text
	})

	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	for _, cue := range cues {
		out.WriteString(formatWebVTTTimestamp(cue.StartMS))
		out.WriteString(" --> ")
		out.WriteString(formatWebVTTTimestamp(cue.EndMS))
		out.WriteByte('\n')
		out.WriteString(escapeWebVTTText(cue.Text))
		out.WriteString("\n\n")
	}
	return []byte(out.String()), nil
}

func normalizeWebVTTText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\x00", "")
	return strings.TrimSpace(value)
}

func escapeWebVTTText(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	return strings.ReplaceAll(value, ">", "&gt;")
	return value
}

func formatWebVTTTimestamp(ms int64) string {
	hours := ms / (60 * 60 * 1000)
	ms %= 60 * 60 * 1000
	minutes := ms / (60 * 1000)
	ms %= 60 * 1000
	seconds := ms / 1000
	milliseconds := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}
