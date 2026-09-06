package stockmedia

import "testing"

func TestSearchRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		req  SearchRequest
		want error
	}{
		{name: "valid image search", req: SearchRequest{Query: "rainy tokyo street", Kind: MediaKindImage, Orientation: OrientationLandscape, Page: 1, PerPage: 20}},
		{name: "valid video search", req: SearchRequest{Query: "ocean waves", Kind: MediaKindVideo, Page: 2, PerPage: 10}},
		{name: "empty query", req: SearchRequest{Kind: MediaKindImage, Page: 1, PerPage: 20}, want: ErrInvalidQuery},
		{name: "unsupported kind", req: SearchRequest{Query: "city", Kind: MediaKind("audio"), Page: 1, PerPage: 20}, want: ErrUnsupportedKind},
		{name: "unsupported orientation", req: SearchRequest{Query: "city", Kind: MediaKindImage, Orientation: Orientation("squareish"), Page: 1, PerPage: 20}, want: ErrUnsupportedOrientation},
		{name: "page zero", req: SearchRequest{Query: "city", Kind: MediaKindImage, Page: 0, PerPage: 20}, want: ErrInvalidPage},
		{name: "per page too large", req: SearchRequest{Query: "city", Kind: MediaKindImage, Page: 1, PerPage: MaxPerPage + 1}, want: ErrInvalidPerPage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.Validate(); got != tt.want {
				t.Fatalf("Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSearchResultRequiresTruthfulProvenance(t *testing.T) {
	valid := SearchResult{
		ProviderKey:      "pexels",
		ProviderResultID: "123",
		Kind:             MediaKindImage,
		PreviewURL:       "https://images.example/preview.jpg",
		SourcePageURL:    "https://example.test/photo/123",
		CreatorName:      "Creator",
		LicenseSummary:   "Pexels license",
		AttributionText:  "Photo by Creator on Pexels",
		Acquirable:       true,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}

	invalid := valid
	invalid.ProviderResultID = ""
	if err := invalid.Validate(); err != ErrInvalidResult {
		t.Fatalf("missing provider result id = %v, want %v", err, ErrInvalidResult)
	}
}

func TestProviderErrorRecoveryClassification(t *testing.T) {
	if !(ProviderError{Kind: ProviderErrorRateLimited}).Recoverable() {
		t.Fatal("rate-limited provider error must be recoverable")
	}
	if (ProviderError{Kind: ProviderErrorRemoved}).Recoverable() {
		t.Fatal("removed source item must not be classified as retryable")
	}
}
