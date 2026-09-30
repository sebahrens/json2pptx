package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// ---------------------------------------------------------------------------
// POST /api/v1/semantic/render — compile a semantic DeckSpec and render it to
// a .pptx through the SAME in-memory presentation runner the `json2pptx
// semantic render` CLI and the render_deck_spec MCP tool use.
//
// The runner (RunPresentation plus the semantic compile / asset / diagnostic
// glue around it) lives in cmd/json2pptx, which internal/api must not import.
// The server therefore receives it as an injected SemanticRenderer at wiring
// time (ServerConfig.SemanticRenderer). The HTTP layer owns only transport:
// request decoding (raw spec body or multipart with a bring-your-own template
// and asset files), a per-request scratch directory, the output filename in
// the download directory, concurrency / timeout bounds, and the response
// envelope. Everything that decides what gets rendered and what is reported is
// the shared runner's.
// ---------------------------------------------------------------------------

// MaxSemanticRenderBodySize bounds a render request. It is larger than the
// plain JSON limit because a multipart request may carry a template .pptx and
// image assets alongside the spec.
const MaxSemanticRenderBodySize = 64 * 1024 * 1024

// semanticRenderMaxMemory is the multipart parse budget held in memory; larger
// parts spill to temporary files (removed after the request).
const semanticRenderMaxMemory = 8 * 1024 * 1024

// SemanticRenderRequest is one render job handed to the injected renderer.
type SemanticRenderRequest struct {
	// Spec is the DeckSpec document; Filename's extension selects the parser
	// (".json" → JSON, otherwise YAML).
	Spec     []byte
	Filename string
	// Strict is the advisory-rule strictness for validation/compilation.
	Strict semantic.Strictness
	// Template is the registered template used when the spec pins none.
	Template string
	// TemplatePath is an uploaded bring-your-own template (.pptx) inside
	// WorkDir, or "" when none was uploaded. Like render_deck_spec's
	// template_path it applies only when the spec pins no meta.template.
	TemplatePath string
	// TemplateFilename is the client's name for the uploaded template, for
	// messages only.
	TemplateFilename string
	// AssetDir holds the uploaded asset files. Relative asset references in
	// the spec resolve against it (the HTTP analogue of the CLI resolving them
	// against the spec file's directory). It is always a fresh per-request
	// directory, so a relative reference to a file that was not uploaded is
	// rejected instead of leaking to the server's working directory.
	AssetDir string
	// WorkDir is the per-request scratch directory (removed after the request).
	WorkDir string
	// OutputValidation is off, warn, or strict (default strict).
	OutputValidation string
	// OutputDir / OutputFilename name the .pptx destination; the handler picks
	// an unguessable filename in the download directory.
	OutputDir      string
	OutputFilename string
}

// SemanticRenderOutcome is the renderer's result. Result is the JSON object the
// CLI `semantic render` command prints (ok, slide_count, content_hash, quality,
// diagnostics with semantic_path, …). OK reports whether a deck was written;
// OutputPath is where.
type SemanticRenderOutcome struct {
	OK         bool
	OutputPath string
	Result     json.RawMessage
}

// SemanticRenderer renders one semantic DeckSpec. A returned error is an
// internal failure (HTTP 500); spec, template, asset and quality refusals are
// reported through an outcome with OK=false.
type SemanticRenderer func(ctx context.Context, req SemanticRenderRequest) (SemanticRenderOutcome, error)

// semanticRenderService adapts an injected SemanticRenderer to HTTP.
type semanticRenderService struct {
	renderer  SemanticRenderer
	outputDir string
	retention time.Duration
	// sem bounds concurrent renders; shared with /convert so the two
	// generation endpoints together respect MaxConcurrentConverts.
	sem chan struct{}
}

// SemanticRenderHandler returns the POST /api/v1/semantic/render handler. With
// a nil renderer (a server built without the render runner wired in) it
// responds 501 and points callers at the CLI / MCP render surfaces.
func SemanticRenderHandler(renderer SemanticRenderer, outputDir string, retention time.Duration, sem chan struct{}) http.HandlerFunc {
	s := &semanticRenderService{renderer: renderer, outputDir: outputDir, retention: retention, sem: sem}
	return s.serve
}

