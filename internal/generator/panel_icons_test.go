package generator

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestResolvePanelIcon(t *testing.T) {
	inlineSVG := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/></svg>`
	dataURI := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(inlineSVG))

	tests := []struct {
		name     string
		field    any
		wantSVG  bool
		wantSkip bool
		contains string // optional substring expected in the resolved SVG
	}{
		{name: "nil", field: nil, wantSVG: false},
		{name: "empty string", field: "", wantSVG: false},
		{name: "bundled name", field: "rocket", wantSVG: true, contains: "<svg"},
		{name: "unknown bundled name", field: "definitely-not-an-icon", wantSVG: false, wantSkip: true},
		{name: "inline svg string", field: inlineSVG, wantSVG: true, contains: "<circle"},
		{name: "data uri string", field: dataURI, wantSVG: true, contains: "<circle"},
		{name: "object name", field: map[string]any{"name": "rocket"}, wantSVG: true, contains: "<svg"},
		{name: "object svg_data", field: map[string]any{"svg_data": inlineSVG}, wantSVG: true, contains: "<circle"},
		{name: "object path (deferred)", field: map[string]any{"path": "logo.svg"}, wantSVG: false, wantSkip: true},
		{name: "object url (deferred)", field: map[string]any{"url": "https://x/i.svg"}, wantSVG: false, wantSkip: true},
		{name: "url string (deferred)", field: "https://example.com/i.svg", wantSVG: false, wantSkip: true},
		{name: "file path string (deferred)", field: "assets/logo.svg", wantSVG: false, wantSkip: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg, _, skip := resolvePanelIcon(tt.field, "")
			if (len(svg) > 0) != tt.wantSVG {
				t.Errorf("resolvePanelIcon() svg present = %v, want %v (skip=%q)", len(svg) > 0, tt.wantSVG, skip)
			}
			if (skip != "") != tt.wantSkip {
				t.Errorf("resolvePanelIcon() skip = %q, wantSkip %v", skip, tt.wantSkip)
			}
			if tt.contains != "" && !strings.Contains(string(svg), tt.contains) {
				t.Errorf("resolvePanelIcon() svg missing %q: %s", tt.contains, string(svg))
			}
		})
	}
}

func TestResolvePanelIconRecolor(t *testing.T) {
	// Bundled outline icons use stroke="currentColor"; an explicit fill must recolor it.
	svg, _, _ := resolvePanelIcon(map[string]any{"name": "rocket", "fill": "#C0504D"}, "")
	if !strings.Contains(string(svg), `stroke="#C0504D"`) {
		t.Errorf("explicit fill not applied to bundled icon: %s", string(svg))
	}
	// defaultFill applies when no explicit fill is set (stylish_panels case).
	svg2, _, _ := resolvePanelIcon("rocket", "#FFFFFF")
	if !strings.Contains(string(svg2), `stroke="#FFFFFF"`) {
		t.Errorf("default fill not applied: %s", string(svg2))
	}
	// Explicit fill wins over defaultFill.
	svg3, _, _ := resolvePanelIcon(map[string]any{"name": "rocket", "fill": "#111111"}, "#FFFFFF")
	if !strings.Contains(string(svg3), `stroke="#111111"`) {
		t.Errorf("explicit fill should win over default: %s", string(svg3))
	}
}

func TestApplyIconFill(t *testing.T) {
	// currentColor is recolored.
	in := []byte(`<svg stroke="currentColor" fill="currentColor"><path/></svg>`)
	out := string(ApplyIconFill(in, "#FF0000"))
	if !strings.Contains(out, `stroke="#FF0000"`) || !strings.Contains(out, `fill="#FF0000"`) {
		t.Errorf("currentColor not recolored: %s", out)
	}
	// Explicit colors are preserved (only currentColor is touched).
	keep := []byte(`<svg fill="#123456"><path/></svg>`)
	if got := string(ApplyIconFill(keep, "#FF0000")); got != string(keep) {
		t.Errorf("explicit color must be preserved, got %s", got)
	}
	// No <svg> tag → unchanged.
	none := []byte(`not an svg`)
	if got := string(ApplyIconFill(none, "#FF0000")); got != string(none) {
		t.Errorf("non-svg input must be unchanged, got %s", got)
	}
}

func TestPanelColumnsBandsNoIconPreservesGeometry(t *testing.T) {
	h := int64(4351338)
	// No icons: geometry must match the original ratios-of-total-height formula.
	iconBand, header, gap, body := panelColumnsBands(h, false)
	if iconBand != 0 {
		t.Errorf("no-icon iconBand = %d, want 0", iconBand)
	}
	wantHeader := int64(float64(h) * panelHeaderHeightRatio)
	wantGap := int64(float64(h) * panelGapHeightRatio)
	if header != wantHeader || gap != wantGap || body != h-wantHeader-wantGap {
		t.Errorf("no-icon bands changed: header=%d gap=%d body=%d (want %d/%d/%d)",
			header, gap, body, wantHeader, wantGap, h-wantHeader-wantGap)
	}
	// With icons: a band is reserved and all bands sum to the total height.
	iconBand2, header2, gap2, body2 := panelColumnsBands(h, true)
	if iconBand2 <= 0 {
		t.Errorf("icon band not reserved: %d", iconBand2)
	}
	if iconBand2+header2+gap2+body2 != h {
		t.Errorf("bands do not sum to total: %d != %d", iconBand2+header2+gap2+body2, h)
	}
}

func TestPanelIconRectsWithinBounds(t *testing.T) {
	bounds := types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}
	withIcon := func(n int) []nativePanelData {
		ps := make([]nativePanelData, n)
		for i := range ps {
			ps[i].iconSVG = []byte("<svg/>")
		}
		return ps
	}
	for _, mode := range []string{"columns", "rows", "stat_cards", "stylish_panels"} {
		t.Run(mode, func(t *testing.T) {
			panels := withIcon(3)
			rects := panelIconRects(mode, bounds, panels)
			if len(rects) != len(panels) {
				t.Fatalf("rects len = %d, want %d", len(rects), len(panels))
			}
			for i, r := range rects {
				if r.CX <= 0 || r.CY <= 0 {
					t.Errorf("%s rect[%d] non-positive: %+v", mode, i, r)
				}
				if r.X < bounds.X || r.Y < bounds.Y ||
					r.X+r.CX > bounds.X+bounds.Width || r.Y+r.CY > bounds.Y+bounds.Height {
					t.Errorf("%s rect[%d] escapes bounds: %+v", mode, i, r)
				}
			}
		})
	}
}

func TestPanelIconDefaultFill(t *testing.T) {
	if got := panelIconDefaultFill("stylish_panels"); got != stylishPanelIconDefaultFill {
		t.Errorf("stylish_panels default fill = %q, want %q", got, stylishPanelIconDefaultFill)
	}
	for _, m := range []string{"columns", "rows", "stat_cards", ""} {
		if got := panelIconDefaultFill(m); got != "" {
			t.Errorf("%s default fill = %q, want empty", m, got)
		}
	}
}
