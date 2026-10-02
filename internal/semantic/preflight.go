package semantic

// This file implements the post-compile raw preflight: after the compiler
// lowers a semantic DeckSpec to a raw PresentationInput it runs the emitted
// patterns through the raw pattern correctness gate — the same gate the
// renderer applies in expandPattern — and maps every failure back to the
// semantic source path the author wrote. Running here means a deck that would
// otherwise only fail deep in render (e.g. a KPI value too long for kpi-2up)
// surfaces at compile/validate time, pointing at the author's field rather than
// at generated pattern JSON. The compiler treats preflight findings as blocking
// because emitting known-invalid raw JSON guarantees a later render failure.

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// preflightRawPatterns validates every emitted pattern slide against the raw
// pattern registry and returns the failures as error-severity diagnostics whose
// Path is the semantic source location (falling back to the raw path on a
// source-map miss). Each diagnostic retains its raw path and a recommended
// semantic edit under Details so the CLI/MCP/API surfaces can present the
// author a concrete fix. A nil input yields no findings.
func preflightRawPatterns(input *deckinput.PresentationInput, sm *SourceMap) []diagnostics.Diagnostic {
	if input == nil {
		return nil
	}
	var out []diagnostics.Diagnostic
	for oi := range input.Slides {
		slide := &input.Slides[oi]
		if slide.ShapeGrid != nil {
			// A pattern nested in a grid cell (a regions slide's stat or
			// timeline) passes the same gate at expansion, and its finding
			// belongs to the region that wrote it (go-slide-creator-fn2ka).
			out = append(out, preflightNestedPatterns(slide.ShapeGrid, fmt.Sprintf("slides[%d].shape_grid", oi), sm)...)
		}
		if slide.Pattern == nil {
			continue
		}
		err := deckinput.ValidatePattern(slide.Pattern, patterns.Default())
		if err == nil {
			continue
		}
		rawPrefix := fmt.Sprintf("slides[%d].pattern", oi)
		for _, d := range diagnostics.FromJoinedError(err, diagnostics.CodeInvalidSlide) {
			out = append(out, remapPreflightDiagnostic(d, rawPrefix, sm))
		}
	}
	return out
}

// preflightNestedPatterns validates every pattern nested in a grid cell (at
// any depth) and remaps each failure to the semantic source.
func preflightNestedPatterns(grid *deckinput.ShapeGridInput, prefix string, sm *SourceMap) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellPath := fmt.Sprintf("%s.rows[%d].cells[%d]", prefix, ri, ci)
			if len(cell.Pattern) > 0 {
				var p deckinput.PatternInput
				if err := json.Unmarshal(cell.Pattern, &p); err == nil {
					if verr := deckinput.ValidatePattern(&p, patterns.Default()); verr != nil {
						for _, d := range diagnostics.FromJoinedError(verr, diagnostics.CodeInvalidSlide) {
							out = append(out, remapPreflightDiagnostic(d, cellPath+".pattern", sm))
						}
					}
				}
			}
			if cell.Grid != nil {
				out = append(out, preflightNestedPatterns(cell.Grid, cellPath+".grid", sm)...)
			}
		}
	}
	return out
}

// remapPreflightDiagnostic rewrites a raw pattern-local diagnostic (whose Path
// is a pattern-relative pointer like "values[0].big") so it points at the
// semantic source. It joins the diagnostic path under the slide's raw pattern
// prefix, resolves that through the source map (nearest-ancestor), and rewrites
// Path to the semantic location while retaining the raw path and a recommended
// semantic edit in Details. The severity is forced to error.
func remapPreflightDiagnostic(d diagnostics.Diagnostic, rawPrefix string, sm *SourceMap) diagnostics.Diagnostic {
	rawPath := rawPrefix
	if d.Path != "" {
		rawPath = rawPrefix + "." + d.Path
	}
	mapped := MapFinding(sm, RawFinding{
		Code:     d.Code,
		Message:  d.Message,
		Severity: diagnostics.SeverityError,
		RawPath:  rawPath,
	})

	if d.Details == nil {
		d.Details = map[string]any{}
	}
	d.Details["raw_path"] = rawPath
	if mapped.Edit != nil {
		d.Details["recommended_edit"] = mapped.Edit
	}
	if mapped.SemanticPath != "" {
		d.Path = mapped.SemanticPath
	} else {
		d.Path = rawPath
	}
	d.Severity = diagnostics.SeverityError
	return d
}
