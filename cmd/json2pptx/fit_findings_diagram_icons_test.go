package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/types"
)

// panelIconDeck builds a one-slide deck whose panel_layout carries the given
// icon references.
func panelIconDeck(icons ...string) *PresentationInput {
	panels := make([]any, len(icons))
	for i, ic := range icons {
		p := map[string]any{"title": "Panel", "body": "Body."}
		if ic != "" {
			p["icon"] = ic
		}
		panels[i] = p
	}
	return &PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			LayoutID:  "content",
			SlideType: "content",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Moves")},
				{PlaceholderID: "body", Type: "diagram", DiagramValue: &types.DiagramSpec{
					Type: "panel_layout",
					Data: map[string]any{"panels": panels},
				}},
			},
		}},
	}
}

// iconFindings returns the unknown-icon findings for a deck.
func iconFindings(in *PresentationInput) []string {
	var out []string
	for _, f := range collectDiagramIconFindings(in) {
		if f.Code == string(diagnostics.CodeIconBundledNameUnknown) {
			out = append(out, f.Path)
		}
	}
	return out
}

// TestUnknownPanelIconIsReported is go-slide-creator-puki: the panel rendered
// without its icon while its siblings kept theirs, and the only trace was a
// server log line.
func TestUnknownPanelIconIsReported(t *testing.T) {
	got := collectDiagramIconFindings(panelIconDeck("layers", "rocket", "chart-bar"))
	if len(got) != 1 {
		t.Fatalf("expected exactly one finding for the bad name, got %d: %+v", len(got), got)
	}
	f := got[0]
	if f.Code != string(diagnostics.CodeIconBundledNameUnknown) {
		t.Errorf("code = %q", f.Code)
	}
	if f.Path != "slides[0].content[1].diagram_value.data.panels[0].icon" {
		t.Errorf("path = %q, want the offending panel's icon", f.Path)
	}
	if f.Action != "review" {
		t.Errorf("action = %q, want review — the panel still renders, minus its icon", f.Action)
	}
	if f.Fix == nil || f.Fix.Kind != "replace_value" {
		t.Fatalf("fix = %+v, want replace_value", f.Fix)
	}
	if _, ok := f.Fix.Params["to"]; !ok {
		t.Errorf("fix should carry the nearest valid name: %+v", f.Fix.Params)
	}
	if !strings.Contains(f.Message, "siblings") {
		t.Errorf("message should say what the reader will see: %q", f.Message)
	}
}

// TestKnownAndNonNameIconsAreQuiet: valid names and every non-name reference
// (inline SVG, data URI, url, file path) must produce nothing.
func TestKnownAndNonNameIconsAreQuiet(t *testing.T) {
	cases := map[string]string{
		"bundled name":     "rocket",
		"qualified name":   "filled:camera",
		"inline svg":       `<svg viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg>`,
		"data uri":         "data:image/svg+xml;base64,PHN2Zy8+",
		"https url":        "https://example.com/icon.svg",
		"file path":        "assets/icon.svg",
		"empty (no icon)":  "",
		"whitespace only ": "   ",
	}
	for name, ref := range cases {
		t.Run(name, func(t *testing.T) {
			if got := iconFindings(panelIconDeck(ref)); len(got) != 0 {
				t.Errorf("%q produced %v", ref, got)
			}
		})
	}
}

// TestUnknownIconReachesTheFitReport: the collector is wired into the shared
// set, so validate / generate / score all carry it.
func TestUnknownIconReachesTheFitReport(t *testing.T) {
	in := panelIconDeck("layers")
	for _, f := range collectFitFindings(in, nil, 0, 0, nil) {
		if f.Code == string(diagnostics.CodeIconBundledNameUnknown) {
			return
		}
	}
	t.Error("the unknown icon name did not reach collectFitFindings")
}

// TestUnknownIconInAGridCellDiagram: the same typo inside a shape_grid cell's
// diagram is reported at the cell's own path.
func TestUnknownIconInAGridCellDiagram(t *testing.T) {
	grid := &ShapeGridInput{
		Columns: json.RawMessage(`1`),
		Rows: []GridRowInput{{Cells: []*GridCellInput{{
			Diagram: &types.DiagramSpec{
				Type: "panel_layout",
				Data: map[string]any{"panels": []any{map[string]any{"title": "A", "icon": "layers"}}},
			},
		}}}},
	}
	in := &PresentationInput{Template: "midnight-blue", Slides: []SlideInput{{SlideType: "content", ShapeGrid: grid}}}
	got := iconFindings(in)
	if len(got) != 1 || !strings.Contains(got[0], "/rows/0/cells/0/diagram") {
		t.Errorf("grid cell diagram icon findings = %v", got)
	}
}
