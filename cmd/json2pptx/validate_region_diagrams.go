package main

import (
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

// contentDiagramValidationDiagnostics checks a native diagram placed in a body
// placeholder. generate refuses one whose data would draw empty labels, so
// validate does too (go-slide-creator-hdx2l). Other content diagrams keep
// their existing checks: an svggen failure there degrades to a placeholder
// image and is predicted by the fit report, not refused.
func contentDiagramValidationDiagnostics(item ContentInput, slideIdx, contentIdx int) []diagnostics.Diagnostic {
	if item.Type != "diagram" {
		return nil
	}
	resolved, err := item.ResolveValue()
	if err != nil {
		return nil // parse errors are reported elsewhere
	}
	spec, ok := resolved.(*types.DiagramSpec)
	if !ok || spec == nil {
		return nil
	}
	field := "diagram_value"
	if item.UsesLegacyValue() {
		field = "value"
	}
	return nativeDiagramDataDiagnostics(spec, slidepath.ContentField(slideIdx, contentIdx, field),
		fmt.Sprintf("slide %d, content %d", slideIdx+1, contentIdx+1))
}
