package httpserver

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

type VideoGenerationOptionsService interface {
	GetOwnerVideoGenerationOptions(context.Context, uuid.UUID) (providersettings.VideoGenerationOptionsResponse, error)
}

type videoGenerationOptionsHandler struct {
	service       VideoGenerationOptionsService
	actorResolver actor.Resolver
}

func (h videoGenerationOptionsHandler) get(w http.ResponseWriter, r *http.Request) {
	if h.actorResolver == nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeAPIError(w, project.ErrUnauthenticated)
		return
	}
	response, err := h.service.GetOwnerVideoGenerationOptions(r.Context(), principal.OwnerID)
	if err != nil {
		writeProviderSettingsAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, response)
}
