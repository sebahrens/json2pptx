package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// titleGeometryDeckSpec mixes every DeckSpec content kind that hosts a native
// visual — table, org, SWOT, five forces — with pattern kinds. Each authored
// item is distinctive so the test can find it in the rendered XML.
func titleGeometryDeckSpec(template string) map[string]any {
	return map[string]any{
		"meta": map[string]any{"title": "Title geometry", "template": template},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Title geometry", "subtitle": "One title treatment"},
			map[string]any{
				"kind": "executive_summary", "title": "One title position on every content slide",
				"points": []any{"Tables ride in a grid", "Org charts ride in a grid", "Frameworks ride in a grid"},
			},
			map[string]any{
				"kind": "table", "title": "Enterprise carried the year",
				"headers": []any{"Segment", "FY25", "FY26"},
				"rows":    []any{[]any{"Enterprise", "$28.4M", "$41.2M"}, []any{"SMB", "$9.6M", "$8.9M"}},
			},
			map[string]any{
				"kind": "framework", "title": "Where we stand", "framework": "swot",
				"sections": map[string]any{
					"strengths":     []any{"SWOT-S1 clearers migrated", "SWOT-S2 regulator onside"},
					"weaknesses":    []any{"SWOT-W1 manual reconciliation"},
					"opportunities": []any{"SWOT-O1 T+1 mandate"},
					"threats":       []any{"SWOT-T1 competitor live", "SWOT-T2 fixed deadline"},
				},
			},
			map[string]any{
				"kind": "framework", "title": "Industry forces favour scale", "framework": "porters_five_forces",
				"sections": map[string]any{
					"rivalry":      map[string]any{"factors": []any{"PFF-R1 four national players"}, "intensity": "high"},
					"new_entrants": []any{"PFF-E1 licensing barrier"},
					"substitutes":  []any{"PFF-S1 fintech wallets"},
					"suppliers":    []any{"PFF-P1 two core vendors"},
					"buyers":       []any{"PFF-B1 corporates negotiate"},
				},
			},
			map[string]any{
				"kind": "org", "title": "Programme governance",
				"nodes": []any{
					map[string]any{"id": "steer", "name": "Steering group", "title": "Decision owner"},
					map[string]any{"id": "platform", "name": "Platform lead", "parent": "steer"},
					map[string]any{"id": "data", "name": "Data lead", "parent": "steer"},
				},
			},
			map[string]any{
				"kind": "pillars", "title": "Three pillars support the plan",
				"objective": "Become the trusted settlement platform",
				"pillars": []any{
					map[string]any{"title": "Trust", "body": []any{"Transparent pricing"}},
					map[string]any{"title": "Velocity", "body": []any{"Weekly releases"}},
					map[string]any{"title": "Growth", "body": []any{"Enterprise focus"}},
				},
				"foundation": "People · Data · Controls",
			},
		},
	}
}

var (
	slideLayoutRelRe = regexp.MustCompile(`Target="\.\./slideLayouts/(slideLayout\d+\.xml)"`)
	layoutMasterRe   = regexp.MustCompile(`Target="\.\./slideMasters/(slideMaster\d+\.xml)"`)
	spRe             = regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`)
	titlePhRe        = regexp.MustCompile(`<p:ph[^>]*type="title"`)
	xfrmRe           = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/>\s*<a:ext cx="(\d+)" cy="(\d+)"/>`)
)

// titleXfrm returns the title placeholder's xfrm in part, or "" when the part
// has no title or the title inherits its geometry.
func titleXfrm(part []byte) string {
	for _, sp := range spRe.FindAll(part, -1) {
		if !titlePhRe.Match(sp) {
			continue
		}
		if m := xfrmRe.FindSubmatch(sp); m != nil {
			return fmt.Sprintf("%s,%s %sx%s", m[1], m[2], m[3], m[4])
		}
		return ""
	}
	return ""
}

