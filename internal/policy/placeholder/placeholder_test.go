package placeholder

import (
	"testing"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"bare token", "__FILL__", true},
		{"token embedded in text", "Q3 __FILL__ results", true},
		{"clean string", "Quarterly results", false},
		{"empty string", "", false},
		{"lookalike but not token", "FILL", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Contains(tc.s); got != tc.want {
				t.Errorf("Contains(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

// TestScanCoversAllUserVisibleSurfaces asserts that the JSON-based walk catches
// the token in plain text, shape_grid cell text, table cells, chart labels, and
// speaker notes — the surfaces called out in the acceptance criteria — in a
// single pass, with stable path ordering.
func TestScanCoversAllUserVisibleSurfaces(t *testing.T) {
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id":     "title",
				"speaker_notes": "remember to mention __FILL__",
				"content": []any{
					map[string]any{
						"placeholder_id": "title",
						"type":           "text",
						"text_value":     "__FILL__",
					},
					map[string]any{
						"type": "table",
						"table": map[string]any{
							"rows": []any{
								[]any{"Header", "__FILL__"},
							},
						},
					},
					map[string]any{
						"type": "chart",
						"chart": map[string]any{
							"labels": []any{"Q1", "__FILL__"},
						},
					},
				},
				"shape_grid": map[string]any{
					"rows": []any{
						map[string]any{
							"cells": []any{
								map[string]any{
									"shape": map[string]any{"text": "__FILL__"},
								},
							},
						},
					},
				},
			},
		},
	}

	got := Scan(deck)
	if len(got) != 5 {
		t.Fatalf("Scan found %d violations, want 5:\n%+v", len(got), got)
	}

	// Every violation must carry a non-empty path and the token.
	for i, v := range got {
		if v.Path == "" {
			t.Errorf("violation[%d] has empty path: %+v", i, v)
		}
		if v.Token != Token {
			t.Errorf("violation[%d] token = %q, want %q", i, v.Token, Token)
		}
		if !Contains(v.Value) {
			t.Errorf("violation[%d] value %q does not contain the token", i, v.Value)
		}
	}

	// Stable, sorted-by-path order.
	for i := 1; i < len(got); i++ {
		if got[i-1].Path > got[i].Path {
			t.Errorf("violations not sorted by path: %q before %q", got[i-1].Path, got[i].Path)
		}
	}

	// Spot-check that the specific surfaces are represented.
	wantPaths := map[string]bool{
		"slides[0].content[0].text_value":                  false,
		"slides[0].content[1].table.rows[0][1]":            false,
		"slides[0].content[2].chart.labels[1]":             false,
		"slides[0].shape_grid.rows[0].cells[0].shape.text": false,
		"slides[0].speaker_notes":                          false,
	}
	for _, v := range got {
		if _, ok := wantPaths[v.Path]; ok {
			wantPaths[v.Path] = true
		}
	}
	for p, seen := range wantPaths {
		if !seen {
			t.Errorf("expected a violation at path %q, got none", p)
		}
	}
}

func TestScanCleanDeckReturnsNothing(t *testing.T) {
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{
				"layout_id": "title",
				"content": []any{
					map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Real Title"},
				},
			},
		},
	}
	if got := Scan(deck); len(got) != 0 {
		t.Errorf("Scan of clean deck returned %d violations, want 0:\n%+v", len(got), got)
	}
}

func TestScanNilReturnsNothing(t *testing.T) {
	if got := Scan(nil); got != nil {
		t.Errorf("Scan(nil) = %+v, want nil", got)
	}
}

func TestDisplayPath(t *testing.T) {
	if got := DisplayPath(""); got != "<root>" {
		t.Errorf("DisplayPath(\"\") = %q, want \"<root>\"", got)
	}
	if got := DisplayPath("slides[0].content[0]"); got != "slides[0].content[0]" {
		t.Errorf("DisplayPath round-trip failed: %q", got)
	}
}
