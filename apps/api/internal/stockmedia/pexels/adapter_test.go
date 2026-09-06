package pexels

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia"
)

func TestSearchImagesNormalizesTruthfulMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.URL.Path != "/v1/search" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "rainy city" {
			t.Fatalf("query = %q", got)
		}
		if got := r.URL.Query().Get("orientation"); got != "landscape" {
			t.Fatalf("orientation = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"page":1,"per_page":20,"total_results":21,"photos":[{"id":123,"url":"https://www.pexels.com/photo/123/","photographer":"Ada","photographer_url":"https://www.pexels.com/@ada","src":{"medium":"https://images.pexels.com/photos/123/medium.jpeg","original":"https://images.pexels.com/photos/123/original.jpeg"}}]}`)
	}))
	defer server.Close()

	adapter, err := New(Config{BaseURL: server.URL, APIKey: "secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	page, err := adapter.Search(context.Background(), stockmedia.SearchRequest{Query: "rainy city", Kind: stockmedia.MediaKindImage, Orientation: stockmedia.OrientationLandscape, Page: 1, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("results = %d", len(page.Results))
	}
	result := page.Results[0]
	if result.ProviderKey != ProviderKey || result.ProviderResultID != "123" || result.CreatorName != "Ada" {
		t.Fatalf("unexpected normalized result: %#v", result)
	}
	if result.SourcePageURL == "" || result.LicenseSummary == "" || result.AttributionText == "" || !result.Acquirable {
		t.Fatalf("missing provenance/acquisition truth: %#v", result)
	}
	if !page.HasNextPage {
		t.Fatal("expected bounded next page")
	}
}

func TestSearchClassifiesRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	adapter, err := New(Config{BaseURL: server.URL, APIKey: "secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Search(context.Background(), stockmedia.SearchRequest{Query: "city", Kind: stockmedia.MediaKindImage, Page: 1, PerPage: 20})
	providerErr, ok := err.(stockmedia.ProviderError)
	if !ok {
		t.Fatalf("error = %T %v", err, err)
	}
	if providerErr.Kind != stockmedia.ProviderErrorRateLimited || providerErr.RetryAfter != "30" {
		t.Fatalf("provider error = %#v", providerErr)
	}
}

func TestSearchRejectsUnsupportedOrientationForVideo(t *testing.T) {
	adapter, err := New(Config{BaseURL: "https://api.pexels.com", APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Search(context.Background(), stockmedia.SearchRequest{Query: "ocean", Kind: stockmedia.MediaKindVideo, Orientation: stockmedia.OrientationSquare, Page: 1, PerPage: 20})
	if err != stockmedia.ErrUnsupportedOrientation {
		t.Fatalf("error = %v, want %v", err, stockmedia.ErrUnsupportedOrientation)
	}
}
