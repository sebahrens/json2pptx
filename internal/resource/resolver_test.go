package resource

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// testResolver creates a Resolver that bypasses SSRF checks for local test servers.
func testResolver(t *testing.T, opts ResolverOptions) *Resolver {
	t.Helper()
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{}
	}
	r, err := NewResolver(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com/image.png", true},
		{"http://example.com/image.png", true},
		{"/local/path/image.png", false},
		{"relative/path.png", false},
		{"ftp://example.com/file.png", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsURL(tt.input); got != tt.want {
			t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestResolveImage_PNG(t *testing.T) {
	pngData := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 100)...)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngData)
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{})

	path, err := resolver.ResolveImage(srv.URL + "/test.png")
	if err != nil {
		t.Fatalf("ResolveImage: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	// Second call should return cached path
	path2, err := resolver.ResolveImage(srv.URL + "/test.png")
	if err != nil {
		t.Fatalf("cached ResolveImage: %v", err)
	}
	if path2 != path {
		t.Errorf("expected cached path %q, got %q", path, path2)
	}
}

func TestResolveImage_JPEG(t *testing.T) {
	jpegData := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 100)...)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(jpegData)
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{})

	path, err := resolver.ResolveImage(srv.URL + "/photo.jpg")
	if err != nil {
		t.Fatalf("ResolveImage: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestResolveImage_RejectsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>Not an image</body></html>"))
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{})

	_, err := resolver.ResolveImage(srv.URL + "/page.html")
	if err == nil {
		t.Fatal("expected error for HTML content")
	}
}

func TestResolveSVG(t *testing.T) {
	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/></svg>`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(svgData)
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{})

	path, err := resolver.ResolveSVG(srv.URL + "/icon.svg")
	if err != nil {
		t.Fatalf("ResolveSVG: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestResolveSVG_XMLDeclaration(t *testing.T) {
	svgData := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(svgData)
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{})

	_, err := resolver.ResolveSVG(srv.URL + "/icon.svg")
	if err != nil {
		t.Fatalf("ResolveSVG with XML declaration: %v", err)
	}
}

func TestResolveSVG_RejectsNonSVG(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is just plain text"))
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{})

	_, err := resolver.ResolveSVG(srv.URL + "/not-svg.txt")
	if err == nil {
		t.Fatal("expected error for non-SVG content")
	}
}

func TestResolveImage_SizeLimit(t *testing.T) {
	bigData := make([]byte, 1024)
	copy(bigData, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bigData)
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{MaxSize: 512})

	_, err := resolver.ResolveImage(srv.URL + "/big.png")
	if err == nil {
		t.Fatal("expected error for oversized content")
	}
}

func TestResolveImage_InvalidScheme(t *testing.T) {
	resolver := testResolver(t, ResolverOptions{})

	_, err := resolver.ResolveImage("ftp://example.com/file.png")
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestResolveImage_DomainAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{
		AllowedDomains: []string{"allowed.example.com"},
	})

	// Server URL uses 127.0.0.1 which is not in the allowlist
	_, err := resolver.ResolveImage(srv.URL + "/test.png")
	if err == nil {
		t.Fatal("expected error for non-allowed domain")
	}
}

func TestResolveImage_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	resolver := testResolver(t, ResolverOptions{})

	_, err := resolver.ResolveImage(srv.URL + "/missing.png")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestIsImageContent(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, true},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0}, true},
		{"gif", []byte("GIF89a"), true},
		{"bmp", []byte("BM\x00\x00"), true},
		{"webp", append([]byte("RIFF\x00\x00\x00\x00WEBP"), make([]byte, 10)...), true},
		{"html", []byte("<html>"), false},
		{"empty", []byte{}, false},
		{"short", []byte{0x89}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isImageContent(tt.data); got != tt.valid {
				t.Errorf("isImageContent(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestHashFilename(t *testing.T) {
	f1 := hashFilename("https://example.com/a.png", ".png")
	f2 := hashFilename("https://example.com/b.png", ".png")
	if f1 == f2 {
		t.Error("different URLs should produce different filenames")
	}
	f3 := hashFilename("https://example.com/a.png", ".png")
	if f1 != f3 {
		t.Error("same URL should produce same filename")
	}
}
