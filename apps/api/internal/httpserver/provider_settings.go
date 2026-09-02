package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

type ProviderSettingsService interface {
	ListSettings(ctx context.Context, ownerID uuid.UUID) (providersettings.ProviderSettingsListResponse, error)
	PutSetting(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, input providersettings.PutSettingInput) (providersettings.ProviderSettingView, error)
	DeleteSetting(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, revision int) error
	GetOwnerImageGenerationOptions(ctx context.Context, ownerID uuid.UUID) (providersettings.ImageGenerationOptionsResponse, error)
	GetOwnerTTSOptions(ctx context.Context, ownerID uuid.UUID) (providersettings.TTSOptionsResponse, error)
}

type providerSettingsHandler struct {
	service       ProviderSettingsService
	actorResolver actor.Resolver
}

func (h providerSettingsHandler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	resp, err := h.service.ListSettings(r.Context(), principal.OwnerID)
	if err != nil {
		writeProviderSettingsAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, resp)
}

func (h providerSettingsHandler) put(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	providerIDStr := r.PathValue("provider_id")
	if providerIDStr == "" {
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "invalid_provider_id",
			Message: "provider_id path parameter is required",
		}})
		return
	}

	var input providersettings.PutSettingInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "invalid_json",
			Message: "request body is not valid JSON",
		}})
		return
	}

	view, err := h.service.PutSetting(r.Context(), principal.OwnerID, providers.ProviderID(providerIDStr), input)
	if err != nil {
		writeProviderSettingsAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, view)
}

func (h providerSettingsHandler) delete(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	providerIDStr := r.PathValue("provider_id")
	if providerIDStr == "" {
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "invalid_provider_id",
			Message: "provider_id path parameter is required",
		}})
		return
	}

	revisionStr := r.URL.Query().Get("revision")
	if revisionStr == "" {
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "revision_required",
			Message: "revision query parameter is required",
		}})
		return
	}

	revision, err := strconv.Atoi(revisionStr)
	if err != nil || revision <= 0 {
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "invalid_revision",
			Message: "revision must be a positive integer",
		}})
		return
	}

	if err := h.service.DeleteSetting(r.Context(), principal.OwnerID, providers.ProviderID(providerIDStr), revision); err != nil {
		writeProviderSettingsAPIError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h providerSettingsHandler) getImageGenerationOptions(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	resp, err := h.service.GetOwnerImageGenerationOptions(r.Context(), principal.OwnerID)
	if err != nil {
		writeProviderSettingsAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, resp)
}

func (h providerSettingsHandler) getTTSOptions(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}

	resp, err := h.service.GetOwnerTTSOptions(r.Context(), principal.OwnerID)
	if err != nil {
		writeProviderSettingsAPIError(w, err)
		return
	}

	writeProjectJSON(w, http.StatusOK, resp)
}

func (h providerSettingsHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	if h.actorResolver == nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func writeProviderSettingsAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, providersettings.ErrStaleRevision):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{
			Code:    "STALE_REVISION",
			Message: "Provider setting revision is stale.",
		}})
	case errors.Is(err, providersettings.ErrSettingNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "SETTING_NOT_FOUND",
			Message: "Provider setting not found.",
		}})
	case errors.Is(err, providersettings.ErrProviderNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "PROVIDER_NOT_FOUND",
			Message: "Provider not found in catalog.",
		}})
	case errors.Is(err, providersettings.ErrModelNotFound):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "MODEL_NOT_FOUND",
			Message: "Model or voice not found in provider catalog.",
		}})
	case errors.Is(err, providersettings.ErrCredentialRequired):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "CREDENTIAL_REQUIRED",
			Message: "api_key is required for initial configuration.",
		}})
	case errors.Is(err, providersettings.ErrInvalidSettingInput):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{
			Code:    "INVALID_INPUT",
			Message: err.Error(),
		}})
	case errors.Is(err, providersettings.ErrMasterKeyMissing):
		writeProjectJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{
			Code:    "BYOK_UNAVAILABLE",
			Message: "Credential encryption is unavailable.",
		}})
	case errors.Is(err, providersettings.ErrUnauthenticated):
		writeAPIError(w, project.ErrUnauthenticated)
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{
			Code:    "INTERNAL_ERROR",
			Message: "An unexpected error occurred.",
		}})
	}
}
