package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// HTTP integration tests for POST /api/v1/semantic/render driven by the shared
// presentation runner (go-slide-creator-b7qqg.25). Each test stands up the
// real api.Server wired exactly as `json2pptx serve` wires it.

func newSemanticRenderTestServer(t *testing.T, cfg config.Config) *httptest.Server {
	t.Helper()
	srv := api.NewServer(api.ServerConfig{
		TemplatesDir:     cfg.Templates.Dir,
		OutputDir:        cfg.Storage.OutputDir,
		SemanticRenderer: newHTTPSemanticRenderer(cfg),
		SemanticFindings: finishSemanticFindingsForHTTP,
	})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func semanticRenderTestConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Templates.Dir = testTemplatesDir
	cfg.Storage.OutputDir = t.TempDir()
	return cfg
}

type httpRenderResponse struct {
	OK          bool                 `json:"ok"`
	FileURL     string               `json:"file_url"`
	ExpiresAt   string               `json:"expires_at"`
	OutputPath  string               `json:"output_path"`
	Template    string               `json:"template"`
	SlideCount  int                  `json:"slide_count"`
	ContentHash string               `json:"content_hash"`
	Quality     json.RawMessage      `json:"quality"`
	Warnings    []string             `json:"warnings"`
	Diagnostics []semanticDiagnostic `json:"diagnostics"`
	Error       string               `json:"error"`
}

func postRender(t *testing.T, ts *httptest.Server, query, contentType string, body io.Reader) (int, httpRenderResponse) {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/v1/semantic/render"+query, contentType, body)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var out httpRenderResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("response is not JSON (%d): %s", resp.StatusCode, raw)
	}
	if t.Failed() || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnprocessableEntity) {
		t.Logf("response %d: %s", resp.StatusCode, raw)
	}
	return resp.StatusCode, out
}

type renderPart struct{ field, filename, content string }

func postRenderMultipart(t *testing.T, ts *httptest.Server, query string, parts ...renderPart) (int, httpRenderResponse) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		fw, err := mw.CreateFormFile(p.field, p.filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte(p.content))
	}
	_ = mw.Close()
	return postRender(t, ts, query, mw.FormDataContentType(), &buf)
}

// downloadDeck fetches the rendered deck through the advertised file_url and
// returns its zip entries.
func downloadDeck(t *testing.T, ts *httptest.Server, fileURL string) map[string][]byte {
	t.Helper()
	if fileURL == "" {
		t.Fatal("no file_url in a successful render")
	}
	resp, err := http.Get(ts.URL + fileURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download %s: %d %s", fileURL, resp.StatusCode, data)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("downloaded deck is not a zip: %v", err)
	}
	entries := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		entries[f.Name] = b
	}
	return entries
}

func countSlides(entries map[string][]byte) int {
	n := 0
	re := regexp.MustCompile(`^ppt/slides/slide\d+\.xml$`)
	for name := range entries {
		if re.MatchString(name) {
			n++
		}
	}
	return n
}

var themeNameRe = regexp.MustCompile(`<a:theme[^>]*\bname="([^"]*)"`)

func themeName(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	for name, b := range entries {
		if strings.HasPrefix(name, "ppt/theme/theme") {
			if m := themeNameRe.FindSubmatch(b); m != nil {
				return string(m[1])
			}
		}
	}
	t.Fatal("no theme part")
	return ""
}

func templateThemeName(t *testing.T, path string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	entries := map[string][]byte{}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/theme/theme") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			entries[f.Name] = b
		}
	}
	return themeName(t, entries)
}

func testPNG(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../internal/generator/testdata/test_image_small.png")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hasMedia(entries map[string][]byte) bool {
	for name := range entries {
		if strings.HasPrefix(name, "ppt/media/") {
			return true
		}
	}
	return false
}

const httpNoTemplateSpec = `{
  "meta": {"title": "Quarterly Review"},
  "slides": [
    {"kind": "title", "title": "Quarterly Review", "subtitle": "FY26 Q2"},
    {"kind": "closing", "title": "Questions?"}
  ]
}`

