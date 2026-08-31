package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type creativeProposalHandler struct {
	service       CreativeProposalService
	actorResolver actor.Resolver
}

type creativeProposalSummaryResponse struct {
	Version             int                     `json:"version"`
	Revision            int                     `json:"revision"`
	Status              creativeproposal.Status `json:"status"`
	SourceBriefRevision int                     `json:"source_brief_revision"`
	CreatedAt           string                  `json:"created_at"`
	UpdatedAt           string                  `json:"updated_at"`
	ApprovedAt          *string                 `json:"approved_at"`
}

type creativeProposalResponse struct {
	ProjectID                string                           `json:"project_id"`
	Version                  int                              `json:"version"`
	Revision                 int                              `json:"revision"`
	Status                   creativeproposal.Status          `json:"status"`
	SourceBriefRevision      int                              `json:"source_brief_revision"`
	TitleOptions             []string                         `json:"title_options"`
	HookOptions              []string                         `json:"hook_options"`
	AudienceSummary          string                           `json:"audience_summary"`
	ObjectiveSummary         string                           `json:"objective_summary"`
	NarrativeAngle           string                           `json:"narrative_angle"`
	EstimatedDurationSeconds *int                             `json:"estimated_duration_seconds"`
	FormatRationale          string                           `json:"format_rationale"`
	Structure                []creativeproposal.StructureItem `json:"structure"`
	VisualDirection          string                           `json:"visual_direction"`
	VoiceDirection           string                           `json:"voice_direction"`
	MusicDirection           string                           `json:"music_direction"`
	CaptionDirection         string                           `json:"caption_direction"`
	CallToAction             string                           `json:"call_to_action"`
	ResearchGaps             []string                         `json:"research_gaps"`
	Warnings                 []string                         `json:"warnings"`
	CreatedAt                string                           `json:"created_at"`
	UpdatedAt                string                           `json:"updated_at"`
	ApprovedAt               *string                          `json:"approved_at"`
}

func (h creativeProposalHandler) list(w http.ResponseWriter, r *http.Request) {
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
		writeCreativeProposalAPIError(w, err)
		return
	}

	response := make([]creativeProposalSummaryResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toCreativeProposalSummaryResponse(item))
	}
	writeProjectJSON(w, http.StatusOK, response)
}

func (h creativeProposalHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseVersion(w, r)
	if !ok {
		return
	}

	item, err := h.service.Get(r.Context(), principal, projectID, version)
	if err != nil {
		writeCreativeProposalAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toCreativeProposalResponse(item))
}

func (h creativeProposalHandler) put(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseVersion(w, r)
	if !ok {
		return
	}

	input, ok := decodePutCreativeProposalRequest(w, r)
	if !ok {
		return
	}

	updated, err := h.service.UpdateDraft(r.Context(), principal, projectID, version, input)
	if err != nil {
		writeCreativeProposalAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toCreativeProposalResponse(updated))
}

func (h creativeProposalHandler) approve(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	version, ok := parseVersion(w, r)
	if !ok {
		return
	}

	revision, ok := decodeApproveCreativeProposalRequest(w, r)
	if !ok {
		return
	}

	approved, err := h.service.Approve(r.Context(), principal, projectID, version, revision)
	if err != nil {
		writeCreativeProposalAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toCreativeProposalResponse(approved))
}

