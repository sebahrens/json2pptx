package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/svggen/safeyaml"
)

// The field a reworded finding names (go-slide-creator-nuq30).
//
// A pattern's finding names a value of the pattern ("items[3].name");
// deckSpecWording turns it into the field of the spec that carries it and
// renames the pattern's words for its lists to the kind's. Both were worked
// out from text alone: the first list of the spec whose entry at the same
// index carried the same text. A risk whose impact was "Major" at index 3,
// beside impact_levels whose fourth level is "Major", made the risk-heatmap
// pattern's items the kind's impact_levels, and the message read
// "/slides/1/impact_levels/0/name shares the … cell" on a finding whose path
// was /slides/1/items/0/name.

// messagePointerRE matches a JSON Pointer into a DeckSpec inside a sentence.
var messagePointerRE = regexp.MustCompile(`/(?:slides/\d+|structure/(?:cover|closing|sections/\d+(?:/slides/\d+)?))(?:/[A-Za-z0-9_]+)*`)

// sameLine reports whether two pointers name the same field, one a part of the
// other, or two fields of one entry ("options[0].name/detail use 30/44
// characters" is about both, and is filed on the first).
func sameLine(a, b string) bool {
	parent := func(p string) string { return p[:strings.LastIndexByte(p, '/')] }
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") || parent(a) == parent(b)
}

// assertMessagePointers checks every pointer a finding's sentence quotes: it
// resolves in the spec, and it is on the line of the finding's own address —
// the path, one of the paths, or a field the finding's patch names. A field
// the sentence says to set ("set /slides/1/layout to …") may be one to add:
// then the slide it is added to resolves.
func assertMessagePointers(t *testing.T, where string, spec any, message string, own []string) {
	t.Helper()
	for _, at := range messagePointerRE.FindAllStringIndex(message, -1) {
		p := message[at[0]:at[1]]
		resolved := p
		if strings.HasSuffix(message[:at[0]], "set ") {
			resolved = p[:strings.LastIndexByte(p, '/')]
		}
		if !pointerResolves(spec, resolved) {
			t.Errorf("%s: the message names %s, which is not in the spec: %s", where, p, message)
			continue
		}
		if !slices.ContainsFunc(own, func(o string) bool { return sameLine(o, p) }) {
			t.Errorf("%s: the message names %s, which is not on the line of the finding's own address %v: %s", where, p, own, message)
		}
	}
}

// crowdedHeatmapSpec is the bead's deck: a 5 × 5 risk heat map with its own
// level names, three risks in one cell and a takeaway. "Key-person loss" is
// the fourth risk and its impact, Major, the fourth impact level. more adds
// risks to the crowded cell, for a template whose grid holds three.
func crowdedHeatmapSpec(template string, more int) map[string]any {
	risk := func(name, likelihood, impact string) any {
		return map[string]any{"name": name, "likelihood": likelihood, "impact": impact}
	}
	items := []any{
		risk("Cyber attack on payments", "Possible", "Severe"),
		risk("Regulatory change in core market", "Likely", "Major"),
		risk("Supplier concentration in Asia", "Likely", "Major"),
		risk("Key-person loss", "Likely", "Major"),
		risk("Data centre outage", "Rare", "Minor"),
	}
	for _, name := range []string{"Talent attrition in engineering", "Currency exposure in export sales"}[:more] {
		items = append(items, risk(name, "Likely", "Major"))
	}
	return map[string]any{
		"meta": map[string]any{"title": "Risk map", "template": template},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Enterprise risk map", "subtitle": "Board risk committee"},
			map[string]any{
				"kind":              "risk_heatmap",
				"title":             "Three risks crowd the likely and major cell",
				"size":              5,
				"impact_levels":     []any{"Negligible", "Minor", "Moderate", "Major", "Severe"},
				"likelihood_levels": []any{"Rare", "Unlikely", "Possible", "Likely", "Almost certain"},
				"items":             items,
				"takeaway":          "Three of the five top risks sit in one cell, so mitigation budget should go there first.",
			},
		},
	}
}

