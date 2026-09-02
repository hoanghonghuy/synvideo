package mediaasset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Kind string

const (
	KindImage    Kind = "image"
	KindVideo    Kind = "video"
	KindAudio    Kind = "audio"
	KindDocument Kind = "document"
	KindOther    Kind = "other"
)

type Origin string

const (
	OriginUpload         Origin = "upload"
	OriginCreatorMedia   Origin = "creator_media"
	OriginStock          Origin = "stock"
	OriginGeneratedImage Origin = "generated_image"
	OriginGeneratedVideo Origin = "generated_video"
	OriginGeneratedAudio Origin = "generated_audio"
	OriginSystem         Origin = "system"
)

var (
	ErrNotFound          = errors.New("media asset not found")
	ErrUnauthenticated   = errors.New("media asset principal is required")
	ErrInvalidInput      = errors.New("media asset input is invalid")
	ErrStorageFailed     = errors.New("media asset storage failed")
	ErrPersistenceFailed = errors.New("media asset persistence failed")
	ErrObjectNotFound    = errors.New("media asset object not found")
	ErrTooLarge          = errors.New("media asset exceeds upload limit")
	ErrInUse             = errors.New("media asset is in use")
	ErrUnsupportedType   = errors.New("media asset type is unsupported")
	ErrRangeInvalid      = errors.New("media asset byte range is invalid")
)

type ValidationError struct{ Fields map[string]string }

func (e ValidationError) Error() string { return "media asset validation failed" }

type MediaAsset struct {
	ID               uuid.UUID       `json:"id"`
	OwnerID          uuid.UUID       `json:"-"`
	ProjectID        uuid.UUID       `json:"project_id"`
	Kind             Kind            `json:"kind"`
	Origin           Origin          `json:"origin"`
	ObjectKey        string          `json:"-"`
	MimeType         string          `json:"mime_type"`
	ByteSize         int64           `json:"byte_size"`
	SHA256           string          `json:"sha256"`
	OriginalFilename string          `json:"original_filename,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type CreateInput struct {
	Kind             Kind
	Origin           Origin
	MimeType         string
	OriginalFilename string
	Metadata         json.RawMessage
	Reader           io.Reader
	MaxBytes         int64
}

func (input *CreateInput) NormalizeAndValidate() error {
	fields := map[string]string{}
	input.MimeType = strings.TrimSpace(input.MimeType)
	input.OriginalFilename = strings.TrimSpace(input.OriginalFilename)
	if !validKind(input.Kind) {
		fields["kind"] = "invalid"
	}
	if !validOrigin(input.Origin) {
		fields["origin"] = "invalid"
	}
	if input.MimeType == "" {
		fields["mime_type"] = "required"
	} else if utf8.RuneCountInString(input.MimeType) > 255 {
		fields["mime_type"] = "max_length"
	}
	if utf8.RuneCountInString(input.OriginalFilename) > 500 {
		fields["original_filename"] = "max_length"
	}
	if input.Reader == nil {
		fields["reader"] = "required"
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	} else if !isJSONObject(input.Metadata) {
		fields["metadata"] = "object_required"
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func (asset MediaAsset) Validate() error {
	fields := map[string]string{}
	if asset.ID == uuid.Nil {
		fields["id"] = "required"
	}
	if asset.OwnerID == uuid.Nil {
		fields["owner_id"] = "required"
	}
	if asset.ProjectID == uuid.Nil {
		fields["project_id"] = "required"
	}
	if !validKind(asset.Kind) {
		fields["kind"] = "invalid"
	}
	if !validOrigin(asset.Origin) {
		fields["origin"] = "invalid"
	}
	if err := ValidateObjectKeyForAsset(asset.ObjectKey, asset.ProjectID, asset.ID); err != nil {
		fields["object_key"] = "invalid"
	}
	if strings.TrimSpace(asset.MimeType) == "" || utf8.RuneCountInString(asset.MimeType) > 255 {
		fields["mime_type"] = "invalid"
	}
	if asset.ByteSize < 0 {
		fields["byte_size"] = "non_negative"
	}
	if !sha256Pattern.MatchString(asset.SHA256) {
		fields["sha256"] = "invalid"
	}
	if utf8.RuneCountInString(asset.OriginalFilename) > 500 {
		fields["original_filename"] = "max_length"
	}
	if !isJSONObject(asset.Metadata) {
		fields["metadata"] = "object_required"
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validKind(value Kind) bool {
	switch value {
	case KindImage, KindVideo, KindAudio, KindDocument, KindOther:
		return true
	default:
		return false
	}
}

func validOrigin(value Origin) bool {
	switch value {
	case OriginUpload, OriginCreatorMedia, OriginStock, OriginGeneratedImage, OriginGeneratedVideo, OriginGeneratedAudio, OriginSystem:
		return true
	default:
		return false
	}
}

func isJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if decoder.Decode(&value) != nil {
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return false
	}
	_, ok := value.(map[string]any)
	return ok
}

func ValidateObjectKey(key string) error {
	_, _, err := parseCanonicalObjectKey(key)
	return err
}

func ValidateObjectKeyForAsset(key string, projectID, assetID uuid.UUID) error {
	keyProjectID, keyAssetID, err := parseCanonicalObjectKey(key)
	if err != nil {
		return err
	}
	if projectID == uuid.Nil || assetID == uuid.Nil || keyProjectID != projectID || keyAssetID != assetID {
		return fmt.Errorf("object key identity does not match asset")
	}
	return nil
}

func parseCanonicalObjectKey(key string) (uuid.UUID, uuid.UUID, error) {
	if key == "" || strings.ContainsAny(key, "\\\r\n") || strings.Contains(key, "..") || strings.HasPrefix(key, "/") || strings.Contains(key, "://") {
		return uuid.Nil, uuid.Nil, fmt.Errorf("object key is unsafe")
	}
	parts := strings.Split(key, "/")
	if len(parts) != 4 || parts[0] != "projects" || parts[2] != "assets" {
		return uuid.Nil, uuid.Nil, fmt.Errorf("object key is not canonical")
	}
	projectID, projectErr := uuid.Parse(parts[1])
	assetID, assetErr := uuid.Parse(parts[3])
	if projectErr != nil || assetErr != nil || projectID.String() != parts[1] || assetID.String() != parts[3] {
		return uuid.Nil, uuid.Nil, fmt.Errorf("object key is not canonical")
	}
	return projectID, assetID, nil
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
}

type PutObjectInput struct {
	Key         string
	Body        io.Reader
	ContentType string
}

type ObjectStorage interface {
	Put(context.Context, PutObjectInput) (ObjectInfo, error)
	Stat(context.Context, string) (ObjectInfo, error)
	Open(context.Context, string) (io.ReadCloser, error)
	OpenRange(context.Context, string, int64, int64) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
