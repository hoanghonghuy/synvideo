package pexels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia"
)

const (
	ProviderKey             = "pexels"
	defaultBaseURL          = "https://api.pexels.com"
	defaultTimeout          = 15 * time.Second
	defaultMaxResponseBytes = int64(2 << 20)
	licenseSummary          = "Pexels License"
	licenseReference        = "https://www.pexels.com/license/"
)

type Config struct {
	BaseURL          string
	APIKey           string
	HTTPClient       *http.Client
	Timeout          time.Duration
	MaxResponseBytes int64
}

type Adapter struct {
	baseURL          string
	apiKey           string
	client           *http.Client
	maxResponseBytes int64
}

func New(cfg Config) (*Adapter, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("pexels: API key is required")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("pexels: invalid base URL")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	maxResponseBytes := cfg.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}
	return &Adapter{
		baseURL:          strings.TrimRight(baseURL, "/"),
		apiKey:           apiKey,
		client:           client,
		maxResponseBytes: maxResponseBytes,
	}, nil
}

func (a *Adapter) Search(ctx context.Context, request stockmedia.SearchRequest) (stockmedia.SearchPage, error) {
	if err := request.Validate(); err != nil {
		return stockmedia.SearchPage{}, err
	}

	path := "/v1/search"
	if request.Kind == stockmedia.MediaKindVideo {
		path = "/videos/search"
	}
	values := url.Values{}
	values.Set("query", strings.TrimSpace(request.Query))
	values.Set("page", strconv.Itoa(request.Page))
	values.Set("per_page", strconv.Itoa(request.PerPage))
	if request.Orientation != stockmedia.OrientationAny {
		values.Set("orientation", string(request.Orientation))
	}

	var body searchResponse
	if err := a.getJSON(ctx, path+"?"+values.Encode(), &body); err != nil {
		return stockmedia.SearchPage{}, err
	}
	return body.normalize(request.Kind)
}

type searchResponse struct {
	Page         int     `json:"page"`
	PerPage      int     `json:"per_page"`
	TotalResults int     `json:"total_results"`
	Photos       []photo `json:"photos"`
	Videos       []video `json:"videos"`
}

type photo struct {
	ID              int64  `json:"id"`
	URL             string `json:"url"`
	Photographer    string `json:"photographer"`
	PhotographerURL string `json:"photographer_url"`
	Src             struct {
		Medium   string `json:"medium"`
		Original string `json:"original"`
	} `json:"src"`
}

type video struct {
	ID   int64  `json:"id"`
	URL  string `json:"url"`
	User struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"user"`
	Image      string      `json:"image"`
	VideoFiles []videoFile `json:"video_files"`
}

type videoFile struct {
	ID       int64  `json:"id"`
	Quality  string `json:"quality"`
	FileType string `json:"file_type"`
	Link     string `json:"link"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

func (r searchResponse) normalize(kind stockmedia.MediaKind) (stockmedia.SearchPage, error) {
	page := stockmedia.SearchPage{Page: r.Page, PerPage: r.PerPage}
	if r.Page > 0 && r.PerPage > 0 {
		page.HasNextPage = r.Page*r.PerPage < r.TotalResults
	}

	switch kind {
	case stockmedia.MediaKindImage:
		page.Results = make([]stockmedia.SearchResult, 0, len(r.Photos))
		for _, item := range r.Photos {
			result := stockmedia.SearchResult{
				ProviderKey:      ProviderKey,
				ProviderResultID: strconv.FormatInt(item.ID, 10),
				Kind:             stockmedia.MediaKindImage,
				PreviewURL:       strings.TrimSpace(item.Src.Medium),
				SourcePageURL:    strings.TrimSpace(item.URL),
				CreatorName:      strings.TrimSpace(item.Photographer),
				CreatorURL:       strings.TrimSpace(item.PhotographerURL),
				LicenseSummary:   licenseSummary,
				LicenseReference: licenseReference,
				AttributionText:  attribution(item.Photographer),
				Acquirable:       strings.TrimSpace(item.Src.Original) != "",
			}
			if err := result.Validate(); err != nil {
				return stockmedia.SearchPage{}, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: fmt.Errorf("malformed image result %d: %w", item.ID, err)}
			}
			page.Results = append(page.Results, result)
		}
	case stockmedia.MediaKindVideo:
		page.Results = make([]stockmedia.SearchResult, 0, len(r.Videos))
		for _, item := range r.Videos {
			result := stockmedia.SearchResult{
				ProviderKey:      ProviderKey,
				ProviderResultID: strconv.FormatInt(item.ID, 10),
				Kind:             stockmedia.MediaKindVideo,
				PreviewURL:       strings.TrimSpace(item.Image),
				SourcePageURL:    strings.TrimSpace(item.URL),
				CreatorName:      strings.TrimSpace(item.User.Name),
				CreatorURL:       strings.TrimSpace(item.User.URL),
				LicenseSummary:   licenseSummary,
				LicenseReference: licenseReference,
				AttributionText:  attribution(item.User.Name),
				Acquirable:       hasDownloadableVideo(item.VideoFiles),
			}
			if err := result.Validate(); err != nil {
				return stockmedia.SearchPage{}, stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: fmt.Errorf("malformed video result %d: %w", item.ID, err)}
			}
			page.Results = append(page.Results, result)
		}
	default:
		return stockmedia.SearchPage{}, stockmedia.ErrUnsupportedKind
	}
	return page, nil
}

func attribution(creator string) string {
	creator = strings.TrimSpace(creator)
	if creator == "" {
		return "Content provided by Pexels"
	}
	return "Content by " + creator + " on Pexels"
}

func hasDownloadableVideo(files []videoFile) bool {
	for _, file := range files {
		if strings.TrimSpace(file.Link) != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(file.FileType)), "video/") {
			return true
		}
	}
	return false
}

func (a *Adapter) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+path, nil)
	if err != nil {
		return stockmedia.ProviderError{Kind: stockmedia.ProviderErrorInvalid, Provider: ProviderKey, Err: err}
	}
	req.Header.Set("Authorization", a.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		kind := stockmedia.ProviderErrorTransient
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			kind = stockmedia.ProviderErrorRateLimited
		case http.StatusUnauthorized, http.StatusForbidden:
			kind = stockmedia.ProviderErrorUnauthorized
		case http.StatusNotFound:
			kind = stockmedia.ProviderErrorRemoved
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			kind = stockmedia.ProviderErrorInvalid
		}
		return stockmedia.ProviderError{Kind: kind, Provider: ProviderKey, RetryAfter: resp.Header.Get("Retry-After"), Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}

	limited := io.LimitReader(resp.Body, a.maxResponseBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: err}
	}
	if int64(len(payload)) > a.maxResponseBytes {
		return stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: errors.New("response exceeds configured limit")}
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return stockmedia.ProviderError{Kind: stockmedia.ProviderErrorTransient, Provider: ProviderKey, Err: err}
	}
	return nil
}
