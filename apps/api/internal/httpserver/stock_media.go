package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia"
)

type StockMediaService interface {
	Search(context.Context, project.Principal, uuid.UUID, string, stockmedia.SearchRequest) (stockmedia.SearchPage, error)
	Acquire(context.Context, project.Principal, uuid.UUID, stockmedia.AcquireInput) (stockmedia.Acquisition, error)
}

type stockMediaHandler struct {
	service       StockMediaService
	actorResolver actor.Resolver
}

type stockAcquireRequest struct {
	ProviderKey      string               `json:"provider_key"`
	ProviderResultID string               `json:"provider_result_id"`
	Kind             stockmedia.MediaKind `json:"kind"`
}

func (h stockMediaHandler) search(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identities(w, r)
	if !ok {
		return
	}
	page, err := positiveQueryInt(r, "page", 1)
	if err != nil {
		writeStockMediaAPIError(w, stockmedia.ErrInvalidPage)
		return
	}
	perPage, err := positiveQueryInt(r, "per_page", 20)
	if err != nil {
		writeStockMediaAPIError(w, stockmedia.ErrInvalidPerPage)
		return
	}
	result, err := h.service.Search(r.Context(), principal, projectID, r.URL.Query().Get("provider"), stockmedia.SearchRequest{
		Query:       r.URL.Query().Get("q"),
		Kind:        stockmedia.MediaKind(strings.TrimSpace(r.URL.Query().Get("kind"))),
		Orientation: stockmedia.Orientation(strings.TrimSpace(r.URL.Query().Get("orientation"))),
		Page:        page,
		PerPage:     perPage,
	})
	if err != nil {
		writeStockMediaAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, result)
}

func (h stockMediaHandler) acquire(w http.ResponseWriter, r *http.Request) {
	principal, projectID, ok := h.identities(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()
	var body stockAcquireRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeStockMediaAPIError(w, stockmedia.ErrInvalidSelection)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		writeStockMediaAPIError(w, stockmedia.ErrInvalidSelection)
		return
	}
	result, err := h.service.Acquire(r.Context(), principal, projectID, stockmedia.AcquireInput{
		ProviderKey:      body.ProviderKey,
		ProviderResultID: body.ProviderResultID,
		Kind:             body.Kind,
	})
	if err != nil {
		writeStockMediaAPIError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Reused {
		status = http.StatusOK
	}
	writeProjectJSON(w, status, result)
}

func (h stockMediaHandler) identities(w http.ResponseWriter, r *http.Request) (project.Principal, uuid.UUID, bool) {
	if h.actorResolver == nil {
		writeStockMediaAPIError(w, mediaasset.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeStockMediaAPIError(w, mediaasset.ErrUnauthenticated)
		return project.Principal{}, uuid.Nil, false
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, false
	}
	return principal, projectID, true
}

func positiveQueryInt(r *http.Request, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, stockmedia.ErrInvalidPage
	}
	return value, nil
}

func writeStockMediaAPIError(w http.ResponseWriter, err error) {
	var providerErr stockmedia.ProviderError
	switch {
	case errors.Is(err, mediaasset.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, project.ErrNotFound), errors.Is(err, mediaasset.ErrNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "STOCK_MEDIA_NOT_FOUND", Message: "The project or stock media item was not found."}})
	case errors.Is(err, stockmedia.ErrInvalidQuery), errors.Is(err, stockmedia.ErrUnsupportedKind), errors.Is(err, stockmedia.ErrUnsupportedOrientation), errors.Is(err, stockmedia.ErrInvalidPage), errors.Is(err, stockmedia.ErrInvalidPerPage), errors.Is(err, stockmedia.ErrInvalidSelection):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "STOCK_MEDIA_INVALID", Message: "Stock media request is invalid."}})
	case errors.Is(err, stockmedia.ErrProviderUnavailable):
		writeProjectJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "STOCK_MEDIA_PROVIDER_UNAVAILABLE", Message: "Stock media provider is unavailable."}})
	case errors.As(err, &providerErr):
		status := http.StatusBadGateway
		code := "STOCK_MEDIA_PROVIDER_FAILED"
		message := "Stock media provider could not complete the request."
		switch providerErr.Kind {
		case stockmedia.ProviderErrorRateLimited:
			status = http.StatusTooManyRequests
			code = "STOCK_MEDIA_RATE_LIMITED"
			message = "Stock media provider is rate limited."
			if providerErr.RetryAfter != "" {
				w.Header().Set("Retry-After", providerErr.RetryAfter)
			}
		case stockmedia.ProviderErrorRemoved:
			status = http.StatusGone
			code = "STOCK_MEDIA_SOURCE_UNAVAILABLE"
			message = "The selected stock media item is no longer available."
		case stockmedia.ProviderErrorUnauthorized:
			status = http.StatusServiceUnavailable
			code = "STOCK_MEDIA_PROVIDER_AUTH_FAILED"
			message = "Stock media provider authorization is unavailable."
		case stockmedia.ProviderErrorInvalid:
			status = http.StatusBadRequest
			code = "STOCK_MEDIA_PROVIDER_REJECTED"
			message = "Stock media provider rejected the request."
		}
		writeProjectJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
	case errors.Is(err, mediaasset.ErrTooLarge):
		writeProjectJSON(w, http.StatusRequestEntityTooLarge, errorEnvelope{Error: apiError{Code: "STOCK_MEDIA_TOO_LARGE", Message: "Selected stock media exceeds the project media limit."}})
	case errors.Is(err, mediaasset.ErrStorageFailed):
		writeProjectJSON(w, http.StatusBadGateway, errorEnvelope{Error: apiError{Code: "STOCK_MEDIA_STORAGE_FAILED", Message: "Stock media could not be stored durably."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}
