package generator

import (
	"archive/zip"
	"context"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestUnmarkNestedUnderUnmarkedBase_Synthetic(t *testing.T) {
	lvl0, lvl1 := 0, 1
	ls := &listStyleXML{Inner: `<a:lvl1pPr marL="0" indent="0"><a:buNone/></a:lvl1pPr><a:lvl2pPr marL="685800" indent="-285750"><a:buFont typeface="Arial"/><a:buChar char="•"/><a:defRPr sz="1600"/></a:lvl2pPr>`}
	paras := []paragraphXML{
		{Properties: &paragraphPropertiesXML{Level: &lvl0}},
		{Properties: &paragraphPropertiesXML{Level: &lvl1, Inner: `<a:buFont typeface="Arial"/><a:buChar char="•"/><a:defRPr sz="1600"/>`}},
	}
	unmarkNestedUnderUnmarkedBase(paras, ls, 0)
	child := paras[1].Properties
	if strings.Contains(child.Inner, "buChar") || strings.Contains(child.Inner, "buFont") {
		t.Errorf("nested paragraph keeps a marker: %s", child.Inner)
	}
	if child.Inner != `<a:buNone/><a:defRPr sz="1600"/>` {
		t.Errorf("buNone not inserted before defRPr: %s", child.Inner)
	}
	if child.MarL == nil || *child.MarL != nestedUnmarkedIndentEMU || child.Indent == nil || *child.Indent != 0 {
		t.Errorf("nested indent = %v/%v, want %d/0", child.MarL, child.Indent, nestedUnmarkedIndentEMU)
	}

	// A marked base level leaves nested markers alone.
	paras = []paragraphXML{
		{Properties: &paragraphPropertiesXML{Level: &lvl0, Inner: `<a:buChar char="•"/>`}},
		{Properties: &paragraphPropertiesXML{Level: &lvl1, Inner: `<a:buChar char="–"/>`}},
	}
	unmarkNestedUnderUnmarkedBase(paras, ls, 0)
	if !strings.Contains(paras[1].Properties.Inner, "buChar") {
		t.Error("nested marker removed although the parent level is marked")
	}
}

var (
	nbParaRegexp = regexp.MustCompile(`(?s)<a:p>(.*?)</a:p>`)
	nbLvlRegexp  = regexp.MustCompile(`<a:pPr[^>]*\blvl="(\d+)"`)
)

// go-slide-creator-yjl8d: on a template whose content body lvl1 is buNone
// (abstract), no nested bullet paragraph carries a buChar while its parent
// level resolves to buNone.
func TestNestedBulletsFollowUnmarkedParentOnShippedTemplates(t *testing.T) {
	for _, templatePath := range testutil.TestTemplatePaths() {
		name := filepath.Base(templatePath)
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(templatePath)
			if err != nil {
				t.Fatal(err)
			}
			layouts, err := template.ParseLayouts(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			layoutID := layout.ResolveAllCanonicalLayouts(layouts)["content"]
			if layoutID == "" {
				t.Skip("no canonical content layout")
			}
			output := filepath.Join(t.TempDir(), "nested.pptx")
			if _, err := Generate(context.Background(), GenerationRequest{
				TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: layoutID, Content: []ContentItem{{
					PlaceholderID: "body", Type: ContentBullets,
					Value: []string{"Consolidate the desks", "  Migrate ticket history", "  Retrain 40 agents", "Introduce one intake form"},
				}}}},
			}); err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			slideXML := string(zipSlideXML(t, &z.Reader, 1))
			// Find the body shape's list style and the bullet paragraphs.
			start := strings.Index(slideXML, ">Consolidate the desks<")
			if start < 0 {
				t.Fatal("body text missing")
			}
			spStart := strings.LastIndex(slideXML[:start], "<p:sp>")
			spEnd := strings.Index(slideXML[start:], "</p:sp>") + start
			sp := slideXML[spStart:spEnd]
			ls := ""
			if i := strings.Index(sp, "<a:lstStyle>"); i >= 0 {
				ls = sp[i:strings.Index(sp, "</a:lstStyle>")]
			}
			var parentUnmarked bool
			for _, m := range nbParaRegexp.FindAllStringSubmatch(sp, -1) {
				body := m[1]
				lm := nbLvlRegexp.FindStringSubmatch(body)
				if lm == nil {
					continue
				}
				lvl, _ := strconv.Atoi(lm[1])
				state := levelMarkerState(body)
				if state == "" {
					state = levelMarkerState(listStyleLevelBlock(ls, lvl))
				}
				switch {
				case strings.Contains(body, ">Consolidate the desks<"):
					parentUnmarked = state == "none"
				case strings.Contains(body, ">Migrate ticket history<") && parentUnmarked:
					if strings.Contains(body, "<a:buChar") || state == "marked" {
						t.Errorf("nested bullet is marked under an unmarked parent: %s", body)
					}
				}
			}
			if name == "abstract.pptx" && !parentUnmarked {
				t.Error("abstract's content body lvl1 should resolve to buNone (test premise)")
			}
		})
	}
}
