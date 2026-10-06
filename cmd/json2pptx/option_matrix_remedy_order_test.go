package main

import (
	"context"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// The remedies of an over-full option_matrix run from the one that keeps most
// (go-slide-creator-u8orh).
//
// Two agent runs sent a five-row status board with a short detail under each
// name. Validation predicted 5-6pt text for a table 5pt too tall, offered as
// its verified fix the switch to a bullet list (the board was the point of the
// slide), and listed "drop the slide takeaway" among the remedies — which
// SEMANTIC_TAKEAWAY_REQUIRED answered on the next call.

func statusBoardSpec() map[string]any {
	option := func(name, detail string, scores ...any) map[string]any {
		return map[string]any{"name": name, "detail": detail, "scores": scores}
	}
	return map[string]any{
		"meta": map[string]any{"title": "ITGC readout"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "ITGC readout", "subtitle": "Audit committee"},
			map[string]any{
				"kind": "option_matrix", "title": "Two of five domains are rated red: access management and third-party",
				"corner_label": "Domain",
				"criteria": []any{
					map[string]any{"label": "Controls tested", "scale": "text"},
					map[string]any{"label": "Exceptions", "scale": "text"},
					map[string]any{"label": "Rating", "scale": "rag"},
				},
				"options": []any{
					option("Access management", "Joiners, movers, leavers; privileged access", "14", "3", "red"),
					option("Change management", "Standard and emergency change", "11", "1", "amber"),
					option("Computer operations", "Batch, backup, incident and job scheduling", "9", "0", "green"),
					option("Program development", "SDLC gates, testing and release approval", "6", "1", "amber"),
					option("Third-party / cloud", "Provider assurance and contract controls", "8", "2", "red"),
				},
				"decisive_criterion": "Rating",
				"takeaway":           "48 controls tested, 7 exceptions; overall opinion partially effective.",
				"source":             "Internal Audit, FY26 ITGC review",
			},
		},
	}
}

// overfullStatusBoardSpec is the status board with a sixth row and a Harvey
// rating, whose legend names what the balls mean and stays: a few points
// taller than blue-corporate's content area under a takeaway and a source
// line with everything but its text given up (go-slide-creator-dwha2 made the
// five-row board itself fit; the one-line takeaway band of
// go-slide-creator-me53q left 17pt more, and on the five templates with
// midnight-blue's 271pt area the six-row board now fits too).
func overfullStatusBoardSpec() map[string]any {
	spec := statusBoardSpec()
	board := spec["slides"].([]any)[1].(map[string]any)
	board["criteria"].([]any)[2] = map[string]any{"label": "Rating", "scale": "harvey"}
	options := append(board["options"].([]any), map[string]any{
		"name": "End-user computing", "detail": "Spreadsheets and desktop databases", "scores": []any{"5", "1", "amber"},
	})
	for i, o := range options {
		o.(map[string]any)["scores"].([]any)[2] = []any{0, 2, 4, 2, 0, 2}[i]
	}
	board["options"] = options
	return spec
}

