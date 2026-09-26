package pipeline

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureCannotSupplyPipelineContentCapacity(t *testing.T) {
	layout := types.LayoutMetadata{ID: "closing", Placeholders: []types.PlaceholderInfo{
		{ID: "legal_disclosure", Type: types.PlaceholderBody, Role: types.PlaceholderRoleDisclosure, Index: 1},
		{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 2},
	}}
	if countContentPHs(layout) != 1 {
		t.Error("pipeline counts legal slot as ordinary capacity")
	}
	if got := findBodyPlaceholder(layout.ID, []types.LayoutMetadata{layout}); got == nil || got.ID != "body" {
		t.Errorf("pipeline overflow measured legal frame instead of ordinary body: %+v", got)
	}
}
