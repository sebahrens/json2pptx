package main

import "testing"

// Numeric values connect header rows/columns geometrically, but are a
// different text role. They must not make unrelated header dimensions peers.
func heatmapPeerRoleFixture() []peerShape {
	shape := func(x, y, w, h int64, size float64, bold bool, text string) peerShape {
		role := "bfalse/ifalse/black"
		if bold {
			role = "btrue/ifalse/black"
		}
		return peerShape{x: x * 12700, y: y * 12700, w: w * 12700, h: h * 12700, sig: "rect|none|none", scale: 1, paras: []peerPara{{sizePt: size, role: role, text: text}}}
	}
	return []peerShape{
		shape(120, 0, 240, 54, 14, true, "Q1"), shape(360, 0, 240, 54, 14, true, "Q2"),
		shape(0, 54, 120, 120, 18, true, "North"), shape(0, 174, 120, 120, 18, true, "South"),
		shape(120, 54, 240, 120, 12, false, "2"), shape(360, 54, 240, 120, 12, false, "4"),
		shape(120, 174, 240, 120, 12, false, "3"), shape(360, 174, 240, 120, 12, false, "5"),
	}
}

func TestPeerConnectivitySeparatesRolesBeforeGrouping(t *testing.T) {
	for _, tc := range []struct {
		name         string
		alter        int
		size         float64
		wantMismatch bool
	}{{"distinct axis groups", -1, 0, false}, {"column axis peers", 1, 13, true}, {"row axis peers", 3, 16, true}, {"numeric value peers", 6, 11, true}} {
		t.Run(tc.name, func(t *testing.T) {
			shapes := heatmapPeerRoleFixture()
			if tc.alter >= 0 {
				shapes[tc.alter].paras[0].sizePt = tc.size
			}
			mismatches := peerRoleMismatches(shapes)
			if (len(mismatches) > 0) != tc.wantMismatch {
				t.Fatalf("mismatches=%v want mismatch=%v", mismatches, tc.wantMismatch)
			}
		})
	}
}