func TestOptionMatrixRemediesKeepTheVisualAndTheTakeaway(t *testing.T) {
	mc := handleTestConfig(t)
	res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": overfullStatusBoardSpec(), "template": "blue-corporate"}))
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Findings []struct {
			Code, Path, Message string
			PatchVerified       bool `json:"patch_verified"`
			Evidence            struct {
				Measured map[string]any `json:"measured"`
			} `json:"evidence"`
			NextToolCall struct {
				ArgsTemplate struct {
					Patch []map[string]any `json:"patch"`
				} `json:"args_template"`
			} `json:"next_tool_call"`
		} `json:"findings"`
	}
	structuredInto(t, res.StructuredContent, &env)
	found := false
	for _, f := range env.Findings {
		if !strings.HasSuffix(f.Code, "BODY_TOO_LONG") {
			continue
		}
		if strings.Contains(f.Message, "takeaway") {
			t.Errorf("%s at %s proposes dropping a takeaway the kind requires: %s", f.Code, f.Path, f.Message)
		}
		if f.Path != "/slides/1" {
			continue
		}
		found = true
		if !f.PatchVerified || len(f.NextToolCall.ArgsTemplate.Patch) == 0 {
			t.Fatalf("the capacity finding offers no verified patch: %s", f.Message)
		}
		for _, op := range f.NextToolCall.ArgsTemplate.Patch {
			path, _ := op["path"].(string)
			if op["op"] != "remove" || !strings.HasSuffix(path, "/detail") {
				t.Errorf("verified patch op %v: want the removal of an option's detail line, which keeps the board and every option", op)
			}
		}
		if !strings.Contains(f.Message, "shorten the option details, drop them, or use fewer options or criteria") {
			t.Errorf("remedies are not in order of what they keep: %s", f.Message)
		}
		// The board is a few points too tall, not half its size.
		if pt, _ := f.Evidence.Measured["font_pt"].(float64); pt < 9 {
			t.Errorf("predicted %.1fpt for a table a few points taller than its area: %s", pt, f.Message)
		}
	}
	if !found {
		t.Fatal("a six-row board with details, a Harvey legend, a takeaway and a source line fits blue-corporate: nothing was remedied")
	}
}

// appetiteBoardSpec is the risk appetite board of the second run: two rows
// tagged "Breached", which gave each a third line.
func appetiteBoardSpec() map[string]any {
	option := func(name, detail string, scores ...any) map[string]any {
		return map[string]any{"name": name, "detail": detail, "scores": scores}
	}
	return map[string]any{
		"meta": map[string]any{"title": "ERM uplift"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "ERM uplift", "subtitle": "Board risk committee"},
			map[string]any{
				"kind": "option_matrix", "title": "Two of five risk appetite metrics are breached, one is amber",
				"corner_label": "Risk type",
				"criteria": []any{
					map[string]any{"label": "Status", "scale": "rag"},
					map[string]any{"label": "Current", "scale": "text"},
					map[string]any{"label": "Limit", "scale": "text"},
				},
				"options": []any{
					option("Credit", "Non-performing loans", "green", "NPL 2.1%", "3%"),
					option("Liquidity", "Liquidity coverage ratio", "green", "LCR 148%", "110%"),
					option("Operational", "Annual losses", "red", "EUR 11.2M", "EUR 8M"),
					option("Conduct", "Complaints per 10k customers", "amber", "37", "30"),
					option("Cyber", "Critical vulnerabilities unpatched >30 days", "red", "2", "0"),
				},
				"recommended":        []any{"Operational", "Cyber"},
				"highlight_label":    "Breached",
				"decisive_criterion": "Status",
				"takeaway":           "Operational and cyber are breached; conduct is drifting towards its limit.",
				"source":             "Regulator's 2025 review; management information 2023-2025",
			},
		},
	}
}

// A status board of five rows and three columns with a line of detail under
// each name, a takeaway and a source line is readable on every template
// (go-slide-creator-dwha2). Both boards were refused on the four templates the
// runs used, 5pt and 35pt taller than the 253pt content area (94pt on
// modern-template, whose face wrapped the details).
//
// The short race run measures both boards on modern-template, the template
// that refused them by the most, and on modern, the shortest content area; the
// integration corpus job measures every template (go-slide-creator-efhg2).
func TestStatusBoardWithDetailsIsReadableOnEveryTemplate(t *testing.T) {
	mc := handleTestConfig(t)
	templates := testutil.AllTestTemplateNames()
	if testing.Short() {
		templates = []string{"modern-template", "modern"}
	}
	for _, tmpl := range templates {
		for name, spec := range map[string]map[string]any{"audit": statusBoardSpec(), "appetite": appetiteBoardSpec()} {
			res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec, "template": tmpl}))
			if err != nil {
				t.Fatal(err)
			}
			var env struct {
				Findings []struct {
					Code, Severity, Path, Message string
					Evidence                      struct {
						Symptoms []struct{ Code, Message string } `json:"symptoms"`
					} `json:"evidence"`
				} `json:"findings"`
			}
			structuredInto(t, res.StructuredContent, &env)
			for _, f := range env.Findings {
				if f.Path != "/slides/1" && !strings.HasPrefix(f.Path, "/slides/1/") {
					continue
				}
				if f.Severity == "error" || strings.HasSuffix(f.Code, "BODY_TOO_LONG") || strings.HasSuffix(f.Code, "TEXT_BELOW_READABLE_MIN") {
					t.Errorf("%s board on %s: %s %s at %s: %s", name, tmpl, f.Severity, f.Code, f.Path, f.Message)
				}
				for _, s := range f.Evidence.Symptoms {
					if s.Code == "TEXT_BELOW_READABLE_MIN" {
						t.Errorf("%s board on %s: %s", name, tmpl, s.Message)
					}
				}
			}
		}
	}
}

