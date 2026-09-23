package generator

import (
	"archive/zip"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDefaultTableStyleResolver_Empty(t *testing.T) {
	r := defaultTableStyleResolver{}
	got := r.ResolveTableStyleID("")
	if got != types.DefaultTableStyleID {
		t.Errorf("got %q, want %q", got, types.DefaultTableStyleID)
	}
}

func TestDefaultTableStyleResolver_Passthrough(t *testing.T) {
	r := defaultTableStyleResolver{}
	guid := "{ABC}"
	got := r.ResolveTableStyleID(guid)
	if got != guid {
		t.Errorf("got %q, want %q", got, guid)
	}
}

// Regression for the @template-default sentinel leaking into output. When no
// template-aware resolver is supplied (e.g. a standalone table render), the
// default resolver must still translate the sentinel to the engine-default
// GUID; otherwise the generated <a:tableStyleId> contains an invalid value
// and LibreOffice refuses to open the file.
func TestDefaultTableStyleResolver_TemplateDefaultSentinel(t *testing.T) {
	r := defaultTableStyleResolver{}
	got := r.ResolveTableStyleID("@template-default")
	if got != types.DefaultTableStyleID {
		t.Errorf("got %q, want %q", got, types.DefaultTableStyleID)
	}
}

func TestTemplateTableStyleResolverMissingTemplateFails(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	if _, err := ctx.templateTableStyleResolver(); err == nil {
		t.Fatal("missing template ZIP must fail instead of silently using engine default")
	}
	if ctx.tableStyleReader != nil {
		t.Error("failed lookup cached a resolver")
	}
}

func TestTemplateTableStyleResolverIsReused(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templatePath = "../../templates/modern.pptx"
	var err error
	ctx.templateReader, err = zip.OpenReader(ctx.templatePath)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.templateReader.Close()
	first, err := ctx.templateTableStyleResolver()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.tableStyleReader.Close()
	ctx.templatePath = filepath.Join(t.TempDir(), "missing.pptx")
	second, err := ctx.templateTableStyleResolver()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("template table-style resolver was recreated instead of reused")
	}
	if got := second.ResolveTableStyleID(template.TemplateDefaultSentinel); got == "" {
		t.Error("borrowed resolver did not read the open ZIP")
	}
}

func TestPrepareImagesDoesNotRenderTableWithUnresolvedTemplateStyle(t *testing.T) {
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.templatePath = filepath.Join(t.TempDir(), "missing.pptx")
	ctx.templateSlideData[1] = &slideXML{CommonSlideData: commonSlideDataXML{
		ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
			NonVisualProperties: nonVisualPropertiesXML{
				ConnectionNonVisual: connectionNonVisualXML{Name: "body"},
				NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
			},
		}}},
	}}
	ctx.slideContentMap[1] = SlideSpec{Content: []ContentItem{{
		PlaceholderID: "body",
		Type:          ContentTable,
		Value: &types.TableSpec{
			Headers: []string{"A"},
			Style:   types.TableStyle{StyleID: "@template-default"},
		},
	}}}
	if err := ctx.prepareImages(); err == nil {
		t.Fatal("expected table style resolution failure to stop generation")
	}
	if len(ctx.tableInserts[1]) != 0 {
		t.Error("table was rendered despite missing template style resolver")
	}
}

// stubResolver lets tests control resolved style IDs.
type stubResolver struct {
	resolved string
}

func (s stubResolver) ResolveTableStyleID(_ string) string { return s.resolved }

func TestPopulateTableInShape_WithResolver(t *testing.T) {
	// A well-formed OOXML table style GUID — the render sink only emits
	// GUID-shaped style IDs (see types.IsValidTableStyleID).
	customGUID := "{ABCDEF01-1234-5678-9ABC-DEF012345678}"
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.TableStyle{
			StyleID: "@template-default",
			Borders: "all",
		},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
		Bounds: types.BoundingBox{
			X: 914400, Y: 914400,
			Width: 5486400, Height: 3657600,
		},
	}

	result, err := PopulateTableInShape(table, placeholder, nil, stubResolver{resolved: customGUID})
	if err != nil {
		t.Fatalf("PopulateTableInShape: %v", err)
	}
	if !strings.Contains(result.XML, customGUID) {
		t.Errorf("XML should contain resolved GUID %s", customGUID)
	}
}

func TestPopulateTableInShape_NilResolverUsesDefault(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"A", "B"},
		Rows: [][]types.TableCell{
			{{Content: "1", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
		Bounds: types.BoundingBox{
			X: 914400, Y: 914400,
			Width: 5486400, Height: 3657600,
		},
	}

	result, err := PopulateTableInShape(table, placeholder, nil, nil)
	if err != nil {
		t.Fatalf("PopulateTableInShape: %v", err)
	}
	if !strings.Contains(result.XML, types.DefaultTableStyleID) {
		t.Errorf("XML should contain default style ID %s", types.DefaultTableStyleID)
	}
}
