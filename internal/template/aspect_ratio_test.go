package template

import "testing"

// go-slide-creator-r9nn: callers hardcoded "16:9" and only overrode it from
// template metadata, so a 10x7.5in (4:3) template and a 17.5x7.5in (21:9) one
// were both reported as widescreen. list_templates fields=compact exposes
// nothing but this ratio, so an agent sizing columns and text for a 4:3
// corporate deck was told it had 16:9 to work with.
func TestAspectRatio(t *testing.T) {
	const in = int64(914400)

	tests := []struct {
		name     string
		widthIn  float64
		heightIn float64
		want     string
	}{
		{"widescreen 13.333x7.5", 13.333, 7.5, "16:9"},
		{"widescreen 10x5.625", 10, 5.625, "16:9"},
		{"classic 10x7.5", 10, 7.5, "4:3"},
		{"classic 8x6", 8, 6, "4:3"},
		{"ultrawide 17.5x7.5", 17.5, 7.5, "21:9"},
		{"16:10 16x10", 16, 10, "16:10"},
		{"business-template 14.698x8.267", 14.698, 8.267, "16:9"},
		{"square is neither", 8, 8, "1.000:1"},
		{"portrait", 7.5, 10, "0.750:1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AspectRatio(int64(tt.widthIn*float64(in)), int64(tt.heightIn*float64(in)))
			if got != tt.want {
				t.Errorf("AspectRatio(%gin x %gin) = %q, want %q", tt.widthIn, tt.heightIn, got, tt.want)
			}
		})
	}
}

// Unknown dimensions must not be reported as a real ratio.
func TestAspectRatio_UnknownDimensions(t *testing.T) {
	for _, dims := range [][2]int64{{0, 0}, {914400, 0}, {0, 914400}, {-1, 914400}} {
		if got := AspectRatio(dims[0], dims[1]); got != "unknown" {
			t.Errorf("AspectRatio(%d, %d) = %q, want \"unknown\"", dims[0], dims[1], got)
		}
	}
}