func (s *semanticRenderService) serve(w http.ResponseWriter, r *http.Request) {
	if s.renderer == nil {
		writeError(w, http.StatusNotImplemented, apierrors.CodeUnsupportedFeature,
			"Semantic render is not configured on this server. Use the `json2pptx semantic render` CLI "+
				"or the render_deck_spec MCP tool.",
			map[string]any{"alternatives": []string{"json2pptx semantic render", "render_deck_spec (MCP)"}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DefaultConvertTimeout)
	defer cancel()

	strict, ok := semanticStrictParam(w, r)
	if !ok {
		return
	}
	outputValidation, ok := semanticOutputValidationParam(w, r)
	if !ok {
		return
	}

	if s.sem != nil {
		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
		default:
			w.Header().Set("Retry-After", "5")
			writeError(w, http.StatusServiceUnavailable, apierrors.CodeRateLimited,
				"Too many concurrent renders; retry later", nil)
			return
		}
	}

	workDir, err := os.MkdirTemp("", "json2pptx-semantic-render-*")
	if err != nil {
		slog.Error("semantic render: create work dir", "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.CodeInternalError, "Failed to prepare render", nil)
		return
	}
	defer func() { _ = os.RemoveAll(workDir) }()
	// Resolve symlinks (macOS /var → /private/var) so the asset directory is
	// the same path the image allow-list containment check sees.
	if resolved, evalErr := filepath.EvalSymlinks(workDir); evalErr == nil {
		workDir = resolved
	}
	assetDir := filepath.Join(workDir, "assets")
	if err := os.Mkdir(assetDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, apierrors.CodeInternalError, "Failed to prepare render", nil)
		return
	}

	req := SemanticRenderRequest{
		Strict:           strict,
		Template:         strings.TrimSpace(r.URL.Query().Get("template")),
		AssetDir:         assetDir,
		WorkDir:          workDir,
		OutputValidation: outputValidation,
		OutputDir:        s.outputDir,
	}
	if !readSemanticRenderInput(w, r, &req) {
		return
	}

	name, err := generateUniqueFilename()
	if err != nil {
		writeError(w, http.StatusInternalServerError, apierrors.CodeGenerationError, "Failed to generate output filename", nil)
		return
	}
	req.OutputFilename = name + ".pptx"

	outcome, err := s.renderer(ctx, req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			writeError(w, http.StatusGatewayTimeout, apierrors.CodeRequestTimeout, "Request processing timed out", nil)
			return
		}
		slog.Error("semantic render failed", "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.CodeRenderError, "Semantic render failed", nil)
		return
	}
	s.writeOutcome(w, req, outcome)
}

// writeOutcome answers with the renderer's result object. Server-local paths
// (output_path, manifest_path) are replaced by the download URL of the
// written deck; a refused render answers 422 with the same object (ok=false,
// error, source-addressed diagnostics).
func (s *semanticRenderService) writeOutcome(w http.ResponseWriter, req SemanticRenderRequest, outcome SemanticRenderOutcome) {
	body := map[string]json.RawMessage{}
	if len(outcome.Result) > 0 {
		if err := json.Unmarshal(outcome.Result, &body); err != nil {
			writeError(w, http.StatusInternalServerError, apierrors.CodeInternalError, "Malformed render result", nil)
			return
		}
	}
	delete(body, "output_path")
	delete(body, "manifest_path")
	body["ok"] = json.RawMessage(fmt.Sprint(outcome.OK))

	status := http.StatusUnprocessableEntity
	if outcome.OK {
		status = http.StatusOK
		// The deck must sit in the download directory under the filename the
		// handler chose, or the advertised URL would 404.
		if filepath.Clean(outcome.OutputPath) != filepath.Join(s.outputDir, req.OutputFilename) {
			slog.Error("semantic render wrote outside the download directory", "path", outcome.OutputPath)
			writeError(w, http.StatusInternalServerError, apierrors.CodeInternalError, "Render output misplaced", nil)
			return
		}
		body["file_url"] = mustJSON("/api/v1/download/" + req.OutputFilename)
		body["expires_at"] = mustJSON(time.Now().Add(effectiveRetention(s.retention)).Format(time.RFC3339))
	}
	writeJSON(w, status, body)
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func effectiveRetention(d time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return DefaultFileRetention
}

// readSemanticRenderInput fills req from the request body: a raw spec document
// (application/json or YAML, as for validate/compile) or multipart/form-data
// carrying a "spec" part plus an optional "template" .pptx part and any number
// of "assets" file parts. It writes the error response and returns false on a
// malformed request.
func readSemanticRenderInput(w http.ResponseWriter, r *http.Request, req *SemanticRenderRequest) bool {
	mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if !strings.EqualFold(mediaType, "multipart/form-data") {
		data, filename, ok := readSemanticBody(w, r)
		if !ok {
			return false
		}
		req.Spec, req.Filename = data, filename
		return true
	}
	if params["boundary"] == "" {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "multipart request has no boundary", nil)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxSemanticRenderBodySize)
	if err := r.ParseMultipartForm(semanticRenderMaxMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, apierrors.CodeRequestTooLarge,
				"Request body exceeds the maximum allowed size", nil)
			return false
		}
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
			"Failed to parse multipart request: "+err.Error(), nil)
		return false
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()
	form := r.MultipartForm

	if !readMultipartSpec(w, form, req) {
		return false
	}
	if files := form.File["template"]; len(files) > 0 {
		if len(files) > 1 {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "at most one template part is allowed", nil)
			return false
		}
		if !strings.EqualFold(filepath.Ext(files[0].Filename), ".pptx") {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidTemplate,
				"template part must be a .pptx file", map[string]any{"filename": files[0].Filename})
			return false
		}
		dst := filepath.Join(req.WorkDir, "template.pptx")
		if err := saveMultipartFile(files[0], dst); err != nil {
			writeError(w, http.StatusInternalServerError, apierrors.CodeFileError, "Failed to store uploaded template", nil)
			return false
		}
		req.TemplatePath, req.TemplateFilename = dst, filepath.Base(files[0].Filename)
	}
	seen := map[string]bool{}
	for _, fh := range form.File["assets"] {
		name := filepath.Base(filepath.Clean("/" + filepath.ToSlash(fh.Filename)))
		if name == "" || name == "/" || name == "." || strings.HasPrefix(name, ".") {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
				"asset part has an invalid filename", map[string]any{"filename": fh.Filename})
			return false
		}
		if seen[name] {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
				"duplicate asset filename", map[string]any{"filename": name})
			return false
		}
		seen[name] = true
		if err := saveMultipartFile(fh, filepath.Join(req.AssetDir, name)); err != nil {
			writeError(w, http.StatusInternalServerError, apierrors.CodeFileError, "Failed to store uploaded asset", nil)
			return false
		}
	}
	return true
}

