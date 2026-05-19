package pipeline

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// gridWithFills constructs a single-row ShapeGrid whose cells use the
// supplied fill values in order. Empty strings produce cells without fills.
func gridWithFills(fills ...string) *jsonschema.ShapeGridInput {
	cells := make([]*jsonschema.GridCellInput, 0, len(fills))
	for _, f := range fills {
		c := &jsonschema.GridCellInput{}
		if f != "" {
			c.Shape = &jsonschema.ShapeSpecInput{
				Fill: json.RawMessage(`"` + f + `"`),
			}
		}
		cells = append(cells, c)
	}
	return &jsonschema.ShapeGridInput{
		Rows: []jsonschema.GridRowInput{{Cells: cells}},
	}
}

func TestDetectAccentOverload(t *testing.T) {
	cases := []struct {
		name       string
		fills      []string
		wantFinding bool
	}{
		{
			name:       "one accent — no finding",
			fills:      []string{"accent1", "accent1", "accent1"},
			wantFinding: false,
		},
		{
			name:       "two accents — no finding (paired comparison ok)",
			fills:      []string{"accent1", "accent2", "accent1"},
			wantFinding: false,
		},
		{
			name:       "three distinct accents — finding",
			fills:      []string{"accent1", "accent2", "accent3"},
			wantFinding: true,
		},
		{
			name:       "four distinct accents — finding",
			fills:      []string{"accent1", "accent2", "accent3", "accent4"},
			wantFinding: true,
		},
		{
			name:       "accent + neutrals — no finding",
			fills:      []string{"accent1", "lt2", "dk1"},
			wantFinding: false,
		},
		{
			name:       "hex fills ignored — no finding",
			fills:      []string{"#FF0000", "#00FF00", "#0000FF"},
			wantFinding: false,
		},
		{
			name:       "mixed accent + hex — only accent hues count",
			fills:      []string{"accent1", "#FF0000", "accent2"},
			wantFinding: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			grid := gridWithFills(tc.fills...)
			findings := detectAccentOverload(grid, 3)

			var found bool
			for _, f := range findings {
				if errors.Is(f, patterns.ErrAccentOverload) || f.Code == patterns.ErrCodeAccentOverload {
					found = true
					break
				}
			}
			if found != tc.wantFinding {
				t.Errorf("accent_overload: got=%v want=%v findings=%+v", found, tc.wantFinding, findings)
			}

			if tc.wantFinding && len(findings) > 0 {
				msg := findings[0].Message
				if !strings.Contains(msg, "slide 4") {
					t.Errorf("expected slide number in message; got %q", msg)
				}
				if !strings.Contains(msg, "cell_accent_mode") {
					t.Errorf("expected fix guidance referencing cell_accent_mode; got %q", msg)
				}
			}
		})
	}
}

// TestDetectAccentOverloadObjectFill verifies that object-form fills
// (with tint/shade modifiers) are extracted correctly so a tinted accent
// counts as the same hue as the bare scheme name.
func TestDetectAccentOverloadObjectFill(t *testing.T) {
	grid := &jsonschema.ShapeGridInput{
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{
				{Shape: &jsonschema.ShapeSpecInput{
					Fill: json.RawMessage(`{"color":"accent1","lumMod":75000}`),
				}},
				{Shape: &jsonschema.ShapeSpecInput{
					Fill: json.RawMessage(`{"color":"accent1"}`),
				}},
				{Shape: &jsonschema.ShapeSpecInput{
					Fill: json.RawMessage(`"accent1"`),
				}},
			},
		}},
	}

	findings := detectAccentOverload(grid, 0)
	if len(findings) != 0 {
		t.Errorf("expected no finding for one tinted accent; got %d: %+v", len(findings), findings)
	}
}
