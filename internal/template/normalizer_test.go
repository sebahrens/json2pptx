package template

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestNormalizePlaceholderNames(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name           string
		shapes         []shapeXML
		wantRenames    []PlaceholderRename
		wantTypeFixes  int
		wantWarnings   int
		wantShapeNames []string // expected names after normalization, in order
	}{
		{
			name: "title and body renamed",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
						Placeholder:         &placeholderXML{Type: "title"},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Content Placeholder 2"},
						Placeholder:         &placeholderXML{Type: "body", Index: intPtr(1)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 100, Y: 200},
							Extents: &extentsXML{CX: 5000, CY: 3000},
						},
					},
				},
			},
			wantRenames: []PlaceholderRename{
				{OldName: "Title 1", NewName: "title"},
				{OldName: "Content Placeholder 2", NewName: "body"},
			},
			wantShapeNames: []string{"title", "body"},
		},
		{
			name: "implicit body gets type injected and renamed",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Text Placeholder 15"},
						Placeholder:         &placeholderXML{Index: intPtr(15)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 100, Y: 200},
							Extents: &extentsXML{CX: 5000, CY: 3000},
						},
					},
				},
			},
			wantRenames: []PlaceholderRename{
				{OldName: "Text Placeholder 15", NewName: "body"},
			},
			wantTypeFixes:  1,
			wantShapeNames: []string{"body"},
		},
		{
			name: "multiple bodies sorted by X position",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Right Body"},
						Placeholder:         &placeholderXML{Type: "body", Index: intPtr(2)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 5000, Y: 100},
							Extents: &extentsXML{CX: 4000, CY: 3000},
						},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Left Body"},
						Placeholder:         &placeholderXML{Type: "body", Index: intPtr(1)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 100, Y: 100},
							Extents: &extentsXML{CX: 4000, CY: 3000},
						},
					},
				},
			},
			wantRenames: []PlaceholderRename{
				{OldName: "Left Body", NewName: "body"},
				{OldName: "Right Body", NewName: "body_2"},
			},
			wantShapeNames: []string{"body_2", "body"}, // index order, not sort order
		},
		{
			name: "pic placeholders become image",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Picture Placeholder 10"},
						Placeholder:         &placeholderXML{Type: "pic", Index: intPtr(10)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 100, Y: 200},
							Extents: &extentsXML{CX: 5000, CY: 3000},
						},
					},
				},
			},
			wantRenames: []PlaceholderRename{
				{OldName: "Picture Placeholder 10", NewName: "image"},
			},
			wantShapeNames: []string{"image"},
		},
		{
			name: "ctrTitle becomes title",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
						Placeholder:         &placeholderXML{Type: "ctrTitle"},
					},
				},
			},
			wantRenames: []PlaceholderRename{
				{OldName: "Title 1", NewName: "title"},
			},
			wantShapeNames: []string{"title"},
		},
		{
			name: "subTitle becomes subtitle",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Subtitle 2"},
						Placeholder:         &placeholderXML{Type: "subTitle"},
					},
				},
			},
			wantRenames: []PlaceholderRename{
				{OldName: "Subtitle 2", NewName: "subtitle"},
			},
			wantShapeNames: []string{"subtitle"},
		},
		{
			name: "utility placeholders preserved",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Date Placeholder 3"},
						Placeholder:         &placeholderXML{Type: "dt", Index: intPtr(10)},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Footer Placeholder 4"},
						Placeholder:         &placeholderXML{Type: "ftr", Index: intPtr(11)},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Slide Number 5"},
						Placeholder:         &placeholderXML{Type: "sldNum", Index: intPtr(12)},
					},
				},
			},
			wantRenames:    nil,
			wantShapeNames: []string{"Date Placeholder 3", "Footer Placeholder 4", "Slide Number 5"},
		},
		{
			name: "non-placeholder shapes untouched",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Decorative Line"},
					},
				},
			},
			wantRenames:    nil,
			wantShapeNames: []string{"Decorative Line"},
		},
		{
			name: "already canonical names not renamed",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "title"},
						Placeholder:         &placeholderXML{Type: "title"},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
						Placeholder:         &placeholderXML{Type: "body", Index: intPtr(1)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 100, Y: 200},
							Extents: &extentsXML{CX: 5000, CY: 3000},
						},
					},
				},
			},
			wantRenames:    nil,
			wantShapeNames: []string{"title", "body"},
		},
		{
			name: "empty shapes",
			shapes: []shapeXML{},
		},
		{
			name: "mixed layout: title + two bodies + footer",
			shapes: []shapeXML{
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
						Placeholder:         &placeholderXML{Type: "title"},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Content Placeholder 2"},
						Placeholder:         &placeholderXML{Type: "body", Index: intPtr(1)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 100, Y: 200},
							Extents: &extentsXML{CX: 4000, CY: 3000},
						},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Text Placeholder 3"},
						Placeholder:         &placeholderXML{Type: "body", Index: intPtr(2)},
					},
					ShapeProperties: shapePropertiesXML{
						Transform: &transformXML{
							Offset:  &offsetXML{X: 5000, Y: 200},
							Extents: &extentsXML{CX: 4000, CY: 3000},
						},
					},
				},
				{
					NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Footer Placeholder 4"},
						Placeholder:         &placeholderXML{Type: "ftr", Index: intPtr(11)},
					},
				},
			},
			wantRenames: []PlaceholderRename{
				{OldName: "Title 1", NewName: "title"},
				{OldName: "Content Placeholder 2", NewName: "body"},
				{OldName: "Text Placeholder 3", NewName: "body_2"},
			},
			wantShapeNames: []string{"title", "body", "body_2", "Footer Placeholder 4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePlaceholderNames(tt.shapes)

			// Check renames
			if tt.wantRenames == nil && len(result.Renames) > 0 {
				t.Errorf("expected no renames, got %d: %v", len(result.Renames), result.Renames)
			}
			if tt.wantRenames != nil {
				if len(result.Renames) != len(tt.wantRenames) {
					t.Errorf("rename count = %d, want %d", len(result.Renames), len(tt.wantRenames))
				} else {
					for i, want := range tt.wantRenames {
						got := result.Renames[i]
						if got.OldName != want.OldName || got.NewName != want.NewName {
							t.Errorf("rename[%d] = {%q→%q}, want {%q→%q}",
								i, got.OldName, got.NewName, want.OldName, want.NewName)
						}
					}
				}
			}

			// Check type fixes
			if tt.wantTypeFixes > 0 && len(result.TypeFixes) != tt.wantTypeFixes {
				t.Errorf("type fix count = %d, want %d", len(result.TypeFixes), tt.wantTypeFixes)
			}

			// Check warnings
			if tt.wantWarnings > 0 && len(result.Warnings) != tt.wantWarnings {
				t.Errorf("warning count = %d, want %d", len(result.Warnings), tt.wantWarnings)
			}

			// Check final shape names
			if tt.wantShapeNames != nil {
				for i, wantName := range tt.wantShapeNames {
					if i >= len(tt.shapes) {
						t.Errorf("shape index %d out of range", i)
						continue
					}
					gotName := tt.shapes[i].NonVisualProperties.ConnectionNonVisual.Name
					if gotName != wantName {
						t.Errorf("shape[%d].Name = %q, want %q", i, gotName, wantName)
					}
				}
			}
		})
	}
}

