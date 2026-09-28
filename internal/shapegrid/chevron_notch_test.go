package shapegrid

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestDefaultChevronAdj(t *testing.T) {
	cases := []struct {
		name   string
		geom   string
		cx, cy int64
		want   int64
		ok     bool
	}{
		{"wide keeps preset", "chevron", 3000, 1000, 50000, true},
		{"exactly 2:1 keeps preset", "chevron", 2000, 1000, 50000, true},
		{"square capped to quarter width", "chevron", 1000, 1000, 25000, true},
		{"4:3 capped", "chevron", 1333, 1000, 33325, true},
		{"taller than wide caps against width", "chevron", 1000, 2000, 25000, true},
		{"other geometry", "homePlate", 1000, 1000, 0, false},
		{"degenerate", "chevron", 0, 1000, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DefaultChevronAdj(tc.geom, tc.cx, tc.cy)
			if ok != tc.ok || got != tc.want {
				t.Errorf("DefaultChevronAdj(%q, %d, %d) = %d, %v; want %d, %v", tc.geom, tc.cx, tc.cy, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// A stubby chevron gets a reduced notch so its label keeps half the width;
// an authored adj is never overridden (go-slide-creator-5wm83).
func TestGenerateShapeXML_StubbyChevronNotch(t *testing.T) {
	bounds := pptx.RectEmu{CX: 1000000, CY: 1000000}
	out, err := GenerateShapeXML(&ShapeSpec{Geometry: "chevron"}, 2, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name="adj" fmla="val 25000"`) {
		t.Errorf("square chevron missing capped adj: %s", out)
	}
	out, err = GenerateShapeXML(&ShapeSpec{Geometry: "chevron", Adjustments: map[string]int64{"adj": 40000}}, 2, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `fmla="val 40000"`) || strings.Contains(string(out), "val 25000") {
		t.Errorf("authored adj must win: %s", out)
	}
	out, err = GenerateShapeXML(&ShapeSpec{Geometry: "chevron"}, 2, pptx.RectEmu{CX: 3000000, CY: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), `name="adj"`) {
		t.Errorf("wide chevron must keep the preset default: %s", out)
	}
}