func TestHTTPSemanticRender_JSONAndYAMLProducePPTX(t *testing.T) {
	ts := newSemanticRenderTestServer(t, semanticRenderTestConfig(t))

	code, res := postRender(t, ts, "", "application/x-yaml", strings.NewReader(validSemanticSpec))
	if code != http.StatusOK || !res.OK {
		t.Fatalf("yaml render: %d %+v", code, res)
	}
	if res.OutputPath != "" {
		t.Errorf("server-local output_path leaked: %q", res.OutputPath)
	}
	if res.SlideCount != 2 || res.ContentHash == "" || len(res.Quality) == 0 || res.Template != "midnight-blue" {
		t.Errorf("result = %+v", res)
	}
	entries := downloadDeck(t, ts, res.FileURL)
	if n := countSlides(entries); n != 2 {
		t.Errorf("downloaded deck has %d slides, want 2", n)
	}

	code, res = postRender(t, ts, "?template=midnight-blue", "application/json", strings.NewReader(httpNoTemplateSpec))
	if code != http.StatusOK || !res.OK || res.SlideCount != 2 {
		t.Fatalf("json render: %d %+v", code, res)
	}

	// No template anywhere (spec, query, archetype default) is a refusal
	// addressed at meta.template, as on the CLI.
	code, res = postRender(t, ts, "", "application/json", strings.NewReader(httpNoTemplateSpec))
	if code != http.StatusUnprocessableEntity || res.OK || len(res.Diagnostics) == 0 || res.Diagnostics[len(res.Diagnostics)-1].SemanticPath != "meta.template" {
		t.Fatalf("templateless render: %d %+v", code, res)
	}
}

// A registered template named by the template query parameter renders the
// deck, whether or not the spec pins one; when it replaces the spec's
// meta.template the response says so (go-slide-creator-ifkxs).
func TestHTTPSemanticRender_RegisteredTemplate(t *testing.T) {
	ts := newSemanticRenderTestServer(t, semanticRenderTestConfig(t))

	code, res := postRender(t, ts, "?template=warm-coral", "application/json", strings.NewReader(httpNoTemplateSpec))
	if code != http.StatusOK || res.Template != "warm-coral" {
		t.Fatalf("render: %d %+v", code, res)
	}
	want := templateThemeName(t, filepath.Join(testTemplatesDir, "warm-coral.pptx"))
	if got := themeName(t, downloadDeck(t, ts, res.FileURL)); got != want {
		t.Errorf("deck theme %q, want warm-coral's %q", got, want)
	}

	code, res = postRender(t, ts, "?template=warm-coral", "application/x-yaml", strings.NewReader(validSemanticSpec))
	if code != http.StatusOK || res.Template != "warm-coral" {
		t.Fatalf("pinned render: %d %+v", code, res)
	}
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], `overrides meta.template "midnight-blue"`) {
		t.Errorf("the response does not say the template argument replaced the pin: %v", res.Warnings)
	}
	if got := themeName(t, downloadDeck(t, ts, res.FileURL)); got != want {
		t.Errorf("pinned deck theme %q, want warm-coral's %q", got, want)
	}

	code, res = postRender(t, ts, "?template=no-such-template", "application/json", strings.NewReader(httpNoTemplateSpec))
	if code != http.StatusUnprocessableEntity || res.OK {
		t.Fatalf("unknown template: %d %+v", code, res)
	}
	found := false
	for _, d := range res.Diagnostics {
		found = found || (d.Code == "TEMPLATE_NOT_FOUND" && d.SemanticPath == "meta.template")
	}
	if !found {
		t.Errorf("no TEMPLATE_NOT_FOUND at meta.template: %+v", res.Diagnostics)
	}
}

// A bring-your-own template uploaded as the multipart "template" part renders
// the deck (it is not registered in the server's templates dir).
func TestHTTPSemanticRender_BYOTemplate(t *testing.T) {
	cfg := semanticRenderTestConfig(t)
	cfg.Templates.Dir = t.TempDir() // nothing registered on disk
	ts := newSemanticRenderTestServer(t, cfg)

	tplPath := filepath.Join(testTemplatesDir, "forest-green.pptx")
	tpl, err := os.ReadFile(tplPath)
	if err != nil {
		t.Fatal(err)
	}
	code, res := postRenderMultipart(t, ts, "",
		renderPart{"spec", "deck.json", httpNoTemplateSpec},
		renderPart{"template", "brand.pptx", string(tpl)})
	if code != http.StatusOK || !res.OK || res.Template != "brand.pptx" {
		t.Fatalf("byo render: %d %+v", code, res)
	}
	if got, want := themeName(t, downloadDeck(t, ts, res.FileURL)), templateThemeName(t, tplPath); got != want {
		t.Errorf("deck theme %q, want the uploaded template's %q", got, want)
	}

	// A corrupt upload is a refusal, not a server error.
	code, res = postRenderMultipart(t, ts, "",
		renderPart{"spec", "deck.json", httpNoTemplateSpec},
		renderPart{"template", "brand.pptx", "not a zip"})
	if code == http.StatusOK || res.OK {
		t.Fatalf("corrupt template rendered: %d %+v", code, res)
	}
}

