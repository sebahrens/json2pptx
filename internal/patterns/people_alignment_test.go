package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

type alignProbe struct {
	Paragraphs []struct {
		Content string `json:"content"`
		Bold    bool   `json:"bold"`
		Color   string `json:"color"`
		Align   string `json:"align"`
	} `json:"paragraphs"`
	Align string `json:"align"`
}

func decodeAlignProbe(t *testing.T, cell *jsonschema.GridCellInput) alignProbe {
	t.Helper()
	if cell == nil || cell.Shape == nil {
		t.Fatalf("cell has no shape: %+v", cell)
	}
	var p alignProbe
	if err := json.Unmarshal(cell.Shape.Text, &p); err != nil {
		t.Fatalf("decode text: %v", err)
	}
	return p
}

// team-bios and contact-directory draw the initials placeholder with one
// people primitive: same geometry, fill and initials styling
// (go-slide-creator-q4fut).
func TestPeoplePatternsShareHeadshotPrimitive(t *testing.T) {
	ctx := ExpandContext{}
	tb, _ := Default().Get("team-bios")
	tbGrid, err := tb.Expand(ctx, &TeamBiosValues{Members: []TeamBiosMember{
		{Name: "Jane Smith", Role: "Partner", Bio: "Leads retail transformations."},
		{Name: "Raj Patel", Role: "Data lead", Bio: "Builds pricing engines."},
	}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cd, _ := Default().Get("contact-directory")
	cdGrid, err := cd.Expand(ctx, &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{
		Name:   "Europe",
		People: []ContactDirectoryPerson{{Name: "Jane Smith", Title: "Partner"}, {Name: "Raj Patel", Title: "Director"}},
	}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cdDisc *jsonschema.GridCellInput
	for _, row := range cdGrid.Rows {
		for _, c := range row.Cells {
			if c != nil && c.Shape != nil && c.Shape.Geometry == "ellipse" {
				cdDisc = c
			}
		}
	}
	if cdDisc == nil {
		t.Fatal("contact-directory drew no initials disc")
	}
	tbDisc := tbGrid.Rows[0].Cells[0]
	if tbDisc.Shape == nil || tbDisc.Shape.Geometry != "ellipse" || tbDisc.Fit != "contain" {
		t.Fatalf("team-bios headshot = %+v, want a contained ellipse", tbDisc)
	}
	if string(tbDisc.Shape.Fill) != string(cdDisc.Shape.Fill) {
		t.Errorf("fill: team-bios %s, contact-directory %s", tbDisc.Shape.Fill, cdDisc.Shape.Fill)
	}
	tp, cp := decodeAlignProbe(t, tbDisc).Paragraphs[0], decodeAlignProbe(t, cdDisc).Paragraphs[0]
	if !tp.Bold || !cp.Bold || tp.Color != cp.Color || tp.Align != "ctr" {
		t.Errorf("initials styling differs: team-bios %+v, contact-directory %+v", tp, cp)
	}

	// The text block centres under the centred disc (go-slide-creator-wd6p6).
	text := decodeAlignProbe(t, tbGrid.Rows[1].Cells[0])
	if text.Align != "ctr" {
		t.Errorf("team-bios text align = %q, want ctr", text.Align)
	}
	for _, p := range text.Paragraphs {
		if p.Align != "ctr" {
			t.Errorf("team-bios paragraph %q align = %q, want ctr", p.Content, p.Align)
		}
	}
}

// The quote block hangs flush off its accent rule; it is centred only when
// there is no rule (go-slide-creator-wd6p6).
func TestPullQuoteAlignsToAccentRule(t *testing.T) {
	pq, _ := Default().Get("pull-quote")
	for side, want := range map[string]string{"": "l", "left": "l", "right": "r", "none": "ctr"} {
		grid, err := pq.Expand(ExpandContext{}, &PullQuoteValues{
			Quote: "Pricing discipline paid for the whole programme in year one.", Attribution: "Jane Smith", AccentSide: side,
		}, nil, nil)
		if err != nil {
			t.Fatalf("%q: %v", side, err)
		}
		var quote *jsonschema.GridCellInput
		for _, c := range grid.Rows[0].Cells {
			if c.Shape != nil && len(c.Shape.Text) > 0 {
				quote = c
			}
		}
		for name, cell := range map[string]*jsonschema.GridCellInput{"quote": quote, "attribution": grid.Rows[1].Cells[0]} {
			p := decodeAlignProbe(t, cell)
			if p.Align != want || p.Paragraphs[0].Align != want {
				t.Errorf("accent_side %q: %s align = %q/%q, want %q", side, name, p.Align, p.Paragraphs[0].Align, want)
			}
		}
	}
}
