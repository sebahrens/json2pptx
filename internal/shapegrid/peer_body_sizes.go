package shapegrid

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

type siblingParagraph struct {
	bounds         pptx.RectEmu
	index, sibling int
	role           string
	size           float64
	fitted         float64
}

// Body copy and captions can share a reading size even in compact mode.
// Headings, figures and bold labels are separate roles and never participate.
func siblingBodyTargets(siblings []*ResolveResult) map[pptx.RectEmu]map[int]float64 {
	var paras []siblingParagraph
	least := map[string]float64{}
	owners := map[string]map[int]bool{}
	invalid := map[string]bool{}
	for si, sib := range siblings {
		if sib == nil {
			continue
		}
		for _, c := range sib.Cells {
			for _, p := range siblingCellParagraphs(c, si) {
				floor := 12.0
				if p.role == "caption" {
					floor = 10
				}
				if p.fitted < floor {
					invalid[p.role] = true
					continue
				}
				if owners[p.role] == nil {
					owners[p.role] = map[int]bool{}
				}
				owners[p.role][si] = true
				if value, ok := least[p.role]; !ok || p.fitted < value {
					least[p.role] = p.fitted
				}
				paras = append(paras, p)
			}
		}
	}
	out := map[pptx.RectEmu]map[int]float64{}
	for _, p := range paras {
		target := least[p.role]
		if invalid[p.role] || len(owners[p.role]) < 2 || target >= p.size {
			continue
		}
		if out[p.bounds] == nil {
			out[p.bounds] = map[int]float64{}
		}
		out[p.bounds][p.index] = target
	}
	return out
}
func (s *PeerScope) shareBodySizes(cells []ResolvedCell) {
	if s == nil || len(s.bodySizes) == 0 {
		return
	}
	for i := range cells {
		c := &cells[i]
		targets := s.bodySizes[c.Bounds]
		if len(targets) == 0 || c.ShapeSpec == nil {
			continue
		}
		raw := c.ShapeSpec.Text
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			var text string
			if json.Unmarshal(raw, &text) != nil {
				continue
			}
			value, _ := json.Marshal(text)
			obj = map[string]json.RawMessage{"content": value}
		}
		var paras []map[string]json.RawMessage
		if json.Unmarshal(obj["paragraphs"], &paras) == nil && len(paras) > 0 {
			for index, size := range targets {
				if index < len(paras) {
					paras[index]["size"], _ = json.Marshal(size)
				}
			}
			obj["paragraphs"], _ = json.Marshal(paras)
		} else if size, ok := targets[0]; ok {
			obj["size"], _ = json.Marshal(size)
		}
		text, err := json.Marshal(obj)
		if err != nil {
			continue
		}
		spec := *c.ShapeSpec
		spec.Text = text
		c.ShapeSpec = &spec
	}
}

func siblingCellParagraphs(c ResolvedCell, sibling int) []siblingParagraph {
	if c.Layer || c.Kind != CellKindShape || c.ShapeSpec == nil {
		return nil
	}
	tb, err := ResolveTextInput(c.ShapeSpec.Text)
	if err != nil || tb == nil {
		return nil
	}
	measured := scaleParagraphs(tb, c.ShapeSpec.ThemeFonts)
	if len(measured) != len(tb.Paragraphs) {
		return nil
	}
	fit := pptx.AutofitScaleFor(tb, c.Bounds)
	var out []siblingParagraph
	for pi, p := range measured {
		if p.bold || p.role == "kpi-value" || p.fontPt >= 24 || strings.TrimSpace(p.text) == "" {
			continue
		}
		out = append(out, siblingParagraph{bounds: c.Bounds, index: pi, sibling: sibling, role: p.role, size: p.fontPt, fitted: math.Floor(p.fontPt*fit*100) / 100})
	}
	return out
}
