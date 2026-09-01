package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
)

type ScenePlanService interface {
	List(ctx context.Context, principal project.Principal, projectID uuid.UUID) ([]sceneplan.Plan, error)
	GetByVersion(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int) (sceneplan.Plan, error)
	UpdateDraft(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, input sceneplan.PutInput) (sceneplan.Plan, error)
	Approve(ctx context.Context, principal project.Principal, projectID uuid.UUID, version int, revision int) (sceneplan.Plan, error)
}

type scenePlanHandler struct {
	service       ScenePlanService
	actorResolver actor.Resolver
}

type scenePlanSummaryResponse struct {
	Version               int              `json:"version"`
	Revision              int              `json:"revision"`
	Status                sceneplan.Status `json:"status"`
	SourceScriptVersion   int              `json:"source_script_version"`
	SourceProposalVersion int              `json:"source_proposal_version"`
	ContentLocale         string           `json:"content_locale"`
	CreatedAt             string           `json:"created_at"`
	UpdatedAt             string           `json:"updated_at"`
	ApprovedAt            *string          `json:"approved_at"`
}

type sceneResponse struct {
	Key                     string               `json:"key"`
	ScriptSectionKey        string               `json:"script_section_key"`
	Narration               string               `json:"narration"`
	VisualInstruction       string               `json:"visual_instruction"`
	PlannedSourceType       sceneplan.SourceType `json:"planned_source_type"`
	ExpectedDurationSeconds int                  `json:"expected_duration_seconds"`
	CaptionIntent           string               `json:"caption_intent,omitempty"`
	TransitionNotes         string               `json:"transition_notes,omitempty"`
}

type scenePlanResponse struct {
	ProjectID             string           `json:"project_id"`
	Version               int              `json:"version"`
	Revision              int              `json:"revision"`
	Status                sceneplan.Status `json:"status"`
	SourceScriptVersion   int              `json:"source_script_version"`
	SourceProposalVersion int              `json:"source_proposal_version"`
	ContentLocale         string           `json:"content_locale"`
	Scenes                []sceneResponse  `json:"scenes"`
	CreatedAt             string           `json:"created_at"`
	UpdatedAt             string           `json:"updated_at"`
	ApprovedAt            *string          `json:"approved_at"`
}

type putScenePlanRequest struct {
	Revision *int              `json:"revision"`
	Scenes   []sceneplan.Scene `json:"scenes"`
}

type approveScenePlanRequest struct {
	Revision *int `json:"revision"`
}

func (h scenePlanHandler) list(w http.ResponseWriter, r *http.Request) {
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
		writeScenePlanAPIError(w, err)
		return
	}

	response := make([]scenePlanSummaryResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toScenePlanSummaryResponse(item))
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h scenePlanHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseScenePlanVersion(w, r)
	if !ok {
		return
	}

	item, err := h.service.GetByVersion(r.Context(), principal, projectID, version)
	if err != nil {
		writeScenePlanAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toScenePlanResponse(item))
}

func (h scenePlanHandler) put(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseScenePlanVersion(w, r)
	if !ok {
		return
	}

	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req putScenePlanRequest
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

	input := sceneplan.PutInput{
		Revision: req.Revision,
		Content: sceneplan.Content{
			Scenes: req.Scenes,
		},
	}

	updated, err := h.service.UpdateDraft(r.Context(), principal, projectID, version, input)
	if err != nil {
		writeScenePlanAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toScenePlanResponse(updated))
}

func (h scenePlanHandler) approve(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseScenePlanVersion(w, r)
	if !ok {
		return
	}

	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req approveScenePlanRequest
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
				Message: "Scene plan approval payload failed validation.",
				Fields: map[string]string{
					"revision": "required",
				},
			},
		})
		return
	}

	approved, err := h.service.Approve(r.Context(), principal, projectID, version, *req.Revision)
	if err != nil {
		writeScenePlanAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, toScenePlanResponse(approved))
}

func (h scenePlanHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
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

func parseScenePlanVersion(w http.ResponseWriter, r *http.Request) (int, bool) {
	rawVersion := r.PathValue("version")
	version, err := strconv.Atoi(rawVersion)
	if err != nil || version < 1 {
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "scene_plan_not_found",
			Message: "Scene plan was not found.",
		}})
		return 0, false
	}
	return version, true
}

func writeScenePlanAPIError(w http.ResponseWriter, err error) {
	var valErr sceneplan.ValidationError
	switch {
	case errors.As(err, &valErr):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Scene plan content failed validation.",
			Fields:  valErr.Fields,
		}})
	case errors.Is(err, sceneplan.ErrInvalidInput):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Scene plan payload is invalid.",
		}})
	case errors.Is(err, sceneplan.ErrStaleRevision):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "STALE_REVISION",
			Message: "Scene plan revision is stale.",
		}})
	case errors.Is(err, sceneplan.ErrScenePlanImmutable):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "SCENE_PLAN_IMMUTABLE",
			Message: "Scene plan is immutable.",
		}})
	case errors.Is(err, sceneplan.ErrScriptNotApproved):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "SCRIPT_NOT_APPROVED",
			Message: "Source script is not approved.",
		}})
	case errors.Is(err, sceneplan.ErrScriptSourceInvalid):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "SCENE_PLAN_SOURCE_INVALID",
			Message: "Source script or proposal is invalid.",
		}})
	case errors.Is(err, sceneplan.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "scene_plan_not_found",
			Message: "Scene plan was not found.",
		}})
	case errors.Is(err, project.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "project_not_found",
			Message: "Project was not found.",
		}})
	case errors.Is(err, sceneplan.ErrUnauthenticated), errors.Is(err, project.ErrUnauthenticated):
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

func toScenePlanSummaryResponse(item sceneplan.Plan) scenePlanSummaryResponse {
	var approvedAt *string
	if item.ApprovedAt != nil {
		tStr := item.ApprovedAt.UTC().Format(time.RFC3339Nano)
		approvedAt = &tStr
	}

	return scenePlanSummaryResponse{
		Version:               item.Version,
		Revision:              item.Revision,
		Status:                item.Status,
		SourceScriptVersion:   item.SourceScriptVersion,
		SourceProposalVersion: item.SourceProposalVersion,
		ContentLocale:         item.ContentLocale,
		CreatedAt:             item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		ApprovedAt:            approvedAt,
	}
}

func toScenePlanResponse(item sceneplan.Plan) scenePlanResponse {
	var approvedAt *string
	if item.ApprovedAt != nil {
		tStr := item.ApprovedAt.UTC().Format(time.RFC3339Nano)
		approvedAt = &tStr
	}

	scenes := make([]sceneResponse, 0, len(item.Scenes))
	for _, s := range item.Scenes {
		scenes = append(scenes, sceneResponse{
			Key:                     s.Key,
			ScriptSectionKey:        s.ScriptSectionKey,
			Narration:               s.Narration,
			VisualInstruction:       s.VisualInstruction,
			PlannedSourceType:       s.PlannedSourceType,
			ExpectedDurationSeconds: s.ExpectedDurationSeconds,
			CaptionIntent:           s.CaptionIntent,
			TransitionNotes:         s.TransitionNotes,
		})
	}

	return scenePlanResponse{
		ProjectID:             item.ProjectID.String(),
		Version:               item.Version,
		Revision:              item.Revision,
		Status:                item.Status,
		SourceScriptVersion:   item.SourceScriptVersion,
		SourceProposalVersion: item.SourceProposalVersion,
		ContentLocale:         item.ContentLocale,
		Scenes:                scenes,
		CreatedAt:             item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:             item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		ApprovedAt:            approvedAt,
	}
}