func TestCutCandidatesKeepARequiredTakeawayAndOrderByWhatIsKept(t *testing.T) {
	slide := func(kind string) map[string]any {
		return map[string]any{
			"kind": kind, "takeaway": "Short.",
			"options": []any{
				map[string]any{"name": "Alpha", "detail": "First detail line"},
				map[string]any{"name": "Beta", "detail": "Second detail"},
				map[string]any{"name": "Gamma", "detail": "Third"},
			},
		}
	}
	removes := func(c cutCandidate) []string {
		var out []string
		for _, op := range c.ops {
			out = append(out, op.(map[string]any)["path"].(string))
		}
		return out
	}

	required := cutCandidates("/slides/3", slide("option_matrix"), "/slides/3", nil)
	rank := map[string]int{}
	for i, c := range required {
		paths := removes(c)
		switch {
		case len(paths) > 1:
			rank["list"] = i
		case c.entry:
			rank["entry"] = i
		default:
			rank["line"] = i
		}
		for _, p := range paths {
			if p == "/slides/3/takeaway" {
				t.Errorf("a cut removes the takeaway an option_matrix requires")
			}
		}
	}
	if len(rank) != 3 || rank["line"] >= rank["list"] || rank["list"] >= rank["entry"] {
		t.Errorf("cuts are not ordered line < every detail line < entry: %v", rank)
	}

	// image_case carries no required takeaway: its removal stays a candidate.
	optional := cutCandidates("/slides/3", slide("image_case"), "/slides/3", nil)
	kept := false
	for _, c := range optional {
		kept = kept || removes(c)[0] == "/slides/3/takeaway"
	}
	if !kept {
		t.Errorf("an optional takeaway is no longer a cut candidate")
	}

	// The search has a fixed number of tries: a long list still gets its
	// every-detail-line cut and an entry.
	long := slide("option_matrix")
	for i := 0; i < 12; i++ {
		long["options"] = append(long["options"].([]any), map[string]any{"name": "N", "detail": "D"})
	}
	capped := cutCandidates("/slides/3", long, "/slides/3", nil)
	if len(capped) > maxCutCandidates {
		t.Fatalf("%d candidates, over the limit of %d", len(capped), maxCutCandidates)
	}
	var lists, entries int
	for _, c := range capped {
		if len(c.ops) > 1 {
			lists++
		}
		if c.entry {
			entries++
		}
	}
	if lists != 1 || entries != 1 {
		t.Errorf("capped candidates hold %d list cuts and %d entry cuts, want one of each", lists, entries)
	}
}

func TestDropTakeawayAdviceIsRemovedFromASentence(t *testing.T) {
	for in, want := range map[string]string{
		"taller than this layout's content area holds — drop the headline, so_what or source, use fewer or shorter insights, or drop the slide takeaway": "taller than this layout's content area holds — drop the headline, so_what or source, or use fewer or shorter insights",
		"so the pillar text is written smaller — shorten the pillars, drop a level, or drop the slide's takeaway":                                        "so the pillar text is written smaller — shorten the pillars, or drop a level",
		"a takeaway or source band shortens it": "a takeaway or source band shortens it",
	} {
		if got := dropTakeawayAdviceRE.ReplaceAllString(in, ", or ${1}"); got != want {
			t.Errorf("got  %q\nwant %q", got, want)
		}
	}
}
