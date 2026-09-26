package patterns

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func contactDirectoryPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("contact-directory")
	if !ok {
		t.Fatal("contact-directory not registered")
	}
	return p
}

func cdPeople(n int) []ContactDirectoryPerson {
	people := make([]ContactDirectoryPerson, n)
	for i := range people {
		people[i] = ContactDirectoryPerson{Name: fmt.Sprintf("Person Number%d", i), Title: "Partner, London"}
	}
	return people
}

func TestContactDirectory_Metadata(t *testing.T) {
	p := contactDirectoryPattern(t)
	if p.Version() != 1 || p.UseWhen() == "" || p.NotWhen() == "" || p.CellsHint() == "" || p.Description() == "" {
		t.Fatal("metadata incomplete")
	}
	if !strings.Contains(p.UseWhen(), "team-bios") || !strings.Contains(p.NotWhen(), "team-bios") {
		t.Errorf("use_when / not_when must contrast with team-bios: %q / %q", p.UseWhen(), p.NotWhen())
	}
	tb, _ := Default().Get("team-bios")
	if !strings.Contains(tb.NotWhen(), "contact-directory") {
		t.Errorf("team-bios not_when should point at contact-directory: %q", tb.NotWhen())
	}
	tax := p.Taxonomy()
	if tax.Category == "" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 || len(tax.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tax)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), "2020-12") {
		t.Fatalf("schema invalid: %v", err)
	}
	if _, ok := p.(ImageAssetPattern); !ok {
		t.Fatal("contact-directory must expose its photos for host path/url resolution")
	}
}

func TestContactDirectory_Validate(t *testing.T) {
	p := contactDirectoryPattern(t)
	if err := p.Validate(p.(Exemplar).ExemplarValues(), nil, nil); err != nil {
		t.Fatalf("exemplar must validate: %v", err)
	}
	// Budgets count characters, not bytes: 40 umlauts are 80 bytes.
	umlauts := &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{Name: "Zürich", People: []ContactDirectoryPerson{{Name: strings.Repeat("ü", cdNameMax), Title: strings.Repeat("€", cdTitleMax)}}}}}
	if err := p.Validate(umlauts, nil, nil); err != nil {
		t.Errorf("multi-byte values inside the budget must validate: %v", err)
	}
	long := func(n int) string { return strings.Repeat("x", n) }
	group := func(people ...ContactDirectoryPerson) ContactDirectoryGroup {
		return ContactDirectoryGroup{Name: "Region", People: people}
	}
	cases := []struct {
		name string
		vals *ContactDirectoryValues
		ovr  *ContactDirectoryOverrides
		co   map[int]any
		want string
	}{
		{name: "no groups", vals: &ContactDirectoryValues{}, want: "groups"},
		{name: "too many groups", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(cdPeople(1)...), group(cdPeople(1)...), group(cdPeople(1)...), group(cdPeople(1)...), group(cdPeople(1)...)}}, want: "groups"},
		{name: "too many people", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(cdPeople(13)...), group(cdPeople(12)...)}}, want: "24"},
		{name: "empty group", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{Name: "Empty"}}}, want: "groups[0].people"},
		{name: "missing group name", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{People: cdPeople(1)}}}, want: "groups[0].name"},
		{name: "group name too long", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{Name: long(cdGroupNameMax + 1), People: cdPeople(1)}}}, want: "groups[0].name"},
		{name: "missing person name", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(ContactDirectoryPerson{Title: "x"})}}, want: "people[0].name"},
		{name: "name too long", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(ContactDirectoryPerson{Name: long(cdNameMax + 1)})}}, want: "people[0].name"},
		{name: "title too long", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(ContactDirectoryPerson{Name: "A", Title: long(cdTitleMax + 1)})}}, want: "people[0].title"},
		{name: "photo without source", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(ContactDirectoryPerson{Name: "A", Photo: &jsonschema.GridImageInput{Alt: "x"}})}}, want: "path or url"},
		{name: "bad columns", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(cdPeople(1)...)}}, ovr: &ContactDirectoryOverrides{Columns: 7}, want: "columns"},
		{name: "cell overrides rejected", vals: &ContactDirectoryValues{Groups: []ContactDirectoryGroup{group(cdPeople(1)...)}}, co: map[int]any{0: &CellOverride{}}, want: "cell_overrides"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ovr any
			if tc.ovr != nil {
				ovr = tc.ovr
			}
			err := p.Validate(tc.vals, ovr, tc.co)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want mention of %q", err, tc.want)
			}
		})
	}
}

