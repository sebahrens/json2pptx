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

var (
	bgParaRegexp   = regexp.MustCompile(`(?s)<a:p>(.*?)</a:p>`)
	bgSpcBefRegexp = regexp.MustCompile(`<a:spcBef><a:spcPts val="(\d+)"/></a:spcBef>`)
	bgSpcAftRegexp = regexp.MustCompile(`<a:spcAft><a:spcPts val="(\d+)"/></a:spcAft>`)
	bgMarLRegexp   = regexp.MustCompile(`<a:pPr[^>]*\bmarL="(\d+)"`)
	bgTextRegexp   = regexp.MustCompile(`<a:t>([^<]*)</a:t>`)
)

type bgPara struct {
	text     string
	bef, aft int
	hasBef   bool
	hasAft   bool
	marL     int
}

func parseBGParas(t *testing.T, xml string) []bgPara {
	t.Helper()
	var out []bgPara
	for _, m := range bgParaRegexp.FindAllStringSubmatch(xml, -1) {
		body := m[1]
		p := bgPara{}
		if tm := bgTextRegexp.FindStringSubmatch(body); tm != nil {
			p.text = tm[1]
		}
		if b := bgSpcBefRegexp.FindStringSubmatch(body); b != nil {
			p.bef, _ = strconv.Atoi(b[1])
			p.hasBef = true
		}
		if a := bgSpcAftRegexp.FindStringSubmatch(body); a != nil {
			p.aft, _ = strconv.Atoi(a[1])
			p.hasAft = true
		}
		if ml := bgMarLRegexp.FindStringSubmatch(body); ml != nil {
			p.marL, _ = strconv.Atoi(ml[1])
		}
		out = append(out, p)
	}
	return out
}

// go-slide-creator-b1vkj: on every shipped template, bullet_groups read by
// proximity — the gap header -> first bullet is no larger than the gap
// between bullets, the gap last bullet -> next header is at least 1.5x it,
// and sub-bullets are indented at most 24pt from the header's edge. Every
// group paragraph carries explicit spcBef and spcAft, so the master's
// inherited spacing (e.g. p-style lvl1 spcAft 12pt) cannot override them.
func TestBulletGroupsGroupByProximityOnShippedTemplates(t *testing.T) {
	groups := BulletGroupsContent{Groups: []BulletGroup{
		{Header: "People", Bullets: []string{"Retrain 40 agents", "Appoint one owner", "Hold headcount flat"}},
		{Header: "Process", Bullets: []string{"Single intake form", "Weekly scorecard", "Skill-based routing"}},
		{Header: "Technology", Bullets: []string{"One ITSM platform", "Automate two requests", "Retire legacy tools"}},
	}}
	headers := map[string]bool{"People": true, "Process": true, "Technology": true}
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
			output := filepath.Join(t.TempDir(), "groups.pptx")
			if _, err := Generate(context.Background(), GenerationRequest{
				TemplatePath: templatePath, OutputPath: output, ExcludeTemplateSlides: true,
				Slides: []SlideSpec{{LayoutID: layoutID, Content: []ContentItem{{
					PlaceholderID: "body", Type: ContentBulletGroups, Value: groups,
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
			var paras []bgPara
			for _, p := range parseBGParas(t, slideXML) {
				if p.text != "" && (headers[p.text] || strings.Contains("Retrain 40 agents|Appoint one owner|Hold headcount flat|Single intake form|Weekly scorecard|Skill-based routing|One ITSM platform|Automate two requests|Retire legacy tools", p.text)) {
					paras = append(paras, p)
				}
			}
			if len(paras) != 12 {
				t.Fatalf("found %d group paragraphs, want 12", len(paras))
			}
			for i, p := range paras {
				if !p.hasBef || !p.hasAft {
					t.Errorf("paragraph %d %q lacks explicit spcBef/spcAft", i, p.text)
				}
				if !headers[p.text] && p.marL > bulletGroupSubMarL {
					t.Errorf("sub-bullet %q marL %d exceeds the 24pt cap", p.text, p.marL)
				}
			}
			for i := 1; i < len(paras); i++ {
				prev, cur := paras[i-1], paras[i]
				gap := prev.aft + cur.bef
				switch {
				case headers[prev.text]: // header -> first bullet
					next := paras[i+1]
					pitch := cur.aft + next.bef
					if gap > pitch {
						t.Errorf("header %q -> first bullet gap %d > bullet pitch %d", prev.text, gap, pitch)
					}
				case headers[cur.text]: // last bullet -> next header
					pitch := paras[i-2].aft + prev.bef
					if gap*2 < pitch*3 {
						t.Errorf("last bullet -> header %q gap %d < 1.5x bullet pitch %d", cur.text, gap, pitch)
					}
				}
			}
		})
	}
}
