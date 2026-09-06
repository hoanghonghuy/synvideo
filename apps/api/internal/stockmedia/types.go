package stockmedia

import (
	"errors"
	"strings"
)

const MaxPerPage = 50

type MediaKind string

const (
	MediaKindImage MediaKind = "image"
	MediaKindVideo MediaKind = "video"
)

type Orientation string

const (
	OrientationAny       Orientation = ""
	OrientationLandscape Orientation = "landscape"
	OrientationPortrait  Orientation = "portrait"
	OrientationSquare    Orientation = "square"
)

var (
	ErrInvalidQuery           = errors.New("stockmedia: invalid query")
	ErrUnsupportedKind        = errors.New("stockmedia: unsupported media kind")
	ErrUnsupportedOrientation = errors.New("stockmedia: unsupported orientation")
	ErrInvalidPage            = errors.New("stockmedia: invalid page")
	ErrInvalidPerPage         = errors.New("stockmedia: invalid per-page value")
	ErrInvalidResult          = errors.New("stockmedia: invalid provider result")
)

type SearchRequest struct {
	Query       string
	Kind        MediaKind
	Orientation Orientation
	Page        int
	PerPage     int
}

func (r SearchRequest) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return ErrInvalidQuery
	}
	if r.Kind != MediaKindImage && r.Kind != MediaKindVideo {
		return ErrUnsupportedKind
	}
	switch r.Orientation {
	case OrientationAny, OrientationLandscape, OrientationPortrait, OrientationSquare:
	default:
		return ErrUnsupportedOrientation
	}
	if r.Page < 1 {
		return ErrInvalidPage
	}
	if r.PerPage < 1 || r.PerPage > MaxPerPage {
		return ErrInvalidPerPage
	}
	return nil
}

type SearchResult struct {
	ProviderKey      string    `json:"provider_key"`
	ProviderResultID string    `json:"provider_result_id"`
	Kind             MediaKind `json:"kind"`
	PreviewURL       string    `json:"preview_url"`
	SourcePageURL    string    `json:"source_page_url"`
	CreatorName      string    `json:"creator_name"`
	CreatorURL       string    `json:"creator_url"`
	LicenseSummary   string    `json:"license_summary"`
	LicenseReference string    `json:"license_reference"`
	AttributionText  string    `json:"attribution_text"`
	Acquirable       bool      `json:"acquirable"`
}

func (r SearchResult) Validate() error {
	if strings.TrimSpace(r.ProviderKey) == "" || strings.TrimSpace(r.ProviderResultID) == "" {
		return ErrInvalidResult
	}
	if r.Kind != MediaKindImage && r.Kind != MediaKindVideo {
		return ErrInvalidResult
	}
	if strings.TrimSpace(r.PreviewURL) == "" || strings.TrimSpace(r.SourcePageURL) == "" || strings.TrimSpace(r.LicenseSummary) == "" || strings.TrimSpace(r.AttributionText) == "" {
		return ErrInvalidResult
	}
	return nil
}

type SearchPage struct {
	Results     []SearchResult `json:"results"`
	Page        int            `json:"page"`
	PerPage     int            `json:"per_page"`
	HasNextPage bool           `json:"has_next_page"`
}

type ProviderErrorKind string

const (
	ProviderErrorRateLimited  ProviderErrorKind = "rate_limited"
	ProviderErrorRemoved      ProviderErrorKind = "source_unavailable"
	ProviderErrorUnauthorized ProviderErrorKind = "authorization_failed"
	ProviderErrorTransient    ProviderErrorKind = "transient"
	ProviderErrorInvalid      ProviderErrorKind = "invalid_request"
	ProviderErrorAcquisition  ProviderErrorKind = "acquisition_failed"
)

type ProviderError struct {
	Kind       ProviderErrorKind
	Provider   string
	RetryAfter string
	Err        error
}

func (e ProviderError) Error() string {
	if e.Err != nil {
		return "stockmedia: provider " + string(e.Kind) + ": " + e.Err.Error()
	}
	return "stockmedia: provider " + string(e.Kind)
}

func (e ProviderError) Unwrap() error { return e.Err }

func (e ProviderError) Recoverable() bool {
	return e.Kind == ProviderErrorRateLimited || e.Kind == ProviderErrorTransient || e.Kind == ProviderErrorAcquisition
}