func TestContactDirectory_ExpandSideBySide(t *testing.T) {
	p := contactDirectoryPattern(t)
	vals := &ContactDirectoryValues{Groups: []ContactDirectoryGroup{
		{Name: "Americas", People: cdPeople(6)},
		{Name: "Europe", People: cdPeople(3)},
	}}
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 2*cdDefaultColumns {
		t.Fatalf("want photo+text column pairs for %d people per row, got %s", cdDefaultColumns, grid.Columns)
	}
	// heading, 2 person rows, heading, 1 person row
	if len(grid.Rows) != 5 {
		t.Fatalf("want 5 rows (2 headings + 3 person rows), got %d", len(grid.Rows))
	}
	_, areaH := sizingAreaPt(fullThemeCtx())
	total := grid.RowGap * float64(len(grid.Rows)-1)
	for i, row := range grid.Rows {
		if row.MaxHeight <= 0 || row.MinHeight != row.MaxHeight {
			t.Errorf("row %d must be point-capped, got min=%v max=%v", i, row.MinHeight, row.MaxHeight)
		}
		total += row.MaxHeight
	}
	if total > areaH {
		t.Errorf("grid is %.0fpt, taller than the %.0fpt content area", total, areaH)
	}
	head := grid.Rows[0].Cells[0]
	if head.ColSpan != len(cols) || head.AccentBar == nil || head.AccentBar.Position != "bottom" {
		t.Errorf("group heading should span the grid with a bottom rule: %+v", head)
	}
	if paras := cellText(t, head.Shape.Text).Paragraphs; paras[0].Content != "Americas" || !paras[0].Bold || paras[0].Color != "accent1" {
		t.Errorf("heading should be the accent bold group name, got %+v", paras[0])
	}
	// Second person row of Americas: 2 people + one filler spanning the rest.
	short := grid.Rows[2].Cells
	if len(short) != 5 || short[4].ColSpan != 4 {
		t.Errorf("short row should pad with one spanning filler, got %d cells", len(short))
	}
	disc := grid.Rows[1].Cells[0]
	if disc.Shape == nil || disc.Shape.Geometry != "ellipse" || disc.Fit != "contain" {
		t.Fatalf("no photo → initials disc (contained ellipse), got %+v", disc)
	}
	if paras := cellText(t, disc.Shape.Text).Paragraphs; paras[0].Content != "PN" {
		t.Errorf("initials = %q, want PN", paras[0].Content)
	}
	text := cellText(t, grid.Rows[1].Cells[1].Shape.Text).Paragraphs
	if len(text) != 2 || !text[0].Bold || text[1].Bold || text[1].Size >= text[0].Size {
		t.Errorf("person text should be a bold name over a smaller title, got %+v", text)
	}
	for _, para := range text {
		if para.Size < 12 {
			t.Errorf("text below the 12pt renderer floor: %+v", para)
		}
	}
}

