package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidJSON,
				"Failed to parse request body", nil)
			return
		}

		values, overrides, cellOverrides, err := unmarshalPatternInputs(pat, &body)
		if err != nil {
			writePatternValidationError(w, name, err)
			return
		}

		if err := pat.Validate(values, overrides, cellOverrides); err != nil {
			writePatternValidationError(w, name, err)
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
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, apierrors.CodeInvalidJSON,
				"Failed to parse request body", nil)
			return
		}

		values, overrides, cellOverrides, err := unmarshalPatternInputs(pat, &body)
		if err != nil {
			writePatternValidationError(w, name, err)
			return
		}

		if err := pat.Validate(values, overrides, cellOverrides); err != nil {
			writePatternValidationError(w, name, err)
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
	Version         int    `json:"version"`
	CellsHint       string `json:"cells_hint,omitempty"`
	SupportsCallout bool   `json:"supports_callout"`
}

type patternListResponse struct {
	Patterns []patternListItem `json:"patterns"`
}

type patternShowResponse struct {
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	UseWhen         string           `json:"use_when"`
	Version         int              `json:"version"`
	CellsHint       string           `json:"cells_hint,omitempty"`
	SupportsCallout bool             `json:"supports_callout"`
	Schema          json.RawMessage  `json:"schema"`
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

// writePatternValidationError converts a (possibly joined) validation error into
// structured diagnostics and writes an HTTP error response with the individual
// entries in the details map. This mirrors what MCP does via splitValidationErrors
// so agents get the same machine-readable shape from both transports.
func writePatternValidationError(w http.ResponseWriter, patternName string, err error) {
	ds := diagnostics.FromJoinedError(err, "validation_failed")
	entries := make([]map[string]any, len(ds))
	for i, d := range ds {
		entry := map[string]any{
			"code":    d.Code,
			"message": d.Message,
		}
		if d.Path != "" {
			entry["path"] = d.Path
		}
		if d.Fix != nil {
			entry["fix"] = d.Fix
		}
		entries[i] = entry
	}
	writeError(w, http.StatusBadRequest, apierrors.CodePatternValidationFailed,
		diagnostics.Summary(ds), map[string]any{
			"pattern":           patternName,
			"validation_errors": entries,
		})
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