// effectiveTitle resolves a slide's title geometry through the inheritance
// chain (slide → layout → master) and names the layout that styles it.
func effectiveTitle(t *testing.T, parts map[string][]byte, slide string) (layout, xfrm string) {
	t.Helper()
	rels := parts["ppt/slides/_rels/"+slide+".rels"]
	m := slideLayoutRelRe.FindSubmatch(rels)
	if m == nil {
		t.Fatalf("%s: no slide layout relationship", slide)
	}
	layout = string(m[1])
	if x := titleXfrm(parts["ppt/slides/"+slide]); x != "" {
		return layout, x
	}
	if x := titleXfrm(parts["ppt/slideLayouts/"+layout]); x != "" {
		return layout, x
	}
	if mm := layoutMasterRe.FindSubmatch(parts["ppt/slideLayouts/_rels/"+layout+".rels"]); mm != nil {
		return layout, titleXfrm(parts["ppt/slideMasters/"+string(mm[1])])
	}
	return layout, ""
}

// go-slide-creator-ngbnf: every DeckSpec content kind renders under one title
// treatment. Table and org already rode in a one-cell grid on blank-title; SWOT
// and five forces were pinned to the content layout because their native shapes
// needed a body placeholder, so on abstract the title jumped to a smaller,
// letter-spaced treatment on those slides. Now the native framework shapes
// render in a grid cell too, and every non-cover slide shares one layout and
// one title xfrm — with every authored framework item still on the slide.
func TestDeckSpecContentKindsShareTitleGeometry(t *testing.T) {
	templates := []string{"abstract"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			mc := semanticTestConfig(t)
			out := renderDeckSpecCall(t, mc, map[string]any{"spec": titleGeometryDeckSpec(name)})
			if out.PptxPath == "" {
				t.Fatalf("render produced no deck: %s %+v", out.Error, out.Diagnostics)
			}
			parts := pptxParts(t, out.PptxPath)

			var slides []string
			for p := range parts {
				if m := regexp.MustCompile(`^ppt/slides/(slide\d+\.xml)$`).FindStringSubmatch(p); m != nil {
					slides = append(slides, m[1])
				}
			}
			sort.Slice(slides, func(i, j int) bool { return slideNum(slides[i]) < slideNum(slides[j]) })
			if len(slides) != 7 {
				t.Fatalf("rendered %d slides, want 7", len(slides))
			}

			kinds := []string{"executive_summary", "table", "swot", "porters_five_forces", "org", "pillars"}
			wantLayout, wantXfrm := effectiveTitle(t, parts, slides[1])
			if wantXfrm == "" {
				t.Fatalf("%s: could not resolve the title geometry", kinds[0])
			}
			for i, slide := range slides[1:] {
				layout, xfrm := effectiveTitle(t, parts, slide)
				if layout != wantLayout || xfrm != wantXfrm {
					t.Errorf("%s slide: title on %s at %s, want %s at %s (as on the %s slide)",
						kinds[i], layout, xfrm, wantLayout, wantXfrm, kinds[0])
				}
			}

			// The diagrams keep every authored item.
			all := string(parts["ppt/slides/"+slides[3]]) + string(parts["ppt/slides/"+slides[4]])
			for _, want := range []string{
				"SWOT-S1", "SWOT-S2", "SWOT-W1", "SWOT-O1", "SWOT-T1", "SWOT-T2",
				"Strengths", "Weaknesses", "Opportunities", "Threats",
				"PFF-R1", "PFF-E1", "PFF-S1", "PFF-P1", "PFF-B1",
				"Competitive rivalry", "Threat of new entrants", "Threat of substitutes",
				"Bargaining power of suppliers", "Bargaining power of buyers", "High",
			} {
				if !strings.Contains(all, want) {
					t.Errorf("the framework slides lost %q", want)
				}
			}
		})
	}
}

func slideNum(name string) int {
	n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "slide"), ".xml"))
	return n
}
