package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// PatternsHandler provides HTTP endpoints for the pattern library.
type PatternsHandler struct {
	registry *patterns.Registry
}

// NewPatternsHandler creates a new PatternsHandler using the given registry.
func NewPatternsHandler(reg *patterns.Registry) *PatternsHandler {
	return &PatternsHandler{registry: reg}
}

// ListHandler returns GET /api/v1/patterns — compact listing of all patterns.
func (h *PatternsHandler) ListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all := h.registry.List()
		items := make([]patternListItem, len(all))
		for i, p := range all {
			item := patternListItem{
				Name:        p.Name(),
				Description: p.Description(),
				UseWhen:     p.UseWhen(),
				NotWhen:     p.NotWhen(),
				Version:     p.Version(),
			}
			item.CellsHint = p.CellsHint()
			if cs, ok := p.(patterns.CalloutSupport); ok {
				item.SupportsCallout = cs.SupportsCallout()
			}
			items[i] = item
		}
		writeJSON(w, http.StatusOK, patternListResponse{Patterns: items})
	}
}

// ShowHandler returns GET /api/v1/patterns/{name} — full detail with schema.
func (h *PatternsHandler) ShowHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		pat, ok := h.registry.Get(name)
		if !ok {
			msg := fmt.Sprintf("Pattern %q not found", name)
			if suggestion, ok := h.registry.Suggest(name); ok {
				msg += fmt.Sprintf("; did you mean %q?", suggestion)
			}
			writeError(w, http.StatusNotFound, apierrors.CodePatternNotFound, msg, nil)
			return
		}

		resp := patternShowResponse{
			Name:        pat.Name(),
			Description: pat.Description(),
			UseWhen:     pat.UseWhen(),
			NotWhen:     pat.NotWhen(),
			Version:     pat.Version(),
			Schema:      patterns.SchemaJSON(pat),
		}
		resp.CellsHint = pat.CellsHint()
		if cs, ok := pat.(patterns.CalloutSupport); ok {
			resp.SupportsCallout = cs.SupportsCallout()
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// ValidateHandler returns POST /api/v1/patterns/{name}/validate.
func (h *PatternsHandler) ValidateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		pat, ok := h.registry.Get(name)
		if !ok {
			msg := fmt.Sprintf("Pattern %q not found", name)
			if suggestion, ok := h.registry.Suggest(name); ok {
				msg += fmt.Sprintf("; did you mean %q?", suggestion)
			}
			writeError(w, http.StatusNotFound, apierrors.CodePatternNotFound, msg, nil)
			return
		}

		var body patternRequestBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			return // decodeJSONBody already wrote the error response
		}

		values, overrides, cellOverrides, err := unmarshalPatternInputs(pat, &body)
		if err != nil {
			writePatternValidationError(w, "validate_pattern", name, err)
			return
		}

		if err := pat.Validate(values, overrides, cellOverrides); err != nil {
			writePatternValidationError(w, "validate_pattern", name, err)
			return
		}

		writeJSON(w, http.StatusOK, patternValidateResponse{OK: true})
	}
}

// ExpandHandler returns POST /api/v1/patterns/{name}/expand.
func (h *PatternsHandler) ExpandHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		pat, ok := h.registry.Get(name)
		if !ok {
			msg := fmt.Sprintf("Pattern %q not found", name)
			if suggestion, ok := h.registry.Suggest(name); ok {
				msg += fmt.Sprintf("; did you mean %q?", suggestion)
			}
			writeError(w, http.StatusNotFound, apierrors.CodePatternNotFound, msg, nil)
			return
		}

		var body patternRequestBody
		if err := decodeJSONBody(w, r, &body); err != nil {
			return // decodeJSONBody already wrote the error response
		}

		values, overrides, cellOverrides, err := unmarshalPatternInputs(pat, &body)
		if err != nil {
			writePatternValidationError(w, "expand_pattern", name, err)
			return
		}

		if err := pat.Validate(values, overrides, cellOverrides); err != nil {
			writePatternValidationError(w, "expand_pattern", name, err)
			return
		}

		// Build a minimal expand context. Callers can optionally provide theme
		// info, but we use sensible defaults (standard 10×7.5 in slide).
		ctx := patterns.ExpandContext{
			SlideWidth:  9144000, // 10 inches in EMU
			SlideHeight: 6858000, // 7.5 inches in EMU
			LayoutBounds: patterns.LayoutBounds{
				X:      457200,  // 0.5 inch margin
				Y:      1371600, // 1.5 inch top (title area)
				Width:  8229600, // 9 inches
				Height: 5029200, // 5.5 inches
			},
		}

		grid, err := pat.Expand(ctx, values, overrides, cellOverrides)
		if err != nil {
			writeError(w, http.StatusInternalServerError, apierrors.CodePatternExpandFailed,
				err.Error(), map[string]any{"pattern": name})
			return
		}

		writeJSON(w, http.StatusOK, patternExpandResponse{ShapeGrid: grid})
	}
}

// ---------------------------------------------------------------------------
// Request / response types
// ---------------------------------------------------------------------------

