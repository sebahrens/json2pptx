package visualqa

import "testing"

func TestElementPathAt(t *testing.T) {
	// Four KPI cells in a row plus an enclosing region.
	elements := []ElementBox{
		{Path: "/grid", Box: BBox{X: 0.05, Y: 0.3, W: 0.9, H: 0.4}},
		{Path: "/cell0", Box: BBox{X: 0.05, Y: 0.3, W: 0.2, H: 0.4}},
		{Path: "/cell1", Box: BBox{X: 0.28, Y: 0.3, W: 0.2, H: 0.4}},
		{Path: "/cell2", Box: BBox{X: 0.51, Y: 0.3, W: 0.2, H: 0.4}},
		{Path: "/cell3", Box: BBox{X: 0.74, Y: 0.3, W: 0.2, H: 0.4}},
	}
	cases := []struct {
		name   string
		b      BBox
		want   string
		wantOK bool
	}{
		{"centre inside cell 2 picks smallest container", BBox{X: 0.33, Y: 0.4, W: 0.05, H: 0.05}, "/cell1", true},
		{"centre in gap between cells picks enclosing region", BBox{X: 0.485, Y: 0.35, W: 0.02, H: 0.1}, "/grid", true},
		{"outside everything", BBox{X: 0.1, Y: 0.85, W: 0.1, H: 0.05}, "", false},
		{"invalid bbox", BBox{X: 0.1, Y: 0.1, W: 0, H: 0.1}, "", false},
		{"out of slide", BBox{X: 0.9, Y: 0.9, W: 0.5, H: 0.5}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ElementPathAt(tc.b, elements)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ElementPathAt = (%q,%v), want (%q,%v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}

	// Overlap fallback without a container: bbox straddles cell 3's left edge
	// with most of its area inside cell 3.
	noGrid := elements[1:]
	if got, ok := ElementPathAt(BBox{X: 0.72, Y: 0.72, W: 0.1, H: 0.02}, noGrid); ok {
		t.Errorf("bbox below all cells must miss, got %q", got)
	}
	if got, ok := ElementPathAt(BBox{X: 0.73, Y: 0.29, W: 0.1, H: 0.02}, noGrid); !ok || got != "/cell3" {
		t.Errorf("mostly-inside bbox = (%q,%v), want /cell3", got, ok)
	}
}

func TestParseFindingsBBox(t *testing.T) {
	info := SlideInfo{Index: 2, Type: "content"}
	text := `[{"severity":"P1","category":"text_overflow","description":"d","location":"card 2","bbox":{"x":0.3,"y":0.4,"w":0.1,"h":0.1}},
	          {"severity":"P2","category":"spacing","description":"d","location":"x","bbox":{"x":0.9,"y":0.9,"w":0.5,"h":0.5}},
	          {"severity":"P2","category":"spacing","description":"d","location":"x"}]`
	findings, err := parseFindings(text, info)
	if err != nil {
		t.Fatalf("parseFindings: %v", err)
	}
	if findings[0].BBox == nil || findings[0].BBox.X != 0.3 {
		t.Errorf("valid bbox not parsed: %+v", findings[0].BBox)
	}
	if findings[1].BBox != nil {
		t.Errorf("out-of-range bbox must be dropped, got %+v", findings[1].BBox)
	}
	if findings[2].BBox != nil {
		t.Error("absent bbox must stay nil")
	}
}
