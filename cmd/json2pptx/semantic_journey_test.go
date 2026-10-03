package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// The twelve-flaw draft of the agent journey review, replayed by an agent that
// does nothing but apply the findings it is given (go-slide-creator-ipahe,
// go-slide-creator-c2j5b).
//
// The reviewer's draft (persona c-repair, call 4 of its log) carries twelve
// planted flaws: an unknown archetype, an unknown kind, two misspelled keys, a
// chart series one value short, a summary with too many and too long points,
// seven KPIs, a ten-row table, over-long decision labels and process steps,
// topic titles, missing takeaways and no source. It took seven validates to
// clear, because each tier of findings appeared only once the tier above it
// was clean.

// twelveFlawDraft loads the draft from the journey evidence.
func twelveFlawDraft(t *testing.T) map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "tests", "quality", "results", "agent-journey-20261003", "c-repair", "log-calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var call struct {
			N      int `json:"n"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		if json.Unmarshal(sc.Bytes(), &call) != nil || call.N != 4 {
			continue
		}
		spec, ok := call.Params.Arguments["spec"].(map[string]any)
		if call.Params.Name != "validate_deck_spec" || !ok {
			t.Fatalf("call 4 of the c-repair log is not the draft's first validate: %s", call.Params.Name)
		}
		return spec
	}
	t.Fatal("the c-repair log has no call 4")
	return nil
}

// journeyCopy is what the agent writes when a finding asks for words: an
// action title, a takeaway, a source. It is keyed by the draft's own titles,
// so it follows a slide when the deck is reordered. Everything else the agent
// does is read off the finding.
var journeyCopy = struct {
	titles    map[string]string
	takeaways map[string]string
	source    string
	archetype string
}{
	titles: map[string]string{
		"Executive summary":       "Cloud spend is 23% over budget and a FinOps team recovers €9.6M",
		"Key metrics":             "Spend is €7.8M over budget with a third of compute idle",
		"Monthly spend trend":     "Monthly spend rose 26% in six months",
		"Savings funnel":          "€9.6M of €14.2M identified savings is committed",
		"Options comparison":      "Option B saves three times more than a clean-up",
		"Savings by lever":        "Three levers deliver 80% of the savings",
		"Savings by lever (2/2)":  "Five smaller levers add the last €2M",
		"Decision":                "We ask for a decision on option B today",
		"Implementation approach": "Eight steps deliver the savings in 12 months",
		"Thank you":               "Approve option B and start on 1 November",
	},
	takeaways: map[string]string{
		"Executive summary":       "Approve option B to recover €9.6M a year.",
		"Key metrics":             "Spend is out of control.",
		"Monthly spend trend":     "Spend grows every month.",
		"Savings funnel":          "Most identified savings are already committed.",
		"Options comparison":      "Option B pays back fastest.",
		"Savings by lever":        "Compute and commitments carry the case.",
		"Savings by lever (2/2)":  "The long tail is worth €2M.",
		"Implementation approach": "Each step has an owner and a date.",
	},
	source:    "Cloud billing exports, Oct 2025 to Sep 2026",
	archetype: "strategy_proposal",
}

// journeyAgent applies findings to a spec. It reads each finding's code, path,
// paths, evidence and remediation params; it fails the test when a finding
// does not say enough to act on.
type journeyAgent struct {
	t    *testing.T
	spec map[string]any
	// inserts are slides to add, and deferred the list cuts to make, once the
	// round's field edits are done.
	inserts  []journeyInsert
	deferred []func()
}

type journeyInsert struct {
	after map[string]any
	slide map[string]any
}

func newJourneyAgent(t *testing.T, spec map[string]any) *journeyAgent {
	return &journeyAgent{t: t, spec: spec}
}

func (a *journeyAgent) slides() []any { s, _ := a.spec["slides"].([]any); return s }

// draftTitle is the title a slide had in the draft.
func draftTitle(slide map[string]any) string {
	if t, ok := slide["draft_title"].(string); ok {
		return t
	}
	t, _ := slide["title"].(string)
	return t
}

// at resolves a JSON Pointer in the spec.
func (a *journeyAgent) at(pointer string) any {
	a.t.Helper()
	var node any = a.spec
	for _, tok := range pointerTokens(pointer) {
		switch cur := node.(type) {
		case map[string]any:
			node = cur[tok]
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(cur) {
				a.t.Fatalf("finding path %s does not resolve", pointer)
			}
			node = cur[i]
		default:
			a.t.Fatalf("finding path %s does not resolve", pointer)
		}
	}
	return node
}

// parent returns the container of a pointer's last token and that token.
func (a *journeyAgent) parent(pointer string) (any, string) {
	i := strings.LastIndexByte(pointer, '/')
	return a.at(pointer[:i]), strings.ReplaceAll(strings.ReplaceAll(pointer[i+1:], "~1", "/"), "~0", "~")
}

func (a *journeyAgent) set(pointer string, value any) {
	holder, key := a.parent(pointer)
	switch cur := holder.(type) {
	case map[string]any:
		cur[key] = value
	case []any:
		i, _ := strconv.Atoi(key)
		cur[i] = value
	default:
		a.t.Fatalf("cannot set %s", pointer)
	}
}

// patch applies remove, move and add ops to the working spec in place.
func (a *journeyAgent) patch(ops []any) {
	a.t.Helper()
	take := func(pointer string) any {
		holder, key := a.parent(pointer)
		switch cur := holder.(type) {
		case map[string]any:
			v := cur[key]
			delete(cur, key)
			return v
		case []any:
			i, _ := strconv.Atoi(key)
			v := cur[i]
			grand, name := a.parent(pointer[:strings.LastIndexByte(pointer, '/')])
			grand.(map[string]any)[name] = append(cur[:i:i], cur[i+1:]...)
			return v
		}
		a.t.Fatalf("cannot remove %s", pointer)
		return nil
	}
	for _, raw := range ops {
		op, _ := raw.(map[string]any)
		path, _ := op["path"].(string)
		switch op["op"] {
		case "remove":
			take(path)
		case "move":
			from, _ := op["from"].(string)
			v := take(from)
			holder, key := a.parent(path)
			holder.(map[string]any)[key] = v
		case "add", "replace":
			a.set(path, op["value"])
		default:
			a.t.Fatalf("the agent cannot apply op %v", op)
		}
	}
}

// slideOf returns the slide object a pointer sits in.
func (a *journeyAgent) slideOf(pointer string) map[string]any {
	toks := pointerTokens(pointer)
	if len(toks) < 2 || toks[0] != "slides" {
		a.t.Fatalf("finding path %s is not on a slide", pointer)
	}
	slide, _ := a.at("/slides/" + toks[1]).(map[string]any)
	return slide
}

// pathsOf lists every path a finding stands for.
func pathsOf(f diagnostics.Finding) []string {
	if len(f.Paths) > 0 {
		return f.Paths
	}
	if f.Path != nil {
		return []string{*f.Path}
	}
	return nil
}

func fixParams(f diagnostics.Finding) map[string]any {
	if f.Remediation == nil || f.Remediation.Primary == nil {
		return nil
	}
	return f.Remediation.Primary.Params
}

// budgetAt returns the i-th budget of an evidence fact that is a number or a
// list aligned with paths.
func budgetAt(v any, i int) (int, bool) {
	switch b := v.(type) {
	case float64:
		return int(b), true
	case []any:
		if i < len(b) {
			if n, ok := b[i].(float64); ok {
				return int(n), true
			}
		}
	}
	return 0, false
}

// shorten cuts text to at most n characters at a word boundary.
func shorten(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	r := []rune(text)[:n]
	cut := strings.TrimRight(string(r), " ,;:.")
	if i := strings.LastIndexByte(cut, ' '); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:.—-")
}

var leadingNumberRE = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)

// heightRE reads the heights out of a capacity message.
var heightRE = regexp.MustCompile(`([0-9]+)pt`)

// apply acts on one finding.
func (a *journeyAgent) apply(f diagnostics.Finding) {
	a.t.Helper()
	code := f.Code[strings.IndexByte(f.Code, '.')+1:]
	params := fixParams(f)
	if f.PatchVerified {
		// The server tried this patch on the spec: the agent runs it as given
		// (go-slide-creator-vihnl), once the round's other findings — which
		// address list items by today's indexes — are applied.
		patch, _ := f.NextToolCall.ArgsTemplate["patch"].([]any)
		if f.NextToolCall.Tool != "validate_deck_spec" || len(patch) == 0 {
			a.t.Fatalf("%s is marked patch_verified but carries no patch: %+v", f.Code, f.NextToolCall)
		}
		a.deferred = append(a.deferred, func() { a.patch(patch) })
		return
	}
	switch code {
	case "QUALITY_GATE":
		// The gate names the findings it counted; each is applied on its own.
		if !strings.Contains(f.Message, "advisories counted: ") {
			a.t.Fatalf("%s does not name the findings behind it: %s", code, f.Message)
		}

	case "SEMANTIC_UNKNOWN_ARCHETYPE":
		available, _ := f.Evidence["available"].([]any)
		for _, v := range available {
			if v == journeyCopy.archetype {
				a.set(*f.Path, journeyCopy.archetype)
				return
			}
		}
		a.t.Fatalf("%s lists no archetype to choose from: %+v", code, f.Evidence)

	case "SEMANTIC_UNKNOWN_FIELD":
		to, _ := params["did_you_mean"].(string)
		for _, p := range pathsOf(f) {
			holder, key := a.parent(p)
			obj, ok := holder.(map[string]any)
			if !ok {
				a.t.Fatalf("%s at %s is not a key of an object", code, p)
			}
			if to != "" {
				obj[to] = obj[key]
			}
			delete(obj, key)
		}

	case "SEMANTIC_UNKNOWN_KIND":
		kind, _ := params["did_you_mean"].(string)
		hosted, _ := params["hosted_type"].(string)
		if kind != "chart_insight" || hosted != "funnel" {
			a.t.Fatalf("%s does not say which kind hosts the funnel: %+v", code, params)
		}
		slide := a.slideOf(*f.Path)
		stages, _ := slide["stages"].([]any)
		var categories, values []any
		for _, s := range stages {
			stage, _ := s.(map[string]any)
			categories = append(categories, stage["label"])
			n, _ := strconv.ParseFloat(leadingNumberRE.FindString(stage["value"].(string)), 64)
			values = append(values, n)
		}
		delete(slide, "stages")
		slide["kind"] = kind
		slide["chart"] = map[string]any{"type": hosted, "data": map[string]any{"categories": categories, "values": values}}
		slide["insights"] = []any{"Two thirds of the identified savings are committed", "Delivery has only started"}
		slide["takeaway"] = journeyCopy.takeaways[draftTitle(slide)]

	case "CHART_SERIES_LENGTH_MISMATCH":
		values, _ := a.at(*f.Path).([]any)
		data, _ := a.at(strings.Split(*f.Path, "/series/")[0]).(map[string]any)
		categories, _ := data["categories"].([]any)
		for len(values) < len(categories) {
			values = append(values, 4.1)
		}
		a.set(*f.Path, values)

	case "SEMANTIC_TAKEAWAY_REQUIRED":
		for _, p := range pathsOf(f) {
			slide := a.slideOf(p)
			text, ok := journeyCopy.takeaways[draftTitle(slide)]
			if !ok {
				a.t.Fatalf("no takeaway written for %q", draftTitle(slide))
			}
			slide["takeaway"] = text
		}

	case "SEMANTIC_PATTERN_DEGRADED":
		if allowed, budgeted := f.Evidence["allowed"]; budgeted {
			for i, p := range pathsOf(f) {
				n, ok := budgetAt(allowed, i)
				text, isText := a.at(p).(string)
				if !ok || !isText {
					a.t.Fatalf("%s at %s names no budget for a text field: %+v", code, p, f.Evidence)
				}
				if strings.Contains(f.Message, "{lead, support}") {
					// "a plain string is all lead": keep the first clause as the
					// lead and the next as its support.
					lead := shorten(text, n)
					rest := strings.TrimLeft(strings.TrimPrefix(text, lead), " ,;:.")
					a.set(p, map[string]any{"lead": lead, "support": shorten(rest, n)})
					continue
				}
				a.set(p, shorten(text, n))
			}
			return
		}
		maxItems, ok := params["max_items"].(float64)
		list, isList := a.at(*f.Path).([]any)
		if !ok || !isList {
			a.t.Fatalf("%s at %s names neither a text budget nor a count: %s %+v", code, *f.Path, f.Message, params)
		}
		if len(list) > int(maxItems) {
			// Other findings of this response still address the items by
			// index: cut the list once they are applied.
			path := *f.Path
			a.deferred = append(a.deferred, func() { a.set(path, list[:int(maxItems)]) })
		}

	case "SEMANTIC_DENSITY":
		maxRows, ok := params["max_rows"].(float64)
		rows, isList := a.at(*f.Path).([]any)
		if !ok || !isList {
			a.t.Fatalf("%s at %s does not say how many rows fit: %s %+v", code, *f.Path, f.Message, params)
		}
		slide := a.slideOf(*f.Path)
		keep := (len(rows) + 1) / 2
		if keep > int(maxRows)-1 {
			keep = int(maxRows) - 1
		}
		second := map[string]any{}
		for k, v := range slide {
			second[k] = v
		}
		slide["rows"], second["rows"] = rows[:keep], rows[keep:]
		second["draft_title"] = draftTitle(slide) + " (2/2)"
		second["title"] = second["draft_title"]
		delete(second, "id")
		a.inserts = append(a.inserts, journeyInsert{after: slide, slide: second})

	case "TITLE_NOT_ACTION":
		for _, p := range pathsOf(f) {
			slide := a.slideOf(p)
			title, ok := journeyCopy.titles[draftTitle(slide)]
			if !ok {
				a.t.Fatalf("no action title written for %q", draftTitle(slide))
			}
			if _, kept := slide["draft_title"]; !kept {
				slide["draft_title"] = draftTitle(slide)
			}
			slide["title"] = title
		}

	case "DATA_WITHOUT_SOURCE":
		meta, _ := a.spec["meta"].(map[string]any)
		meta["source"] = journeyCopy.source

	case "SEMANTIC_RHYTHM_MONOTONY":
		// "vary the slide kinds or insert a section break": the agent inserts
		// the break before the slide the finding names.
		run, _ := f.Evidence["run"].([]any)
		if len(run) < 3 {
			a.t.Fatalf("%s does not list the slides of the run: %+v", code, f.Evidence)
		}
		// Break after every second slide of the run.
		for i := 1; i < len(run)-1; i += 2 {
			a.inserts = append(a.inserts, journeyInsert{after: a.slideOf(run[i].(string)), slide: map[string]any{"kind": "section", "title": "The decision"}})
		}

	case "BODY_TOO_LONG":
		// On a summary whose leads are already inside the budget of the first
		// response, the option left of the three the finding lists is "use
		// fewer points".
		slide := a.slideOf(*f.Path)
		points, isSummary := slide["points"].([]any)
		heights := heightRE.FindAllStringSubmatch(f.Message, 2)
		if slide["kind"] != "executive_summary" || !isSummary || !strings.Contains(f.Message, "use fewer points") || len(heights) != 2 {
			a.t.Fatalf("the agent does not know how to apply %s at %s: %s", code, *f.Path, f.Message)
		}
		// "needs 337pt … holds about 311pt": drop as many rows as the deficit
		// is tall, a row being the needed height shared between the points
		// and the bottom line.
		needs, _ := strconv.ParseFloat(heights[0][1], 64)
		holds, _ := strconv.ParseFloat(heights[1][1], 64)
		row := needs / float64(len(points)+1)
		drop := int((needs-holds)/row) + 1
		if len(points)-drop < 3 {
			a.t.Fatalf("%s at %s cannot be cleared by fewer points: %s", code, *f.Path, f.Message)
		}
		slide["points"] = points[:len(points)-drop]

	case "chart.label_truncated":
		// A category label that does not fit its band: write a shorter one.
		original, _ := params["original"].(string)
		slide := a.slideOf(*f.Path)
		chart, _ := slide["chart"].(map[string]any)
		data, _ := chart["data"].(map[string]any)
		categories, _ := data["categories"].([]any)
		done := false
		for i, c := range categories {
			if label, _ := c.(string); label != "" && strings.HasPrefix(original, label) && utf8.RuneCountInString(label) > 5 {
				categories[i] = string([]rune(label)[:5]) + "."
				done = true
			}
		}
		if !done {
			a.t.Fatalf("%s does not name a category to shorten: %+v", code, params)
		}

	case "CLOSING_WITHOUT_NEXT_STEPS":
		slide := a.slideOf(*f.Path)
		slide["draft_title"] = draftTitle(slide)
		slide["title"] = journeyCopy.titles[draftTitle(slide)]

	default:
		if f.Severity == diagnostics.SeverityInfo {
			// A note with nothing in it to act on (a contrast the renderer
			// fixes itself, a word wider than its box). It does not block and
			// is not a warning; the agent leaves it.
			a.t.Logf("  left as is: %s at %v", f.Code, pathsOf(f))
			return
		}
		a.t.Fatalf("the agent does not know how to apply %s at %v: %s\n  evidence %+v\n  params %+v", f.Code, pathsOf(f), f.Message, f.Evidence, params)
	}
}

// journeyClean reports whether a validate response leaves the agent nothing to
// do: the deck is ok and carries no error and no warning. strict also rules
// out info-level notes.
func journeyClean(env deckSpecEnvelopeResponse, strict bool) bool {
	if !env.OK {
		return false
	}
	for _, f := range env.Findings {
		if strict || f.Severity != diagnostics.SeverityInfo {
			return false
		}
	}
	return true
}

// finishRound applies the round's structural edits and returns the spec to
// send: the working copy without the agent's own bookkeeping.
func (a *journeyAgent) finishRound() map[string]any {
	for _, cut := range a.deferred {
		cut()
	}
	a.deferred = nil
	for _, ins := range a.inserts {
		slides := a.slides()
		for i, s := range slides {
			if m, ok := s.(map[string]any); ok && sameMap(m, ins.after) {
				next := append([]any{}, slides[:i+1]...)
				next = append(next, ins.slide)
				a.spec["slides"] = append(next, slides[i+1:]...)
				break
			}
		}
	}
	a.inserts = nil
	raw, err := json.Marshal(a.spec)
	if err != nil {
		a.t.Fatal(err)
	}
	var send map[string]any
	if err := json.Unmarshal(raw, &send); err != nil {
		a.t.Fatal(err)
	}
	for _, s := range send["slides"].([]any) {
		delete(s.(map[string]any), "draft_title")
	}
	return send
}

// sameMap reports whether a and b are the same map object.
func sameMap(a, b map[string]any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// TestTwelveFlawDraftCleanInThreeRoundTrips is the go-slide-creator-ipahe
// acceptance test: applying each finding of each response, the draft is clean
// on the third validate at the latest. Clean means ok with no error and no
// warning. A note the author cannot act on from the finding alone may remain:
// a predicted contrast fix on the inserted divider, or TEXT_WRAPS_NARROW on the
// eight-step process, whose boxes wrap although every step is inside the
// budget the first response stated (the composition check that reports it
// arrived after this test and names no field).
func TestTwelveFlawDraftCleanInThreeRoundTrips(t *testing.T) {
	templates := []string{"modern", "midnight-blue"}
	if !testing.Short() {
		templates = shippedTemplateNames(t)
	}
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			mc := refusalTestConfig(t)
			agent := newJourneyAgent(t, twelveFlawDraft(t))
			send := agent.finishRound()
			const maxRoundTrips = 3
			for round := 1; ; round++ {
				env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": send, "template": tpl}))
				t.Logf("validate %d: ok=%v, %s", round, env.OK, env.Summary)
				for _, f := range env.Findings {
					t.Logf("  %s %s %v: %s", f.Severity, f.Code, pathsOf(f), f.Message)
				}
				if journeyClean(env, false) {
					// What validate calls clean, render writes and calls ready.
					render := renderDeckSpecCall(t, mc, map[string]any{"spec": send, "template": tpl})
					if !render.Success || render.DeterministicReady == nil || !*render.DeterministicReady {
						t.Errorf("the clean spec does not render ready: %q %v", render.Error, render.DeterministicBlockingReasons)
					}
					return
				}
				if round == maxRoundTrips {
					t.Fatalf("validate %d is not clean: %s", round, env.Summary)
				}
				for _, f := range env.Findings {
					agent.apply(f)
				}
				send = agent.finishRound()
			}
		})
	}
}
