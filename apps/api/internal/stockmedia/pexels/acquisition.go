package pexels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia"
)

func (a *Adapter) ResolveForAcquisition(ctx context.Context, resultID string, kind stockmedia.MediaKind) (stockmedia.AcquisitionSource, error) {
	resultID = strings.TrimSpace(resultID)
	if resultID == "" || strings.ContainsAny(resultID, "/?#") {
		return stockmedia.AcquisitionSource{}, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorInvalid, Provider: ProviderKey, Err: errors.New("invalid provider result id")}
	}

	switch kind {
	case stockmedia.MediaKindImage:
		var item photo
		if err := a.getJSON(ctx, "/v1/photos/"+url.PathEscape(resultID), &item); err != nil {
			return stockmedia.AcquisitionSource{}, err
		}
		link := strings.TrimSpace(item.Src.Original)
		if link == "" {
			return stockmedia.AcquisitionSource{}, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorRemoved, Provider: ProviderKey, Err: errors.New("original image is unavailable")}
		}
		remote, err := a.remote(link, "image/jpeg")
		if err != nil {
			return stockmedia.AcquisitionSource{}, err
		}
		result := stockmedia.SearchResult{
			ProviderKey:      ProviderKey,
			ProviderResultID: resultID,
			Kind:             stockmedia.MediaKindImage,
			PreviewURL:       strings.TrimSpace(item.Src.Medium),
			SourcePageURL:    strings.TrimSpace(item.URL),
			CreatorName:      strings.TrimSpace(item.Photographer),
			CreatorURL:       strings.TrimSpace(item.PhotographerURL),
			LicenseSummary:   licenseSummary,
			LicenseReference: licenseReference,
			AttributionText:  attribution(item.Photographer),
			Acquirable:       true,
		}
		if err := result.Validate(); err != nil {
			return stockmedia.AcquisitionSource{}, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: err}
		}
		return stockmedia.AcquisitionSource{Result: result, Filename: "pexels-" + resultID + ".jpg", Remote: remote}, nil
	case stockmedia.MediaKindVideo:
		var item video
		if err := a.getJSON(ctx, "/videos/videos/"+url.PathEscape(resultID), &item); err != nil {
			return stockmedia.AcquisitionSource{}, err
		}
		file, ok := selectVideoFile(item.VideoFiles)
		if !ok {
			return stockmedia.AcquisitionSource{}, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorRemoved, Provider: ProviderKey, Err: errors.New("downloadable video is unavailable")}
		}
		remote, err := a.remote(file.Link, file.FileType)
		if err != nil {
			return stockmedia.AcquisitionSource{}, err
		}
		result := stockmedia.SearchResult{
			ProviderKey:      ProviderKey,
			ProviderResultID: resultID,
			Kind:             stockmedia.MediaKindVideo,
			PreviewURL:       strings.TrimSpace(item.Image),
			SourcePageURL:    strings.TrimSpace(item.URL),
			CreatorName:      strings.TrimSpace(item.User.Name),
			CreatorURL:       strings.TrimSpace(item.User.URL),
			LicenseSummary:   licenseSummary,
			LicenseReference: licenseReference,
			AttributionText:  attribution(item.User.Name),
			Acquirable:       true,
		}
		if err := result.Validate(); err != nil {
			return stockmedia.AcquisitionSource{}, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: err}
		}
		return stockmedia.AcquisitionSource{Result: result, Filename: "pexels-" + resultID + ".mp4", Remote: remote}, nil
	default:
		return stockmedia.AcquisitionSource{}, stockmedia.ErrUnsupportedKind
	}
}

func (a *Adapter) remote(link, contentType string) (stockmedia.RemoteAsset, error) {
	parsed, err := url.Parse(strings.TrimSpace(link))
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return nil, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: errors.New("provider returned invalid download URL")}
	}
	return &remoteAsset{client: a.client, link: parsed.String(), contentType: strings.TrimSpace(contentType)}, nil
}

func selectVideoFile(files []videoFile) (videoFile, bool) {
	candidates := make([]videoFile, 0, len(files))
	for _, file := range files {
		if strings.TrimSpace(file.Link) == "" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(file.FileType)), "video/") {
			continue
		}
		candidates = append(candidates, file)
	}
	if len(candidates) == 0 {
		return videoFile{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		areaI := candidates[i].Width * candidates[i].Height
		areaJ := candidates[j].Width * candidates[j].Height
		if areaI == areaJ {
			return candidates[i].ID < candidates[j].ID
		}
		return areaI > areaJ
	})
	return candidates[0], true
}

type remoteAsset struct {
	client      *http.Client
	link        string
	contentType string
}

func (r *remoteAsset) ContentType() string { return r.contentType }

func (r *remoteAsset) Open(ctx context.Context) (stockmedia.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.link, nil)
	if err != nil {
		return nil, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorAcquisition, Provider: ProviderKey, Err: err}
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorAcquisition, Provider: ProviderKey, Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		kind := stockmedia.ProviderErrorAcquisition
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			kind = stockmedia.ProviderErrorRemoved
		}
		return nil, stockmedia.ProviderError{Kind: kind, Provider: ProviderKey, RetryAfter: resp.Header.Get("Retry-After"), Err: fmt.Errorf("download HTTP %d", resp.StatusCode)}
	}
	return resp.Body, nil
}

var _ stockmedia.Provider = (*Adapter)(nil)
