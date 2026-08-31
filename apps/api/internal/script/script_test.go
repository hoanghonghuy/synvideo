package script_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

func validSection(key string) script.Section {
	return script.Section{
		Key:     key,
		Heading: "Heading for " + key,
		Body:    "Body content for " + key,
	}
}

func validContent() script.Content {
	duration := 120
	return script.Content{
		Sections: []script.Section{
			validSection("intro"),
			validSection("section-1"),
			validSection("outro"),
		},
		EstimatedDurationSeconds: &duration,
		Notes:                    "Some script notes",
	}
}

func repeatRunes(r rune, count int) string {
	runes := make([]rune, count)
	for i := range runes {
		runes[i] = r
	}
	return string(runes)
}

func TestContentValidation(t *testing.T) {
	t.Run("valid content", func(t *testing.T) {
		content := validContent()
		if err := content.NormalizeAndValidate(); err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
	})

	t.Run("empty sections rejected", func(t *testing.T) {
		content := script.Content{
			Sections: []script.Section{},
		}
		err := content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected validation error for empty sections")
		}
		var valErr script.ValidationError
		if !errors.As(err, &valErr) || valErr.Fields["sections"] == "" {
			t.Fatalf("expected validation error on sections field, got %v", err)
		}
	})

	t.Run("exceeds 200 sections rejected", func(t *testing.T) {
		sections := make([]script.Section, 201)
		for i := 0; i < 201; i++ {
			sections[i] = validSection("sec-" + strings.Repeat("a", 3))
			sections[i].Key = "sec-" + string(rune('a'+(i%26))) + "-" + string(rune('0'+(i/26))) + "-" + string(rune('a'+(i%10)))
		}
		content := script.Content{
			Sections: sections,
		}
		err := content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected validation error for >200 sections")
		}
	})

	t.Run("invalid section keys", func(t *testing.T) {
		// Key with spaces or uppercase or special characters
		content := validContent()
		content.Sections[0].Key = "Invalid Key!"
		err := content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected validation error for uppercase/space key")
		}

		// Key too long (>64 chars)
		content = validContent()
		content.Sections[0].Key = strings.Repeat("a", 65)
		err = content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected validation error for key >64 chars")
		}
	})

	t.Run("duplicate section keys rejected", func(t *testing.T) {
		content := validContent()
		content.Sections = []script.Section{
			validSection("intro"),
			validSection("intro"),
		}
		err := content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected error for duplicate section keys")
		}
	})

	t.Run("section body validation", func(t *testing.T) {
		// Empty body
		content := validContent()
		content.Sections[0].Body = "   "
		err := content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected error for empty section body")
		}

		// Body > 20000 chars (ASCII)
		content = validContent()
		content.Sections[0].Body = strings.Repeat("a", 20001)
		err = content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected error for section body >20000 chars")
		}

		// Body exactly 20000 multibyte characters (e.g. Vietnamese 'ế' = 3 bytes, total 60000 bytes) -> must pass
		content = validContent()
		content.Sections[0].Body = repeatRunes('ế', 20000)
		if err := content.NormalizeAndValidate(); err != nil {
			t.Fatalf("expected exactly 20000 multibyte runes body to pass, got error: %v", err)
		}

		// Body 20001 multibyte characters -> must fail
		content = validContent()
		content.Sections[0].Body = repeatRunes('ế', 20001)
		if err := content.NormalizeAndValidate(); err == nil {
			t.Fatalf("expected 20001 multibyte runes body to fail")
		}
	})

	t.Run("heading length validation", func(t *testing.T) {
		content := validContent()
		content.Sections[0].Heading = strings.Repeat("h", 301)
		err := content.NormalizeAndValidate()
		if err == nil {
			t.Fatalf("expected error for heading >300 chars")
		}

		// Heading exactly 300 multibyte characters -> must pass
		content = validContent()
		content.Sections[0].Heading = repeatRunes('đ', 300)
		if err := content.NormalizeAndValidate(); err != nil {
			t.Fatalf("expected exactly 300 multibyte runes heading to pass, got error: %v", err)
		}

		// Heading 301 multibyte characters -> must fail
		content = validContent()
		content.Sections[0].Heading = repeatRunes('đ', 301)
		if err := content.NormalizeAndValidate(); err == nil {
			t.Fatalf("expected 301 multibyte runes heading to fail")
		}
	})

	t.Run("estimated duration bounds", func(t *testing.T) {
		invalidMin := 0
		content := validContent()
		content.EstimatedDurationSeconds = &invalidMin
		if err := content.NormalizeAndValidate(); err == nil {
			t.Fatalf("expected error for duration 0")
		}

		invalidMax := 43201
		content = validContent()
		content.EstimatedDurationSeconds = &invalidMax
		if err := content.NormalizeAndValidate(); err == nil {
			t.Fatalf("expected error for duration >43200")
		}
	})

	t.Run("notes length validation", func(t *testing.T) {
		content := validContent()
		content.Notes = strings.Repeat("n", 10001)
		if err := content.NormalizeAndValidate(); err == nil {
			t.Fatalf("expected error for notes >10000 chars")
		}

		// Notes exactly 10000 multibyte characters -> must pass
		content = validContent()
		content.Notes = repeatRunes('ạ', 10000)
		if err := content.NormalizeAndValidate(); err != nil {
			t.Fatalf("expected exactly 10000 multibyte runes notes to pass, got error: %v", err)
		}

		// Notes 10001 multibyte characters -> must fail
		content = validContent()
		content.Notes = repeatRunes('ạ', 10001)
		if err := content.NormalizeAndValidate(); err == nil {
			t.Fatalf("expected 10001 multibyte runes notes to fail")
		}
	})
}
