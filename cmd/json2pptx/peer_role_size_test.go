package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// One type size per peer group (go-slide-creator-riyh7).
//
// A designer picks one size for the rows of one SCQA, the people of one
// directory row, the cards of one grid. A per-shape fit that shrinks or grows
// only the cells that need it sets peers at different sizes, and that is the
// clearest machine tell a generated slide carries. This test generates every
// pattern exemplar, and an UNEVEN variant of it (one item far longer than its
// siblings, the payload that provokes a per-cell fit), reads the slide XML and
// fails when text of one role in one peer group is written at more than one
// size.
//
// Peers and roles are read off the written slide, not off the pattern's code:
//
//   - two shapes are peers when they have the same geometry, fill and outline
//     and share a row (same top and height) or a column (same left and width);
//     peer groups are the connected sets of that relation (a 3 x 2 card grid is
//     one group);
//   - within a group, the role of a paragraph is its index in the shape, its
//     weight, slant and ink.
//
// The size compared is the one the file holds: the run's sz times the autofit
// scale stored on the shape.

// peerRoleTemplates are the templates swept: the private p-style when present
// (Arial, the widest face) and midnight-blue (Calibri).
var peerRoleTemplates = []string{"midnight-blue", "p-style"}

// peerRoleTypeScales are the deck type scales swept: the raw default, and the
// two grow-to-fill policies ("comfortable" is what every DeckSpec deck and the
// pattern gallery are generated with).
var peerRoleTypeScales = []string{"compact", "comfortable", "presentation"}

type peerShape struct {
	x, y, w, h int64
	sig        string // geometry | fill | line
	paras      []peerPara
	scale      float64
}

type peerPara struct {
	sizePt float64
	role   string // weight / slant / ink
	text   string
}

var (
	peerOffRe     = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"`)
	peerExtRe     = regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"`)
	peerGeomRe    = regexp.MustCompile(`<a:prstGeom prst="([a-zA-Z0-9]+)"`)
	peerSpPrRe    = regexp.MustCompile(`(?s)<p:spPr.*?</p:spPr>`)
	peerLnRe      = regexp.MustCompile(`(?s)<a:ln[ >].*?</a:ln>|<a:ln[^>]*/>`)
	peerFillRe    = regexp.MustCompile(`(?s)<a:solidFill>(.*?)</a:solidFill>`)
	peerParaRe    = regexp.MustCompile(`(?s)<a:p>(.*?)</a:p>`)
	peerRunRe     = regexp.MustCompile(`(?s)<a:r>(.*?)</a:r>`)
	peerRPrRe     = regexp.MustCompile(`(?s)<a:rPr([^>]*?)(/>|>(.*?)</a:rPr>)`)
	peerTagRe     = regexp.MustCompile(`<[^>]+>`)
	peerValRe     = regexp.MustCompile(`val="([^"]+)"`)
	peerAttrSzRe  = regexp.MustCompile(`\bsz="(\d+)"`)
	peerBoldRe    = regexp.MustCompile(`\bb="1"`)
	peerItalicRe  = regexp.MustCompile(`\bi="1"`)
	peerSpaceRe   = regexp.MustCompile(`\s+`)
	peerTxBodyRe  = regexp.MustCompile(`(?s)<p:txBody>.*?</p:txBody>`)
	peerPhRe      = regexp.MustCompile(`<p:ph[ />]`)
	peerNoFillRe  = regexp.MustCompile(`<a:noFill/>`)
	peerGrpOpenRe = regexp.MustCompile(`<p:grpSpPr>`)
)

// peerColorKey flattens the colour children of a solidFill ("accent1+lumMod
// 20000+lumOff80000") so a tint and the plain accent are different fills.
func peerColorKey(inner string) string {
	var parts []string
	for _, tag := range peerTagRe.FindAllString(inner, -1) {
		if strings.HasPrefix(tag, "</") {
			continue
		}
		name := strings.TrimLeft(tag, "<")
		if i := strings.IndexAny(name, " />"); i >= 0 {
			name = name[:i]
		}
		val := ""
		if m := peerValRe.FindStringSubmatch(tag); m != nil {
			val = m[1]
		}
		parts = append(parts, strings.TrimPrefix(name, "a:")+"="+val)
	}
	return strings.Join(parts, "+")
}

