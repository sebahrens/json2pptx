package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

var shapeOffYRegexp = regexp.MustCompile(`<a:off x="-?\d+" y="(\d+)"`)

// go-slide-creator-yhzxt: at the default placement a sparse content-sized
// block is composed — one type step up and the optical centre of the content
// area — and raises neither SLIDE_UNDERUSED, VERTICAL_IMBALANCE nor a
// content-top grid_violation; a dense block keeps its sizes and its place.
func TestSparsePatternBlockIsComposed(t *testing.T) {
	templates := []string{"midnight-blue", "modern-template"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	sparse := `{"layout_id":"content","pattern":{"name":"kpi-3up","values":[{"big":"42%","small":"Faster resolution"},{"big":"$3.1M","small":"Annual savings"},{"big":"4.6","small":"CSAT score"}]}}`
	dense := `{"layout_id":"content","pattern":{"name":"labeled-rows","values":{"rows":[` +
		`{"label":"WHY","body":"The industry grew 11% in 2025, but 80% of revenue growth came from market performance while fee compression continues across every segment we serve."},` +
		`{"label":"WHAT","body":"Rebuilt with AI at its core: 2x to 5x research coverage, 3x client coverage per relationship manager, and 300 to 500 bps of value across the P&L."},` +
		`{"label":"HOW","body":"Three plays, starting with two or three workflows reimagined end to end, owned by the business and funded from the savings of the first wave."},` +
		`{"label":"WHEN","body":"Wave one in the first two quarters, wave two by year end, and the operating model changes that hold the gains in place the year after."},` +
		`{"label":"WHO","body":"A joint team of business owners, engineers and risk partners, with one accountable executive per workflow and a single steering group."}]}}}`
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tctx.reader.Close() }()
			in := geomSlides(t, "["+sparse+","+dense+"]")
			title := "Results so far"
			for i := range in.Slides {
				in.Slides[i].Content = []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}
			}
			for _, f := range collectFitFindings(in, tctx.layouts, tctx.slideWidth, tctx.slideHeight, nil) {
				// The composed block's top is off the frame's content line on
				// purpose; that must not read as a grid_violation.
				composedFrame := f.Code == "grid_violation" && strings.HasPrefix(f.Path, "/slides/0/")
				if f.Code == patterns.ErrCodeSlideUnderused || f.Code == patterns.ErrCodeVerticalImbalance || composedFrame {
					t.Errorf("%s: %s %s", f.Path, f.Code, f.Message)
				}
			}
			expanded, _ := expandPatternsForFit(in, tctx.slideWidth, tctx.slideHeight, nil, tctx.layouts...)
			for si, wantComposed := range []bool{true, false} {
				slide := expanded.Slides[si]
				geom := resolveGridGeometry(slide, tctx.layouts, tctx.slideWidth, tctx.slideHeight)
				res := resolveGridForStructural(slide.ShapeGrid, geom.OverrideBounds, geom.Zone, tctx.slideWidth, tctx.slideHeight)
				if res == nil {
					t.Fatalf("slide %d did not resolve", si)
				}
				if res.Composed != wantComposed {
					t.Fatalf("slide %d: composed = %v, want %v", si, res.Composed, wantComposed)
				}
				area := resolveGridBounds(slide.ShapeGrid, geom.OverrideBounds, geom.Zone, tctx.slideWidth, tctx.slideHeight)
				top, bottom := blockExtent(res.Cells)
				above, below := top-area.Y, area.Y+area.CY-bottom
				if !wantComposed {
					if geom.Zone != nil && geom.Zone.BodyTop > 0 && top > max(geom.Zone.BodyTop, area.Y)+bodyLineToleranceEMU {
						t.Errorf("dense block starts %.1fpt below the body line", float64(top-geom.Zone.BodyTop)/12700)
					}
					continue
				}
				// Optical centre: a little more room below than above.
				if share := float64(above) / float64(above+below); share < 0.35 || share > 0.5 {
					t.Errorf("sparse block has %.0f%% of its spare height above it (above %.0fpt, below %.0fpt), want about %.0f%%",
						share*100, float64(above)/12700, float64(below)/12700, shapegrid.ComposeOpticalTop*100)
				}
				// One step up: the caption leaves the 14pt subhead step.
				stepped := false
				for _, c := range res.Cells {
					if c.ShapeSpec != nil && strings.Contains(string(c.ShapeSpec.Text), `"size":18`) {
						stepped = true
					}
				}
				if !stepped {
					t.Errorf("sparse KPI row kept its type sizes; want the caption stepped to 18pt")
				}
			}
		})
	}
}