// Uploaded assets resolve relative asset paths in the spec; a relative path
// to a file that was not uploaded is refused with a source-addressed
// diagnostic instead of reaching the server's working directory.
func TestHTTPSemanticRender_Assets(t *testing.T) {
	ts := newSemanticRenderTestServer(t, semanticRenderTestConfig(t))
	spec := fmt.Sprintf(rawImageSemanticSpec, "logo.png")

	code, res := postRenderMultipart(t, ts, "",
		renderPart{"spec", "deck.yaml", spec},
		renderPart{"assets", "logo.png", testPNG(t)})
	if code != http.StatusOK || !res.OK {
		t.Fatalf("asset render: %d %+v", code, res)
	}
	if !hasMedia(downloadDeck(t, ts, res.FileURL)) {
		t.Error("uploaded image was not embedded")
	}

	code, res = postRenderMultipart(t, ts, "", renderPart{"spec", "deck.yaml", spec})
	if code != http.StatusUnprocessableEntity || res.OK || res.FileURL != "" {
		t.Fatalf("missing asset: %d %+v", code, res)
	}
	found := false
	for _, d := range res.Diagnostics {
		if d.Code == "IMAGE_PATH" && strings.HasPrefix(d.SemanticPath, "slides[1]") &&
			d.RawPath == "/slides/1/content/1/image_value/path" && d.SlideIndex != nil && *d.SlideIndex == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("no source-addressed IMAGE_PATH diagnostic: %+v", res.Diagnostics)
	}
}

