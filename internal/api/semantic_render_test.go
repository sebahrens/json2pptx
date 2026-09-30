package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// stubRenderer records the request it was handed and writes a fake deck to the
// requested destination, returning a CLI-shaped result object.
type stubRenderer struct {
	got       SemanticRenderRequest
	assets    map[string]string
	template  []byte
	ok        bool
	misplaced bool
}

func (s *stubRenderer) render(_ context.Context, req SemanticRenderRequest) (SemanticRenderOutcome, error) {
	s.got = req
	s.assets = map[string]string{}
	entries, _ := os.ReadDir(req.AssetDir)
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(req.AssetDir, e.Name()))
		s.assets[e.Name()] = string(b)
	}
	if req.TemplatePath != "" {
		s.template, _ = os.ReadFile(req.TemplatePath)
	}
	if !s.ok {
		return SemanticRenderOutcome{OK: false, Result: json.RawMessage(
			`{"ok":false,"error":"refused","diagnostics":[{"code":"TEXT_BELOW_READABLE_MIN","semantic_path":"slides[1]"}]}`)}, nil
	}
	out := filepath.Join(req.OutputDir, req.OutputFilename)
	if s.misplaced {
		out = filepath.Join(req.WorkDir, "elsewhere.pptx")
	}
	if err := os.WriteFile(out, []byte("PK fake"), 0o600); err != nil {
		return SemanticRenderOutcome{}, err
	}
	res, _ := json.Marshal(map[string]any{"ok": true, "output_path": out, "manifest_path": out + ".authoring.json", "slide_count": 2})
	return SemanticRenderOutcome{OK: true, OutputPath: out, Result: res}, nil
}

func TestSemanticRenderHandler_RawBodyPassesOptionsAndReturnsDownload(t *testing.T) {
	outDir := t.TempDir()
	stub := &stubRenderer{ok: true}
	h := SemanticRenderHandler(stub.render, outDir, 0, nil)
	w := postSemantic(t, h, "/api/v1/semantic/render?strict=strict&template=warm-coral&output_validation=warn",
		"application/json", validDeckSpecJSON)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body)
	}
	if stub.got.Strict != semantic.StrictnessStrict || stub.got.Template != "warm-coral" ||
		stub.got.OutputValidation != "warn" || stub.got.Filename != "spec.json" || string(stub.got.Spec) != validDeckSpecJSON {
		t.Errorf("renderer request = %+v", stub.got)
	}
	if stub.got.AssetDir == "" || stub.got.TemplatePath != "" {
		t.Errorf("asset dir %q / template path %q", stub.got.AssetDir, stub.got.TemplatePath)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != true || resp["slide_count"] != float64(2) {
		t.Errorf("response = %v", resp)
	}
	if _, leaked := resp["output_path"]; leaked {
		t.Error("server-local output_path leaked into the response")
	}
	if _, leaked := resp["manifest_path"]; leaked {
		t.Error("server-local manifest_path leaked into the response")
	}
	want := "/api/v1/download/" + stub.got.OutputFilename
	if resp["file_url"] != want || resp["expires_at"] == nil {
		t.Errorf("file_url = %v (want %s), expires_at = %v", resp["file_url"], want, resp["expires_at"])
	}
	if !validDownloadFilename.MatchString(stub.got.OutputFilename) {
		t.Errorf("output filename %q is not servable by the download endpoint", stub.got.OutputFilename)
	}
	// The per-request work dir is removed afterwards.
	if _, err := os.Stat(stub.got.WorkDir); !os.IsNotExist(err) {
		t.Errorf("work dir %s not cleaned up (%v)", stub.got.WorkDir, err)
	}
}

func TestSemanticRenderHandler_RefusalIs422WithDiagnostics(t *testing.T) {
	stub := &stubRenderer{ok: false}
	w := postSemantic(t, SemanticRenderHandler(stub.render, t.TempDir(), 0, nil),
		"/api/v1/semantic/render", "application/x-yaml", "meta: {title: x}\nslides: []\n")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != false || resp["file_url"] != nil {
		t.Errorf("response = %v", resp)
	}
	diags, _ := resp["diagnostics"].([]any)
	if len(diags) != 1 || diags[0].(map[string]any)["semantic_path"] != "slides[1]" {
		t.Errorf("diagnostics = %v", resp["diagnostics"])
	}
	if stub.got.Filename != "spec.yaml" {
		t.Errorf("yaml body parsed as %q", stub.got.Filename)
	}
}

func TestSemanticRenderHandler_MisplacedOutputIs500(t *testing.T) {
	stub := &stubRenderer{ok: true, misplaced: true}
	w := postSemantic(t, SemanticRenderHandler(stub.render, t.TempDir(), 0, nil),
		"/api/v1/semantic/render", "application/json", validDeckSpecJSON)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func multipartRender(t *testing.T, parts func(mw *multipart.Writer)) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	parts(mw)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/semantic/render", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func writePart(t *testing.T, mw *multipart.Writer, field, filename, content string) {
	t.Helper()
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
}

