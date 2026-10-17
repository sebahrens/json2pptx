package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/template"
)

func TestHTTPAuditValidationMatchesRenderRefusal(t *testing.T) {
	cfg := semanticRenderTestConfig(t)
	raw, err := os.ReadFile(filepath.Join("testdata", "native_readability", "bmc_dense.json"))
	if err != nil {
		t.Fatal(err)
	}
	var deck struct{ Slides []json.RawMessage }
	if err := json.Unmarshal(raw, &deck); err != nil {
		t.Fatal(err)
	}
	spec := fmt.Sprintf(`{"meta":{"title":"Canvas","template":"modern"},"slides":[{"kind":"title","title":"Canvas"},{"kind":"raw_json2pptx","slide":%s}]}`, deck.Slides[1])
	out, err := newHTTPSemanticValidator(cfg)(context.Background(), []byte(spec), "spec.json", semantic.StrictnessWarn, "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Valid || out.ValidationScope != "render" {
		t.Fatalf("unreadable canvas passed validation: %+v", out)
	}
	found := false
	for _, f := range out.Findings.Findings {
		if strings.Contains(f.Code, patterns.ErrCodeTextBelowReadableMin) && f.Path != nil && strings.HasPrefix(*f.Path, "/slides/1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no source-addressed render refusal: %+v", out.Findings)
	}
	entries, _ := os.ReadDir(cfg.Storage.OutputDir)
	if len(entries) != 0 {
		t.Error("trial validation published an artifact")
	}
	out, err = newHTTPSemanticValidator(cfg)(context.Background(), []byte(httpNoTemplateSpec), "spec.json", semantic.StrictnessWarn, "missing-audit-template")
	if err != nil || out.Valid {
		t.Fatalf("template query ignored: %+v %v", out, err)
	}
}

func TestHTTPAuditKindsAndThumbnails(t *testing.T) {
	cfg := semanticRenderTestConfig(t)
	server := api.NewServer(api.ServerConfig{TemplatesDir: cfg.Templates.Dir, OutputDir: cfg.Storage.OutputDir, Cache: template.NewMemoryCache(time.Hour), SemanticKinds: newHTTPSemanticKinds(cfg, template.NewMemoryCache(time.Hour))})
	w := httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/semantic/kinds?kinds=title&fields=budgets&template=modern", nil))
	var catalogue semanticKindsResult
	if err := json.Unmarshal(w.Body.Bytes(), &catalogue); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(catalogue.SlideKinds) != 1 || catalogue.SlideKinds[0].Kind != "title" || catalogue.SlideKinds[0].Budgets == nil {
		t.Fatalf("catalogue drift: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/semantic/kinds?fields=unknown", nil))
	if w.Code != 400 {
		t.Errorf("invalid fields accepted: %d", w.Code)
	}
	if err := render.CheckDependencies(); err != nil {
		t.Skipf("catalogue verified; thumbnail render tools unavailable: %v", err)
	}
	name := strings.Repeat("a", 32) + ".pptx"
	source := filepath.Join(testTemplatesDir, "modern.pptx")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Storage.OutputDir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/thumbnails/"+name+"?slide_indices=0&density=36", nil))
	var thumbs render.DeckResult
	if err := json.Unmarshal(w.Body.Bytes(), &thumbs); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(thumbs.Slides) != 1 {
		t.Fatalf("thumbnails: %d %s", w.Code, w.Body)
	}
	image := thumbs.Slides[0]
	png, err := base64.StdEncoding.DecodeString(image.PNG64)
	if err != nil || len(png) < 8 || string(png[:8]) != "\x89PNG\r\n\x1a\n" || image.Path != "" || image.ContentHash == "" || image.SourceHash == "" {
		t.Errorf("thumbnail is not a portable verifiable PNG: %+v %v", image, err)
	}
}

func TestHTTPAuditBindAndShutdown(t *testing.T) {
	for _, tc := range []struct{ bind, want string }{{"", "127.0.0.1:8080"}, {"0.0.0.0", "0.0.0.0:8080"}, {"::1", "[::1]:8080"}} {
		if got := newAPIHTTPServer(config.ServerConfig{Port: 8080, Bind: tc.bind}, http.NotFoundHandler()).Addr; got != tc.want {
			t.Errorf("bind %q became %s", tc.bind, got)
		}
	}
	var servers []*http.Server
	var addresses []string
	for range 2 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
		servers = append(servers, server)
		addresses = append(addresses, listener.Addr().String())
		go func() { _ = server.Serve(listener) }()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := shutdownHTTPServers(ctx, servers...); err != nil {
		t.Fatal(err)
	}
	for _, addr := range addresses {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			t.Errorf("listener survived shutdown: %s", addr)
		}
	}
	if err := shutdownHTTPServers(ctx, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatal(err)
	}
}
