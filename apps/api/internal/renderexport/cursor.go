package renderexport

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

type historyCursorPayload struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func EncodeHistoryCursor(cursor jobs.ListCursor) (string, error) {
	payload := historyCursorPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		ID:        cursor.ID.String(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeHistoryCursor(value string) (jobs.ListCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return jobs.ListCursor{}, fmt.Errorf("decode history cursor: %w", err)
	}
	var payload historyCursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return jobs.ListCursor{}, fmt.Errorf("parse history cursor: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.CreatedAt)
	if err != nil {
		return jobs.ListCursor{}, fmt.Errorf("parse history cursor created_at: %w", err)
	}
	id, err := uuid.Parse(payload.ID)
	if err != nil {
		return jobs.ListCursor{}, fmt.Errorf("parse history cursor id: %w", err)
	}
	return jobs.ListCursor{CreatedAt: createdAt, ID: id}, nil
}
