package generator

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestExternalHyperlinksInGeneratedDeck(t *testing.T) {
	output := filepath.Join(t.TempDir(), "links.pptx")
	_, err := Generate(context.Background(), GenerationRequest{
		TemplatePath: "../template/testdata/standard.pptx", OutputPath: output, ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{
			LayoutID: "slideLayout2", SourceNote: "Annual report", SourceLink: &LinkSpec{URL: "https://example.com/report?a=1&b=2"},
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: "Linked evidence"},
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{"<b>Revenue</b> grew", "Margins improved"}, Link: &LinkSpec{URL: "https://example.com/data"}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	slide := string(readZipEntry(t, zr, "ppt/slides/slide1.xml"))
	relsData := readZipEntry(t, zr, "ppt/slides/_rels/slide1.xml.rels")
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(relsData, &rels); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Revenue", "grew", "Margins improved", "Source: Annual report"} {
		if !strings.Contains(slide, want) {
			t.Errorf("slide lost %q", want)
		}
	}
	for _, want := range []string{"https://example.com/data", "https://example.com/report?a=1&b=2"} {
		found := false
		for _, rel := range rels.Relationships {
			if rel.Target != want {
				continue
			}
			found = true
			if rel.Type != pptx.RelTypeHyperlink || rel.TargetMode != "External" {
				t.Errorf("bad hyperlink relationship: %+v", rel)
			}
			if !strings.Contains(slide, `r:id="`+rel.ID+`"`) {
				t.Errorf("slide lacks hyperlink click for %q", want)
			}
		}
		if !found {
			t.Errorf("missing relationship for %q", want)
		}
	}
	if strings.Contains(slide, "json2pptx_content_link_") {
		t.Error("unresolved hyperlink marker")
	}
}

func TestExternalHyperlinkRejectsUnsafeURL(t *testing.T) {
	ctx := &singlePassContext{SlideContext: SlideContext{slideContentMap: map[int]SlideSpec{1: {
		SourceNote: "Source", SourceLink: &LinkSpec{URL: "javascript:alert(1)"},
	}}}}
	_, _, err := ctx.hyperlinkRelationships(1)
	if err == nil {
		t.Fatal("expected URL validation error")
	}
}

func TestShapeSlideJumpInGeneratedDeck(t *testing.T) {
	const marker = "json2pptx_shape_link_201"
	shape, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID: 201, Geometry: pptx.GeomRoundRect, Bounds: pptx.RectEmu{X: 1000000, Y: 2000000, CX: 2000000, CY: 500000},
		HyperlinkRelID: marker, HyperlinkAction: "ppaction://hlinksldjump",
		Text: &pptx.TextBody{Paragraphs: []pptx.Paragraph{{Runs: []pptx.Run{{Text: "Go to appendix"}}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "jump.pptx")
	_, err = Generate(context.Background(), GenerationRequest{
		TemplatePath: "../template/testdata/standard.pptx", OutputPath: output, ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{LayoutID: "slideLayout2", SpeakerNotes: "Presenter note", RawShapeXML: [][]byte{shape},
			ShapeLinks: []ShapeLink{{Marker: marker, Link: LinkSpec{Slide: 2}}}},
			{LayoutID: "slideLayout2", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Appendix"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	slide := string(readZipEntry(t, zr, "ppt/slides/slide1.xml"))
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(readZipEntry(t, zr, "ppt/slides/_rels/slide1.xml.rels"), &rels); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(slide, "Go to appendix") || !strings.Contains(slide, `action="ppaction://hlinksldjump"`) {
		t.Fatalf("slide jump shape missing text/action: %s", slide)
	}
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlide && rel.Target == "slide2.xml" {
			if rel.TargetMode != "" || !strings.Contains(slide, `r:id="`+rel.ID+`"`) {
				t.Fatalf("invalid internal click relationship: %+v", rel)
			}
			return
		}
	}
	t.Fatal("missing internal slide relationship")
}

func TestShapeSlideJumpRejectsOutOfRangeTarget(t *testing.T) {
	ctx := &singlePassContext{SlideContext: SlideContext{excludeTemplateSlides: true, slideSpecs: []SlideSpec{{}}, slideContentMap: map[int]SlideSpec{1: {
		ShapeLinks: []ShapeLink{{Marker: "badge", Link: LinkSpec{Slide: 12}}},
	}}}}
	_, _, err := ctx.hyperlinkRelationships(1)
	if err == nil || !strings.Contains(err.Error(), "outside 1..1") {
		t.Fatalf("expected target range error, got %v", err)
	}
}

func TestHyperlinkIDsFollowPreallocatedSVGIDs(t *testing.T) {
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			slideContentMap: map[int]SlideSpec{1: {SourceNote: "Source", SourceLink: &LinkSpec{URL: "https://example.com"}}},
			slideNotes:      map[int]string{1: "Notes"},
		},
		SVGContext: SVGContext{nativeSVGInserts: map[int][]nativeSVGInsert{1: {{pngRelID: "rId8", svgRelID: "rId9"}}}},
	}
	rels, _, err := ctx.hyperlinkRelationships(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 1 || rels[0].ID != "rId11" {
		t.Fatalf("hyperlink collides with preallocated SVG/notes IDs: %+v", rels)
	}
}