func TestSemanticRenderHandler_MultipartTemplateAndAssets(t *testing.T) {
	stub := &stubRenderer{ok: true}
	req := multipartRender(t, func(mw *multipart.Writer) {
		writePart(t, mw, "spec", "deck.json", validDeckSpecJSON)
		writePart(t, mw, "template", "brand.pptx", "PK template bytes")
		writePart(t, mw, "assets", "logo.png", "png-bytes")
		writePart(t, mw, "assets", "../../etc/chart.svg", "<svg/>")
	})
	w := httptest.NewRecorder()
	SemanticRenderHandler(stub.render, t.TempDir(), 0, nil)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", w.Code, w.Body)
	}
	if stub.got.Filename != "spec.json" || string(stub.got.Spec) != validDeckSpecJSON {
		t.Errorf("spec part = %q (%s)", stub.got.Filename, stub.got.Spec)
	}
	if string(stub.template) != "PK template bytes" || stub.got.TemplateFilename != "brand.pptx" {
		t.Errorf("template = %q (%s)", stub.template, stub.got.TemplateFilename)
	}
	// Asset names are reduced to their base name, so an upload cannot escape
	// the per-request asset directory.
	if stub.assets["logo.png"] != "png-bytes" || stub.assets["chart.svg"] != "<svg/>" || len(stub.assets) != 2 {
		t.Errorf("assets = %v", stub.assets)
	}
}

func TestSemanticRenderHandler_MultipartRejections(t *testing.T) {
	cases := map[string]func(t *testing.T, mw *multipart.Writer){
		"missing spec": func(t *testing.T, mw *multipart.Writer) {
			writePart(t, mw, "assets", "logo.png", "x")
		},
		"template not pptx": func(t *testing.T, mw *multipart.Writer) {
			writePart(t, mw, "spec", "deck.yaml", "meta: {title: x}")
			writePart(t, mw, "template", "brand.key", "x")
		},
		"duplicate asset": func(t *testing.T, mw *multipart.Writer) {
			writePart(t, mw, "spec", "deck.yaml", "meta: {title: x}")
			writePart(t, mw, "assets", "a/logo.png", "x")
			writePart(t, mw, "assets", "b/logo.png", "y")
		},
		"hidden asset": func(t *testing.T, mw *multipart.Writer) {
			writePart(t, mw, "spec", "deck.yaml", "meta: {title: x}")
			writePart(t, mw, "assets", ".env", "x")
		},
	}
	for name, parts := range cases {
		t.Run(name, func(t *testing.T) {
			stub := &stubRenderer{ok: true}
			req := multipartRender(t, func(mw *multipart.Writer) { parts(t, mw) })
			w := httptest.NewRecorder()
			SemanticRenderHandler(stub.render, t.TempDir(), 0, nil)(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body)
			}
			if stub.got.Spec != nil {
				t.Error("renderer ran on a rejected request")
			}
		})
	}
}

func TestSemanticRenderHandler_RequestErrors(t *testing.T) {
	stub := &stubRenderer{ok: true}
	h := SemanticRenderHandler(stub.render, t.TempDir(), 0, nil)
	if w := postSemantic(t, h, "/api/v1/semantic/render?output_validation=loose", "application/json", validDeckSpecJSON); w.Code != http.StatusBadRequest {
		t.Errorf("bad output_validation: status %d", w.Code)
	}
	if w := postSemantic(t, h, "/api/v1/semantic/render?strict=nope", "application/json", validDeckSpecJSON); w.Code != http.StatusBadRequest {
		t.Errorf("bad strict: status %d", w.Code)
	}
	if w := postSemantic(t, h, "/api/v1/semantic/render", "text/csv", validDeckSpecJSON); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("csv: status %d", w.Code)
	}
	if w := postSemantic(t, h, "/api/v1/semantic/render", "application/json", ""); w.Code != http.StatusBadRequest {
		t.Errorf("empty: status %d", w.Code)
	}
}

// Capabilities advertise render as supported, not deferred
// (go-slide-creator-b7qqg.25).
func TestCapabilities_SemanticRenderSupported(t *testing.T) {
	resp := getCapabilities(t)
	found := false
	for _, op := range resp.SemanticCapabilities.SupportedOperations {
		found = found || op == "render"
	}
	if !found {
		t.Errorf("render missing from supported_operations: %v", resp.SemanticCapabilities.SupportedOperations)
	}
	if len(resp.SemanticCapabilities.DeferredOperations) != 0 {
		t.Errorf("deferred_operations = %v, want none", resp.SemanticCapabilities.DeferredOperations)
	}
}

func TestSemanticRenderHandler_ConcurrencyBound(t *testing.T) {
	sem := make(chan struct{}, 1)
	sem <- struct{}{} // the only slot is taken
	stub := &stubRenderer{ok: true}
	w := postSemantic(t, SemanticRenderHandler(stub.render, t.TempDir(), 0, sem),
		"/api/v1/semantic/render", "application/json", validDeckSpecJSON)
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
		t.Fatalf("status = %d, Retry-After=%q", w.Code, w.Header().Get("Retry-After"))
	}
}
