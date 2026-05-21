package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// --- layout construction helpers ---

func canonicalLayout(name string, ct types.CanonicalLayoutType, phs ...types.PlaceholderInfo) types.LayoutMetadata {
	return types.LayoutMetadata{Name: name, CanonicalType: ct, Placeholders: phs}
}

func bodyPH(maxChars int) types.PlaceholderInfo {
	return types.PlaceholderInfo{ID: "body", Type: types.PlaceholderBody, MaxChars: maxChars}
}

func titlePH() types.PlaceholderInfo {
	return types.PlaceholderInfo{ID: "title", Type: types.PlaceholderTitle, MaxChars: 60}
}

func largeImagePH() types.PlaceholderInfo {
	return types.PlaceholderInfo{
		ID:     "image",
		Type:   types.PlaceholderImage,
		Bounds: types.BoundingBox{Width: 6000000, Height: 4000000},
	}
}

// fullTemplate covers every content-bearing family plus a large image and a blank.
func fullTemplate() *types.TemplateAnalysis {
	return &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			canonicalLayout("Title", types.CanonicalLayoutTitleSlide, titlePH(), bodyPH(120)),
			canonicalLayout("Section", types.CanonicalLayoutSectionDivider, titlePH()),
			canonicalLayout("Content", types.CanonicalLayoutOneContent, titlePH(), bodyPH(400)),
			canonicalLayout("Two", types.CanonicalLayoutTwoContent, titlePH(), bodyPH(200), bodyPH(200)),
			canonicalLayout("Image", types.CanonicalLayoutBlank, largeImagePH()),
			canonicalLayout("Blank", types.CanonicalLayoutBlank),
		},
		Metadata: &types.TemplateMetadata{
			DataPalette: []string{"accent1", "accent2", "accent3"},
		},
	}
}

// minimalTemplate carries only a One Content layout (title + body): grid base
// ready, two-column derivable by split, no native divider/image/blank.
func minimalTemplate() *types.TemplateAnalysis {
	return &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			canonicalLayout("Content", types.CanonicalLayoutOneContent, titlePH(), bodyPH(400)),
		},
	}
}

func TestSupport_FullTemplate_PlaceholdersSupported(t *testing.T) {
	tc := NewTemplateSupportContext(fullTemplate(), patterns.Default())
	cases := map[string]string{
		"title":      patterns.TemplateSupportSupported,
		"section":    patterns.TemplateSupportSupported,
		"content":    patterns.TemplateSupportSupported,
		"two-column": patterns.TemplateSupportSupported,
		"image":      patterns.TemplateSupportSupported,
		"blank":      patterns.TemplateSupportSupported,
	}
	for name, want := range cases {
		ts := tc.Support(patterns.VisualCategoryPlaceholder, name, nil)
		if ts == nil {
			t.Fatalf("%s: nil support", name)
		}
		if ts.Status != want {
			t.Errorf("%s: status=%q want %q (reasons=%v)", name, ts.Status, want, ts.Reasons)
		}
		if len(ts.Reasons) == 0 {
			t.Errorf("%s: expected at least one reason", name)
		}
	}
}

func TestSupport_MinimalTemplate_TwoColumnRisky(t *testing.T) {
	tc := NewTemplateSupportContext(minimalTemplate(), patterns.Default())

	two := tc.Support(patterns.VisualCategoryPlaceholder, "two-column", nil)
	if two.Status != patterns.TemplateSupportRisky {
		t.Errorf("two-column on One-Content-only template: status=%q want risky (reasons=%v)", two.Status, two.Reasons)
	}
	if two.RequiredLayout != "Two Content" {
		t.Errorf("two-column required_layout=%q want %q", two.RequiredLayout, "Two Content")
	}

	// No native divider, but no title-slide/blank-title to repurpose → unsupported.
	section := tc.Support(patterns.VisualCategoryPlaceholder, "section", nil)
	if section.Status != patterns.TemplateSupportUnsupported {
		t.Errorf("section on dividerless template: status=%q want unsupported (reasons=%v)", section.Status, section.Reasons)
	}

	// One Content layout has a large body image? No → full-image only via blank;
	// minimal has neither large image nor blank → unsupported.
	img := tc.Support(patterns.VisualCategoryPlaceholder, "image", nil)
	if img.Status != patterns.TemplateSupportUnsupported {
		t.Errorf("image on template with no large image / blank: status=%q want unsupported (reasons=%v)", img.Status, img.Reasons)
	}
}

func TestSupport_NoGridBase_OverlaysUnsupported(t *testing.T) {
	// A template whose only layout has just a large image placeholder (no title,
	// no body, no blank-title) cannot host an overlaid shape_grid.
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			canonicalLayout("ImageOnly", types.CanonicalLayoutBlank, largeImagePH()),
		},
	}
	tc := NewTemplateSupportContext(analysis, patterns.Default())

	for _, cat := range []patterns.VisualCategory{
		patterns.VisualCategoryPattern,
		patterns.VisualCategoryChart,
		patterns.VisualCategoryDiagram,
		patterns.VisualCategoryShapeGrid,
		patterns.VisualCategoryCompose,
	} {
		ts := tc.Support(cat, "kpi-3up", nil)
		if ts.Status != patterns.TemplateSupportUnsupported {
			t.Errorf("category %s on grid-baseless template: status=%q want unsupported (reasons=%v)", cat, ts.Status, ts.Reasons)
		}
		if ts.RequiredLayout != "grid base" {
			t.Errorf("category %s required_layout=%q want %q", cat, ts.RequiredLayout, "grid base")
		}
	}
}