func TestApplyNormalizationToBytes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		result NormalizationResult
		want   string
	}{
		{
			name: "name rename",
			input: `<p:sp><p:nvSpPr><p:cNvPr id="3" name="Title 1"/></p:nvSpPr></p:sp>`,
			result: NormalizationResult{
				Renames: []PlaceholderRename{
					{OldName: "Title 1", NewName: "title"},
				},
			},
			want: `<p:sp><p:nvSpPr><p:cNvPr id="3" name="title"/></p:nvSpPr></p:sp>`,
		},
		{
			name: "type injection",
			input: `<p:nvPr><p:ph idx="1"/></p:nvPr>`,
			result: NormalizationResult{
				TypeFixes: []TypeInjection{
					{ShapeName: "body", PhIndex: 1},
				},
			},
			want: `<p:nvPr><p:ph type="body" idx="1"/></p:nvPr>`,
		},
		{
			name: "type injection skips existing type",
			input: `<p:nvPr><p:ph type="title" idx="0"/></p:nvPr>`,
			result: NormalizationResult{
				TypeFixes: []TypeInjection{
					{ShapeName: "title", PhIndex: 0},
				},
			},
			want: `<p:nvPr><p:ph type="title" idx="0"/></p:nvPr>`,
		},
		{
			name: "combined rename and type injection",
			input: `<p:sp><p:nvSpPr><p:cNvPr id="3" name="Text Placeholder 15"/><p:cNvSpPr/><p:nvPr><p:ph idx="15"/></p:nvPr></p:nvSpPr></p:sp>`,
			result: NormalizationResult{
				Renames: []PlaceholderRename{
					{OldName: "Text Placeholder 15", NewName: "body"},
				},
				TypeFixes: []TypeInjection{
					{ShapeName: "body", PhIndex: 15},
				},
			},
			want: `<p:sp><p:nvSpPr><p:cNvPr id="3" name="body"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="15"/></p:nvPr></p:nvSpPr></p:sp>`,
		},
		{
			name:   "no changes returns original",
			input:  `<p:sp><p:nvSpPr><p:cNvPr id="3" name="body"/></p:nvSpPr></p:sp>`,
			result: NormalizationResult{},
			want:   `<p:sp><p:nvSpPr><p:cNvPr id="3" name="body"/></p:nvSpPr></p:sp>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyNormalizationToBytes([]byte(tt.input), tt.result)
			if string(got) != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", string(got), tt.want)
			}
		})
	}
}

func TestUpdateLayoutPlaceholderIDs(t *testing.T) {
	layout := types.LayoutMetadata{
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle},
			{ID: "Content Placeholder 2", Type: types.PlaceholderContent},
			{ID: "Text Placeholder 3", Type: types.PlaceholderBody},
		},
	}

	result := NormalizationResult{
		Renames: []PlaceholderRename{
			{OldName: "Title 1", NewName: "title"},
			{OldName: "Content Placeholder 2", NewName: "body"},
		},
	}

	updateLayoutPlaceholderIDs(&layout, result)

	wantIDs := []string{"title", "body", "Text Placeholder 3"}
	for i, want := range wantIDs {
		if layout.Placeholders[i].ID != want {
			t.Errorf("placeholder[%d].ID = %q, want %q", i, layout.Placeholders[i].ID, want)
		}
	}

	// Content type should be updated to Body
	if layout.Placeholders[1].Type != types.PlaceholderBody {
		t.Errorf("placeholder[1].Type = %q, want %q", layout.Placeholders[1].Type, types.PlaceholderBody)
	}
}

func TestUpdateLayoutPlaceholderIDs_DuplicateNames(t *testing.T) {
	// Regression test: when multiple placeholders share the same original name
	// (e.g., three "Content Placeholder 2" in a three-column layout), each
	// must receive a distinct canonical name ("body", "body_2", "body_3").
	layout := types.LayoutMetadata{
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title Placeholder 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content Placeholder 2", Type: types.PlaceholderContent, Index: 1},
			{ID: "Content Placeholder 2", Type: types.PlaceholderContent, Index: 10},
			{ID: "Content Placeholder 2", Type: types.PlaceholderContent, Index: 11},
		},
	}

	result := NormalizationResult{
		Renames: []PlaceholderRename{
			{OldName: "Title Placeholder 1", NewName: "title"},
			{OldName: "Content Placeholder 2", NewName: "body"},
			{OldName: "Content Placeholder 2", NewName: "body_2"},
			{OldName: "Content Placeholder 2", NewName: "body_3"},
		},
	}

	updateLayoutPlaceholderIDs(&layout, result)

	wantIDs := []string{"title", "body", "body_2", "body_3"}
	for i, want := range wantIDs {
		if layout.Placeholders[i].ID != want {
			t.Errorf("placeholder[%d].ID = %q, want %q", i, layout.Placeholders[i].ID, want)
		}
	}
}

func TestNormalizationResult_HasChanges(t *testing.T) {
	tests := []struct {
		name   string
		result NormalizationResult
		want   bool
	}{
		{
			name:   "empty",
			result: NormalizationResult{},
			want:   false,
		},
		{
			name: "has renames",
			result: NormalizationResult{
				Renames: []PlaceholderRename{{OldName: "a", NewName: "b"}},
			},
			want: true,
		},
		{
			name: "has type fixes",
			result: NormalizationResult{
				TypeFixes: []TypeInjection{{ShapeName: "body", PhIndex: 1}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasChanges(); got != tt.want {
				t.Errorf("HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInjectPhTypeAttr(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		idx    int
		phType string
		want   string
	}{
		{
			name:   "basic injection",
			input:  `<p:ph idx="1"/>`,
			idx:    1,
			phType: "body",
			want:   `<p:ph type="body" idx="1"/>`,
		},
		{
			name:   "skips when type already present",
			input:  `<p:ph type="title" idx="0"/>`,
			idx:    0,
			phType: "body",
			want:   `<p:ph type="title" idx="0"/>`,
		},
		{
			name:   "handles idx not found",
			input:  `<p:ph idx="5"/>`,
			idx:    99,
			phType: "body",
			want:   `<p:ph idx="5"/>`,
		},
		{
			name:   "handles no p:ph element",
			input:  `<p:sp idx="1"/>`,
			idx:    1,
			phType: "body",
			want:   `<p:sp idx="1"/>`,
		},
		{
			name:   "preserves surrounding content",
			input:  `<before/><p:ph idx="2"/><after/>`,
			idx:    2,
			phType: "body",
			want:   `<before/><p:ph type="body" idx="2"/><after/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := injectPhTypeAttr([]byte(tt.input), tt.idx, tt.phType)
			if string(got) != tt.want {
				t.Errorf("got:  %s\nwant: %s", string(got), tt.want)
			}
		})
	}
}

