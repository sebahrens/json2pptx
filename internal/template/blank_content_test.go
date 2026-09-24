package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestIsTrueBlankLayout(t *testing.T) {
	tests := []struct {
		name   string
		layout *types.LayoutMetadata
		want   bool
	}{
		{"nil", nil, false},
		{"blank", &types.LayoutMetadata{CanonicalType: types.CanonicalLayoutBlank}, true},
		{"title-bearing blank", &types.LayoutMetadata{CanonicalType: types.CanonicalLayoutBlank, Placeholders: []types.PlaceholderInfo{{Type: types.PlaceholderTitle}}}, false},
		{"body-bearing blank", &types.LayoutMetadata{CanonicalType: types.CanonicalLayoutBlank, Placeholders: []types.PlaceholderInfo{{Type: types.PlaceholderBody}}}, false},
		{"content-bearing blank", &types.LayoutMetadata{CanonicalType: types.CanonicalLayoutBlank, Placeholders: []types.PlaceholderInfo{{Type: types.PlaceholderContent}}}, false},
		{"utility-only blank", &types.LayoutMetadata{CanonicalType: types.CanonicalLayoutBlank, Placeholders: []types.PlaceholderInfo{{Type: types.PlaceholderOther, Role: types.PlaceholderRoleFooter}}}, true},
		{"other layout", &types.LayoutMetadata{CanonicalType: types.CanonicalLayoutOneContent}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTrueBlankLayout(tt.layout); got != tt.want {
				t.Errorf("IsTrueBlankLayout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlankContentRect(t *testing.T) {
	const width, height int64 = 12_192_000, 6_858_000
	tests := []struct {
		name   string
		layout *types.LayoutMetadata
		bottom int64
	}{
		{"plain", nil, height * 95 / 100},
		{"footer placeholder", &types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{{Role: types.PlaceholderRoleFooter, Bounds: types.BoundingBox{Y: 6_000_000}}}}, 6_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BlankContentRect(tt.layout, width, height)
			if got.X != width*5/100 || got.Y != height*5/100 || got.X+got.Width != width*95/100 || got.Y+got.Height != tt.bottom {
				t.Errorf("BlankContentRect() = %+v, want 5%% margins and bottom %d", got, tt.bottom)
			}
		})
	}
}
