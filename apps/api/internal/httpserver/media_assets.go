package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

type MediaAssetService interface {
	Store(context.Context, project.Principal, uuid.UUID, mediaasset.CreateInput) (mediaasset.MediaAsset, error)
	Get(context.Context, project.Principal, uuid.UUID, uuid.UUID) (mediaasset.MediaAsset, error)
	List(context.Context, project.Principal, uuid.UUID, int) (mediaasset.ListResult, error)
	Open(context.Context, project.Principal, uuid.UUID, uuid.UUID) (io.ReadCloser, error)
	OpenRange(context.Context, project.Principal, uuid.UUID, uuid.UUID, int64, int64) (io.ReadCloser, error)
	Delete(context.Context, project.Principal, uuid.UUID, uuid.UUID) error
}

type mediaAssetHandler struct {
	service       MediaAssetService
	actorResolver actor.Resolver
	maxUploadSize int64
}

type mediaAssetListResponse struct {
	Assets []mediaasset.MediaAsset `json:"assets"`
}

func (h mediaAssetHandler) upload(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	reader, err := r.MultipartReader()
	if err != nil {
		writeMediaAssetAPIError(w, mediaasset.ErrInvalidInput)
		return
	}
	var metadata []byte
	var stored *mediaasset.MediaAsset
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			writeMediaAssetAPIError(w, mediaasset.ErrInvalidInput)
			return
		}
		if stored != nil {
			_ = part.Close()
			h.deleteAfterUploadFailure(r, principal, projectID, stored.ID)
			writeMediaAssetAPIError(w, mediaasset.ErrInvalidInput)
			return
		}

		if filename := part.FileName(); filename != "" {
			if part.FormName() != "file" {
				_ = part.Close()
				writeMediaAssetAPIError(w, mediaasset.ErrInvalidInput)
				return
			}
			mimeType, kind, valid := uploadMIME(part.Header.Get("Content-Type"))
			if !valid {
				_ = part.Close()
				writeMediaAssetAPIError(w, mediaasset.ErrUnsupportedType)
				return
			}
			asset, storeErr := h.service.Store(r.Context(), principal, projectID, mediaasset.CreateInput{
				Kind:             kind,
				Origin:           mediaasset.OriginUpload,
				MimeType:         mimeType,
				OriginalFilename: safeFilename(filename),
				Metadata:         metadata,
				Reader:           part,
				MaxBytes:         h.maxUploadSize,
			})
			_ = part.Close()
			if storeErr != nil {
				writeMediaAssetAPIError(w, storeErr)
				return
			}
			stored = &asset
			continue
		}

		if part.FormName() != "metadata" || len(metadata) != 0 {
			_ = part.Close()
			writeMediaAssetAPIError(w, mediaasset.ErrInvalidInput)
			return
		}
		limited := io.LimitReader(part, 64*1024+1)
		metadata, err = io.ReadAll(limited)
		_ = part.Close()
		if err != nil || len(metadata) > 64*1024 {
			writeMediaAssetAPIError(w, mediaasset.ErrInvalidInput)
			return
		}
	}

	if stored == nil {
		writeMediaAssetAPIError(w, mediaasset.ErrInvalidInput)
		return
	}
	writeProjectJSON(w, http.StatusCreated, *stored)
}

func (h mediaAssetHandler) list(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeMediaAssetAPIError(w, mediaasset.ValidationError{Fields: map[string]string{"limit": "invalid"}})
			return
		}
		limit = parsed
	}
	result, err := h.service.List(r.Context(), principal, projectID, limit)
	if err != nil {
		writeMediaAssetAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, mediaAssetListResponse{Assets: result.Assets})
}

func (h mediaAssetHandler) get(w http.ResponseWriter, r *http.Request) {
	principal, projectID, assetID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	asset, err := h.service.Get(r.Context(), principal, projectID, assetID)
	if err != nil {
		writeMediaAssetAPIError(w, err)
		return
	}
	writeProjectJSON(w, http.StatusOK, asset)
}

func (h mediaAssetHandler) content(w http.ResponseWriter, r *http.Request) {
	principal, projectID, assetID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	asset, err := h.service.Get(r.Context(), principal, projectID, assetID)
	if err != nil {
		writeMediaAssetAPIError(w, err)
		return
	}

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", safeContentType(asset.MimeType))
	w.Header().Set("Content-Disposition", `inline; filename="`+safeFilename(asset.OriginalFilename)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	start, length := int64(0), asset.ByteSize
	status := http.StatusOK
	if rawRange := r.Header.Get("Range"); rawRange != "" {
		var rangeErr error
		start, length, rangeErr = parseSingleRange(rawRange, asset.ByteSize)
		if rangeErr != nil {
			w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(asset.ByteSize, 10))
			writeProjectJSON(w, http.StatusRequestedRangeNotSatisfiable, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_INVALID", Message: "The requested byte range is invalid."}})
			return
		}
		status = http.StatusPartialContent
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, asset.ByteSize))
	}
	var reader io.ReadCloser
	if status == http.StatusPartialContent {
		reader, err = h.service.OpenRange(r.Context(), principal, projectID, assetID, start, length)
	} else {
		reader, err = h.service.Open(r.Context(), principal, projectID, assetID)
	}
	if err != nil {
		writeMediaAssetAPIError(w, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(status)
	_, _ = io.Copy(w, reader)
}

func (h mediaAssetHandler) delete(w http.ResponseWriter, r *http.Request) {
	principal, projectID, assetID, ok := h.identifiers(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), principal, projectID, assetID); err != nil {
		writeMediaAssetAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h mediaAssetHandler) identifiers(w http.ResponseWriter, r *http.Request) (project.Principal, uuid.UUID, uuid.UUID, bool) {
	principal, ok := h.resolvePrincipal(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, uuid.Nil, false
	}
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return project.Principal{}, uuid.Nil, uuid.Nil, false
	}
	assetID, err := uuid.Parse(r.PathValue("asset_id"))
	if err != nil || assetID == uuid.Nil {
		writeMediaAssetAPIError(w, mediaasset.ErrNotFound)
		return project.Principal{}, uuid.Nil, uuid.Nil, false
	}
	return principal, projectID, assetID, true
}

