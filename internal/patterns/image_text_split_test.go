package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func imageTextSplitPattern(t *testing.T) Pattern {
	t.Helper()
	p, ok := Default().Get("image-text-split")
	if !ok {
		t.Fatal("image-text-split not registered")
	}
	return p
}

func TestImageTextSplit_Metadata(t *testing.T) {
	p := imageTextSplitPattern(t)
	if p.Version() != 1 || p.UseWhen() == "" || p.NotWhen() == "" || p.CellsHint() == "" {
		t.Fatal("metadata incomplete")
	}
	tax := p.Taxonomy()
	if tax.Category == "" || tax.DensityClass == "" || tax.AccentWeight == "" || len(tax.NarrativeRole) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tax)
	}
	data, err := json.Marshal(p.Schema())
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), "2020-12") {
		t.Fatalf("schema invalid: %v", err)
	}
	if _, ok := p.(ImageAssetPattern); !ok {
		t.Fatal("image-text-split must expose its image for host path/url resolution")
	}
}

func TestImageTextSplit_Validate(t *testing.T) {
	p := imageTextSplitPattern(t)
	if err := p.Validate(p.(Exemplar).ExemplarValues(), nil, nil); err != nil {
		t.Fatalf("exemplar must validate: %v", err)
	}
	long := func(n int) string { return strings.Repeat("x", n) }
	cases := []struct {
		name string
		vals *ImageTextSplitValues
		ovr  *ImageTextSplitOverrides
		co   map[int]any
		want string
	}{
		{name: "no narrative", vals: &ImageTextSplitValues{Heading: "Only a heading"}, want: "body"},
		{name: "empty image", vals: &ImageTextSplitValues{Body: "b", Image: &jsonschema.GridImageInput{Alt: "x"}}, want: "path or url"},
		{name: "image overlay not allowed", vals: &ImageTextSplitValues{Body: "b", Image: &jsonschema.GridImageInput{Path: "a.png", Overlay: &jsonschema.GridOverlayInput{Color: "dk1"}}}, want: "overlay"},
		{name: "heading too long", vals: &ImageTextSplitValues{Body: "b", Heading: long(itsHeadingMax + 1)}, want: "heading"},
		{name: "body too long", vals: &ImageTextSplitValues{Body: long(itsBodyMax + 1)}, want: "body"},
		{name: "too many bullets", vals: &ImageTextSplitValues{Bullets: []string{"a", "b", "c", "d", "e", "f"}}, want: "bullets"},
		{name: "blank bullet", vals: &ImageTextSplitValues{Bullets: []string{"a", " "}}, want: "bullets[1]"},
		{name: "too many metrics", vals: &ImageTextSplitValues{Body: "b", Metrics: []ImageTextSplitMetric{{"1", "a"}, {"2", "b"}, {"3", "c"}, {"4", "d"}}}, want: "kpi-Nup"},
		{name: "metric value too long", vals: &ImageTextSplitValues{Body: "b", Metrics: []ImageTextSplitMetric{{long(11), "a"}}}, want: "metrics[0].value"},
		{name: "metric label missing", vals: &ImageTextSplitValues{Body: "b", Metrics: []ImageTextSplitMetric{{"1", ""}}}, want: "metrics[0].label"},
		{name: "bad side", vals: &ImageTextSplitValues{Body: "b"}, ovr: &ImageTextSplitOverrides{ImageSide: "top"}, want: "image_side"},
		{name: "bad width", vals: &ImageTextSplitValues{Body: "b"}, ovr: &ImageTextSplitOverrides{ImageWidthPct: 80}, want: "image_width_pct"},
		{name: "cell overrides rejected", vals: &ImageTextSplitValues{Body: "b"}, co: map[int]any{0: &CellOverride{}}, want: "cell_overrides"},
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

func TestImageTextSplit_PlaceholderExpand(t *testing.T) {
	p := imageTextSplitPattern(t)
	vals := p.(Exemplar).ExemplarValues().(*ImageTextSplitValues)
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 1 || len(grid.Rows[0].Cells) != 2 {
		t.Fatalf("want one row with image + text columns, got %+v", grid.Rows)
	}
	row := grid.Rows[0]
	_, areaH := sizingAreaPt(fullThemeCtx())
	if row.MaxHeight <= 0 || row.MinHeight != row.MaxHeight || row.MaxHeight > areaH {
		t.Errorf("row should be point-capped within the content area, got min=%v max=%v (area %v)", row.MinHeight, row.MaxHeight, areaH)
	}
	ph := row.Cells[0]
	if ph.Shape == nil || !strings.Contains(string(ph.Shape.Line), "dash") {
		t.Fatalf("placeholder should be a dashed wireframe shape: %+v", ph)
	}
	if paras := cellText(t, ph.Shape.Text).Paragraphs; len(paras) != 2 || paras[1].Content != vals.ImageLabel {
		t.Errorf("placeholder text = %+v", paras)
	}
	// Text column: nested grid (text + metrics) because metrics are present.
	txt := row.Cells[1]
	if txt.Grid == nil || len(txt.Grid.Rows) != 2 {
		t.Fatalf("text column should nest text + metrics rows: %+v", txt)
	}
	body := cellText(t, txt.Grid.Rows[0].Cells[0].Shape.Text).Paragraphs
	if body[0].Content != "CASE STUDY" || !body[0].Bold {
		t.Errorf("eyebrow should be bold uppercase, got %+v", body[0])
	}
	for _, para := range body {
		if para.Size < 12 {
			t.Errorf("paragraph below 12pt: %+v", para)
		}
	}
	metrics := txt.Grid.Rows[1].Cells[0].Grid
	if metrics == nil || len(metrics.Rows[0].Cells) != 3 {
		t.Fatalf("metric row should hold 3 metric cells: %+v", txt.Grid.Rows[1])
	}
	m := metrics.Rows[0].Cells[0]
	if m.AccentBar == nil || m.AccentBar.Position != "top" {
		t.Errorf("metric cells carry a top accent rule: %+v", m.AccentBar)
	}
}

func TestImageTextSplit_ImageRightAndCaption(t *testing.T) {
	p := imageTextSplitPattern(t)
	vals := &ImageTextSplitValues{
		Image:   &jsonschema.GridImageInput{Path: "/tmp/photo.jpg"},
		Caption: "Northern DC after the rollout",
		Heading: "Heading",
		Body:    "Body copy.",
	}
	grid, err := p.Expand(fullThemeCtx(), vals, &ImageTextSplitOverrides{ImageSide: "right", ImageWidthPct: 40}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 2 || cols[1] != 40 {
		t.Fatalf("image column should be on the right at 40%%, got %s", grid.Columns)
	}
	imgCol := grid.Rows[0].Cells[1]
	if imgCol.Grid == nil || len(imgCol.Grid.Rows) != 2 {
		t.Fatalf("caption should nest under the image: %+v", imgCol)
	}
	pic := imgCol.Grid.Rows[0].Cells[0]
	if pic.Image == nil || pic.Image.Path != "/tmp/photo.jpg" || pic.Image.Alt != vals.Caption {
		t.Errorf("image cell = %+v (alt should default to the caption)", pic.Image)
	}
	if pic.Fit != "" {
		t.Errorf("image cell must not set fit so it fills (cover) its cell, got %q", pic.Fit)
	}
	// No metrics: text column is a plain text cell.
	if grid.Rows[0].Cells[0].Shape == nil {
		t.Error("text column without metrics should be a single text cell")
	}
}

func TestImageTextSplit_DenseStepsDownTypeScale(t *testing.T) {
	p := imageTextSplitPattern(t)
	bullets := make([]string, itsMaxBullets)
	for i := range bullets {
		bullets[i] = strings.Repeat("Evidence words ", 9)[:130]
	}
	vals := &ImageTextSplitValues{Heading: strings.Repeat("Heading words ", 6)[:78], Body: strings.Repeat("Body sentence words. ", 14)[:290], Bullets: bullets}
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	paras := cellText(t, grid.Rows[0].Cells[1].Shape.Text).Paragraphs
	if paras[0].Size >= 20 {
		t.Errorf("dense text should step the heading below 20pt, got %v", paras[0].Size)
	}
	for _, para := range paras {
		if para.Size < 12 {
			t.Errorf("step-down must not go below 12pt: %+v", para)
		}
	}
}

func TestImageTextSplit_ImageAssets(t *testing.T) {
	p := imageTextSplitPattern(t).(ImageAssetPattern)
	vals := &ImageTextSplitValues{Image: &jsonschema.GridImageInput{Path: "rel/photo.jpg"}}
	refs := p.ImageAssets(vals)
	if len(refs) != 1 || refs[0].Field != "image" || refs[0].Image != vals.Image {
		t.Fatalf("refs = %+v", refs)
	}
	refs[0].Image.Path = "/abs/photo.jpg"
	if vals.Image.Path != "/abs/photo.jpg" {
		t.Error("refs must point into the values so hosts can rewrite paths")
	}
	if got := p.ImageAssets(&ImageTextSplitValues{}); len(got) != 0 {
		t.Errorf("no image → no refs, got %+v", got)
	}
}

func TestImageTextSplit_Recommend(t *testing.T) {
	res := Recommend(Default(), "customer case study with a photo", nil, 3)
	if len(res.Candidates) == 0 || res.Candidates[0].PatternName != "image-text-split" {
		t.Errorf("case-study intent should rank image-text-split first, got %+v", res.Candidates)
	}
}