func TestContactDirectory_SparseStacksHeadshots(t *testing.T) {
	p := contactDirectoryPattern(t)
	vals := &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{Name: "Leadership", People: cdPeople(3)}}}
	grid, err := p.Expand(fullThemeCtx(), vals, &ContactDirectoryOverrides{Columns: 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 3 {
		t.Fatalf("a sparse directory stacks one column per person, got %s", grid.Columns)
	}
	if len(grid.Rows) != 3 {
		t.Fatalf("want heading + photo row + text row, got %d rows", len(grid.Rows))
	}
	if grid.Rows[1].MaxHeight < cdStackPhotoMinPt {
		t.Errorf("sparse headshots should be large, got a %.0fpt row", grid.Rows[1].MaxHeight)
	}
	txt := cellText(t, grid.Rows[2].Cells[0].Shape.Text)
	if txt.Paragraphs[0].Align != "ctr" || txt.Paragraphs[0].Size < 14 {
		t.Errorf("stacked names are centred and promoted, got %+v", txt.Paragraphs[0])
	}
}

func TestContactDirectory_PhotosAreCircularImages(t *testing.T) {
	p := contactDirectoryPattern(t)
	vals := &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{Name: "Team", People: []ContactDirectoryPerson{
		{Name: "Jane Smith", Title: "Partner", Photo: &jsonschema.GridImageInput{Path: "/tmp/jane.jpg"}},
		{Name: "Arun Patel"},
		{Name: "Lila Romero"}, {Name: "Tom Becker"}, {Name: "Ana Souza"}, {Name: "Omar Haddad"},
		{Name: "Elena Rossi"}, {Name: "Sean Murphy"}, {Name: "Mei Huang"},
	}}}}
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	photo := grid.Rows[1].Cells[0]
	if photo.Image == nil || photo.Image.Geometry != "ellipse" || photo.Fit != "contain" {
		t.Fatalf("headshot should be a contained ellipse image cell, got %+v", photo)
	}
	if photo.Image.Alt != "Jane Smith, Partner" {
		t.Errorf("alt should default to name and title, got %q", photo.Image.Alt)
	}
	refs := p.(ImageAssetPattern).ImageAssets(vals)
	if len(refs) != 1 || refs[0].Field != "groups/0/people/0/photo" || refs[0].Image != vals.Groups[0].People[0].Photo {
		t.Fatalf("refs = %+v", refs)
	}
}

func TestContactDirectory_AccentOverride(t *testing.T) {
	p := contactDirectoryPattern(t)
	vals := &ContactDirectoryValues{Groups: []ContactDirectoryGroup{{Name: "Team", People: cdPeople(5)}}}
	grid, err := p.Expand(ExpandContext{}, vals, &ContactDirectoryOverrides{Accent: "accent3"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	head := grid.Rows[0].Cells[0]
	if head.AccentBar.Color != "accent3" || cellText(t, head.Shape.Text).Paragraphs[0].Color != "accent3" {
		t.Errorf("accent override should color the heading and rule, got %+v", head.AccentBar)
	}
}

func TestContactDirectory_WarnsWhenTooDense(t *testing.T) {
	p := contactDirectoryPattern(t).(PostExpandWarner)
	var groups []ContactDirectoryGroup
	for g := 0; g < 4; g++ {
		people := cdPeople(6)
		for i := range people {
			people[i].Title = "Managing Director and Partner, Global Financial Services"
		}
		groups = append(groups, ContactDirectoryGroup{Name: fmt.Sprintf("Region %d", g), People: people})
	}
	warnings := p.PostExpandWarnings(fullThemeCtx(), &ContactDirectoryValues{Groups: groups}, &ContactDirectoryOverrides{Columns: 3})
	if len(warnings) == 0 || !strings.HasPrefix(warnings[len(warnings)-1], ErrCodeBodyTooLong+":") {
		t.Fatalf("an over-full directory should report BODY_TOO_LONG, got %v", warnings)
	}
	if got := p.PostExpandWarnings(fullThemeCtx(), p.(Exemplar).ExemplarValues(), nil); len(got) != 0 {
		t.Errorf("the exemplar should fit without warnings, got %v", got)
	}
}

func TestContactDirectory_Golden(t *testing.T) {
	p := contactDirectoryPattern(t)
	grid, err := p.Expand(fullThemeCtx(), p.(Exemplar).ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "contact-directory", "default.golden.json"))
}

func TestContactDirectory_Recommend(t *testing.T) {
	for _, intent := range []string{"key contacts", "contacts", "directory", "who to call"} {
		res := Recommend(Default(), intent, nil, 3)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != "contact-directory" {
			t.Errorf("%q should rank contact-directory first, got %+v", intent, res.Candidates)
		}
	}
}