func (h mediaAssetHandler) resolvePrincipal(w http.ResponseWriter, r *http.Request) (project.Principal, bool) {
	if h.actorResolver == nil {
		writeMediaAssetAPIError(w, mediaasset.ErrUnauthenticated)
		return project.Principal{}, false
	}
	principal, err := h.actorResolver.Resolve(r)
	if err != nil {
		writeMediaAssetAPIError(w, mediaasset.ErrUnauthenticated)
		return project.Principal{}, false
	}
	return principal, true
}

func (h mediaAssetHandler) deleteAfterUploadFailure(r *http.Request, principal project.Principal, projectID, assetID uuid.UUID) {
	_ = h.service.Delete(r.Context(), principal, projectID, assetID)
}

func uploadMIME(raw string) (string, mediaasset.Kind, bool) {
	parsed, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return "", "", false
	}
	typeInfo := supportedUploadMIMETypes
	kind, ok := typeInfo[strings.ToLower(parsed)]
	return strings.ToLower(parsed), kind, ok
}

func safeContentType(raw string) string {
	parsed, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return "application/octet-stream"
	}
	canonical := strings.ToLower(parsed)
	if _, ok := supportedUploadMIMETypes[canonical]; !ok {
		return "application/octet-stream"
	}
	return canonical
}

var supportedUploadMIMETypes = map[string]mediaasset.Kind{
	"image/avif": mediaasset.KindImage, "image/gif": mediaasset.KindImage, "image/jpeg": mediaasset.KindImage,
	"image/png": mediaasset.KindImage, "image/webp": mediaasset.KindImage,
	"video/mp4": mediaasset.KindVideo, "video/quicktime": mediaasset.KindVideo, "video/webm": mediaasset.KindVideo,
	"audio/aac": mediaasset.KindAudio, "audio/flac": mediaasset.KindAudio, "audio/mpeg": mediaasset.KindAudio,
	"audio/mp4": mediaasset.KindAudio, "audio/ogg": mediaasset.KindAudio, "audio/wav": mediaasset.KindAudio,
	"audio/x-wav": mediaasset.KindAudio,
}

func safeFilename(raw string) string {
	name := filepath.Base(strings.TrimSpace(raw))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return "asset"
	}
	return name
}

func parseSingleRange(raw string, size int64) (int64, int64, error) {
	if size < 1 || !strings.HasPrefix(raw, "bytes=") {
		return 0, 0, mediaasset.ErrRangeInvalid
	}
	value := strings.TrimSpace(strings.TrimPrefix(raw, "bytes="))
	if value == "" || strings.Contains(value, ",") {
		return 0, 0, mediaasset.ErrRangeInvalid
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, mediaasset.ErrRangeInvalid
	}
	if strings.TrimSpace(parts[0]) == "" {
		suffix, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || suffix < 1 {
			return 0, 0, mediaasset.ErrRangeInvalid
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, suffix, nil
	}
	start, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, mediaasset.ErrRangeInvalid
	}
	end := size - 1
	if strings.TrimSpace(parts[1]) != "" {
		end, err = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || end < start {
			return 0, 0, mediaasset.ErrRangeInvalid
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end - start + 1, nil
}

func writeMediaAssetAPIError(w http.ResponseWriter, err error) {
	var validation mediaasset.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_INVALID", Message: "Media asset request is invalid.", Fields: validation.Fields}})
	case errors.Is(err, mediaasset.ErrTooLarge):
		writeProjectJSON(w, http.StatusRequestEntityTooLarge, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_TOO_LARGE", Message: "Media asset exceeds the upload limit."}})
	case errors.Is(err, mediaasset.ErrUnsupportedType):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_UNSUPPORTED_TYPE", Message: "Media asset type is not supported."}})
	case errors.Is(err, mediaasset.ErrInvalidInput), errors.Is(err, mediaasset.ErrRangeInvalid):
		writeProjectJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_INVALID", Message: "Media asset request is invalid."}})
	case errors.Is(err, mediaasset.ErrNotFound), errors.Is(err, mediaasset.ErrObjectNotFound):
		writeProjectJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_NOT_FOUND", Message: "Media asset was not found."}})
	case errors.Is(err, mediaasset.ErrInUse):
		writeProjectJSON(w, http.StatusConflict, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_IN_USE", Message: "Media asset is referenced by scene media bindings."}})
	case errors.Is(err, mediaasset.ErrUnauthenticated):
		writeProjectJSON(w, http.StatusUnauthorized, errorEnvelope{Error: apiError{Code: "principal_required", Message: "A request principal is required."}})
	case errors.Is(err, mediaasset.ErrStorageFailed):
		writeProjectJSON(w, http.StatusBadGateway, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_STORAGE_FAILED", Message: "Media asset storage could not complete the request."}})
	case errors.Is(err, mediaasset.ErrPersistenceFailed):
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "MEDIA_ASSET_PERSISTENCE_FAILED", Message: "Media asset metadata could not be persisted."}})
	default:
		writeProjectJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "internal_error", Message: "The request could not be completed."}})
	}
}
