package project

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type ContentFormat string
type AspectRatio string
type Locale string
type Status string

const (
	ContentFormatShort    ContentFormat = "short"
	ContentFormatLong     ContentFormat = "long"
	ContentFormatFlexible ContentFormat = "flexible"

	AspectRatio16x9 AspectRatio = "16:9"
	AspectRatio9x16 AspectRatio = "9:16"
	AspectRatio1x1  AspectRatio = "1:1"
	AspectRatio4x5  AspectRatio = "4:5"

	LocaleVI Locale = "vi"
	LocaleEN Locale = "en"

	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type Project struct {
	ID                    uuid.UUID
	OwnerID               uuid.UUID
	Title                 string
	Description           string
	ContentFormat         ContentFormat
	AspectRatio           AspectRatio
	TargetDurationSeconds *int
	Locale                Locale
	Status                Status
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CreateInput struct {
	Title                 string
	Description           string
	ContentFormat         ContentFormat
	AspectRatio           AspectRatio
	TargetDurationSeconds *int
	Locale                Locale
}

type UpdateInput struct {
	Title                 *string
	Description           *string
	ContentFormat         *ContentFormat
	AspectRatio           *AspectRatio
	TargetDurationSeconds **int
	Locale                *Locale
	Status                *Status
}

func (input UpdateInput) hasChanges() bool {
	return input.Title != nil ||
		input.Description != nil ||
		input.ContentFormat != nil ||
		input.AspectRatio != nil ||
		input.TargetDurationSeconds != nil ||
		input.Locale != nil ||
		input.Status != nil
}

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	return "project validation failed"
}

func (e ValidationError) HasFields() bool {
	return len(e.Fields) > 0
}

func (input *CreateInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		fields["title"] = "required"
	} else if len([]rune(input.Title)) > 160 {
		fields["title"] = "max_160"
	}

	if len([]rune(input.Description)) > 5000 {
		fields["description"] = "max_5000"
	}

	if !validContentFormat(input.ContentFormat) {
		fields["content_format"] = "invalid"
	}
	if !validAspectRatio(input.AspectRatio) {
		fields["aspect_ratio"] = "invalid"
	}

	if input.TargetDurationSeconds != nil {
		value := *input.TargetDurationSeconds
		if value < 1 || value > 43200 {
			fields["target_duration_seconds"] = "range_1_43200"
		}
	}

	if input.Locale == "" {
		input.Locale = LocaleVI
	}
	if !validLocale(input.Locale) {
		fields["locale"] = "invalid"
	}

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (input *UpdateInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	if !input.hasChanges() {
		fields["body"] = "empty"
		return ValidationError{Fields: fields}
	}

	if input.Title != nil {

		title := strings.TrimSpace(*input.Title)
		input.Title = &title
		if title == "" {
			fields["title"] = "required"
		} else if len([]rune(title)) > 160 {
			fields["title"] = "max_160"
		}
	}

	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		input.Description = &description
		if len([]rune(description)) > 5000 {
			fields["description"] = "max_5000"
		}
	}

	if input.ContentFormat != nil && !validContentFormat(*input.ContentFormat) {
		fields["content_format"] = "invalid"
	}
	if input.AspectRatio != nil && !validAspectRatio(*input.AspectRatio) {
		fields["aspect_ratio"] = "invalid"
	}
	if input.TargetDurationSeconds != nil && *input.TargetDurationSeconds != nil {
		value := **input.TargetDurationSeconds
		if value < 1 || value > 43200 {
			fields["target_duration_seconds"] = "range_1_43200"
		}
	}
	if input.Locale != nil && !validLocale(*input.Locale) {
		fields["locale"] = "invalid"
	}
	if input.Status != nil && !validStatus(*input.Status) {
		fields["status"] = "invalid"
	}

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validContentFormat(value ContentFormat) bool {
	switch value {
	case ContentFormatShort, ContentFormatLong, ContentFormatFlexible:
		return true
	default:
		return false
	}
}

func validAspectRatio(value AspectRatio) bool {
	switch value {
	case AspectRatio16x9, AspectRatio9x16, AspectRatio1x1, AspectRatio4x5:
		return true
	default:
		return false
	}
}

func validLocale(value Locale) bool {
	switch value {
	case LocaleVI, LocaleEN:
		return true
	default:
		return false
	}
}

func validStatus(value Status) bool {
	switch value {
	case StatusActive, StatusArchived:
		return true
	default:
		return false
	}
}
