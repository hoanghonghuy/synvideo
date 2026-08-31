package project

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type cursorPayload struct {
	UpdatedAt string `json:"updated_at"`
	ID        string `json:"id"`
}

func EncodeCursor(cursor ListCursor) (string, error) {
	payload := cursorPayload{
		UpdatedAt: cursor.UpdatedAt.UTC().Format(timeFormatRFC3339Nano),
		ID:        cursor.ID.String(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeCursor(value string) (ListCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return ListCursor{}, fmt.Errorf("decode cursor: %w", err)
	}

	var payload cursorPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return ListCursor{}, fmt.Errorf("parse cursor: %w", err)
	}

	updatedAt, err := parseTime(payload.UpdatedAt)
	if err != nil {
		return ListCursor{}, err
	}
	id, err := uuid.Parse(payload.ID)
	if err != nil {
		return ListCursor{}, fmt.Errorf("parse cursor id: %w", err)
	}

	return ListCursor{UpdatedAt: updatedAt, ID: id}, nil
}
