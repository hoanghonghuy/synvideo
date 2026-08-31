package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type projectHandler struct {
	service       ProjectService
	actorResolver actor.Resolver
}

type createProjectRequest struct {
	Title                 string                `json:"title"`
	Description           string                `json:"description"`
	ContentFormat         project.ContentFormat `json:"content_format"`
	AspectRatio           project.AspectRatio   `json:"aspect_ratio"`
	TargetDurationSeconds *int                  `json:"target_duration_seconds"`
	Locale                project.Locale        `json:"locale"`
}

type projectResponse struct {
	ID                    string                `json:"id"`
	Title                 string                `json:"title"`
	Description           string                `json:"description"`
	ContentFormat         project.ContentFormat `json:"content_format"`
	AspectRatio           project.AspectRatio   `json:"aspect_ratio"`
	TargetDurationSeconds *int                  `json:"target_duration_seconds"`
	Locale                project.Locale        `json:"locale"`
	Status                project.Status        `json:"status"`
	CreatedAt             string                `json:"created_at"`
	UpdatedAt             string                `json:"updated_at"`
}

type listProjectsResponse struct {
	Projects   []projectResponse `json:"projects"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (h projectHandler) create(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	var request createProjectRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	created, err := h.service.Create(r.Context(), principal, project.CreateInput{
		Title:                 request.Title,
		Description:           request.Description,
		ContentFormat:         request.ContentFormat,
		AspectRatio:           request.AspectRatio,
		TargetDurationSeconds: request.TargetDurationSeconds,
		Locale:                request.Locale,
	})
	if err != nil {
		writeAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusCreated, toProjectResponse(created))
}

func (h projectHandler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	limit := 0
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			writeAPIError(w, project.ValidationError{Fields: map[string]string{"limit": "invalid"}})
			return
		}
		limit = parsed
	}

	result, nextCursor, err := h.service.List(r.Context(), principal, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeAPIError(w, err)
		return
	}

	projects := make([]projectResponse, 0, len(result.Projects))
	for _, item := range result.Projects {
		projects = append(projects, toProjectResponse(item))
	}
	writeProjectJSON(w, http.StatusOK, listProjectsResponse{Projects: projects, NextCursor: nextCursor})
}

func (h projectHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	id, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	item, err := h.service.Get(r.Context(), principal, id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toProjectResponse(item))
}

func (h projectHandler) update(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	id, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	input, ok := decodeUpdateProjectRequest(w, r)
	if !ok {
		return
	}

	updated, err := h.service.Update(r.Context(), principal, id, input)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toProjectResponse(updated))
}

func (h projectHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func parseProjectID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"id": "invalid"}})
		return uuid.Nil, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return false
	}
	return true
}

func decodeUpdateProjectRequest(w http.ResponseWriter, r *http.Request) (project.UpdateInput, bool) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return project.UpdateInput{}, false
	}

	var input project.UpdateInput
	for field, value := range raw {
		switch field {
		case "title":
			var title string
			if !decodeField(w, field, value, &title) {
				return project.UpdateInput{}, false
			}
			input.Title = &title
		case "description":
			var description string
			if !decodeField(w, field, value, &description) {
				return project.UpdateInput{}, false
			}
			input.Description = &description
		case "content_format":
			var contentFormat project.ContentFormat
			if !decodeField(w, field, value, &contentFormat) {
				return project.UpdateInput{}, false
			}
			input.ContentFormat = &contentFormat
		case "aspect_ratio":
			var aspectRatio project.AspectRatio
			if !decodeField(w, field, value, &aspectRatio) {
				return project.UpdateInput{}, false
			}
			input.AspectRatio = &aspectRatio
		case "target_duration_seconds":
			if bytes.Equal(value, []byte("null")) {
				var cleared *int
				input.TargetDurationSeconds = &cleared
				continue
			}
			var duration int
			if !decodeField(w, field, value, &duration) {
				return project.UpdateInput{}, false
			}
			durationPtr := &duration
			input.TargetDurationSeconds = &durationPtr
		case "locale":
			var locale project.Locale
			if !decodeField(w, field, value, &locale) {
				return project.UpdateInput{}, false
			}
			input.Locale = &locale
		case "status":
			var status project.Status
			if !decodeField(w, field, value, &status) {
				return project.UpdateInput{}, false
			}
			input.Status = &status
		default:
			writeAPIError(w, project.ValidationError{Fields: map[string]string{field: "unknown"}})
			return project.UpdateInput{}, false
		}
	}

	return input, true
}

func decodeField(w http.ResponseWriter, field string, raw json.RawMessage, target any) bool {
	if err := json.Unmarshal(raw, target); err != nil {
		writeAPIError(w, project.ValidationError{Fields: map[string]string{field: "invalid"}})
		return false
	}
	return true
}

func writeAPIError(w http.ResponseWriter, err error) {
	var validationErr project.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  validationErr.Fields,
		}})
	case errors.Is(err, project.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "project_not_found",
			Message: "Project was not found.",
		}})
	case errors.Is(err, project.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{
			Code:    "principal_required",
			Message: "A request principal is required.",
		}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{
			Code:    "internal_error",
			Message: "The request could not be completed.",
		}})
	}
}

func writeProjectJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
	}
}

func toProjectResponse(item project.Project) projectResponse {
	return projectResponse{
		ID:                    item.ID.String(),
		Title:                 item.Title,
		Description:           item.Description,
		ContentFormat:         item.ContentFormat,
		AspectRatio:           item.AspectRatio,
		TargetDurationSeconds: item.TargetDurationSeconds,
		Locale:                item.Locale,
		Status:                item.Status,
		CreatedAt:             item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
