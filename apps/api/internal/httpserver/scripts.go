package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

type scriptHandler struct {
	service       ScriptService
	actorResolver actor.Resolver
}

type scriptSummaryResponse struct {
	Version               int           `json:"version"`
	Revision              int           `json:"revision"`
	Status                script.Status `json:"status"`
	SourceProposalVersion int           `json:"source_proposal_version"`
	ContentLocale         string        `json:"content_locale"`
	CreatedAt             string        `json:"created_at"`
	UpdatedAt             string        `json:"updated_at"`
	ApprovedAt            *string       `json:"approved_at"`
}

type sectionResponse struct {
	Key     string `json:"key"`
	Heading string `json:"heading,omitempty"`
	Body    string `json:"body"`
}

type scriptResponse struct {
	ProjectID                string            `json:"project_id"`
	Version                  int               `json:"version"`
	Revision                 int               `json:"revision"`
	Status                   script.Status     `json:"status"`
	SourceProposalVersion    int               `json:"source_proposal_version"`
	ContentLocale            string            `json:"content_locale"`
	Sections                 []sectionResponse `json:"sections"`
	EstimatedDurationSeconds *int              `json:"estimated_duration_seconds,omitempty"`
	Notes                    string            `json:"notes,omitempty"`
	CreatedAt                string            `json:"created_at"`
	UpdatedAt                string            `json:"updated_at"`
	ApprovedAt               *string           `json:"approved_at"`
}

type putScriptRequest struct {
	Revision                 *int             `json:"revision"`
	Sections                 []script.Section `json:"sections"`
	EstimatedDurationSeconds *int             `json:"estimated_duration_seconds"`
	Notes                    *string          `json:"notes"`
}

type approveScriptRequest struct {
	Revision *int `json:"revision"`
}

func (h scriptHandler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	items, err := h.service.List(r.Context(), principal, projectID)
	if err != nil {
		writeScriptAPIError(w, err)
		return
	}

	response := make([]scriptSummaryResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toScriptSummaryResponse(item))
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h scriptHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseScriptVersion(w, r)
	if !ok {
		return
	}

	item, err := h.service.GetByVersion(r.Context(), principal, projectID, version)
	if err != nil {
		writeScriptAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toScriptResponse(item))
}

func (h scriptHandler) put(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseScriptVersion(w, r)
	if !ok {
		return
	}

	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req putScriptRequest
	if err := decoder.Decode(&req); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalTypeErr *json.UnmarshalTypeError
		if errors.Is(err, bytes.ErrTooLarge) || errors.As(err, &syntaxErr) || errors.As(err, &unmarshalTypeErr) {
			writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
				Code:    "malformed_json",
				Message: "The request body contains invalid JSON.",
			}})
			return
		}
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "The request body could not be parsed.",
		}})
		return
	}

	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}

	input := script.PutInput{
		Revision: req.Revision,
		Content: script.Content{
			Sections:                 req.Sections,
			EstimatedDurationSeconds: req.EstimatedDurationSeconds,
			Notes:                    notes,
		},
	}

	updated, err := h.service.UpdateDraft(r.Context(), principal, projectID, version, input)
	if err != nil {
		writeScriptAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toScriptResponse(updated))
}

func (h scriptHandler) approve(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseScriptVersion(w, r)
	if !ok {
		return
	}

	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req approveScriptRequest
	if err := decoder.Decode(&req); err != nil {
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "malformed_json",
			Message: "The request body contains invalid JSON.",
		}})
		return
	}

	if req.Revision == nil {
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{
			Error: apiError{
				Code:    "validation_failed",
				Message: "Script approval payload failed validation.",
				Fields: map[string]string{
					"revision": "required",
				},
			},
		})
		return
	}

	approved, err := h.service.Approve(r.Context(), principal, projectID, version, *req.Revision)
	if err != nil {
		writeScriptAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toScriptResponse(approved))
}

func (h scriptHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	if h.actorResolver == nil {
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{
			Code:    "principal_required",
			Message: "A request principal is required.",
		}})
		return project.Principal{}, false
	}

	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{
			Code:    "principal_required",
			Message: "A request principal is required.",
		}})
		return project.Principal{}, false
	}

	return principal, true
}

func parseScriptVersion(w http.ResponseWriter, r *http.Request) (int, bool) {
	rawVersion := r.PathValue("version")
	version, err := strconv.Atoi(rawVersion)
	if err != nil || version < 1 {
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "script_not_found",
			Message: "Script was not found.",
		}})
		return 0, false
	}
	return version, true
}

func writeScriptAPIError(w http.ResponseWriter, err error) {
	var valErr script.ValidationError
	switch {
	case errors.As(err, &valErr):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Script content failed validation.",
			Fields:  valErr.Fields,
		}})
	case errors.Is(err, script.ErrInvalidInput), errors.Is(err, script.ErrVersionInvalid):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Script payload is invalid.",
		}})
	case errors.Is(err, script.ErrStaleRevision):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "STALE_REVISION",
			Message: "Script revision is stale.",
		}})
	case errors.Is(err, script.ErrScriptImmutable):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "SCRIPT_IMMUTABLE",
			Message: "Script is immutable.",
		}})
	case errors.Is(err, script.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "script_not_found",
			Message: "Script was not found.",
		}})
	case errors.Is(err, project.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "project_not_found",
			Message: "Project was not found.",
		}})
	case errors.Is(err, script.ErrUnauthenticated), errors.Is(err, project.ErrUnauthenticated):
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

func toScriptSummaryResponse(item script.Script) scriptSummaryResponse {
	var approvedAt *string
	if item.ApprovedAt != nil {
		tStr := item.ApprovedAt.UTC().Format(time.RFC3339Nano)
		approvedAt = &tStr
	}

	return scriptSummaryResponse{
		Version:               item.Version,
		Revision:              item.Revision,
		Status:                item.Status,
		SourceProposalVersion: item.SourceProposalVersion,
		ContentLocale:         item.ContentLocale,
		CreatedAt:             item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		ApprovedAt:            approvedAt,
	}
}

func toScriptResponse(item script.Script) scriptResponse {
	var approvedAt *string
	if item.ApprovedAt != nil {
		tStr := item.ApprovedAt.UTC().Format(time.RFC3339Nano)
		approvedAt = &tStr
	}

	sections := make([]sectionResponse, 0, len(item.Sections))
	for _, s := range item.Sections {
		sections = append(sections, sectionResponse{
			Key:     s.Key,
			Heading: s.Heading,
			Body:    s.Body,
		})
	}

	return scriptResponse{
		ProjectID:                item.ProjectID.String(),
		Version:                  item.Version,
		Revision:                 item.Revision,
		Status:                   item.Status,
		SourceProposalVersion:    item.SourceProposalVersion,
		ContentLocale:            item.ContentLocale,
		Sections:                 sections,
		EstimatedDurationSeconds: item.EstimatedDurationSeconds,
		Notes:                    item.Notes,
		CreatedAt:                item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:                item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		ApprovedAt:               approvedAt,
	}
}
