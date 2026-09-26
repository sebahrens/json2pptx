package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureCannotSupplyAutomaticGeneratorSlots(t *testing.T) {
	phs := []types.PlaceholderInfo{
		{ID: "legal_disclosure", Type: types.PlaceholderBody, Role: types.PlaceholderRoleDisclosure, Index: 1},
		{ID: "body", Type: types.PlaceholderBody, Role: types.PlaceholderRoleBody, Index: 2},
	}
	if got := FilterContentPlaceholders(phs); len(got) != 1 || got[0].ID != "body" {
		t.Fatalf("automatic generator slots include required legal text: %+v", got)
	}
}
