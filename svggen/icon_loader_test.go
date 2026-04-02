package svggen

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifyIcon(t *testing.T) {
	tests := []struct {
		input string
		want  IconKind
	}{
		{"", IconKindEmpty},
		{"  ", IconKindEmpty},
		{"https://example.com/icon.svg", IconKindURL},
		{"http://example.com/icon.png", IconKindURL},
		{"data:image/svg+xml;base64,PHN2Zz4=", IconKindDataURI},
		{"data:image/png;base64,iVBOR=", IconKindDataURI},
		{`<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>`, IconKindInlineSVG},
		{"just-a-string", IconKindEmpty},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got := ClassifyIcon(tt.input)
			if got != tt.want {
				t.Errorf("ClassifyIcon(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeDataURI(t *testing.T) {
	// Create a simple SVG and encode it
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>`
	encoded := base64.StdEncoding.EncodeToString([]byte(svgContent))
	uri := "data:image/svg+xml;base64," + encoded

	data, err := decodeDataURI(uri)
	if err != nil {
		t.Fatalf("decodeDataURI() error = %v", err)
	}
	if string(data) != svgContent {
		t.Errorf("decodeDataURI() = %q, want %q", string(data), svgContent)
	}
}

func TestDecodeDataURI_Invalid(t *testing.T) {
	_, err := decodeDataURI("data:no-comma-here")
	if err == nil {
		t.Error("Expected error for invalid data URI (no comma)")
	}
}

func TestLoadIcon_Empty(t *testing.T) {
	img := LoadIcon("", 64)
	if img != nil {
		t.Error("Expected nil for empty icon string")
	}
}

func TestLoadIcon_InlineSVG(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40" fill="red"/></svg>`
	img := LoadIcon(svg, 64)
	if img == nil {
		t.Fatal("Expected non-nil image for inline SVG")
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Error("Expected non-zero image dimensions")
	}
}

func TestLoadIcon_DataURI_SVG(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect width="100" height="100" fill="blue"/></svg>`
	encoded := base64.StdEncoding.EncodeToString([]byte(svg))
	uri := "data:image/svg+xml;base64," + encoded

	img := LoadIcon(uri, 64)
	if img == nil {
		t.Fatal("Expected non-nil image for data URI SVG")
	}
}

func TestLoadIcon_URL(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40" fill="green"/></svg>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, svg)
	}))
	defer server.Close()

	img := LoadIcon(server.URL+"/icon.svg", 64)
	if img == nil {
		t.Fatal("Expected non-nil image for URL icon")
	}
}

func TestLoadIcon_URL_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	img := LoadIcon(server.URL+"/missing.svg", 64)
	if img != nil {
		t.Error("Expected nil for 404 URL")
	}
}

func TestLoadIcon_InvalidData(t *testing.T) {
	// Garbage data that isn't SVG or raster
	encoded := base64.StdEncoding.EncodeToString([]byte("not an image"))
	uri := "data:image/png;base64," + encoded

	img := LoadIcon(uri, 64)
	if img != nil {
		t.Error("Expected nil for invalid image data")
	}
}

func TestFetchURL_SizeLimit(t *testing.T) {
	// Serve data that exceeds the limit
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		data := make([]byte, iconMaxBytes+100)
		w.Write(data)
	}))
	defer server.Close()

	_, err := fetchURL(server.URL)
	if err == nil {
		t.Error("Expected error for oversized response")
	}
}