func (h creativeProposalHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeCreativeProposalAPIError(w, creativeproposal.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func parseVersion(w http.ResponseWriter, r *http.Request) (int, bool) {
	versionStr := r.PathValue("version")
	v, err := strconv.Atoi(versionStr)
	if err != nil || v < 1 {
		writeCreativeProposalAPIError(w, creativeproposal.ValidationError{Fields: map[string]string{"version": "positive"}})
		return 0, false
	}
	return v, true
}

func decodePutCreativeProposalRequest(w http.ResponseWriter, r *http.Request) (creativeproposal.PutInput, bool) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		writeCreativeProposalAPIError(w, creativeproposal.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return creativeproposal.PutInput{}, false
	}

	var input creativeproposal.PutInput
	for field, value := range raw {
		switch field {
		case "revision":
			var revision int
			if !decodeCreativeProposalField(w, field, value, &revision) {
				return creativeproposal.PutInput{}, false
			}
			input.Revision = &revision
		case "title_options":
			if !decodeCreativeProposalField(w, field, value, &input.TitleOptions) {
				return creativeproposal.PutInput{}, false
			}
		case "hook_options":
			if !decodeCreativeProposalField(w, field, value, &input.HookOptions) {
				return creativeproposal.PutInput{}, false
			}
		case "audience_summary":
			if !decodeCreativeProposalField(w, field, value, &input.AudienceSummary) {
				return creativeproposal.PutInput{}, false
			}
		case "objective_summary":
			if !decodeCreativeProposalField(w, field, value, &input.ObjectiveSummary) {
				return creativeproposal.PutInput{}, false
			}
		case "narrative_angle":
			if !decodeCreativeProposalField(w, field, value, &input.NarrativeAngle) {
				return creativeproposal.PutInput{}, false
			}
		case "estimated_duration_seconds":
			if bytes.Equal(value, []byte("null")) {
				input.EstimatedDurationSeconds = nil
				continue
			}
			var duration int
			if !decodeCreativeProposalField(w, field, value, &duration) {
				return creativeproposal.PutInput{}, false
			}
			input.EstimatedDurationSeconds = &duration
		case "format_rationale":
			if !decodeCreativeProposalField(w, field, value, &input.FormatRationale) {
				return creativeproposal.PutInput{}, false
			}
		case "structure":
			if !decodeCreativeProposalField(w, field, value, &input.Structure) {
				return creativeproposal.PutInput{}, false
			}
		case "visual_direction":
			if !decodeCreativeProposalField(w, field, value, &input.VisualDirection) {
				return creativeproposal.PutInput{}, false
			}
		case "voice_direction":
			if !decodeCreativeProposalField(w, field, value, &input.VoiceDirection) {
				return creativeproposal.PutInput{}, false
			}
		case "music_direction":
			if !decodeCreativeProposalField(w, field, value, &input.MusicDirection) {
				return creativeproposal.PutInput{}, false
			}
		case "caption_direction":
			if !decodeCreativeProposalField(w, field, value, &input.CaptionDirection) {
				return creativeproposal.PutInput{}, false
			}
		case "call_to_action":
			if !decodeCreativeProposalField(w, field, value, &input.CallToAction) {
				return creativeproposal.PutInput{}, false
			}
		case "research_gaps":
			if !decodeCreativeProposalField(w, field, value, &input.ResearchGaps) {
				return creativeproposal.PutInput{}, false
			}
		case "warnings":
			if !decodeCreativeProposalField(w, field, value, &input.Warnings) {
				return creativeproposal.PutInput{}, false
			}
		default:
			writeCreativeProposalAPIError(w, creativeproposal.ValidationError{Fields: map[string]string{field: "unknown"}})
			return creativeproposal.PutInput{}, false
		}
	}
	return input, true
}

func decodeApproveCreativeProposalRequest(w http.ResponseWriter, r *http.Request) (int, bool) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		writeCreativeProposalAPIError(w, creativeproposal.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return 0, false
	}

	var revision *int
	for field, value := range raw {
		switch field {
		case "revision":
			var rev int
			if !decodeCreativeProposalField(w, field, value, &rev) {
				return 0, false
			}
			revision = &rev
		default:
			writeCreativeProposalAPIError(w, creativeproposal.ValidationError{Fields: map[string]string{field: "unknown"}})
			return 0, false
		}
	}

	if revision == nil {
		writeCreativeProposalAPIError(w, creativeproposal.ValidationError{Fields: map[string]string{"revision": "required"}})
		return 0, false
	}
	return *revision, true
}