type patternListItem struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	UseWhen         string `json:"use_when"`
	NotWhen         string `json:"not_when"`
	Version         int    `json:"version"`
	CellsHint       string `json:"cells_hint,omitempty"`
	SupportsCallout bool   `json:"supports_callout"`
}

type patternListResponse struct {
	Patterns []patternListItem `json:"patterns"`
}

type patternShowResponse struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	UseWhen         string          `json:"use_when"`
	NotWhen         string          `json:"not_when"`
	Version         int             `json:"version"`
	CellsHint       string          `json:"cells_hint,omitempty"`
	SupportsCallout bool            `json:"supports_callout"`
	Schema          json.RawMessage `json:"schema"`
}

type patternValidateResponse struct {
	OK bool `json:"ok"`
}

type patternExpandResponse struct {
	ShapeGrid *jsonschema.ShapeGridInput `json:"shape_grid"`
}

type patternRequestBody struct {
	Values        json.RawMessage            `json:"values"`
	Overrides     json.RawMessage            `json:"overrides,omitempty"`
	CellOverrides map[string]json.RawMessage `json:"cell_overrides,omitempty"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// decodeJSONBody validates Content-Type, limits body size, and decodes JSON into
// dst. It writes an appropriate error response and returns a non-nil error if any
// check fails; the caller should return without writing further.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	// Validate Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		mediaType := strings.TrimSpace(strings.Split(contentType, ";")[0])
		if mediaType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, apierrors.CodeInvalidContentType,
				"Content-Type must be application/json", nil)
			return fmt.Errorf("invalid content type")
		}
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if err.Error() == "http: request body too large" {
			writeError(w, http.StatusRequestEntityTooLarge, apierrors.CodeRequestTooLarge,
				fmt.Sprintf("Request body exceeds maximum size of %d bytes", MaxRequestBodySize), nil)
			return err
		}
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidJSON,
			"Failed to parse request body", nil)
		return err
	}
	return nil
}

// writePatternValidationError converts a (possibly joined) validation error into
// the shared diagnostics.FindingEnvelope and writes it as the HTTP response body.
// Pattern validation is the one diagnostic-bearing HTTP serve-mode endpoint, so it
// emits the same agent-facing FindingEnvelope contract as the CLI/MCP surfaces
// (built via diagnostics.BuildEnvelope). Transport errors on the pattern endpoints
// (404 not-found, 415 content-type, 413 too-large, 500 expand-failed) keep the
// simple apierrors.Response shape. The originating pattern name rides on every
// finding's evidence so an agent can correlate the failure without a separate
// top-level field. subcommand names the operation ("validate_pattern" or
// "expand_pattern") in the envelope.
func writePatternValidationError(w http.ResponseWriter, subcommand, patternName string, err error) {
	ds := diagnostics.FromJoinedError(err, diagnostics.CodeValidationFailed)
	// Ensure the pattern name is carried on every diagnostic's evidence, even for
	// the plain (non-ValidationError) inputs where FromValidationError did not set
	// it (e.g. "values field is required").
	for i := range ds {
		if ds[i].Details == nil {
			ds[i].Details = map[string]any{}
		}
		if _, ok := ds[i].Details["pattern"]; !ok {
			ds[i].Details["pattern"] = patternName
		}
	}
	env := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand: subcommand,
	}, ds)
	writeFindingEnvelope(w, http.StatusBadRequest, env)
}

// writeFindingEnvelope writes a diagnostics.FindingEnvelope as the response body
// for a diagnostic-bearing endpoint. Unlike writeError (the transport-error
// apierrors.Response shape), this is the shared agent-facing FindingEnvelope
// contract documented in docs/AGENT_DIAGNOSTICS.md.
func writeFindingEnvelope(w http.ResponseWriter, status int, env diagnostics.FindingEnvelope) {
	writeJSON(w, status, env)
}

// unmarshalPatternInputs deserializes the raw JSON fields from the request body
// into the typed structs expected by the pattern. This mirrors the logic in
// cmd/json2pptx/pattern_resolve.go expandPattern.
func unmarshalPatternInputs(pat patterns.Pattern, body *patternRequestBody) (values, overrides any, cellOverrides map[int]any, err error) {
	if len(body.Values) == 0 {
		return nil, nil, nil, fmt.Errorf("values field is required")
	}

	values = pat.NewValues()
	if err := json.Unmarshal(body.Values, values); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid values: %w", err)
	}

	if len(body.Overrides) > 0 {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal(body.Overrides, overrides); err != nil {
				return nil, nil, nil, fmt.Errorf("invalid overrides: %w", err)
			}
		}
	}

	if len(body.CellOverrides) > 0 {
		cellOverrides = make(map[int]any, len(body.CellOverrides))
		for key, raw := range body.CellOverrides {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("cell_overrides key %q is not an integer", key)
			}
			co := pat.NewCellOverride()
			if co == nil {
				return nil, nil, nil, fmt.Errorf("pattern does not support cell_overrides")
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return nil, nil, nil, fmt.Errorf("invalid cell_overrides[%d]: %w", idx, err)
			}
			cellOverrides[idx] = co
		}
	}

	return values, overrides, cellOverrides, nil
}
