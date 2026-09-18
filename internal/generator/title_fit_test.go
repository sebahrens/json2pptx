package generator

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

const longConsultingTitle = "Procurement is fragmented across 14 business units, leaving an estimated $38-52M of annual savings uncaptured"

func readZipFileString(t *testing.T, path, name string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rc.Close() }()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	t.Fatalf("%s not found in %s", name, path)
	return ""
}

func TestParseMasterTitleStyle_ModernTemplate(t *testing.T) {
	tpl := "../../templates/modern-template.pptx"
	if _, err := os.Stat(tpl); err != nil {
		t.Skip("modern-template not found")
	}
	st := template.ParseMasterTitleStyle([]byte(readZipFileString(t, tpl, "ppt/slideMasters/slideMaster1.xml")))
	if st.SizeHPt != 4500 || !st.CapsAll || st.LineSpacingPct != 80 {
		t.Errorf("modern-template title style = %+v, want 45pt all-caps 80%% line spacing", st)
	}
	if st.Typeface != "+mj-lt" {
		t.Errorf("typeface = %q, want +mj-lt", st.Typeface)
	}
}

// go-slide-creator-6cjs: a 104-char title on modern-template (title geometry
// and 45pt all-caps style inherited from layout/master) must get a real
// measured fit — a baked-in smaller size — or a TITLE_OVERFLOW finding, not a
// bare normAutofit that renderers squash.
func TestTitleInheritedStyle_LongTitleMeasured(t *testing.T) {
	tpl := "../../templates/modern-template.pptx"
	if _, err := os.Stat(tpl); err != nil {
		t.Skip("modern-template not found")
	}
	out := filepath.Join(t.TempDir(), "out.pptx")
	req := GenerationRequest{
		TemplatePath:          tpl,
		OutputPath:            out,
		ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{
			LayoutID: "slideLayout4",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentText, Value: longConsultingTitle},
				{PlaceholderID: "body", Type: ContentBullets, Value: []string{"one", "two"}},
			},
		}},
	}
	result, _, err := generateSinglePass(context.Background(), req)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	slide := readZipFileString(t, out, "ppt/slides/slide1.xml")
	titleSp := slide[:strings.Index(slide, `name="body"`)]

	var overflow bool
	for _, f := range result.FitFindings {
		if f.Code == patterns.ErrCodeTitleOverflow {
			overflow = true
			if f.Fix == nil || f.Fix.Kind != "shorten_title" {
				t.Errorf("TITLE_OVERFLOW fix = %+v, want shorten_title", f.Fix)
			}
		}
	}
	m := regexp.MustCompile(`<a:rPr[^>]*\bsz="(\d+)"`).FindStringSubmatch(titleSp)
	scaled := false
	if m != nil {
		sz, _ := strconv.Atoi(m[1])
		scaled = sz > 0 && sz < 4500
	}
	if !scaled && !overflow {
		t.Fatalf("long title got neither a measured (baked) size below 45pt nor TITLE_OVERFLOW; title XML: %s", titleSp)
	}
	if strings.Contains(titleSp, "fontScale=") {
		t.Errorf("title fit should be baked into run sizes, not left as fontScale: %s", titleSp)
	}
}

func TestTitleShortStaysAtTemplateSize(t *testing.T) {
	tpl := "../../templates/modern-template.pptx"
	if _, err := os.Stat(tpl); err != nil {
		t.Skip("modern-template not found")
	}
	out := filepath.Join(t.TempDir(), "out.pptx")
	req := GenerationRequest{
		TemplatePath:          tpl,
		OutputPath:            out,
		ExcludeTemplateSlides: true,
		Slides: []SlideSpec{{
			LayoutID: "slideLayout4",
			Content:  []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Revenue grew 12%"}},
		}},
	}
	result, _, err := generateSinglePass(context.Background(), req)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, f := range result.FitFindings {
		if f.Code == patterns.ErrCodeTitleOverflow {
			t.Errorf("short title flagged TITLE_OVERFLOW: %s", f.Message)
		}
	}
	slide := readZipFileString(t, out, "ppt/slides/slide1.xml")
	if regexp.MustCompile(`<a:rPr[^>]*\bsz="`).MatchString(slide[:strings.Index(slide, "</p:sp>")]) {
		t.Errorf("short title should keep the inherited template size (no explicit sz)")
	}
}

func TestDetectTitleOverflow(t *testing.T) {
	st := template.InheritedTextStyle{SizeHPt: 4500, CapsAll: true, LineSpacingPct: 80}
	// A very long title in a one-line-tall band cannot fit at 60% scale.
	long := strings.Repeat(longConsultingTitle+" ", 3)
	f := DetectTitleOverflow("/slides/0/content/title", TitleFitInput{
		Title: long, WidthEMU: 10465073, HeightEMU: 700000, Style: st, FontName: "Arial",
	})
	if f == nil {
		t.Fatal("expected TITLE_OVERFLOW")
	}
	if f.Code != patterns.ErrCodeTitleOverflow || f.Action != "shrink_or_split" {
		t.Errorf("finding = %s/%s", f.Code, f.Action)
	}
	maxChars, _ := f.Fix.Params["max_chars"].(int)
	if maxChars <= 0 || maxChars >= len(long) {
		t.Errorf("max_chars = %v, want 0 < n < %d", f.Fix.Params["max_chars"], len(long))
	}
	if DetectTitleOverflow("/p", TitleFitInput{Title: "Short title", WidthEMU: 10465073, HeightEMU: 1132258, Style: st, FontName: "Arial"}) != nil {
		t.Error("short title should not overflow")
	}
}

func TestBakeTitleFit(t *testing.T) {
	shared := &paragraphPropertiesXML{Inner: `<a:lnSpc><a:spcPct val="90000"/></a:lnSpc><a:buNone/>`}
	shape := &shapeXML{TextBody: &textBodyXML{Paragraphs: []paragraphXML{
		{Properties: shared, Runs: []runXML{{Text: "a"}, {Text: "b", RunProperties: &runPropertiesXML{FontSize: "4000"}}}},
	}}}
	p := textfit.Params{FontSizeHPt: 4500, LineSpacing: baseLineSpacing * 0.8}
	bakeTitleFit(shape, p, textfit.FitResult{FontScale: 60000, LnSpcReduction: 10000})
	runs := shape.TextBody.Paragraphs[0].Runs
	if runs[0].RunProperties.FontSize != "2700" || runs[1].RunProperties.FontSize != "2400" {
		t.Errorf("baked sizes = %s,%s want 2700,2400", runs[0].RunProperties.FontSize, runs[1].RunProperties.FontSize)
	}
	inner := shape.TextBody.Paragraphs[0].Properties.Inner
	// 80% template spacing x a 10% reduction is 72%, which is below the
	// collision floor, so the baked pitch is clamped to it
	// (go-slide-creator-g5h7).
	if !strings.HasPrefix(inner, `<a:lnSpc><a:spcPct val="85000"/></a:lnSpc>`) || strings.Count(inner, "lnSpc>") != 2 {
		t.Errorf("baked lnSpc inner = %s", inner)
	}
	if shared.Inner != `<a:lnSpc><a:spcPct val="90000"/></a:lnSpc><a:buNone/>` {
		t.Error("shared paragraph properties were mutated")
	}
}
