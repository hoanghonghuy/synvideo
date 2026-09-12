package renderexport

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildWebVTTDeterministicAndEscaped(t *testing.T) {
	got, err := BuildWebVTT([]WebVTTCue{
		{StartMS: 3_500, EndMS: 4_250, Text: "Second <line> & detail"},
		{StartMS: 0, EndMS: 1_250, Text: "  First\r\nline  "},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "WEBVTT\n\n" +
		"00:00:00.000 --> 00:00:01.250\nFirst\nline\n\n" +
		"00:00:03.500 --> 00:00:04.250\nSecond &lt;line&gt; &amp; detail\n\n"
	if string(got) != want {
		t.Fatalf("BuildWebVTT() = %q, want %q", string(got), want)
	}
}

func TestBuildWebVTTHasNoCueIdentifiers(t *testing.T) {
	got, err := BuildWebVTT([]WebVTTCue{{StartMS: 100, EndMS: 200, Text: "caption text"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "uuid") || strings.Contains(string(got), "document") || strings.Contains(string(got), "lineage") {
		t.Fatalf("sidecar leaked internal metadata: %q", string(got))
	}
}

func TestBuildWebVTTRejectsInvalidCue(t *testing.T) {
	cases := [][]WebVTTCue{
		{{StartMS: -1, EndMS: 100, Text: "bad start"}},
		{{StartMS: 100, EndMS: 100, Text: "bad end"}},
		{{StartMS: 0, EndMS: 100, Text: "   "}},
		{{StartMS: 0, EndMS: maxWebVTTTimestamp + 1, Text: "too long"}},
	}
	for _, cues := range cases {
		if _, err := BuildWebVTT(cues); !errors.Is(err, ErrInvalidWebVTT) {
			t.Fatalf("BuildWebVTT(%+v) err = %v, want ErrInvalidWebVTT", cues, err)
		}
	}
}

func TestBuildWebVTTEmptyProducesHeaderOnly(t *testing.T) {
	got, err := BuildWebVTT(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "WEBVTT\n\n" {
		t.Fatalf("empty WebVTT = %q", string(got))
	}
}
