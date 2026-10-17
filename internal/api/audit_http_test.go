package api

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/template"
)

func auditServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(ServerConfig{Cache: template.NewMemoryCache(0), TemplatesDir: t.TempDir(), OutputDir: t.TempDir(), APIToken: "test-secret", CORSAllowedOrigins: []string{"https://agent.example"}, Version: "build-7", SchemaVersion: "schema-8", SchemaFingerprint: "hash-9"})
}

func TestHTTPAuditAccessAndIdentity(t *testing.T) {
	s := auditServer(t)
	for _, tc := range []struct {
		path, token string
		status      int
	}{{"/api/v1/health", "", 200}, {"/api/v1/capabilities", "", 401}, {"/api/v1/capabilities", "Bearer wrong", 401}, {"/api/v1/capabilities", "Bearer test-secret", 200}, {"/api/v1/ready", "", 401}, {"/api/v1/download/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.pptx", "", 401}} {
		r := httptest.NewRequest("GET", tc.path, nil)
		r.Header.Set("Authorization", tc.token)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%s %q: %d %s", tc.path, tc.token, w.Code, w.Body)
		}
		if tc.status == 401 && (!strings.Contains(w.Body.String(), "UNAUTHORIZED") || w.Header().Get("WWW-Authenticate") != "Bearer") {
			t.Error("missing bearer challenge")
		}
	}
	for _, id := range []string{"trace-ABC_1.2", strings.Repeat("x", 65), "line\nbreak", "", "unicode-é"} {
		r := httptest.NewRequest("GET", "/api/v1/health", nil)
		r.Header.Set("X-Request-ID", id)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		got := w.Header().Get("X-Request-ID")
		if !requestIDPattern.MatchString(got) || (id == "trace-ABC_1.2" && got != id) {
			t.Errorf("request ID %q became %q", id, got)
		}
	}
	for _, path := range []string{"/api/v1/health", "/api/v1/capabilities"} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer test-secret")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["version"] != "build-7" || body["schema_version"] != "schema-8" || body["schema_fingerprint"] != "hash-9" {
			t.Errorf("metadata missing: %s", w.Body)
		}
		if path == "/api/v1/capabilities" {
			for _, entry := range body["mcp_only_features"].([]any) {
				if strings.HasPrefix(entry.(string), "list_patterns ") || strings.HasPrefix(entry.(string), "show_pattern ") || strings.HasPrefix(entry.(string), "expand_pattern ") {
					t.Errorf("HTTP pattern route advertised as MCP-only: %s", entry)
				}
			}
		}
	}
}

func TestHTTPAuditCORS(t *testing.T) {
	for _, tc := range []struct {
		origin, method, headers string
		status                  int
	}{{"https://agent.example", "POST", "Authorization, Content-Type, X-Request-ID", 204}, {"https://evil.example", "POST", "", 403}, {"https://agent.example", "DELETE", "", 403}, {"https://agent.example", "POST", "X-Unknown", 403}} {
		s := auditServer(t)
		r := httptest.NewRequest("OPTIONS", "/api/v1/semantic/render", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Access-Control-Request-Method", tc.method)
		r.Header.Set("Access-Control-Request-Headers", tc.headers)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("preflight %+v: %d %s", tc, w.Code, w.Body)
		}
		if tc.status == 204 && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
			t.Error("allowed origin missing")
		}
	}
	s := auditServer(t)
	r := httptest.NewRequest("POST", "/api/v1/semantic/render", nil)
	r.Header.Set("Origin", "https://agent.example")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 401 || w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Errorf("CORS must retain actual-request authentication: %d", w.Code)
	}
	s.corsAllowedOrigins = nil
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS must be disabled by default")
	}
}

