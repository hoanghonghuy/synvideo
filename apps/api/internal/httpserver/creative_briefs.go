package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type creativeBriefHandler struct {
	service       CreativeBriefService
	actorResolver actor.Resolver
}

type creativeBriefResponse struct {
	ProjectID           string                             `json:"project_id"`
	Revision            int                                `json:"revision"`
	SourceText          string                             `json:"source_text"`
	TargetAudience      string                             `json:"target_audience"`
	Objective           string                             `json:"objective"`
	DesiredStyle        string                             `json:"desired_style"`
	Tone                string                             `json:"tone"`
	DistributionTargets []creativebrief.DistributionTarget `json:"distribution_targets"`
	CallToAction        string                             `json:"call_to_action"`
	MustInclude         []string                           `json:"must_include"`
	MustAvoid           []string                           `json:"must_avoid"`
	CreatedAt           string                             `json:"created_at"`
	UpdatedAt           string                             `json:"updated_at"`
}

func (h creativeBriefHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	brief, err := h.service.Get(r.Context(), principal, projectID)
	if err != nil {
		writeCreativeBriefAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, toCreativeBriefResponse(brief))
}

func (h creativeBriefHandler) put(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	input, ok := decodePutCreativeBriefRequest(w, r)
	if !ok {
		return
	}

	brief, created, err := h.service.Put(r.Context(), principal, projectID, input)
	if err != nil {
		writeCreativeBriefAPIError(w, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeProjectJSON(w, status, toCreativeBriefResponse(brief))
}

func (h creativeBriefHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeCreativeBriefAPIError(w, creativebrief.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func decodePutCreativeBriefRequest(w http.ResponseWriter, r *http.Request) (creativebrief.PutInput, bool) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		writeCreativeBriefAPIError(w, creativebrief.ValidationError{Fields: map[string]string{"body": "invalid_json"}})
		return creativebrief.PutInput{}, false
	}

	var input creativebrief.PutInput
	for field, value := range raw {
		switch field {
		case "revision":
			var revision int
			if !decodeCreativeBriefField(w, field, value, &revision) {
				return creativebrief.PutInput{}, false
			}
			input.Revision = &revision
		case "source_text":
			if !decodeCreativeBriefField(w, field, value, &input.SourceText) {
				return creativebrief.PutInput{}, false
			}
		case "target_audience":
			if !decodeCreativeBriefField(w, field, value, &input.TargetAudience) {
				return creativebrief.PutInput{}, false
			}
		case "objective":
			if !decodeCreativeBriefField(w, field, value, &input.Objective) {
				return creativebrief.PutInput{}, false
			}
		case "desired_style":
			if !decodeCreativeBriefField(w, field, value, &input.DesiredStyle) {
				return creativebrief.PutInput{}, false
			}
		case "tone":
			if !decodeCreativeBriefField(w, field, value, &input.Tone) {
				return creativebrief.PutInput{}, false
			}
		case "distribution_targets":
			if !decodeCreativeBriefField(w, field, value, &input.DistributionTargets) {
				return creativebrief.PutInput{}, false
			}
		case "call_to_action":
			if !decodeCreativeBriefField(w, field, value, &input.CallToAction) {
				return creativebrief.PutInput{}, false
			}
		case "must_include":
			if !decodeCreativeBriefField(w, field, value, &input.MustInclude) {
				return creativebrief.PutInput{}, false
			}
		case "must_avoid":
			if !decodeCreativeBriefField(w, field, value, &input.MustAvoid) {
				return creativebrief.PutInput{}, false
			}
		default:
			writeCreativeBriefAPIError(w, creativebrief.ValidationError{Fields: map[string]string{field: "unknown"}})
			return creativebrief.PutInput{}, false
		}
	}
	return input, true
}

func decodeCreativeBriefField(w http.ResponseWriter, field string, raw json.RawMessage, target any) bool {
	if err := json.Unmarshal(raw, target); err != nil {
		writeCreativeBriefAPIError(w, creativebrief.ValidationError{Fields: map[string]string{field: "invalid"}})
		return false
	}
	return true
}

func writeCreativeBriefAPIError(w http.ResponseWriter, err error) {
	var validationErr creativebrief.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  validationErr.Fields,
		}})
	case errors.Is(err, creativebrief.ErrRevisionRequired):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  map[string]string{"revision": "required"},
		}})
	case errors.Is(err, creativebrief.ErrRevisionUnexpected):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "validation_failed",
			Message: "Request validation failed.",
			Fields:  map[string]string{"revision": "not_allowed"},
		}})
	case errors.Is(err, creativebrief.ErrStaleRevision):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "STALE_REVISION",
			Message: "Creative brief revision is stale.",
		}})
	case errors.Is(err, creativebrief.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "creative_brief_not_found",
			Message: "Creative brief was not found.",
		}})
	case errors.Is(err, creativebrief.ErrUnauthenticated), errors.Is(err, project.ErrUnauthenticated):
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

func toCreativeBriefResponse(item creativebrief.CreativeBrief) creativeBriefResponse {
	return creativeBriefResponse{
		ProjectID:           item.ProjectID.String(),
		Revision:            item.Revision,
		SourceText:          item.SourceText,
		TargetAudience:      item.TargetAudience,
		Objective:           item.Objective,
		DesiredStyle:        item.DesiredStyle,
		Tone:                item.Tone,
		DistributionTargets: item.DistributionTargets,
		CallToAction:        item.CallToAction,
		MustInclude:         item.MustInclude,
		MustAvoid:           item.MustAvoid,
		CreatedAt:           item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:           item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
