package layout

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureCannotSupplyAutomaticOrdinaryContentMappings(t *testing.T) {
	legal := types.PlaceholderInfo{ID: "legal_disclosure", Type: types.PlaceholderBody, Role: types.PlaceholderRoleDisclosure, Index: 1}
	body := types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 2}
	layout := types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{legal, body}}
	if got := findPlaceholder(layout, types.PlaceholderBody); got == nil || got.ID != "body" {
		t.Errorf("generic body selected required legal slot: %+v", got)
	}
	if got := findBodyPlaceholders(layout); len(got) != 1 || got[0].ID != "body" {
		t.Errorf("legal slot supplied ordinary columns: %+v", got)
	}
	if countContentPlaceholders(layout) != 1 {
		t.Error("legal slot supplies automatic slot capacity")
	}
	if got := contentPlaceholdersFromLayout(layout); len(got) != 1 || got[0].ID != "body" {
		t.Errorf("legal slot supplies ordinary slot target: %+v", got)
	}
}