// The bead's repro, on the template it was found on and on the local one
// (whose grid holds three risks in a cell, so it is given five): the crowded
// cell's finding names the risk it is about in its sentence as in its path,
// and gives no advice the kind cannot follow.
func TestCrowdedHeatmapFindingNamesTheRisk(t *testing.T) {
	mc := refusalTestConfig(t)
	more := map[string]int{"modern-template": 0}
	if slices.Contains(testutil.AllTestTemplateNames(), "p-style") {
		more["p-style"] = 2
	}
	for tpl, extra := range more {
		spec := crowdedHeatmapSpec(tpl, extra)
		raw, _ := json.Marshal(spec)
		var doc any
		_ = json.Unmarshal(raw, &doc)

		var out map[string]any
		structuredInto(t, mustCall(t, mc.handleRenderDeckSpec, map[string]any{"spec": spec, "template": tpl}).StructuredContent, &out)
		var validated map[string]any
		structuredInto(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": tpl}).StructuredContent, &validated)
		for tool, list := range map[string]any{"render": out["diagnostics"], "validate": validated["findings"]} {
			findings, _ := list.([]any)
			crowded := 0
			for _, entry := range findings {
				f := entry.(map[string]any)
				message, _ := f["message"].(string)
				path, _ := f["path"].(string)
				where := tpl + " " + tool + " " + fmt.Sprint(f["code"])
				if strings.Contains(message, "_levels/") && strings.Contains(message, "/name") {
					t.Errorf("%s: the message renames the risks to the level names: %s", where, message)
				}
				assertMessagePointers(t, where, doc, message, findingOwnPointers(f))
				if !strings.Contains(message, "shares the") {
					continue
				}
				crowded++
				if path != "/slides/1/items/1/name" || !strings.Contains(message, "/slides/1/items/1/name shares the") {
					t.Errorf("%s: path %s, message %s; want both to name /slides/1/items/1/name", where, path, message)
				}
				// The kind has no switch for the pattern's legend.
				if strings.Contains(message, "legend") {
					t.Errorf("%s: the advice names the legend, which a risk_heatmap slide cannot hide: %s", where, message)
				}
			}
			if crowded == 0 {
				t.Errorf("%s %s: no finding about the crowded cell; the fixture no longer provokes it", tpl, tool)
			}
		}
	}
}

// findingOwnPointers lists the pointers a decoded finding carries as its own
// address: path, paths, and the targets of the patch it suggests.
func findingOwnPointers(f map[string]any) []string {
	var own []string
	if p, ok := f["path"].(string); ok {
		own = append(own, p)
	}
	if paths, ok := f["paths"].([]any); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				own = append(own, s)
			}
		}
	}
	// "verified fix: removing /slides/1/takeaway clears this" names the target
	// of the patch the finding carries.
	var patch func(v any)
	patch = func(v any) {
		switch t := v.(type) {
		case string:
			if strings.HasPrefix(t, "/") {
				own = append(own, t)
			}
		case map[string]any:
			for _, child := range t {
				patch(child)
			}
		case []any:
			for _, child := range t {
				patch(child)
			}
		}
	}
	patch(f["next_tool_call"])
	return own
}

// valueMentions lists every string of a pattern's values that sits under a
// list, as a message would name it ("steps[3].body", "tiers[0].components[1]",
// "values[2].label"), with the text it carries.
func valueMentions(values any) map[string]string {
	out := map[string]string{}
	var walk func(node any, at string, indexed bool)
	walk = func(node any, at string, indexed bool) {
		switch t := node.(type) {
		case string:
			if indexed && t != "" && valueMentionRE.FindString(at) == at {
				out[at] = t
			}
		case map[string]any:
			for k, child := range t {
				next := k
				if at != "" {
					next = at + "." + k
				}
				walk(child, next, indexed)
			}
		case []any:
			if at == "" {
				at = "values"
			}
			for i, child := range t {
				walk(child, fmt.Sprintf("%s[%d]", at, i), true)
			}
		}
	}
	walk(values, "", false)
	return out
}

