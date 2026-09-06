package stockmedia

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

var (
	ErrProviderUnavailable = errors.New("stockmedia: provider unavailable")
	ErrInvalidSelection    = errors.New("stockmedia: invalid selection")
)

type ProjectAccess interface {
	Get(context.Context, uuid.UUID, uuid.UUID) (project.Project, error)
}

type DurableAssetStore interface {
	Store(context.Context, project.Principal, uuid.UUID, mediaasset.CreateInput) (mediaasset.MediaAsset, error)
	FindStockOrigin(context.Context, project.Principal, uuid.UUID, string, string, mediaasset.Kind) (mediaasset.MediaAsset, error)
}

type Service struct {
	projects      ProjectAccess
	assets        DurableAssetStore
	providers     map[string]Provider
	maxAssetBytes int64
}

func NewService(projects ProjectAccess, assets DurableAssetStore, providers map[string]Provider, maxAssetBytes int64) (*Service, error) {
	if projects == nil || assets == nil || maxAssetBytes <= 0 {
		return nil, ErrInvalidSelection
	}
	cloned := make(map[string]Provider, len(providers))
	for key, provider := range providers {
		key = strings.TrimSpace(key)
		if key == "" || provider == nil {
			return nil, ErrInvalidSelection
		}
		cloned[key] = provider
	}
	return &Service{projects: projects, assets: assets, providers: cloned, maxAssetBytes: maxAssetBytes}, nil
}

func (s *Service) Search(ctx context.Context, principal project.Principal, projectID uuid.UUID, providerKey string, request SearchRequest) (SearchPage, error) {
	provider, err := s.authorizeAndProvider(ctx, principal, projectID, providerKey)
	if err != nil {
		return SearchPage{}, err
	}
	return provider.Search(ctx, request)
}

type AcquireInput struct {
	ProviderKey      string
	ProviderResultID string
	Kind             MediaKind
}

type Acquisition struct {
	Asset  mediaasset.MediaAsset
	Reused bool
}

func (s *Service) Acquire(ctx context.Context, principal project.Principal, projectID uuid.UUID, input AcquireInput) (Acquisition, error) {
	providerKey := strings.TrimSpace(input.ProviderKey)
	resultID := strings.TrimSpace(input.ProviderResultID)
	if resultID == "" || input.Kind != MediaKindImage && input.Kind != MediaKindVideo {
		return Acquisition{}, ErrInvalidSelection
	}
	provider, err := s.authorizeAndProvider(ctx, principal, projectID, providerKey)
	if err != nil {
		return Acquisition{}, err
	}
	assetKind := mediaAssetKind(input.Kind)
	if existing, err := s.assets.FindStockOrigin(ctx, principal, projectID, providerKey, resultID, assetKind); err == nil {
		return Acquisition{Asset: existing, Reused: true}, nil
	} else if !errors.Is(err, mediaasset.ErrNotFound) {
		return Acquisition{}, err
	}

	source, err := provider.ResolveForAcquisition(ctx, resultID, input.Kind)
	if err != nil {
		return Acquisition{}, err
	}
	if source.Remote == nil || source.Result.ProviderKey != providerKey || source.Result.ProviderResultID != resultID || source.Result.Kind != input.Kind || !source.Result.Acquirable {
		return Acquisition{}, ProviderError{Kind: ProviderErrorTransient, Provider: providerKey, Err: ErrInvalidSelection}
	}
	if err := source.Result.Validate(); err != nil {
		return Acquisition{}, ProviderError{Kind: ProviderErrorTransient, Provider: providerKey, Err: err}
	}

	reader, err := source.Remote.Open(ctx)
	if err != nil {
		return Acquisition{}, err
	}
	defer reader.Close()
	metadata, err := acquisitionMetadata(source.Result)
	if err != nil {
		return Acquisition{}, err
	}
	created, err := s.assets.Store(ctx, principal, projectID, mediaasset.CreateInput{
		Kind:             assetKind,
		Origin:           mediaasset.OriginStock,
		MimeType:         source.Remote.ContentType(),
		OriginalFilename: source.Filename,
		Metadata:         metadata,
		Reader:           reader,
		MaxBytes:         s.maxAssetBytes,
	})
	if err == nil {
		return Acquisition{Asset: created}, nil
	}

	if errors.Is(err, mediaasset.ErrPersistenceFailed) {
		if existing, lookupErr := s.assets.FindStockOrigin(ctx, principal, projectID, providerKey, resultID, assetKind); lookupErr == nil {
			return Acquisition{Asset: existing, Reused: true}, nil
		}
	}
	return Acquisition{}, err
}

func (s *Service) authorizeAndProvider(ctx context.Context, principal project.Principal, projectID uuid.UUID, providerKey string) (Provider, error) {
	if principal.OwnerID == uuid.Nil || projectID == uuid.Nil {
		return nil, project.ErrNotFound
	}
	if _, err := s.projects.Get(ctx, principal.OwnerID, projectID); err != nil {
		return nil, err
	}
	provider := s.providers[strings.TrimSpace(providerKey)]
	if provider == nil {
		return nil, ErrProviderUnavailable
	}
	return provider, nil
}

func mediaAssetKind(kind MediaKind) mediaasset.Kind {
	if kind == MediaKindVideo {
		return mediaasset.KindVideo
	}
	return mediaasset.KindImage
}

func acquisitionMetadata(result SearchResult) (json.RawMessage, error) {
	payload := map[string]any{
		"stock_provider":          result.ProviderKey,
		"stock_result_id":         result.ProviderResultID,
		"stock_source_page_url":   result.SourcePageURL,
		"stock_creator_name":      result.CreatorName,
		"stock_creator_url":       result.CreatorURL,
		"stock_license_summary":   result.LicenseSummary,
		"stock_license_reference": result.LicenseReference,
		"stock_attribution_text":  result.AttributionText,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}