func TestNormalizePlaceholderNames_BodySortOrder(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	// Three body placeholders at different X positions — should be sorted left-to-right
	shapes := []shapeXML{
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Rightmost"},
				Placeholder:         &placeholderXML{Type: "body", Index: intPtr(3)},
			},
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset:  &offsetXML{X: 9000, Y: 100},
					Extents: &extentsXML{CX: 2000, CY: 3000},
				},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Middle"},
				Placeholder:         &placeholderXML{Type: "body", Index: intPtr(2)},
			},
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset:  &offsetXML{X: 4000, Y: 100},
					Extents: &extentsXML{CX: 2000, CY: 3000},
				},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Leftmost"},
				Placeholder:         &placeholderXML{Type: "body", Index: intPtr(1)},
			},
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset:  &offsetXML{X: 100, Y: 100},
					Extents: &extentsXML{CX: 2000, CY: 3000},
				},
			},
		},
	}

	result := NormalizePlaceholderNames(shapes)

	// Leftmost (x=100) → body, Middle (x=4000) → body_2, Rightmost (x=9000) → body_3
	if len(result.Renames) != 3 {
		t.Fatalf("expected 3 renames, got %d: %v", len(result.Renames), result.Renames)
	}

	// Check shapes were renamed correctly
	wantNames := map[string]string{
		"Rightmost": "body_3",
		"Middle":    "body_2",
		"Leftmost":  "body",
	}

	// After normalization, verify in original slice order
	for i, shape := range shapes {
		gotName := shape.NonVisualProperties.ConnectionNonVisual.Name
		// We need to find what the rename was for this shape
		found := false
		for oldName, wantName := range wantNames {
			// Check if this shape was the one with that old name (by rename result)
			for _, r := range result.Renames {
				if r.OldName == oldName && r.NewName == wantName && gotName == wantName {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found && gotName != "" {
			t.Errorf("shape[%d] has unexpected name %q", i, gotName)
		}
	}

	// Simpler check: verify the names in slice order
	if shapes[0].NonVisualProperties.ConnectionNonVisual.Name != "body_3" {
		t.Errorf("shapes[0] (Rightmost) = %q, want body_3", shapes[0].NonVisualProperties.ConnectionNonVisual.Name)
	}
	if shapes[1].NonVisualProperties.ConnectionNonVisual.Name != "body_2" {
		t.Errorf("shapes[1] (Middle) = %q, want body_2", shapes[1].NonVisualProperties.ConnectionNonVisual.Name)
	}
	if shapes[2].NonVisualProperties.ConnectionNonVisual.Name != "body" {
		t.Errorf("shapes[2] (Leftmost) = %q, want body", shapes[2].NonVisualProperties.ConnectionNonVisual.Name)
	}
}

func TestNormalizePlaceholderNames_YTiebreaker(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	// Two bodies at same X but different Y — Y tiebreaker
	shapes := []shapeXML{
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Bottom"},
				Placeholder:         &placeholderXML{Type: "body", Index: intPtr(2)},
			},
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset:  &offsetXML{X: 100, Y: 5000},
					Extents: &extentsXML{CX: 5000, CY: 2000},
				},
			},
		},
		{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "Top"},
				Placeholder:         &placeholderXML{Type: "body", Index: intPtr(1)},
			},
			ShapeProperties: shapePropertiesXML{
				Transform: &transformXML{
					Offset:  &offsetXML{X: 100, Y: 100},
					Extents: &extentsXML{CX: 5000, CY: 2000},
				},
			},
		},
	}

	NormalizePlaceholderNames(shapes)

	// Top (y=100) → body, Bottom (y=5000) → body_2
	if shapes[1].NonVisualProperties.ConnectionNonVisual.Name != "body" {
		t.Errorf("Top shape = %q, want body",
			shapes[1].NonVisualProperties.ConnectionNonVisual.Name)
	}
	if shapes[0].NonVisualProperties.ConnectionNonVisual.Name != "body_2" {
		t.Errorf("Bottom shape = %q, want body_2",
			shapes[0].NonVisualProperties.ConnectionNonVisual.Name)
	}
}