func TestHTTPAuditReadiness(t *testing.T) {
	toolsDir := t.TempDir()
	t.Setenv("PATH", toolsDir)
	for _, name := range []string{"soffice", "magick"} {
		if err := os.WriteFile(filepath.Join(toolsDir, name), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	templates, output := t.TempDir(), t.TempDir()
	call := func() int {
		w := httptest.NewRecorder()
		ReadyHandler(templates, output)(w, httptest.NewRequest("GET", "/api/v1/ready", nil))
		return w.Code
	}
	if call() != 503 {
		t.Fatal("empty template directory must not be ready")
	}
	if err := os.WriteFile(filepath.Join(templates, "brand.pptx"), []byte("probe"), 0600); err != nil {
		t.Fatal(err)
	}
	if call() != 200 {
		t.Fatal("available dependencies must be ready")
	}
	entries, _ := os.ReadDir(output)
	if len(entries) != 0 {
		t.Error("readiness probe leaked")
	}
	if err := os.Remove(filepath.Join(toolsDir, "magick")); err != nil {
		t.Fatal(err)
	}
	if call() != 503 {
		t.Error("missing toolchain must not be ready")
	}
	output = filepath.Join(output, "missing")
	if call() != 503 {
		t.Error("missing output directory must not be ready")
	}
}

func TestHTTPAuditTrialValidationTransport(t *testing.T) {
	sem := make(chan struct{}, 1)
	calls := 0
	validator := func(ctx context.Context, data []byte, filename string, strict semantic.Strictness, templateName string) (SemanticValidationOutcome, error) {
		calls++
		if templateName != "modern" || strict != semantic.StrictnessStrict || filename != "spec.json" || len(data) == 0 {
			t.Error("validator arguments lost")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("validator has no deadline")
		}
		return SemanticValidationOutcome{ValidationScope: "render", Valid: false, Findings: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{}, []diagnostics.Diagnostic{{Code: "FIT", Severity: diagnostics.SeverityError, Message: "refused"}})}, nil
	}
	h := semanticValidateHandler(validator, sem)
	w := postSemantic(t, h, "/api/v1/semantic/validate?strict=strict&template=modern", "application/json", validDeckSpecJSON)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":false`) || calls != 1 || len(sem) != 0 {
		t.Fatalf("trial response: %d %s", w.Code, w.Body)
	}
	sem <- struct{}{}
	w = postSemantic(t, h, "/api/v1/semantic/validate", "application/json", validDeckSpecJSON)
	if w.Code != 503 || calls != 1 {
		t.Fatal("trial validator must respect shared semaphore")
	}
	<-sem
	w = postSemantic(t, SemanticValidateHandler(), "/api/v1/semantic/validate?template=modern", "application/json", validDeckSpecJSON)
	if w.Code != 400 {
		t.Fatal("schema-only handler must not silently ignore template")
	}
}

func TestHTTPAuditConflictingTemplateSources(t *testing.T) {
	stub := &stubRenderer{ok: true}
	r := multipartRender(t, func(mw *multipart.Writer) {
		writePart(t, mw, "spec", "deck.json", validDeckSpecJSON)
		writePart(t, mw, "template", "brand.pptx", "PK test")
	})
	r.URL.RawQuery = "template=warm-coral"
	w := httptest.NewRecorder()
	SemanticRenderHandler(stub.render, t.TempDir(), 0, nil)(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_REQUEST") || stub.got.WorkDir != "" {
		t.Errorf("conflicting templates reached renderer: %d %s", w.Code, w.Body)
	}
}

func TestHTTPAuditThumbnailsBoundary(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("a", 32) + ".pptx"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("deck"), 0600); err != nil {
		t.Fatal(err)
	}
	sem := make(chan struct{}, 1)
	sem <- struct{}{}
	for _, tc := range []struct {
		name, query string
		status      int
	}{{"../secret.pptx", "", 400}, {name, "?density=999999", 400}, {name, "?slide_indices=-1", 400}, {name, "?slide_indices=" + strings.Repeat("0,", 50) + "0", 400}, {name, "?max_slides=0", 400}, {name, "", 503}, {strings.Repeat("b", 32) + ".pptx", "", 404}} {
		r := httptest.NewRequest("GET", "/"+tc.query, nil)
		r.SetPathValue("filename", tc.name)
		w := httptest.NewRecorder()
		ThumbnailsHandler(dir, sem)(w, r)
		if w.Code != tc.status {
			t.Errorf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
}

func TestHTTPAuditTemplateGeometryAndTheme(t *testing.T) {
	dir := filepath.Join("..", "..", "tests", "quality", "fixtures", "portability", "templates")
	s := NewServer(ServerConfig{TemplatesDir: dir, OutputDir: t.TempDir(), Cache: template.NewMemoryCache(0)})
	for _, tc := range []struct {
		name, aspect  string
		width, height int64
	}{
		{"portability-4x3", "4:3", 9144000, 6858000},
		{"portability-21x9", "21:9", 16002000, 6858000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 2 { // Both cold analysis and cached discovery preserve authored geometry and roles.
				w := httptest.NewRecorder()
				s.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/templates/"+tc.name, nil))
				var details TemplateDetailsResponse
				if err := json.Unmarshal(w.Body.Bytes(), &details); err != nil {
					t.Fatal(err)
				}
				if w.Code != 200 || details.AspectRatio != tc.aspect || details.SlideWidth != tc.width || details.SlideHeight != tc.height || details.Theme.SemanticAccents["positive"] != "accent4" {
					t.Fatalf("template contract lost: %d %+v", w.Code, details)
				}
			}
		})
	}
}
