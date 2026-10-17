package svggen

import (
	"regexp"
	"strings"
	"testing"
)

func TestFunnelHighlight(t *testing.T) {
	for _, style := range []string{"steps", "tapered"} {
		for _, highlight := range []any{[]any{"Qualified"}, []any{1.0}, []any{}, []any{"unknown"}, []any{-1.0}} {
			req := highlightReq(nil, nil, nil, false)
			req.Type = "funnel_chart"
			req.Data = map[string]any{"values": []any{100.0, 60.0, 20.0}, "categories": []any{"Leads", "Qualified", "Won"}, "style": style, "highlight": highlight}
			doc, err := Render(req)
			refs := highlight.([]any)
			invalid := len(refs) > 0 && (refs[0] == "unknown" || refs[0] == -1.0)
			if invalid {
				if err == nil {
					t.Error("invalid highlight accepted")
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			svg := string(doc.Content)
			wantNeutral := 2
			if len(refs) == 0 {
				wantNeutral = 3
			}
			if len(regexp.MustCompile(`<path[^>]*(?:fill="|fill:)`+testNeutral+`[;" ]`).FindAllString(svg, -1)) != wantNeutral {
				t.Fatalf("%s: want %d neutral stage shapes", style, wantNeutral)
			}
			if len(refs) > 0 && len(regexp.MustCompile(`<path[^>]*(?:fill="|fill:)`+testAccent+`[;" ]`).FindAllString(svg, -1)) != 1 {
				t.Fatalf("%s: want exactly one accented stage", style)
			}
			if len(refs) == 0 && strings.Contains(svg, testAccent) {
				t.Fatalf("%s: empty highlight has accent", style)
			}
		}
	}
	d := &FunnelDiagram{NewBaseDiagram("funnel_chart")}
	if _, ok := d.DataSchema().Properties["highlight"]; !ok {
		t.Fatal("highlight missing from capability schema")
	}
}