// parsePeerShapes reads the non-placeholder text shapes of a slide.
func parsePeerShapes(slideXML string) []peerShape {
	var out []peerShape
	for _, sp := range siblingShapeRe.FindAllString(slideXML, -1) {
		if peerPhRe.MatchString(sp) {
			continue
		}
		spPr := peerSpPrRe.FindString(sp)
		off, ext := peerOffRe.FindStringSubmatch(spPr), peerExtRe.FindStringSubmatch(spPr)
		if off == nil || ext == nil {
			continue
		}
		s := peerShape{scale: 1}
		s.x, _ = strconv.ParseInt(off[1], 10, 64)
		s.y, _ = strconv.ParseInt(off[2], 10, 64)
		s.w, _ = strconv.ParseInt(ext[1], 10, 64)
		s.h, _ = strconv.ParseInt(ext[2], 10, 64)

		geom := "rect"
		if m := peerGeomRe.FindStringSubmatch(spPr); m != nil {
			geom = m[1]
		}
		line := "none"
		if ln := peerLnRe.FindString(spPr); ln != "" && !peerNoFillRe.MatchString(ln) {
			line = "line"
			if m := peerFillRe.FindStringSubmatch(ln); m != nil {
				line = peerColorKey(m[1])
			}
		}
		noLine := peerLnRe.ReplaceAllString(spPr, "")
		fill := "none"
		if m := peerFillRe.FindStringSubmatch(noLine); m != nil {
			fill = peerColorKey(m[1])
		}
		s.sig = geom + "|" + fill + "|" + line

		body := peerTxBodyRe.FindString(sp)
		if m := siblingScaleRe.FindStringSubmatch(body); m != nil {
			v, _ := strconv.Atoi(m[1])
			s.scale = float64(v) / 100000
		}
		for _, pm := range peerParaRe.FindAllStringSubmatch(body, -1) {
			var text strings.Builder
			para := peerPara{}
			for _, rm := range peerRunRe.FindAllStringSubmatch(pm[1], -1) {
				run := rm[1]
				for _, tm := range shapeRunTextRe.FindAllStringSubmatch(run, -1) {
					text.WriteString(tm[1])
				}
				rpr := peerRPrRe.FindStringSubmatch(run)
				if rpr == nil || para.role != "" {
					continue
				}
				sz := peerAttrSzRe.FindStringSubmatch(rpr[1])
				if sz == nil {
					continue
				}
				v, _ := strconv.Atoi(sz[1])
				para.sizePt = float64(v) / 100 * s.scale
				ink := ""
				if m := peerFillRe.FindStringSubmatch(rpr[3]); m != nil {
					ink = peerColorKey(m[1])
				}
				para.role = fmt.Sprintf("b%t/i%t/%s", peerBoldRe.MatchString(rpr[1]), peerItalicRe.MatchString(rpr[1]), ink)
			}
			para.text = strings.TrimSpace(peerSpaceRe.ReplaceAllString(text.String(), " "))
			if para.text == "" || para.role == "" {
				continue
			}
			s.paras = append(s.paras, para)
		}
		if len(s.paras) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// peerRoleMismatch is one role of one peer group written at several sizes.
type peerRoleMismatch struct {
	role  string
	sizes map[float64][]string // size → texts
}

func (m peerRoleMismatch) String() string {
	sizes := make([]float64, 0, len(m.sizes))
	for s := range m.sizes {
		sizes = append(sizes, s)
	}
	sort.Float64s(sizes)
	var b strings.Builder
	fmt.Fprintf(&b, "role %s:", m.role)
	for _, s := range sizes {
		texts := m.sizes[s]
		shown := texts
		if len(shown) > 3 {
			shown = shown[:3]
		}
		for i, t := range shown {
			if len(t) > 32 {
				shown[i] = t[:32] + "…"
			}
		}
		fmt.Fprintf(&b, " %.1fpt ×%d %q;", s, len(texts), shown)
	}
	return b.String()
}

// peerGroupsOf returns, per shape, the representative of its peer group: the
// connected sets of shapes that share a row band or a column band and that
// alike calls alike.
func peerGroupsOf(shapes []peerShape, alike func(a, b peerShape) bool) []int {
	parent := make([]int, len(shapes))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	near := func(a, b int64) bool {
		d := a - b
		if d < 0 {
			d = -d
		}
		return d <= 12700 // 1pt
	}
	for i := range shapes {
		for j := i + 1; j < len(shapes); j++ {
			a, b := shapes[i], shapes[j]
			if !alike(a, b) {
				continue
			}
			// The frames stand as peers: the same size, or a shared row or
			// column edge with one dimension in common (a staircase shares
			// its foot and its width, ranked bars their left and height).
			sameW, sameH := near(a.w, b.w), near(a.h, b.h)
			row := near(a.y, b.y) || near(a.y+a.h, b.y+b.h)
			col := near(a.x, b.x) || near(a.x+a.w, b.x+b.w)
			if (sameW && sameH) || ((sameW || sameH) && (row || col)) {
				parent[find(i)] = find(j)
			}
		}
	}
	for i := range parent {
		parent[i] = find(i)
	}
	return parent
}

// peerIsFigure reports a display figure: a short run with a digit set at 24pt
// or more ("3.8x", "+75%", "01"). A figure is its own role, and so is the
// text of the shape it leads: the caption under a headline number is not a
// peer of the bullets in the next shape of its column.
func peerIsFigure(p peerPara) bool {
	return p.sizePt >= 24 && len([]rune(p.text)) <= 12 && strings.ContainsAny(p.text, "0123456789")
}

// peerRoleMismatches groups the shapes into peer groups and returns every role
// a group sets at more than one size.
func peerRoleMismatches(shapes []peerShape) []peerRoleMismatch {
	groups := peerGroupsOf(shapes, func(a, b peerShape) bool { return a.sig == b.sig })
	type key struct {
		group int
		role  string
	}
	seen := map[key]map[float64][]string{}
	var order []key
	for i, s := range shapes {
		lead := ""
		if peerIsFigure(s.paras[0]) {
			lead = "figure-led "
		}
		for pi, p := range s.paras {
			idx := pi
			if idx > 2 {
				idx = 2 // paragraphs past the second are a list: one role
			}
			k := key{groups[i], fmt.Sprintf("%s %spara%d %s", s.sig, lead, idx, p.role)}
			if seen[k] == nil {
				seen[k] = map[float64][]string{}
				order = append(order, k)
			}
			size := math.Round(p.sizePt*10) / 10
			seen[k][size] = append(seen[k][size], p.text)
		}
	}
	var out []peerRoleMismatch
	for _, k := range order {
		if len(seen[k]) > 1 {
			out = append(out, peerRoleMismatch{role: k.role, sizes: seen[k]})
		}
	}
	return out
}

// peerGrowthMismatches compares a slide generated under a grow-to-fill type
// scale with the same slide generated compact: peers that are one size compact
// must still be one size grown. Here peers are shapes of one geometry whose
// lead paragraphs are set alike — whatever their fill or ink — because growth
// is what is under test: the highlighted card of a row must grow with its
// neighbours.
func peerGrowthMismatches(compact, grown []peerShape) []peerRoleMismatch {
	if len(compact) != len(grown) {
		return nil // the composition changed with the scale; roles still hold
	}
	groups := peerGroupsOf(compact, func(a, b peerShape) bool {
		return strings.SplitN(a.sig, "|", 2)[0] == strings.SplitN(b.sig, "|", 2)[0] &&
			a.paras[0].sizePt == b.paras[0].sizePt &&
			strings.HasPrefix(a.paras[0].role, "btrue") == strings.HasPrefix(b.paras[0].role, "btrue")
	})
	type key struct {
		group int
		role  string
	}
	seen := map[key]map[float64][]string{}
	var order []key
	for i, s := range compact {
		if len(grown[i].paras) != len(s.paras) {
			continue
		}
		for pi, p := range s.paras {
			idx := min(pi, 2)
			k := key{groups[i], fmt.Sprintf("%s para%d written %.1fpt compact", strings.SplitN(s.sig, "|", 2)[0], idx, p.sizePt)}
			if seen[k] == nil {
				seen[k] = map[float64][]string{}
				order = append(order, k)
			}
			size := math.Round(grown[i].paras[pi].sizePt*10) / 10
			seen[k][size] = append(seen[k][size], p.text)
		}
	}
	var out []peerRoleMismatch
	for _, k := range order {
		if len(seen[k]) > 1 {
			out = append(out, peerRoleMismatch{role: k.role, sizes: seen[k]})
		}
	}
	return out
}

// peerUnevenSkipKeys are value fields that are not prose: lengthening them
// changes what the item IS (an icon name, a period reference), not how long
// it reads.
var peerUnevenSkipKeys = map[string]bool{
	"icon": true, "photo": true, "image": true, "color": true, "accent": true, "svg_data": true,
	"url": true, "path": true, "alt": true, "id": true, "key": true,
}

const peerUnevenFiller = " across every regional market team and each partner channel in the plan for the coming four quarters of the programme"

// peerLonger returns s grown toward its schema maximum.
func peerLonger(s string, maxLen int, stretch float64) string {
	n := len([]rune(s))
	target := max(int(float64(n)*stretch), n+int(8*stretch))
	if maxLen > 0 && target > maxLen {
		target = maxLen
	}
	if maxLen <= 0 && target > 160 {
		target = 160
	}
	if n >= target || n == 0 {
		return s
	}
	out := []rune(s + peerUnevenFiller + peerUnevenFiller)
	if len(out) > target {
		out = out[:target]
	}
	return strings.TrimSpace(string(out))
}

// peerUnevenValues returns the exemplar with ONE item of every list of peers
// grown far longer than its siblings, keeping each mutation only when the
// pattern still accepts the payload. The bool reports whether anything grew.
func peerUnevenValues(t *testing.T, pat patterns.Pattern, exemplar any, stretch float64) (json.RawMessage, bool) {
	t.Helper()
	raw, err := json.Marshal(exemplar)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	schemaRaw, err := json.Marshal(pat.Schema())
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatal(err)
	}
	defs, _ := schema["$defs"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	valuesSchema, _ := props["values"].(map[string]any)

	accepts := func() bool {
		enc, err := json.Marshal(doc)
		if err != nil {
			return false
		}
		values := pat.NewValues()
		if err := json.Unmarshal(enc, values); err != nil {
			return false
		}
		return pat.Validate(values, nil, nil) == nil
	}
	if !accepts() {
		t.Fatalf("pattern rejects its own exemplar")
	}
	grew := false

	deref := func(node map[string]any) map[string]any {
		for range 4 {
			if node == nil {
				return nil
			}
			if ref, ok := node["$ref"].(string); ok {
				node = resolveSchemaRef(ref, defs)
				continue
			}
			if variants, ok := node["oneOf"].([]any); ok && len(variants) > 0 {
				picked := false
				for _, v := range variants {
					if m, isMap := v.(map[string]any); isMap && (m["type"] == "object" || m["$ref"] != nil) {
						node, picked = m, true
						break
					}
				}
				if !picked {
					node, _ = variants[0].(map[string]any)
				}
				continue
			}
			break
		}
		return node
	}
	maxLenOf := func(node map[string]any) int {
		if v, ok := node["maxLength"].(float64); ok {
			return int(v)
		}
		return 0
	}
	// try sets a longer string through set and keeps it when accepted.
	try := func(cur string, node map[string]any, set func(string)) {
		if node != nil && node["enum"] != nil {
			return
		}
		longer := peerLonger(cur, maxLenOf(node), stretch)
		if longer == cur {
			return
		}
		set(longer)
		if accepts() {
			grew = true
			return
		}
		set(cur)
	}

	var walk func(v any, node map[string]any, depth int)
	// growItem lengthens the prose of one list item.
	growItem := func(holder []any, i int, itemSchema map[string]any) {
		switch item := holder[i].(type) {
		case string:
			try(item, itemSchema, func(s string) { holder[i] = s })
		case map[string]any:
			itemProps, _ := itemSchema["properties"].(map[string]any)
			names := make([]string, 0, len(item))
			for name := range item {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				s, ok := item[name].(string)
				if !ok || peerUnevenSkipKeys[name] {
					continue
				}
				pm, _ := itemProps[name].(map[string]any)
				try(s, deref(pm), func(v string) { item[name] = v })
			}
		}
	}
	walk = func(v any, node map[string]any, depth int) {
		node = deref(node)
		if depth > 6 {
			return
		}
		switch tv := v.(type) {
		case map[string]any:
			nodeProps, _ := node["properties"].(map[string]any)
			names := make([]string, 0, len(tv))
			for name := range tv {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				pm, _ := nodeProps[name].(map[string]any)
				walk(tv[name], pm, depth+1)
			}
		case []any:
			items, _ := node["items"].(map[string]any)
			items = deref(items)
			if len(tv) >= 2 {
				growItem(tv, 1, items)
			}
			for _, child := range tv {
				walk(child, items, depth+1)
			}
		}
	}
	walk(doc, valuesSchema, 0)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out, grew
}

// readSlideXML returns slide 1 of a generated deck.
func readFirstSlideXML(t *testing.T, pptxPath string) string {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	rc, err := zr.Open("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// peerRoleCheck generates one pattern slide and fails on a role of a peer
// group written at several sizes. refused reports that generation refused the
// payload as unreadable: it is past what the pattern holds, and a refusal is
// not a mixed-size slide.
func peerRoleCheck(t *testing.T, tpl, templatesDir, pattern, variant, scale string, values json.RawMessage) (shapes []peerShape, refused bool) {
	t.Helper()
	title := "Peers"
	input := PresentationInput{
		Template:       tpl,
		OutputFilename: "deck.pptx",
		TypeScale:      scale,
		Slides: []SlideInput{{
			LayoutID: "content",
			Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
			Pattern:  &PatternInput{Name: pattern, Values: values},
		}},
	}
	dir := t.TempDir()
	if k := os.Getenv("PEER_KEEP"); k != "" {
		dir = filepath.Join(k, tpl+"_"+pattern+"_"+variant+"_"+scale)
		_ = os.MkdirAll(dir, 0o755)
		raw, _ := json.MarshalIndent(input, "", " ")
		_ = os.WriteFile(filepath.Join(dir, "input.json"), raw, 0o644)
	}
	applyDefaults(&input)
	result, cleanup, err := RunPresentation(context.Background(), &input, RenderOptions{
		OutputDir: dir, TemplatesDir: templatesDir, StrictFit: "off", OutputValidation: "off",
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		if variant != "exemplar" && strings.Contains(err.Error(), "generation refused") {
			return nil, true
		}
		t.Fatalf("generate: %v", err)
	}
	shapes = parsePeerShapes(readFirstSlideXML(t, result.OutputPath))
	for _, m := range peerRoleMismatches(shapes) {
		t.Errorf("%s (%s, type_scale %s) on %s sets one role of one peer group at several sizes — %s", pattern, variant, scale, tpl, m)
	}
	return shapes, false
}

// peerUnevenStretches are how far the uneven variant grows one item, tried in
// order: the first the pattern can still hold at a readable size is the one
// swept.
var peerUnevenStretches = []float64{4, 2, 1.3}

// peerRoleSweep runs one payload through every type scale. It reports false
// when generation refused the payload.
func peerRoleSweep(t *testing.T, tpl, templatesDir, pattern, variant string, values json.RawMessage) bool {
	t.Helper()
	var compact []peerShape
	for _, scale := range peerRoleTypeScales {
		shapes, refused := peerRoleCheck(t, tpl, templatesDir, pattern, variant, scale, values)
		if refused {
			return false
		}
		if scale == "compact" {
			compact = shapes
			continue
		}
		for _, m := range peerGrowthMismatches(compact, shapes) {
			t.Errorf("%s (%s) on %s: type_scale %s grows peers of one size to several — %s", pattern, variant, tpl, scale, m)
		}
	}
	return true
}

// TestPatternPeersShareOneTypeSize is the registry-wide gate: every pattern
// exemplar and its uneven variant, on midnight-blue and the local p-style.
//
// PEER_KEEP=<dir> keeps each generated deck under <dir> for inspection.
func TestPatternPeersShareOneTypeSize(t *testing.T) {
	t.Parallel()
	templatesDir := testutil.TemplatesDir()
	templates := peerRoleTemplates
	var pats []patterns.Pattern
	for _, p := range patterns.Default().List() {
		if _, ok := p.(patterns.Exemplar); ok {
			pats = append(pats, p)
		}
	}
	sort.Slice(pats, func(i, j int) bool { return pats[i].Name() < pats[j].Name() })

	for _, tpl := range templates {
		if _, err := os.Stat(filepath.Join(templatesDir, tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		for _, p := range pats {
			exemplar := p.(patterns.Exemplar).ExemplarValues()
			even, err := json.Marshal(exemplar)
			if err != nil {
				t.Fatal(err)
			}
			tpl, p := tpl, p
			// -short sweeps the exemplar on the first template only; the uneven
			// payload, the one that provokes a per-cell fit, runs on both.
			if !testing.Short() || tpl == templates[0] {
				t.Run(tpl+"/"+p.Name()+"/exemplar", func(t *testing.T) {
					t.Parallel()
					peerRoleSweep(t, tpl, templatesDir, p.Name(), "exemplar", even)
				})
			}
			var uneven []json.RawMessage
			for _, stretch := range peerUnevenStretches {
				if values, grew := peerUnevenValues(t, p, exemplar, stretch); grew {
					uneven = append(uneven, values)
				}
			}
			if len(uneven) == 0 {
				t.Logf("%s: no list of peers to make uneven", p.Name())
				continue
			}
			t.Run(tpl+"/"+p.Name()+"/uneven", func(t *testing.T) {
				t.Parallel()
				for _, values := range uneven {
					if peerRoleSweep(t, tpl, templatesDir, p.Name(), "uneven", values) {
						return
					}
				}
				// The exemplar already fills its shapes on this template: any
				// longer item is refused as unreadable, which is the engine
				// declining to set one peer smaller, not a mixed-size slide.
				t.Skipf("%s on %s: generation refuses every uneven variant of the exemplar", p.Name(), tpl)
			})
		}
	}
}