// go-slide-creator-e17xy: a short pattern block placed with vertical_align
// "top" starts where the template's native body text starts (the body
// placeholder top), not hard under the title. The default placement composes
// a sparse block instead (TestSparsePatternBlockIsComposed).
func TestShortPatternStartsAtBodyPlaceholderTop(t *testing.T) {
	templates := []string{"abstract", "forest-green", "modern-template"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tctx.reader.Close() }()
			title := "Card grid beside a native title"
			slide := SlideInput{
				LayoutID: "content",
				Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
				Pattern: &PatternInput{Name: "card-grid", VerticalAlign: "top", Values: json.RawMessage(`{"columns":3,"rows":1,"cells":[` +
					`{"header":"People","body":"Retrain 40 agents on the unified playbook"},` +
					`{"header":"Process","body":"Single intake form with skill-based routing"},` +
					`{"header":"Technology","body":"Consolidate onto one ITSM platform"}]}`)},
			}
			specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.metadata, nil, "", nil, false)
			if err != nil {
				t.Fatal(err)
			}
			layout := findLayoutByID(tctx.layouts, specs[0].LayoutID)
			if layout == nil {
				t.Fatalf("layout %q not found", specs[0].LayoutID)
			}
			body, ok := firstBodyOrContentBounds(layout)
			if !ok {
				t.Skipf("layout %q has no body placeholder", layout.ID)
			}
			top := int64(-1)
			for _, sh := range specs[0].RawShapeXML {
				for _, m := range shapeOffYRegexp.FindAllSubmatch(sh, -1) {
					y, _ := strconv.ParseInt(string(m[1]), 10, 64)
					if top < 0 || y < top {
						top = y
					}
				}
			}
			if top < 0 {
				t.Fatal("no pattern shapes rendered")
			}
			if d := float64(top-body.Y) / 12700; d < -6 || d > 6 {
				t.Errorf("pattern block starts %.1fpt from the body placeholder top (want within 6pt)", d)
			}
		})
	}
}

