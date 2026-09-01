package script

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusApproved   Status = "approved"
	StatusSuperseded Status = "superseded"
)

var (
	ErrNotFound            = errors.New("script not found")
	ErrStaleRevision       = errors.New("script revision is stale")
	ErrScriptImmutable     = errors.New("script is immutable")
	ErrProposalNotApproved = errors.New("source proposal is not approved")
	ErrUnauthenticated     = errors.New("unauthenticated")
	ErrInvalidInput        = errors.New("invalid script input")

	keyRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)
)

type Section struct {
	Key     string `json:"key"`
	Heading string `json:"heading,omitempty"`
	Body    string `json:"body"`
}

type Script struct {
	ProjectID                uuid.UUID  `json:"project_id"`
	Version                  int        `json:"version"`
	Revision                 int        `json:"revision"`
	Status                   Status     `json:"status"`
	SourceProposalVersion    int        `json:"source_proposal_version"`
	ContentLocale            string     `json:"content_locale"`
	Sections                 []Section  `json:"sections"`
	EstimatedDurationSeconds *int       `json:"estimated_duration_seconds,omitempty"`
	Notes                    string     `json:"notes,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	ApprovedAt               *time.Time `json:"approved_at,omitempty"`
	SourceGenerationJobID    *uuid.UUID `json:"-"`
}

type Content struct {
	Sections                 []Section `json:"sections"`
	EstimatedDurationSeconds *int      `json:"estimated_duration_seconds,omitempty"`
	Notes                    string    `json:"notes,omitempty"`
}

type PutInput struct {
	Revision *int
	Content
}

type CreateDraftInput struct {
	SourceProposalVersion int
	SourceGenerationJobID *uuid.UUID
	Content
}

type ApproveInput struct {
	Revision *int
}

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	return "script validation failed"
}

func (e ValidationError) HasFields() bool {
	return len(e.Fields) > 0
}

func (c *Content) NormalizeAndValidate() error {
	fields := map[string]string{}
	c.normalizeAndValidateFields(fields)
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (c *Content) normalizeAndValidateFields(fields map[string]string) {
	if len(c.Sections) == 0 {
		fields["sections"] = "required"
	} else if len(c.Sections) > 200 {
		fields["sections"] = "max_items"
	} else {
		seenKeys := make(map[string]bool)
		for idx, s := range c.Sections {
			trimmedKey := strings.TrimSpace(s.Key)
			c.Sections[idx].Key = trimmedKey

			if trimmedKey == "" {
				fields["sections["+strconv.Itoa(idx)+"].key"] = "required"
			} else if len([]rune(trimmedKey)) > 64 {
				fields["sections["+strconv.Itoa(idx)+"].key"] = "max_length"
			} else if !keyRegex.MatchString(trimmedKey) {
				fields["sections["+strconv.Itoa(idx)+"].key"] = "invalid_format"
			} else if seenKeys[trimmedKey] {
				fields["sections["+strconv.Itoa(idx)+"].key"] = "duplicate"
			} else {
				seenKeys[trimmedKey] = true
			}

			trimmedHeading := strings.TrimSpace(s.Heading)
			c.Sections[idx].Heading = trimmedHeading
			if len([]rune(trimmedHeading)) > 300 {
				fields["sections["+strconv.Itoa(idx)+"].heading"] = "max_length"
			}

			trimmedBody := strings.TrimSpace(s.Body)
			c.Sections[idx].Body = trimmedBody
			if trimmedBody == "" {
				fields["sections["+strconv.Itoa(idx)+"].body"] = "required"
			} else if len([]rune(trimmedBody)) > 20000 {
				fields["sections["+strconv.Itoa(idx)+"].body"] = "max_length"
			}
		}
	}

	if c.EstimatedDurationSeconds != nil {
		if *c.EstimatedDurationSeconds < 1 || *c.EstimatedDurationSeconds > 43200 {
			fields["estimated_duration_seconds"] = "range"
		}
	}

	c.Notes = strings.TrimSpace(c.Notes)
	if len([]rune(c.Notes)) > 10000 {
		fields["notes"] = "max_length"
	}
}

func (in *PutInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if in.Revision == nil {
		fields["revision"] = "required"
	} else if *in.Revision < 1 {
		fields["revision"] = "positive"
	}

	in.Content.normalizeAndValidateFields(fields)
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (in *CreateDraftInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if in.SourceProposalVersion < 1 {
		fields["source_proposal_version"] = "positive"
	}

	in.Content.normalizeAndValidateFields(fields)
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (in *ApproveInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if in.Revision == nil {
		fields["revision"] = "required"
	} else if *in.Revision < 1 {
		fields["revision"] = "positive"
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}