func decodeCreativeProposalField(w http.ResponseWriter, field string, raw json.RawMessage, target any) bool {
	if err := json.Unmarshal(raw, target); err != nil {
		writeCreativeProposalAPIError(w, creativeproposal.ValidationError{Fields: map[string]string{field: "invalid"}})
		return false
	}
	return true
}

func writeCreativeProposalAPIError(w http.ResponseWriter, err error) {
	var validationErr creativeproposal.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  validationErr.Fields,
		}})
	case errors.Is(err, creativeproposal.ErrVersionInvalid):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  map[string]string{"version": "positive"},
		}})
	case errors.Is(err, creativeproposal.ErrRevisionInvalid):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  map[string]string{"revision": "positive"},
		}})
	case errors.Is(err, creativeproposal.ErrStaleRevision):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "STALE_REVISION",
			Message: "Creative proposal revision is stale.",
		}})
	case errors.Is(err, creativeproposal.ErrProposalImmutable):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "PROPOSAL_IMMUTABLE",
			Message: "Creative proposal is immutable.",
		}})
	case errors.Is(err, creativeproposal.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "proposal_not_found",
			Message: "Creative proposal was not found.",
		}})
	case errors.Is(err, project.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "project_not_found",
			Message: "Project was not found.",
		}})
	case errors.Is(err, creativeproposal.ErrUnauthenticated), errors.Is(err, project.ErrUnauthenticated):
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

func toCreativeProposalSummaryResponse(item creativeproposal.CreativeProposal) creativeProposalSummaryResponse {
	var approvedAt *string
	if item.ApprovedAt != nil {
		tStr := item.ApprovedAt.UTC().Format(time.RFC3339Nano)
		approvedAt = &tStr
	}

	return creativeProposalSummaryResponse{
		Version:             item.Version,
		Revision:            item.Revision,
		Status:              item.Status,
		SourceBriefRevision: item.SourceBriefRevision,
		CreatedAt:           item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:           item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		ApprovedAt:          approvedAt,
	}
}

func toCreativeProposalResponse(item creativeproposal.CreativeProposal) creativeProposalResponse {
	var approvedAt *string
	if item.ApprovedAt != nil {
		tStr := item.ApprovedAt.UTC().Format(time.RFC3339Nano)
		approvedAt = &tStr
	}

	titleOptions := item.TitleOptions
	if titleOptions == nil {
		titleOptions = []string{}
	}
	hookOptions := item.HookOptions
	if hookOptions == nil {
		hookOptions = []string{}
	}
	structure := item.Structure
	if structure == nil {
		structure = []creativeproposal.StructureItem{}
	}
	researchGaps := item.ResearchGaps
	if researchGaps == nil {
		researchGaps = []string{}
	}
	warnings := item.Warnings
	if warnings == nil {
		warnings = []string{}
	}

	return creativeProposalResponse{
		ProjectID:                item.ProjectID.String(),
		Version:                  item.Version,
		Revision:                 item.Revision,
		Status:                   item.Status,
		SourceBriefRevision:      item.SourceBriefRevision,
		TitleOptions:             titleOptions,
		HookOptions:              hookOptions,
		AudienceSummary:          item.AudienceSummary,
		ObjectiveSummary:         item.ObjectiveSummary,
		NarrativeAngle:           item.NarrativeAngle,
		EstimatedDurationSeconds: item.EstimatedDurationSeconds,
		FormatRationale:          item.FormatRationale,
		Structure:                structure,
		VisualDirection:          item.VisualDirection,
		VoiceDirection:           item.VoiceDirection,
		MusicDirection:           item.MusicDirection,
		CaptionDirection:         item.CaptionDirection,
		CallToAction:             item.CallToAction,
		ResearchGaps:             researchGaps,
		Warnings:                 warnings,
		CreatedAt:                item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:                item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		ApprovedAt:               approvedAt,
	}
}