// The renderer runs with the server's runtime configuration: its templates dir
// and its image allow-list. An absolute server path is only embeddable from an
// ALLOWED_IMAGE_PATHS root — never from arbitrary server locations.
func TestHTTPSemanticRender_RuntimeConfig(t *testing.T) {
	imgDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(imgDir); err == nil {
		imgDir = resolved
	}
	img := filepath.Join(imgDir, "logo.png")
	if err := os.WriteFile(img, []byte(testPNG(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := fmt.Sprintf(rawImageSemanticSpec, img)

	// No allow-list configured: HTTP still refuses a server file outside the
	// request's own asset directory.
	cfg := semanticRenderTestConfig(t)
	ts := newSemanticRenderTestServer(t, cfg)
	code, res := postRender(t, ts, "", "application/x-yaml", strings.NewReader(spec))
	if code == http.StatusOK && !strings.Contains(strings.Join(res.Warnings, "\n"), "image path validation failed") {
		t.Fatalf("server file outside any allowed root was embedded: %d %+v", code, res)
	}

	// The operator's ALLOWED_IMAGE_PATHS root admits it.
	cfg = semanticRenderTestConfig(t)
	cfg.Images.AllowedBasePaths = []string{imgDir}
	ts = newSemanticRenderTestServer(t, cfg)
	code, res = postRender(t, ts, "", "application/x-yaml", strings.NewReader(spec))
	if code != http.StatusOK || strings.Contains(strings.Join(res.Warnings, "\n"), "image path validation failed") {
		t.Fatalf("image inside the configured root was refused: %d %+v", code, res)
	}
	if !hasMedia(downloadDeck(t, ts, res.FileURL)) {
		t.Error("allowed image was not embedded")
	}

	// The sample screenshot a recommend_visual recipe points at renders over
	// HTTP as it does over MCP and the CLI: that one file is admitted, with
	// or without a configured root, and nothing beside it is
	// (go-slide-creator-075py).
	sample, err := sampleScreenshot()
	if err != nil {
		t.Fatal(err)
	}
	neighbour := filepath.Join(filepath.Dir(sample), "http-neighbour.png")
	if err := os.WriteFile(neighbour, []byte(testPNG(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(neighbour) })
	for _, roots := range [][]string{nil, {imgDir}} {
		cfg = semanticRenderTestConfig(t)
		cfg.Images.AllowedBasePaths = roots
		ts = newSemanticRenderTestServer(t, cfg)
		code, res = postRender(t, ts, "", "application/x-yaml", strings.NewReader(fmt.Sprintf(rawImageSemanticSpec, sample)))
		if code != http.StatusOK || strings.Contains(strings.Join(res.Warnings, "\n"), "image path validation failed") {
			t.Fatalf("roots %v: the server's sample screenshot was refused: %d %+v", roots, code, res)
		}
		if !hasMedia(downloadDeck(t, ts, res.FileURL)) {
			t.Errorf("roots %v: the sample screenshot was not embedded", roots)
		}
		code, res = postRender(t, ts, "", "application/x-yaml", strings.NewReader(fmt.Sprintf(rawImageSemanticSpec, neighbour)))
		if code == http.StatusOK && !strings.Contains(strings.Join(res.Warnings, "\n"), "image path validation failed") {
			t.Fatalf("roots %v: a file beside the sample was embedded: %d %+v", roots, code, res)
		}
	}

	// Templates resolve from the configured templates dir.
	cfg = semanticRenderTestConfig(t)
	cfg.Templates.Dir = t.TempDir()
	src, err := os.ReadFile(filepath.Join(testTemplatesDir, "warm-coral.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Templates.Dir, "house-style.pptx"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	ts = newSemanticRenderTestServer(t, cfg)
	code, res = postRender(t, ts, "?template=house-style", "application/json", strings.NewReader(httpNoTemplateSpec))
	if code != http.StatusOK || res.Template != "house-style" {
		t.Fatalf("configured templates dir not used: %d %+v", code, res)
	}
}

// A deck generation refuses as unreadable answers 422 with the refusal traced
// back to the semantic slide that carries it.
func TestHTTPSemanticRender_ReadableTextRefusal(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "native_readability", "bmc_dense.json"))
	if err != nil {
		t.Fatal(err)
	}
	var deck struct {
		Slides []json.RawMessage `json:"slides"`
	}
	if err := json.Unmarshal(raw, &deck); err != nil {
		t.Fatal(err)
	}
	spec := fmt.Sprintf(`{"meta": {"title": "Canvas", "template": "modern"},
  "slides": [{"kind": "title", "title": "Canvas"}, {"kind": "raw_json2pptx", "slide": %s}]}`, deck.Slides[1])

	ts := newSemanticRenderTestServer(t, semanticRenderTestConfig(t))
	code, res := postRender(t, ts, "", "application/json", strings.NewReader(spec))
	if code != http.StatusUnprocessableEntity || res.OK || res.FileURL != "" {
		t.Fatalf("dense canvas rendered: %d %+v", code, res)
	}
	if !strings.Contains(res.Error, "unreadable generated text") {
		t.Errorf("error = %q", res.Error)
	}
	var refusal *semanticDiagnostic
	for i := range res.Diagnostics {
		if res.Diagnostics[i].Code == patterns.ErrCodeTextBelowReadableMin {
			refusal = &res.Diagnostics[i]
		}
	}
	if refusal == nil {
		t.Fatalf("no %s diagnostic: %+v", patterns.ErrCodeTextBelowReadableMin, res.Diagnostics)
	}
	if !strings.HasPrefix(refusal.SemanticPath, "slides[1]") || refusal.RawPath != "/slides/1/content/1/diagram_value" ||
		refusal.Severity != "error" || refusal.SlideIndex == nil || *refusal.SlideIndex != 1 {
		t.Errorf("refusal not source-addressed: %+v\nall: %+v", *refusal, res.Diagnostics)
	}
}

// An invalid spec is a 422 with semantic-path diagnostics and no deck.
func TestHTTPSemanticRender_InvalidSpec(t *testing.T) {
	ts := newSemanticRenderTestServer(t, semanticRenderTestConfig(t))
	code, res := postRender(t, ts, "", "application/x-yaml", strings.NewReader(invalidSemanticSpec))
	if code != http.StatusUnprocessableEntity || res.OK || res.FileURL != "" {
		t.Fatalf("invalid spec: %d %+v", code, res)
	}
	found := false
	for _, d := range res.Diagnostics {
		found = found || (d.Severity == "error" && d.SemanticPath != "")
	}
	if !found {
		t.Errorf("no source-addressed error diagnostic: %+v", res.Diagnostics)
	}
}

// The HTTP validate endpoint addresses its findings as the DeckSpec tools do:
// path is a JSON Pointer that resolves in the spec that was sent, a field the
// spec lacks is missing_path, and nothing reports a dotted evidence.path
// (go-slide-creator-pilpn).
func TestHTTPSemanticValidate_FindingsUseTheAuthoredAddress(t *testing.T) {
	ts := newSemanticRenderTestServer(t, semanticRenderTestConfig(t))
	resp, err := http.Post(ts.URL+"/api/v1/semantic/validate", "application/json", strings.NewReader(journeyRT3Spec))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Valid    bool `json:"valid"`
		Findings struct {
			Findings []map[string]any `json:"findings"`
		} `json:"findings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Valid || len(out.Findings.Findings) == 0 {
		t.Fatalf("the spec with unknown keys validated clean: %+v", out)
	}
	var doc any
	_ = json.Unmarshal([]byte(journeyRT3Spec), &doc)
	unknown := false
	for _, f := range out.Findings.Findings {
		assertAuthoredAddress(t, "http validate", doc, f)
		if ev, ok := f["evidence"].(map[string]any); ok {
			if _, dotted := ev["path"]; dotted {
				t.Errorf("finding still reports evidence.path: %v", f)
			}
		}
		if f["path"] == "/slides/2/steps/0/value" {
			unknown = true
		}
	}
	if !unknown {
		t.Errorf("no finding at /slides/2/steps/0/value: %+v", out.Findings.Findings)
	}
}