func TestNormalizeLayoutFiles_Integration(t *testing.T) {
	reader, err := OpenTemplate("testdata/standard.pptx")
	if err != nil {
		t.Skipf("test template not available: %v", err)
	}
	defer func() { _ = reader.Close() }()

	layouts, err := ParseLayouts(reader)
	if err != nil {
		t.Fatalf("ParseLayouts() error = %v", err)
	}

	normalizedFiles, err := NormalizeLayoutFiles(reader, layouts)
	if err != nil {
		t.Fatalf("NormalizeLayoutFiles() error = %v", err)
	}

	// Verify that normalized files are produced for layouts that needed changes
	t.Logf("NormalizeLayoutFiles produced %d normalized files", len(normalizedFiles))
	for path := range normalizedFiles {
		t.Logf("  normalized: %s", path)
	}

	// Verify all placeholder IDs are canonical (no spaces, no template-specific names)
	for _, layout := range layouts {
		for _, ph := range layout.Placeholders {
			// Canonical names shouldn't have spaces (except utility placeholders)
			if ph.Type == types.PlaceholderOther {
				continue // Utility placeholders keep original names
			}
			if strings.Contains(ph.ID, "Placeholder") {
				t.Errorf("layout %q has non-canonical placeholder ID %q", layout.ID, ph.ID)
			}
		}
	}
}
