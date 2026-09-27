package resource

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestResolveImage_PolyglotStoredByMagic verifies a GIF/SVG polyglot fetched
// from a ".svg" URL is cached under its magic-byte extension, not the URL's,
// so it is never routed down the SVG embed path (go-slide-creator-csclk.77).
func TestResolveImage_PolyglotStoredByMagic(t *testing.T) {
	payload := []byte(`GIF89a<svg xmlns="http://www.w3.org/2000/svg"><image href="https://tracker.example/p.png"/></svg>`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	path, err := testResolver(t, ResolverOptions{}).ResolveImage(srv.URL + "/photo.svg")
	if err != nil {
		t.Fatalf("ResolveImage: %v", err)
	}
	if got := filepath.Ext(path); got != ".gif" {
		t.Fatalf("polyglot stored as %q, want .gif (path %s)", got, path)
	}
}
