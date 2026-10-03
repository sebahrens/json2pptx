package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// regionDiagramValidationDiagnostics checks one diagram placed in a region (a
// shape_grid cell, composite sub_diagram, or compose segment) the way
// generate draws it, and reports as errors what generate aborts on:
//
//   - a type no renderer owns (go-slide-creator-hdx2l);
//   - a native type whose data carries a key its builder does not read, or
//     whose labels all parse empty (go-slide-creator-hdx2l);
//   - native heatmap grid-shape problems (go-slide-creator-csclk.18);
//   - an svggen type whose data carries a key its renderer does not read, at
//     any level (go-slide-creator-x9s5i);
//   - svggen data validation failures, via DryRender (go-slide-creator-yzbo).
//
// label names the region's owner ("shape_grid", "compose", "pattern X");
// where locates the diagram inside it ("row 1 cell 2 diagram").
func regionDiagramValidationDiagnostics(spec *types.DiagramSpec, slideIdx int, label, path, where string) []diagnostics.Diagnostic {
	if spec == nil || spec.Type == "" {
		return nil
	}
	prefix := fmt.Sprintf("slide %d: %s: %s", slideIdx+1, label, where)
	switch generator.DiagramGridPipeline(spec) {
	case "native_ooxml":
		var out []diagnostics.Diagnostic
		if spec.Type == "heatmap" {
			if err := generator.ValidateHeatmapData(spec.Data); err != nil {
				out = append(out, diagnostics.Diagnostic{
					Code:     diagnostics.CodeInvalidGrid,
					Path:     path,
					Message:  fmt.Sprintf("%s: %v", prefix, err),
					Severity: diagnostics.SeverityError,
				})
			}
		}
		return append(out, nativeDiagramDataDiagnostics(spec, path, prefix)...)
	case "svg":
		// DryRender runs the same request validation + layout pass as the
		// generate path, so the error text matches what generate reports.
		if _, err := svggen.DryRender(&svggen.RequestEnvelope{Type: spec.Type, Title: spec.Title, Data: spec.Data}); err != nil {
			// A key the renderer never reads is reported at the key with a
			// did-you-mean, the way native diagrams are (go-slide-creator-x9s5i).
			if dd := svggenUnknownKeyDiagnostics(spec, err, path, prefix); len(dd) > 0 {
				return dd
			}
			return []diagnostics.Diagnostic{{
				Code:     diagnostics.CodeInvalidGrid,
				Path:     path,
				Message:  fmt.Sprintf("%s: %v (generate would abort)", prefix, err),
				Severity: diagnostics.SeverityError,
			}}
		}
		return nil
	default:
		// No renderer owns the type. In a region generate aborts the deck
		// with svggen's "unknown diagram type"; validate used to skip it on
		// the assumption another check caught it, and none did.
		available := regionDiagramTypes()
		fix := &diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"available": available}}
		msg := fmt.Sprintf("%s: unknown diagram type %q (generate would abort)", prefix, spec.Type)
		if match, _ := generator.ClosestMatch(spec.Type, available, 3); match != "" {
			fix.Params["did_you_mean"] = match
			msg += fmt.Sprintf("; did you mean %q?", match)
		}
		return []diagnostics.Diagnostic{{
			Code:     diagnostics.CodeUnknownEnum,
			Path:     slidepath.Field(path, "type"),
			Message:  msg,
			Severity: diagnostics.SeverityError,
			Fix:      fix,
		}}
	}
}

// regionDiagramTypes lists the diagram types a region can draw — the native
// set and the svggen registry's canonical types — sorted.
func regionDiagramTypes() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range append(generator.NativeDiagramTypeNames(), svggen.DefaultRegistry().Types()...) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// nativeDiagramDataDiagnostics turns the native data-contract problems of a
// diagram into blocking diagnostics at the offending key: unknown_key with a
// rename_field fix when there is a did-you-mean, otherwise a use_one_of fix
// listing the keys the builder reads. path is the diagram's JSON Pointer.
func nativeDiagramDataDiagnostics(spec *types.DiagramSpec, path, prefix string) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, e := range generator.NativeDiagramDataErrors(spec) {
		d := diagnostics.Diagnostic{
			Code:     patterns.ErrCodeUnknownKey,
			Path:     slidepath.Field(path, e.Field),
			Message:  fmt.Sprintf("%s: %v (generate would refuse this deck)", prefix, e),
			Severity: diagnostics.SeverityError,
		}
		switch {
		case e.Key == "":
			d.Code = string(diagnostics.CodeInvalidSlide)
			d.Fix = &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "data", "diagram_type": spec.Type}}
		case e.DidYouMean != "":
			d.Fix = &diagnostics.Fix{Kind: "rename_field", Params: map[string]any{"from": e.Key, "to": e.DidYouMean, "accepted": e.Expected}}
		default:
			d.Fix = &diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"available": e.Expected}}
		}
		out = append(out, d)
	}
	return out
}