// wordingAuditDecks is what the audit runs: one deck per slide kind and per
// pattern the kind's example can take, each playbook, and the crowded heat map.
func wordingAuditDecks(t *testing.T) map[string][]byte {
	t.Helper()
	decks := map[string][]byte{}
	deck := func(slide map[string]any) []byte {
		raw, err := json.Marshal(map[string]any{
			"meta": map[string]any{"title": "Wording audit", "source": "Illustrative"},
			"slides": []any{
				map[string]any{"kind": "title", "title": "Wording audit", "subtitle": "October 2026"},
				slide,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for _, kind := range semantic.AllSlideKinds() {
		example := semantic.KindExample(kind)
		if example == nil {
			continue
		}
		decks["kind/"+string(kind)] = deck(example)
		for _, c := range semantic.SlideAlternatives(kind, example) {
			if c.Pattern == "" {
				continue
			}
			alt := semantic.KindExample(kind)
			alt["pattern"] = c.Pattern
			decks["kind/"+string(kind)+"/"+c.Pattern] = deck(alt)
		}
	}
	playbooks, err := filepath.Glob(filepath.Join("..", "..", "examples", "semantic", "playbooks", "*.yaml"))
	if err != nil || len(playbooks) == 0 {
		t.Fatalf("playbooks: %v %v", playbooks, err)
	}
	for _, path := range playbooks {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var spec map[string]any
		if err := safeyaml.Unmarshal(raw, &spec); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		asJSON, err := json.Marshal(spec)
		if err != nil {
			t.Fatal(err)
		}
		decks["playbook/"+strings.TrimSuffix(filepath.Base(path), ".yaml")] = asJSON
	}
	crowded, _ := json.Marshal(crowdedHeatmapSpec("modern-template", 0))
	decks["crowded-heatmap"] = crowded
	return decks
}

// TestPatternValueMentionsNameTheirSpecField is the audit of the class: for
// every slide kind with an example (and every pattern the example can take),
// every playbook and the bead's heat map, a finding is synthesised on each
// string of each list of the pattern the slide compiled to, written the way a
// pattern writes it, and reworded. The pointer the sentence then quotes
// resolves in the authored spec, carries the text the pattern named, is the
// finding's own path, and — when the spec has the same field at the same place
// as the pattern (items[3].impact in both) — is that field and no other. The
// pattern's word for a list the spec also calls by that name stays.
func TestPatternValueMentionsNameTheirSpecField(t *testing.T) {
	decks := wordingAuditDecks(t)
	names := make([]string, 0, len(decks))
	for name := range decks {
		names = append(names, name)
	}
	sort.Strings(names)
	mentions, located, kinds := 0, 0, map[semantic.SlideKind]bool{}
	for _, name := range names {
		spec, parseDiags := semantic.ParseJSON(decks[name])
		if spec == nil {
			t.Errorf("%s: does not parse: %v", name, parseDiags)
			continue
		}
		input, cr, err := semantic.Compile(spec, semantic.CompileOptions{})
		if err != nil || input == nil {
			t.Errorf("%s: does not compile: %v", name, err)
			continue
		}
		var doc any
		if err := json.Unmarshal(decks[name], &doc); err != nil {
			t.Fatal(err)
		}
		for rawIdx := range input.Slides {
			slide := &input.Slides[rawIdx]
			if slide.Pattern == nil || rawIdx >= len(cr.IR.Slides) || cr.IR.Slides[rawIdx].Kind == semantic.KindRawJSON2pptx {
				continue
			}
			ir := cr.IR.Slides[rawIdx]
			kinds[ir.Kind] = true
			values := patternValues(slide)
			all := valueMentions(values)
			ordered := make([]string, 0, len(all))
			for m := range all {
				ordered = append(ordered, m)
			}
			sort.Strings(ordered)
			for _, mention := range ordered {
				text := all[mention]
				mentions++
				list := mention[:strings.IndexByte(mention, '[')]
				where := fmt.Sprintf("%s slide %d (%s, %s) %s", name, rawIdx+1, ir.Kind, slide.Pattern.Name, mention)
				fit := []patterns.FitFinding{{Action: "review", ValidationError: patterns.ValidationError{
					Code:    patterns.ErrCodeBodyTooLong,
					Path:    fmt.Sprintf("/slides/%d/pattern", rawIdx),
					Message: fmt.Sprintf("slide %d: %s: %s %s is 99 characters and its cell fits fewer — shorten it, or use fewer %s", rawIdx+1, slide.Pattern.Name, slide.Pattern.Name, mention, list),
				}}}
				diags := finishFitDiagnostics(cr.SourceMap, cr.IR, fit)
				deckSpecWording(diags, input, cr.IR)
				d := diags[0]
				own := specPointer(d.SemanticPath)
				quoted := messagePointerRE.FindAllString(d.Message, -1)
				assertMessagePointers(t, where, doc, d.Message, []string{own})

				// The field the spec has at the pattern's own place.
				same := ""
				relative := mention
				if _, isList := values.([]any); isList {
					relative = "" // a list of values has no name of its own in the spec
				}
				if relative != "" {
					if s, ok := nodeAtDotted(ir.Body, relative).(string); ok && normalizeMatchText(s) == normalizeMatchText(text) {
						same = specPointer(ir.SourcePath + "." + relative)
					}
				}
				if same != "" {
					if len(quoted) != 1 || quoted[0] != same || own != same {
						t.Errorf("%s: the spec carries this at %s; the message names %v and the path is %s: %s", where, same, quoted, own, d.Message)
					}
					if !strings.Contains(d.Message, "use fewer "+list) {
						t.Errorf("%s: the spec calls the list %s too, and the message renames it: %s", where, list, d.Message)
					}
				}
				if len(quoted) == 0 {
					continue
				}
				located++
				if quoted[0] != own {
					t.Errorf("%s: the message names %s and the path is %s", where, quoted[0], own)
				}
				node := nodeAtTokens(doc, pointerTokens(quoted[0]))
				if s, isText := node.(string); isText && normalizeMatchText(s) != normalizeMatchText(text) {
					t.Errorf("%s: the message names %s, which reads %q, for the pattern's %q", where, quoted[0], s, text)
				}
			}
		}
	}
	t.Logf("%d pattern values over %d decks and %d kinds; %d located in the spec", mentions, len(names), len(kinds), located)
	if mentions < 200 || len(kinds) < 15 {
		t.Fatalf("%d pattern values over %d kinds; the audit no longer covers the kinds", mentions, len(kinds))
	}
	if located*10 < mentions*6 {
		t.Errorf("only %d of %d pattern values were located in the spec", located, mentions)
	}
}

// The vocabulary of the crowded heat map: nothing of the pattern is renamed,
// because the spec uses the pattern's own names.
func TestKindVocabularyDoesNotPairAListWithACoincidence(t *testing.T) {
	raw, _ := json.Marshal(crowdedHeatmapSpec("modern-template", 0))
	spec, _ := semantic.ParseJSON(raw)
	input, cr, err := semantic.Compile(spec, semantic.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w := specWording{input: input, ir: cr.IR, vocab: map[int][][2]string{}}
	if got := w.kindVocabulary(patternValues(&input.Slides[1]), 1); len(got) != 0 {
		t.Errorf("vocabulary %v, want none: the spec's lists are the pattern's", got)
	}
	// "Major" is the impact of the fourth risk and the fourth impact level.
	if got := w.fieldOf(patternValues(&input.Slides[1]), "items[3].impact", 1); got != "slides[1].items[3].impact" {
		t.Errorf("items[3].impact located at %q", got)
	}
}

// One rename does not feed another: a label the kind calls a name stays a
// name beside a name the kind calls a title.
func TestRenameWordsReplacesEachWordOnce(t *testing.T) {
	got := renameWords("the label and the names of the steps", [][2]string{{"label", "name"}, {"name", "title"}, {"steps", "options"}})
	if want := "the name and the titles of the options"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// lengthenListTexts appends a clause to every sentence-like string under a
// list of a slide's payload, so the lists of the pattern it compiles to run
// tight. Short strings (levels, dates, values) are left as written.
func lengthenListTexts(node any, clause string, underList bool) any {
	switch t := node.(type) {
	case string:
		if underList && len(t) >= 16 && strings.Contains(t, " ") {
			return t + clause
		}
	case map[string]any:
		for k, child := range t {
			t[k] = lengthenListTexts(child, clause, underList)
		}
	case []any:
		for i, child := range t {
			t[i] = lengthenListTexts(child, clause, true)
		}
	}
	return node
}

// TestLongListFindingsNameTheirField provokes the findings the audit above
// synthesises: every kind's example with the texts of its lists run longer by
// a few words, by a clause and by a sentence, put through validate_deck_spec.
// Each finding's address resolves, and each field a sentence quotes is in the
// spec and on the line of the finding's own path.
//
// The short run lengthens by the few words only, which is the step that
// provokes a pattern's own finding on a field (the longer two mostly degrade
// the slide before its pattern is asked).
func TestLongListFindingsNameTheirField(t *testing.T) {
	mc := refusalTestConfig(t)
	findings, quoted := 0, 0
	clauses := []string{
		" for the year",
		" and what follows from it this year",
		" and the further detail that takes this line well past what its place on the slide can hold at a readable size",
	}
	want := 30
	if testing.Short() {
		clauses, want = clauses[:1], 5
	}
	for _, clause := range clauses {
		for _, kind := range semantic.AllSlideKinds() {
			example := semantic.KindExample(kind)
			if example == nil || kind == semantic.KindRawJSON2pptx {
				continue
			}
			spec := map[string]any{
				"meta": map[string]any{"title": "Wording audit", "source": "Illustrative"},
				"slides": []any{
					map[string]any{"kind": "title", "title": "Wording audit", "subtitle": "October 2026"},
					lengthenListTexts(example, clause, false),
				},
			}
			raw, _ := json.Marshal(spec)
			var doc any
			_ = json.Unmarshal(raw, &doc)
			var out map[string]any
			structuredInto(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "modern-template"}).StructuredContent, &out)
			list, _ := out["findings"].([]any)
			for _, entry := range list {
				f := entry.(map[string]any)
				findings++
				if message, _ := f["message"].(string); messagePointerRE.MatchString(message) {
					quoted++
				}
				assertAuthoredAddress(t, fmt.Sprintf("%s +%d", kind, len(clause)), doc, f)
			}
		}
	}
	t.Logf("%d findings, %d of them quoting a field", findings, quoted)
	if findings < want || quoted == 0 {
		t.Errorf("%d findings, %d quoting a field; the long examples no longer provoke the kinds' findings", findings, quoted)
	}
}

// "options[0].name/detail" names two fields of one option.
func TestSiblingMentions(t *testing.T) {
	for mention, want := range map[string][]string{
		"options[0].name/detail":     {"options[0].name", "options[0].detail"},
		"values[2].date/end_date":    {"values[2].date", "values[2].end_date"},
		"tiers[0].components[1]":     {"tiers[0].components[1]"},
		"steps[3].body":              {"steps[3].body"},
		"values[1]/label":            {"values[1]/label"},
		"rows[1].cells[0].text/note": {"rows[1].cells[0].text", "rows[1].cells[0].note"},
	} {
		if got := siblingMentions(valueMentionsRE.FindString(mention)); !slices.Equal(got, want) && valueMentionsRE.FindString(mention) == mention {
			t.Errorf("%s: %v, want %v", mention, got, want)
		}
	}
}