// readMultipartSpec reads the "spec" part — a file part (its extension, or a
// JSON content type, selects the parser) or a plain form field (JSON when it
// starts with "{", else YAML).
func readMultipartSpec(w http.ResponseWriter, form *multipart.Form, req *SemanticRenderRequest) bool {
	var data []byte
	filename := "spec.yaml"
	switch {
	case len(form.File["spec"]) == 1:
		fh := form.File["spec"][0]
		f, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "Failed to read spec part", nil)
			return false
		}
		data, err = io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "Failed to read spec part", nil)
			return false
		}
		ct, _, _ := mime.ParseMediaType(fh.Header.Get("Content-Type"))
		if strings.EqualFold(filepath.Ext(fh.Filename), ".json") || ct == "application/json" {
			filename = "spec.json"
		}
	case len(form.File["spec"]) > 1 || len(form.Value["spec"]) > 1:
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest, "exactly one spec part is allowed", nil)
		return false
	case len(form.Value["spec"]) == 1:
		data = []byte(form.Value["spec"][0])
		if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
			filename = "spec.json"
		}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidInput,
			"multipart request needs a non-empty \"spec\" part carrying the semantic deck spec", nil)
		return false
	}
	req.Spec, req.Filename = data, filename
	return true
}

func saveMultipartFile(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// semanticOutputValidationParam parses output_validation (off|warn|strict,
// default strict — every successful render passes output validation unless
// the caller opts out, as on the CLI and MCP).
func semanticOutputValidationParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("output_validation"))
	switch raw {
	case "":
		return "strict", true
	case "off", "warn", "strict":
		return raw, true
	default:
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidInput,
			"Invalid output_validation value; must be off, warn, or strict",
			map[string]any{"parameter": "output_validation", "value": raw})
		return "", false
	}
}
