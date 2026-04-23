package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDefaultTableStyleResolver_Empty(t *testing.T) {
	r := defaultTableStyleResolver{}
	got := r.ResolveTableStyleID("")
	if got != types.DefaultTableStyleID {
		t.Errorf("got %q, want %q", got, types.DefaultTableStyleID)
	}
}

func TestDefaultTableStyleResolver_Passthrough(t *testing.T) {
	r := defaultTableStyleResolver{}
	guid := "{ABC}"
	got := r.ResolveTableStyleID(guid)
	if got != guid {
		t.Errorf("got %q, want %q", got, guid)
	}
}

// stubResolver lets tests control resolved style IDs.
type stubResolver struct {
	resolved string
}

func (s stubResolver) ResolveTableStyleID(_ string) string { return s.resolved }

func TestPopulateTableInShape_WithResolver(t *testing.T) {
	customGUID := "{CUSTOM-GUID-1234}"
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.TableStyle{
			StyleID: "@template-default",
			Borders: "all",
		},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
		Bounds: types.BoundingBox{
			X: 914400, Y: 914400,
			Width: 5486400, Height: 3657600,
		},
	}

	result, err := PopulateTableInShape(table, placeholder, nil, stubResolver{resolved: customGUID})
	if err != nil {
		t.Fatalf("PopulateTableInShape: %v", err)
	}
	if !strings.Contains(result.XML, customGUID) {
		t.Errorf("XML should contain resolved GUID %s", customGUID)
	}
}

func TestPopulateTableInShape_NilResolverUsesDefault(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
		Bounds: types.BoundingBox{
			X: 914400, Y: 914400,
			Width: 5486400, Height: 3657600,
		},
	}

	result, err := PopulateTableInShape(table, placeholder, nil, nil)
	if err != nil {
		t.Fatalf("PopulateTableInShape: %v", err)
	}
	if !strings.Contains(result.XML, types.DefaultTableStyleID) {
		t.Errorf("XML should contain default style ID %s", types.DefaultTableStyleID)
	}
}
