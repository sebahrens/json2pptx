package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// ---------------------------------------------------------------------------
// Semantic HTTP endpoints — the compact DeckSpec authoring surface over HTTP.
//
// These handlers expose internal/semantic (the compact semantic deck-spec
// model) under /api/v1/semantic so a client can validate, compile, and
// introspect a deck spec without dropping to the raw PresentationInput model.
// They are thin adapters over the same internal/semantic entry points the
// `json2pptx semantic` CLI subcommands and the semantic MCP tools use, so the
// surfaces cannot drift in behavior.
//
// Request bodies carry the spec document verbatim: a JSON body
// (Content-Type: application/json) is parsed as JSON, a YAML body
// (application/x-yaml or text/yaml) as YAML. Tuning options ride as query
// parameters (strict, template, include_compiled_json) so the body stays a
// clean spec document — matching the `--data-binary @spec.yaml` curl shape.
//
// Diagnostic-bearing responses use the shared diagnostics.FindingEnvelope (the
// same agent-facing contract as the CLI/MCP surfaces). Transport/request errors
// (unsupported content type, oversized or empty body) keep the simple
// apierrors.Response shape.
//
// Render is intentionally deferred: the render orchestration (RunPresentation)
// still lives in cmd/json2pptx, not internal/, so POST /api/v1/semantic/render
// returns 501 and points callers at the CLI / MCP render surfaces until the
// orchestration is extracted into internal/.
// ---------------------------------------------------------------------------

// semanticValidateResponse is the body of POST /api/v1/semantic/validate. valid
// mirrors findings.ok (true when no finding has error severity) for a quick
// pass/fail check; findings is the full shared envelope of semantic-path
// diagnostics.
type semanticValidateResponse struct {
	Valid    bool                        `json:"valid"`
	Findings diagnostics.FindingEnvelope `json:"findings"`
}

// semanticCompileResponse is the body of POST /api/v1/semantic/compile. On
// success (HTTP 200) ok is true and slide_count/template summarize the compiled
// deck; compiled_json carries the full raw PresentationInput only when
// include_compiled_json=true. On a blocking parse/compile failure (HTTP 422) ok
// is false and error names the blocking reason. findings always carries the
// shared envelope of compile diagnostics.
type semanticCompileResponse struct {
	OK           bool                        `json:"ok"`
	SlideCount   int                         `json:"slide_count,omitempty"`
	Template     string                      `json:"template,omitempty"`
	Findings     diagnostics.FindingEnvelope `json:"findings"`
	CompiledJSON json.RawMessage             `json:"compiled_json,omitempty"`
	Error        string                      `json:"error,omitempty"`
}

// SemanticSchemaHandler returns GET /api/v1/semantic/schema — the JSON Schema
// (draft 2020-12) describing the compact semantic DeckSpec authoring format.
func SemanticSchemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, semantic.Schema())
	}
}

// SemanticValidateHandler returns POST /api/v1/semantic/validate — validate a
// semantic deck spec and return a FindingEnvelope of semantic-path diagnostics.
// The request body is the spec document (JSON or YAML by Content-Type); the
// optional strict query parameter (off|warn|strict, default warn) controls
// advisory-rule severity. Always responds 200: validation completing is the
// success case, and valid/findings.ok report whether the spec is clean.
func SemanticValidateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, filename, ok := readSemanticBody(w, r)
		if !ok {
			return // readSemanticBody already wrote the error response
		}
		strict, ok := semanticStrictParam(w, r)
		if !ok {
			return
		}

		ds := semantic.Check(filename, data, strict)
		env := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "semantic_validate",
			InputSHA256: diagnostics.ComputeInputSHA256(data),
		}, ds)
		writeJSON(w, http.StatusOK, semanticValidateResponse{Valid: env.OK, Findings: env})
	}
}