// go-slide-creator-e17xy (a): a full-height (stretch) pattern under a short,
// measured title starts at the native body line too — not hard under the
// title text — except where the template's body placeholder sits further
// below its title box than gridChromeGapPt: starting there would take height
// the schema-maxima pins need, so those templates start full-area grids at
// the title-box line (bodyStartTop).
func TestStretchPatternStartsAtBodyLine(t *testing.T) {
	templates := []string{"abstract", "blue-corporate", "business-template", "forest-green", "midnight-blue", "modern", "modern-template", "modern-yellow", "warm-coral"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	cases := []struct{ pattern, values string }{
		{"comparison-2col", `{"header_left":"Today","header_right":"Target","rows":[` +
			`{"left":"Manual intake by email","right":"Single intake form"},` +
			`{"left":"Five ticketing tools","right":"One ITSM platform"},` +
			`{"left":"No routing rules","right":"Skill-based routing"}]}`},
		{"kpi-3up", `[{"big":"42%","small":"Faster resolution"},{"big":"$3.1M","small":"Annual savings"},{"big":"4.6","small":"CSAT score"}]`},
	}
	for _, name := range templates {
		tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", name+".pptx"))
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range cases {
			t.Run(name+"/"+tc.pattern, func(t *testing.T) {
				title := "Short title"
				slide := SlideInput{
					LayoutID: "content",
					Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
					// Placed "top": the default composes these sparse blocks.
					Pattern: &PatternInput{Name: tc.pattern, VerticalAlign: "top", Values: json.RawMessage(tc.values)},
				}
				specs, _, _, err := convertPresentationSlides([]SlideInput{slide}, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.metadata, nil, "", nil, false)
				if err != nil {
					t.Fatal(err)
				}
				layout := findLayoutByID(tctx.layouts, specs[0].LayoutID)
				if layout == nil {
					t.Fatalf("layout %q not found", specs[0].LayoutID)
				}
				titlePH := layoutTitlePlaceholder(layout)
				if titlePH == nil {
					t.Skip("layout has no title")
				}
				anchor, ok := bodyAnchorTop(layout, titlePH, tctx.layouts)
				if !ok {
					t.Skipf("layout %q has no body line", layout.ID)
				}
				top := int64(-1)
				for _, sh := range specs[0].RawShapeXML {
					for _, m := range shapeOffYRegexp.FindAllSubmatch(sh, -1) {
						y, _ := strconv.ParseInt(string(m[1]), 10, 64)
						if top < 0 || y < top {
							top = y
						}
					}
				}
				if top < 0 {
					t.Fatal("no pattern shapes rendered")
				}
				want := min64(anchor, titlePH.Bounds.Y+titlePH.Bounds.Height+int64(gridChromeGapPt*12700))
				if d := float64(top-want) / 12700; d < -2 {
					t.Errorf("pattern starts %.1fpt above its start line (body line %.1fpt, start line %.1fpt)", -d, float64(anchor)/12700, float64(want)/12700)
				}
				if name == "abstract" || name == "forest-green" || name == "modern-template" {
					if d := float64(top-anchor) / 12700; d < -6 || d > 6 {
						t.Errorf("pattern starts %.1fpt from the body placeholder top (want within 6pt)", d)
					}
				}
			})
		}
		_ = tctx.reader.Close()
	}
}

// go-slide-creator-e17xy (b): the roadmap, timeline and team kind examples
// raise no SLIDE_UNDERUSED / VERTICAL_IMBALANCE at their default placement (a
// dense block on the body line, a sparse one composed —
// go-slide-creator-yhzxt), and placed with vertical_align "top" they hang
// from the body line: their content tops agree within ~10pt on blank-title.
func TestKindDefaultsHangFromBodyLine(t *testing.T) {
	templates := []string{"abstract", "midnight-blue"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	slidesJSON := `[
{"layout_id":"blank-title","takeaway":"Global availability by Q3.","pattern":{"name":"phase-roadmap","values":{"phases":[{"name":"Pilot","date_label":"Q1","description":"Two lighthouse customers"},{"name":"Expand","date_label":"Q2","description":"All EMEA accounts"},{"name":"Scale","date_label":"Q3","description":"Global availability"}]}}},
{"layout_id":"blank-title","takeaway":"Three years of runway, two of them already spent.","pattern":{"name":"timeline-horizontal","values":[{"label":"Mandate published","date":"Mar 2024","body":"The regulator sets the T+1 date."},{"label":"Programme approved","date":"Sep 2024","body":"Board funds the first two waves."},{"label":"Wave 1 live","date":"Jun 2025","body":"Two of the three clearers migrated."},{"label":"Deadline","date":"May 2027","body":"All settlement on the new platform."}]}},
{"layout_id":"blank-title","pattern":{"name":"team-bios","values":{"members":[{"name":"Amara Okafor","role":"Engagement partner","bio":"Led the 2024 settlement migration for two of the three largest clearers."},{"name":"Jonas Weber","role":"Delivery lead","bio":"Ten years in payments platform delivery; runs the cutover rehearsals."},{"name":"Priya Raman","role":"Data lead","bio":"Owns the reconciliation model and the migration waves."}]}}},
{"layout_id":"blank-title","pattern":{"name":"card-grid","values":{"columns":3,"rows":1,"cells":[{"header":"People","body":"Retrain 40 agents"},{"header":"Process","body":"Single intake form"},{"header":"Technology","body":"One ITSM platform"}]}}}
]`
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			tctx, err := loadPreviewTemplate(filepath.Join("..", "..", "templates", name+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tctx.reader.Close() }()
			in := geomSlides(t, slidesJSON)
			for i := range in.Slides {
				title := "Rollout plan"
				in.Slides[i].Content = []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}
			}
			for _, f := range collectGeometryFindings(in, tctx.layouts, tctx.slideWidth, tctx.slideHeight, nil) {
				if f.Code == patterns.ErrCodeSlideUnderused || f.Code == patterns.ErrCodeVerticalImbalance {
					t.Errorf("%s: %s %s", f.Path, f.Code, f.Message)
				}
			}
			for i := range in.Slides {
				in.Slides[i].Pattern.VerticalAlign = "top"
			}
			specs, _, _, err := convertPresentationSlides(in.Slides, tctx.layouts, tctx.slideWidth, tctx.slideHeight, tctx.metadata, nil, "", nil, false)
			if err != nil {
				t.Fatal(err)
			}
			var lo, hi int64 = -1, -1
			for si, spec := range specs {
				top := int64(-1)
				for _, sh := range spec.RawShapeXML {
					for _, m := range shapeOffYRegexp.FindAllSubmatch(sh, -1) {
						y, _ := strconv.ParseInt(string(m[1]), 10, 64)
						if top < 0 || y < top {
							top = y
						}
					}
				}
				if top < 0 {
					t.Fatalf("slide %d rendered no pattern shapes", si)
				}
				if lo < 0 || top < lo {
					lo = top
				}
				if top > hi {
					hi = top
				}
			}
			if d := float64(hi-lo) / 12700; d > 10 {
				t.Errorf("content tops differ by %.1fpt across kinds (want within 10pt)", d)
			}
		})
	}
}
