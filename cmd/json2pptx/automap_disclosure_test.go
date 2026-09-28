package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// A closing layout whose legal-disclosure slot shares the subTitle type (and
// comes first) must still route "subtitle" copy to the real subtitle, not the
// 11pt bottom-right disclosure box (p-style Closing).
func TestAutoMapSubtitleSkipsDisclosureSlot(t *testing.T) {
	layout := types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{
		{ID: "title", Type: types.PlaceholderTitle, Index: 0},
		{ID: "legal_disclosure", Type: types.PlaceholderSubtitle, Index: 1},
		{ID: "subtitle", Type: types.PlaceholderSubtitle, Index: 2},
	}}
	got := autoMapPlaceholders([]ContentInput{{PlaceholderID: "subtitle", Type: "text"}}, layout)
	if got[0].PlaceholderID != "subtitle" {
		t.Fatalf("subtitle mapped to %q, want subtitle", got[0].PlaceholderID)
	}

	onlyDisclosure := types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{
		{ID: "title", Type: types.PlaceholderTitle},
		{ID: "legal_disclosure", Type: types.PlaceholderSubtitle, Index: 1},
	}}
	got = autoMapPlaceholders([]ContentInput{{PlaceholderID: "subtitle", Type: "text"}}, onlyDisclosure)
	if got[0].PlaceholderID == "legal_disclosure" {
		t.Fatal("subtitle copy must never land in the legal disclosure slot")
	}
}