// composeDiagramValidationDiagnostics checks every diagram segment of a
// compose envelope, nested envelopes included. A segment is a region: generate
// draws it through the shape_grid dispatch and aborts on the same failures, so
// validate reports them at the segment's own path (go-slide-creator-hdx2l).
func composeDiagramValidationDiagnostics(c *ComposeInput, slideIdx int, basePath string) []diagnostics.Diagnostic {
	if c == nil {
		return nil
	}
	var out []diagnostics.Diagnostic
	for si := range c.Segments {
		seg := &c.Segments[si]
		segPath := fmt.Sprintf("%s/segments/%d", basePath, si)
		if seg.Diagram != nil {
			out = append(out, regionDiagramValidationDiagnostics(seg.Diagram, slideIdx, "compose",
				segPath+"/diagram", fmt.Sprintf("segment %d diagram", si+1))...)
		}
		if seg.Compose != nil {
			out = append(out, composeDiagramValidationDiagnostics(seg.Compose, slideIdx, segPath+"/compose")...)
		}
	}
	return out
}

// svggenUnknownKeyDiagnostics turns the UNKNOWN_FIELD errors of an svggen
// data-contract failure into blocking unknown_key diagnostics at the key's
// JSON Pointer, with the fixes native diagrams use: rename_field when there is
// a did-you-mean, otherwise use_one_of. It returns nil when err carries no
// UNKNOWN_FIELD error.
func svggenUnknownKeyDiagnostics(spec *types.DiagramSpec, err error, path, prefix string) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	for _, ve := range svggen.GetValidationErrors(err) {
		if ve.Code != svggen.ErrCodeUnknownField {
			continue
		}
		key, _ := ve.Value.(string)
		d := diagnostics.Diagnostic{
			Code:     patterns.ErrCodeUnknownKey,
			Path:     slidepath.Field(path, ve.Field),
			Message:  fmt.Sprintf("%s: %s: %s: %s (generate would refuse this deck)", prefix, spec.Type, ve.Field, ve.Message),
			Severity: diagnostics.SeverityError,
		}
		if ve.DidYouMean != "" {
			d.Fix = &diagnostics.Fix{Kind: "rename_field", Params: map[string]any{"from": key, "to": ve.DidYouMean, "accepted": ve.Expected}}
		} else {
			d.Fix = &diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"available": ve.Expected}}
		}
		out = append(out, d)
	}
	return out
}

// svggenDataContractDiagnostics checks an svggen diagram or chart placed in a
// body placeholder against its type's data contract. Rendered there, a
// contract failure would only degrade to a "Data unavailable" image, so
// generate refuses it up front and validate reports it as an error:
// unknown_key at the key, or a payload whose labels all parse empty
// (go-slide-creator-x9s5i).
func svggenDataContractDiagnostics(spec *types.DiagramSpec, path, prefix string) []diagnostics.Diagnostic {
	err := svggen.CheckDataContract(spec.Type, spec.Data)
	if err == nil {
		return nil
	}
	if dd := svggenUnknownKeyDiagnostics(spec, err, path, prefix); len(dd) > 0 {
		return dd
	}
	return []diagnostics.Diagnostic{{
		Code:     string(diagnostics.CodeInvalidSlide),
		Path:     slidepath.Field(path, "data"),
		Message:  fmt.Sprintf("%s: %v (generate would refuse this deck)", prefix, err),
		Severity: diagnostics.SeverityError,
		Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "data", "diagram_type": spec.Type}},
	}}
}

// contentDiagramValidationDiagnostics checks a diagram or chart placed in a
// body placeholder against its type's data contract. generate refuses a
// native diagram whose data would draw empty labels, and an svggen diagram or
// chart whose data carries a key its renderer does not read, so validate does
// too (go-slide-creator-hdx2l, go-slide-creator-x9s5i). Other svggen failures
// keep their existing checks: they degrade to a placeholder image and are
// predicted by the fit report, not refused.
func contentDiagramValidationDiagnostics(item ContentInput, slideIdx, contentIdx int) []diagnostics.Diagnostic {
	spec := contentDiagramSpec(item)
	if spec == nil {
		return nil
	}
	field := item.Type + "_value"
	if item.UsesLegacyValue() {
		field = "value"
	}
	path := slidepath.ContentField(slideIdx, contentIdx, field)
	prefix := fmt.Sprintf("slide %d, content %d", slideIdx+1, contentIdx+1)
	if generator.IsNativeDiagramType(spec) {
		return nativeDiagramDataDiagnostics(spec, path, prefix)
	}
	return svggenDataContractDiagnostics(spec, path, prefix)
}

// contentDiagramSpec returns the diagram spec a chart or diagram content item
// draws, from the typed field or the legacy value, or nil.
func contentDiagramSpec(item ContentInput) *types.DiagramSpec {
	if item.Type != "diagram" && item.Type != "chart" {
		return nil
	}
	resolved, err := item.ResolveValue()
	if err != nil {
		return nil // parse errors are reported elsewhere
	}
	switch v := resolved.(type) {
	case *types.DiagramSpec:
		return v
	case *types.ChartSpec: //nolint:staticcheck // backward compat
		if v != nil {
			return v.ToDiagramSpec()
		}
		return nil
	}
	if len(item.Value) == 0 {
		return nil
	}
	if item.Type == "chart" {
		var chart types.ChartSpec //nolint:staticcheck // backward compat
		if json.Unmarshal(item.Value, &chart) != nil || chart.Type == "" {
			return nil
		}
		return chart.ToDiagramSpec()
	}
	var diagram types.DiagramSpec
	if json.Unmarshal(item.Value, &diagram) != nil || diagram.Type == "" {
		return nil
	}
	return &diagram
}