// SemanticCompileHandler returns POST /api/v1/semantic/compile — compile a
// semantic deck spec into the raw json2pptx PresentationInput model. The
// request body is the spec document (JSON or YAML by Content-Type). Query
// parameters: strict (off|warn|strict, default warn), template (default
// template when the spec pins none), and include_compiled_json (when true, the
// full compiled PresentationInput is returned under compiled_json). A blocking
// parse/compile failure responds 422 with ok=false and the blocking findings.
func SemanticCompileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, filename, ok := readSemanticBody(w, r)
		if !ok {
			return
		}
		strict, ok := semanticStrictParam(w, r)
		if !ok {
			return
		}
		template := r.URL.Query().Get("template")
		includeJSON := semanticBoolParam(r, "include_compiled_json")

		envOpts := diagnostics.EnvelopeOptions{
			Subcommand:  "semantic_compile",
			InputSHA256: diagnostics.ComputeInputSHA256(data),
		}

		spec, parseDiags := semantic.Parse(filename, data)
		if parseDiags.HasErrors() {
			env := diagnostics.BuildEnvelope(envOpts, parseDiags.ToDiagnostics())
			writeJSON(w, http.StatusUnprocessableEntity, semanticCompileResponse{
				OK:       false,
				Findings: env,
				Error:    "semantic spec could not be parsed",
			})
			return
		}

		input, result, err := semantic.Compile(spec, semantic.CompileOptions{
			Strict:          strict,
			DefaultTemplate: template,
		})
		var ds []diagnostics.Diagnostic
		if result != nil {
			ds = result.Diagnostics
		}
		env := diagnostics.BuildEnvelope(envOpts, ds)

		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, semanticCompileResponse{
				OK:       false,
				Findings: env,
				Error:    err.Error(),
			})
			return
		}

		resp := semanticCompileResponse{
			OK:         true,
			SlideCount: len(input.Slides),
			Template:   input.Template,
			Findings:   env,
		}
		if includeJSON {
			raw, marshalErr := json.Marshal(input)
			if marshalErr != nil {
				writeError(w, http.StatusInternalServerError, apierrors.CodeInternalError,
					"Failed to marshal compiled deck", nil)
				return
			}
			resp.CompiledJSON = raw
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// SemanticRenderHandler returns POST /api/v1/semantic/render — deferred. The
// render orchestration (RunPresentation) currently lives in cmd/json2pptx, not
// internal/, so the HTTP surface cannot drive it without duplicating the flow.
// Until that orchestration is extracted into internal/, this endpoint responds
// 501 and points callers at the CLI / MCP render surfaces.
func SemanticRenderHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotImplemented, apierrors.CodeUnsupportedFeature,
			"Semantic render is not yet available over HTTP. Use the `json2pptx semantic render` CLI "+
				"or the render_deck_spec MCP tool; POST /api/v1/semantic/compile to obtain the raw "+
				"PresentationInput JSON in the meantime.",
			map[string]any{
				"alternatives": []string{
					"json2pptx semantic render",
					"render_deck_spec (MCP)",
					"POST /api/v1/semantic/compile",
				},
			})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// readSemanticBody reads the request body as a semantic spec document, returning
// the bytes and a filename whose extension selects the parser (".json" for
// application/json, ".yaml" otherwise — YAML is a JSON superset, so an unset or
// YAML content type also parses inline JSON). It enforces the shared body-size
// limit and rejects an empty body or an unsupported content type with the simple
// transport-error envelope, returning ok=false when it has written a response.
func readSemanticBody(w http.ResponseWriter, r *http.Request) (data []byte, filename string, ok bool) {
	filename = "spec.yaml"
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
		switch mediaType {
		case "application/json":
			filename = "spec.json"
		case "application/x-yaml", "application/yaml", "text/yaml", "text/x-yaml", "text/plain":
			filename = "spec.yaml"
		default:
			writeError(w, http.StatusUnsupportedMediaType, apierrors.CodeInvalidContentType,
				"Content-Type must be application/json or application/x-yaml", nil)
			return nil, "", false
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, apierrors.CodeRequestTooLarge,
				"Request body exceeds the maximum allowed size", nil)
			return nil, "", false
		}
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
			"Failed to read request body", nil)
		return nil, "", false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidInput,
			"Request body is empty; provide a semantic deck spec", nil)
		return nil, "", false
	}
	return body, filename, true
}

// semanticStrictParam parses the optional strict query parameter into a
// semantic.Strictness, defaulting to warn. An unrecognized value is a request
// error: it writes the transport-error envelope and returns ok=false.
func semanticStrictParam(w http.ResponseWriter, r *http.Request) (semantic.Strictness, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("strict"))
	switch raw {
	case "":
		return semantic.StrictnessWarn, true
	case "off":
		return semantic.StrictnessOff, true
	case "warn":
		return semantic.StrictnessWarn, true
	case "strict":
		return semantic.StrictnessStrict, true
	default:
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidInput,
			"Invalid strict value; must be off, warn, or strict",
			map[string]any{"parameter": "strict", "value": raw})
		return "", false
	}
}

// semanticBoolParam reports whether the named query parameter is set to a
// truthy value ("1", "true", "yes", case-insensitive).
func semanticBoolParam(r *http.Request, name string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}