func TestSupport_TinyBodyCapacity_ContentRisky(t *testing.T) {
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			canonicalLayout("Content", types.CanonicalLayoutOneContent, titlePH(), bodyPH(40)),
		},
	}
	tc := NewTemplateSupportContext(analysis, patterns.Default())

	ts := tc.Support(patterns.VisualCategoryPlaceholder, "content", nil)
	if ts.Status != patterns.TemplateSupportRisky {
		t.Errorf("content on tiny-body template: status=%q want risky (reasons=%v)", ts.Status, ts.Reasons)
	}
}

func TestSupport_ItemCountVsCapacity(t *testing.T) {
	// Body holds ~200 chars; 10 items × ~40 chars/item = 400 → risky.
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			canonicalLayout("Content", types.CanonicalLayoutOneContent, titlePH(), bodyPH(200)),
		},
	}
	tc := NewTemplateSupportContext(analysis, patterns.Default())
	hints := &patterns.VisualHints{ContentHints: patterns.ContentHints{ItemCount: 10}}

	ts := tc.Support(patterns.VisualCategoryPlaceholder, "content", hints)
	if ts.Status != patterns.TemplateSupportRisky {
		t.Errorf("content with 10 items vs 200-char body: status=%q want risky (reasons=%v)", ts.Status, ts.Reasons)
	}

	// A modest item count fits comfortably → stays supported.
	tsOk := tc.Support(patterns.VisualCategoryPlaceholder, "content", &patterns.VisualHints{ContentHints: patterns.ContentHints{ItemCount: 3}})
	if tsOk.Status != patterns.TemplateSupportSupported {
		t.Errorf("content with 3 items vs 200-char body: status=%q want supported (reasons=%v)", tsOk.Status, tsOk.Reasons)
	}
}

func TestSupport_ChartPaletteReason(t *testing.T) {
	tc := NewTemplateSupportContext(fullTemplate(), patterns.Default())
	ts := tc.Support(patterns.VisualCategoryChart, "bar", nil)
	if ts.Status != patterns.TemplateSupportSupported {
		t.Fatalf("chart on full template: status=%q want supported", ts.Status)
	}
	found := false
	for _, r := range ts.Reasons {
		if strings.Contains(r, "data_palette") {
			found = true
		}
	}
	if !found {
		t.Errorf("chart support should mention data_palette; reasons=%v", ts.Reasons)
	}
}

func TestAnnotateTemplateSupport_SetsField(t *testing.T) {
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Category: patterns.VisualCategoryPlaceholder, Name: "title", Score: 0.9},
			{Category: patterns.VisualCategoryChart, Name: "bar", Score: 0.8},
		},
	}
	AnnotateTemplateSupport(result, fullTemplate(), nil, patterns.Default())
	for _, c := range result.Candidates {
		if c.TemplateSupport == nil {
			t.Errorf("candidate %q: template_support not set", c.Name)
		}
	}
}

func TestAnnotateTemplateSupport_NilAnalysisNoop(t *testing.T) {
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{{Category: patterns.VisualCategoryChart, Name: "bar", Score: 0.8}},
	}
	AnnotateTemplateSupport(result, nil, nil, patterns.Default())
	if result.Candidates[0].TemplateSupport != nil {
		t.Error("nil analysis should leave template_support nil")
	}
}

func TestReorderByTemplateSupport_DemotesUnsupported(t *testing.T) {
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Name: "unsupported-top", Score: 0.90, TemplateSupport: &patterns.TemplateSupport{Status: patterns.TemplateSupportUnsupported}},
			{Name: "supported-mid", Score: 0.70, TemplateSupport: &patterns.TemplateSupport{Status: patterns.TemplateSupportSupported}},
		},
	}
	ReorderByTemplateSupport(result)
	if result.Candidates[0].Name != "supported-mid" {
		t.Errorf("expected supported candidate first after reorder; got %q", result.Candidates[0].Name)
	}
	// Score field must be left untouched (transparency).
	for _, c := range result.Candidates {
		if c.Name == "unsupported-top" && c.Score != 0.90 {
			t.Errorf("reorder must not mutate Score; got %v", c.Score)
		}
	}
}

func TestReorderByTemplateSupport_StrongIntentBeatsWeakSupported(t *testing.T) {
	// A risky candidate with a much stronger intent score should still outrank a
	// weak supported one (penalty is modest, not absolute).
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Name: "weak-supported", Score: 0.55, TemplateSupport: &patterns.TemplateSupport{Status: patterns.TemplateSupportSupported}},
			{Name: "strong-risky", Score: 0.95, TemplateSupport: &patterns.TemplateSupport{Status: patterns.TemplateSupportRisky}},
		},
	}
	ReorderByTemplateSupport(result)
	if result.Candidates[0].Name != "strong-risky" {
		t.Errorf("strong risky (0.95-0.15=0.80) should beat weak supported (0.55); got %q first", result.Candidates[0].Name)
	}
}

func TestReorderByTemplateSupport_NoAnnotationNoop(t *testing.T) {
	result := &patterns.RecommendVisualResult{
		Candidates: []patterns.VisualCandidate{
			{Name: "a", Score: 0.5},
			{Name: "b", Score: 0.9},
		},
	}
	ReorderByTemplateSupport(result)
	// Order unchanged (no annotations present).
	if result.Candidates[0].Name != "a" {
		t.Errorf("no-annotation reorder should be a no-op; got %q first", result.Candidates[0].Name)
	}
}
