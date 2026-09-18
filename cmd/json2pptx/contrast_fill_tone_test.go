package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

var tintTestTheme = []types.ThemeColor{
	{Name: "dk1", RGB: "#000000"},
	{Name: "lt1", RGB: "#FFFFFF"},
	{Name: "accent1", RGB: "#2E5090"},
}

func TestEffectiveShapeFillColor(t *testing.T) {
	cases := []struct {
		fill string
		want string
	}{
		{`"accent1"`, "accent1"},
		{`"none"`, ""},
		{`{"color":"accent1"}`, "accent1"},
		{`{"color":"accent1","lumMod":20000,"lumOff":80000}`, "#"},       // composed light tint
		{`{"color":"unknown","lumMod":20000,"lumOff":80000}`, "unknown"}, // unresolvable → base
	}
	for _, tc := range cases {
		got := effectiveShapeFillColor(json.RawMessage(tc.fill), tintTestTheme)
		if tc.want == "#" {
			if !strings.HasPrefix(got, "#") || strings.EqualFold(got, "#2E5090") {
				t.Errorf("%s: got %q, want a composed tint hex", tc.fill, got)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.fill, got, tc.want)
		}
	}
	if got := effectiveShapeFillColor(json.RawMessage(`{"color":"accent1","alpha":50}`), tintTestTheme); !strings.HasPrefix(got, "#") {
		t.Errorf("alpha fill should compose over lt1, got %q", got)
	}
}

// Dark text on a light accent tint is readable; the contrast preflight must
// judge the tint, not the untinted accent (which made it predict a bogus
// auto-replacement to white text).
func TestContrastPreflight_TintedFillNotFlagged(t *testing.T) {
	grid := &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{
		Shape: &ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`{"color":"accent1","lumMod":40000,"lumOff":60000}`),
			Text:     json.RawMessage(`{"paragraphs":[{"content":"Recommended option","color":"dk1"}]}`),
		},
	}}}}}
	in := &PresentationInput{Slides: []SlideInput{{ShapeGrid: grid}}}
	if f := collectContrastPreflightFindings(in, tintTestTheme); len(f) != 0 {
		t.Errorf("dk1 on a light accent tint should not be flagged, got %+v", f)
	}

	grid.Rows[0].Cells[0].Shape.Fill = json.RawMessage(`"accent1"`)
	if f := collectContrastPreflightFindings(in, tintTestTheme); len(f) == 0 {
		t.Error("dk1 on solid accent1 should still be flagged")
	}
}
