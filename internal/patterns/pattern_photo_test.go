package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// gridCells walks a shape grid and returns every cell in row order, descending
// into nested grids so a test cannot be fooled by a layout that moved.
func gridCells(g *jsonschema.ShapeGridInput) []*jsonschema.GridCellInput {
	var out []*jsonschema.GridCellInput
	for _, row := range g.Rows {
		for _, c := range row.Cells {
			if c == nil {
				continue
			}
			out = append(out, c)
			if c.Grid != nil {
				out = append(out, gridCells(c.Grid)...)
			}
		}
	}
	return out
}

// imageCells returns the cells of a grid that carry a picture.
func imageCells(g *jsonschema.ShapeGridInput) []*jsonschema.GridImageInput {
	var out []*jsonschema.GridImageInput
	for _, c := range gridCells(g) {
		if c.Image != nil {
			out = append(out, c.Image)
		}
	}
	return out
}

// TestTeamBiosRendersPhotos: a member with a photo gets a real image cell; a
// member without one keeps the initials placeholder, so the two mix on one
// slide (go-slide-creator-hdpq).
func TestTeamBiosRendersPhotos(t *testing.T) {
	pat, ok := Default().Get("team-bios")
	if !ok {
		t.Fatal("team-bios not registered")
	}
	values := &TeamBiosValues{Members: []TeamBiosMember{
		{Name: "Jane Smith", Role: "Project Lead", Photo: &jsonschema.GridImageInput{Path: "jane.png"}},
		{Name: "Lila Romero", Role: "Design Lead"},
	}}
	if err := pat.Validate(values, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := pat.Expand(ExpandContext{}, values, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	imgs := imageCells(grid)
	if len(imgs) != 1 {
		t.Fatalf("expected exactly one image cell, got %d", len(imgs))
	}
	if imgs[0].Path != "jane.png" {
		t.Errorf("image path = %q, want jane.png", imgs[0].Path)
	}
	if imgs[0].Alt != "Jane Smith, Project Lead" {
		t.Errorf("alt = %q, want the member's name and role", imgs[0].Alt)
	}

	// The member without a photo still gets initials.
	encoded, _ := json.Marshal(grid)
	if !strings.Contains(string(encoded), "LR") {
		t.Error("the photo-less member should keep the initials placeholder")
	}
}

// TestTeamBiosPhotoAssetsExposed: hosts resolve a member photo's path / url
// through the same ImageAssetPattern hook image-text-split uses.
func TestTeamBiosPhotoAssetsExposed(t *testing.T) {
	pat, _ := Default().Get("team-bios")
	ia, ok := pat.(ImageAssetPattern)
	if !ok {
		t.Fatal("team-bios must implement ImageAssetPattern or its photos are never resolved")
	}
	values := &TeamBiosValues{Members: []TeamBiosMember{
		{Name: "A", Role: "R"},
		{Name: "B", Role: "R", Photo: &jsonschema.GridImageInput{URL: "https://example.com/b.png"}},
	}}
	refs := ia.ImageAssets(values)
	if len(refs) != 1 {
		t.Fatalf("expected one asset ref, got %d", len(refs))
	}
	if refs[0].Field != "members/1/photo" {
		t.Errorf("field = %q, want members/1/photo (the JSON pointer the host appends)", refs[0].Field)
	}
	// The ref must point INTO the decoded values so the host's rewrite sticks.
	refs[0].Image.Path = "/cache/b.png"
	if values.Members[1].Photo.Path != "/cache/b.png" {
		t.Error("asset ref does not alias the member's photo; a resolved path would be discarded")
	}
}

// TestPullQuoteRendersHeadshot: the picture is a sibling column of the quote,
// not a wrapper around it — a nested grid hides the quote from the readability
// preflight, which walks top-level cells (go-slide-creator-hdpq).
func TestPullQuoteRendersHeadshot(t *testing.T) {
	pat, _ := Default().Get("pull-quote")
	values := &PullQuoteValues{
		Quote:       "It paid for itself inside two quarters.",
		Attribution: "Jane Smith",
		Role:        "COO",
		Image:       &jsonschema.GridImageInput{Path: "jane.png"},
	}
	if err := pat.Validate(values, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := pat.Expand(ExpandContext{}, values, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, c := range grid.Rows[0].Cells {
		if c != nil && c.Grid != nil {
			t.Fatal("the quote must not be nested inside another grid")
		}
	}
	imgs := imageCells(grid)
	if len(imgs) != 1 {
		t.Fatalf("expected one image cell, got %d", len(imgs))
	}
	if imgs[0].Alt != "Jane Smith, COO" {
		t.Errorf("alt = %q, want the attribution and role", imgs[0].Alt)
	}
}

// TestPullQuoteHeadshotSides: image_side and accent_side place their columns
// independently, and the headshot spans both rows.
func TestPullQuoteHeadshotSides(t *testing.T) {
	pat, _ := Default().Get("pull-quote")
	base := func() *PullQuoteValues {
		return &PullQuoteValues{
			Quote: "Short quote.", Attribution: "Jane", Role: "COO",
			Image: &jsonschema.GridImageInput{Path: "jane.png"},
		}
	}
	cases := []struct {
		name      string
		accent    string
		imageSide string
		wantOrder []string // "img", "rule", "quote"
	}{
		{"defaults", "", "", []string{"img", "rule", "quote"}},
		{"image right", "", "right", []string{"rule", "quote", "img"}},
		{"accent right", "right", "", []string{"img", "quote", "rule"}},
		{"both right", "right", "right", []string{"quote", "rule", "img"}},
		{"no rule", "none", "", []string{"img", "quote"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := base()
			v.AccentSide = tc.accent
			ovr := &PullQuoteOverrides{ImageSide: tc.imageSide}
			grid, err := pat.Expand(ExpandContext{}, v, ovr, nil)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			var got []string
			for _, c := range grid.Rows[0].Cells {
				switch {
				case c.Image != nil:
					got = append(got, "img")
				case c.Shape != nil && c.Shape.Text == nil:
					got = append(got, "rule")
				default:
					got = append(got, "quote")
				}
			}
			if strings.Join(got, ",") != strings.Join(tc.wantOrder, ",") {
				t.Errorf("column order = %v, want %v", got, tc.wantOrder)
			}
			// Both furniture columns span the attribution row.
			for _, c := range grid.Rows[0].Cells {
				if c.Image != nil && c.RowSpan != 2 {
					t.Errorf("headshot row_span = %d, want 2", c.RowSpan)
				}
			}
			if n := len(grid.Rows[1].Cells); n != 1 {
				t.Errorf("attribution row lists %d cells, want just its own", n)
			}
		})
	}
}

// TestPullQuoteWithoutImageIsUnchanged: a quote with no picture keeps exactly
// the geometry it had before the field existed.
func TestPullQuoteWithoutImageIsUnchanged(t *testing.T) {
	pat, _ := Default().Get("pull-quote")
	v := &PullQuoteValues{Quote: "Short quote.", Attribution: "Jane"}
	grid, err := pat.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if n := len(grid.Rows[0].Cells); n != 2 {
		t.Errorf("quote row has %d cells, want rule + quote", n)
	}
	if imgs := imageCells(grid); len(imgs) != 0 {
		t.Errorf("no picture was given, but %d image cells were produced", len(imgs))
	}
}

// TestPatternPhotoValidation: a reference with no source, an over-long alt and
// the shape_grid-only overlay/text keys are each reported.
func TestPatternPhotoValidation(t *testing.T) {
	if errs := validatePatternPhoto("p", "values.image", nil, 200); len(errs) != 0 {
		t.Errorf("a nil photo is valid, got %v", errs)
	}
	if errs := validatePatternPhoto("p", "values.image", &jsonschema.GridImageInput{}, 200); len(errs) != 1 {
		t.Errorf("a photo with neither path nor url should be reported, got %v", errs)
	}
	long := &jsonschema.GridImageInput{Path: "a.png", Alt: strings.Repeat("x", 201)}
	if errs := validatePatternPhoto("p", "values.image", long, 200); len(errs) != 1 {
		t.Errorf("an over-long alt should be reported, got %v", errs)
	}
	overlay := &jsonschema.GridImageInput{Path: "a.png", Overlay: &jsonschema.GridOverlayInput{Color: "000000"}}
	if errs := validatePatternPhoto("p", "values.image", overlay, 200); len(errs) != 1 {
		t.Errorf("overlay belongs to shape_grid image cells, not pattern values; got %v", errs)
	}
}
