package pexels

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/stockmedia"
)

func TestOpenForAcquisitionResolvesFreshProviderURL(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/photos/123":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"id":123,"src":{"original":%q}}`, server.URL+"/content/photo.jpg")
		case "/content/photo.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			fmt.Fprint(w, "image-bytes")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter, err := New(Config{BaseURL: server.URL, APIKey: "secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	remote, err := adapter.OpenForAcquisition(context.Background(), "123", stockmedia.MediaKindImage)
	if err != nil {
		t.Fatal(err)
	}
	if remote.ContentType() != "image/jpeg" {
		t.Fatalf("content type = %q", remote.ContentType())
	}
	reader, err := remote.Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "image-bytes" {
		t.Fatalf("payload = %q", payload)
	}
}

func TestOpenForAcquisitionDoesNotSubstituteRemovedItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()
	adapter, err := New(Config{BaseURL: server.URL, APIKey: "secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.OpenForAcquisition(context.Background(), "404", stockmedia.MediaKindImage)
	providerErr, ok := err.(stockmedia.ProviderError)
	if !ok || providerErr.Kind != stockmedia.ProviderErrorRemoved {
		t.Fatalf("error = %#v", err)
	}
}
