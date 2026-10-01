package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

func storylineDeck(t *testing.T, raw string) *PresentationInput {
	t.Helper()
	var input PresentationInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &input
}

func findingsWithCode(findings []patterns.FitFinding, code string) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range findings {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

func contentSlide(title string) SlideInput {
	t := title
	body := []string{"Enterprise renewals carried the quarter"}
	return SlideInput{SlideType: "content", Content: []ContentInput{
		{PlaceholderID: "title", Type: "text", TextValue: &t},
		{PlaceholderID: "body", Type: "bullets", BulletsValue: &body},
	}}
}

// TestTitleNotActionQualityTable pins QUALITY.md's do / don't table: every
// topic title on the left draws a review-weighted TITLE_NOT_ACTION, every
// action title on the right draws none (go-slide-creator-kuurd).
func TestTitleNotActionQualityTable(t *testing.T) {
	rows := []struct{ topic, action string }{
		{"Revenue Trajectory", "Revenue grew 41% this year, and the EMEA launch drove the Q4 step-up"},
		{"Market Overview", "A $180B market is shifting to managed AI infrastructure"},
		{"Options", "Funding an SMB success pod is the cheapest way to protect retention"},
		{"Next Steps", "Approve the pilot budget by 15 March to launch in Q3"},
	}
	for _, row := range rows {
		// A closing slide follows, so neither title sits on the deck's last
		// slide (where a short label is allowed).
		input := &PresentationInput{Slides: []SlideInput{contentSlide(row.topic), contentSlide(row.action), {SlideType: "title"}}}
		got := collectTitleNotActionFindings(input, nil)
		if len(got) != 1 || got[0].Path != "/slides/0/content/0" || got[0].Action != "review" {
			t.Errorf("%q / %q: TITLE_NOT_ACTION = %+v, want one review finding on the topic title", row.topic, row.action, got)
		}
	}
}

func TestTitleNotActionReasons(t *testing.T) {
	cases := []struct {
		title       string
		hasTakeaway bool
		want        string
	}{
		{"Revenue Trajectory And Regional Performance Across The Fiscal Year With A Focus On The EMEA Launch Impact", false, "too_long"},
		{"Revenue Trajectory And Regional Performance Across The Fiscal Year With A Focus On The EMEA Launch Impact", true, "too_long"},
		{"Executive Summary", true, "stock_label"},
		{"Key Metrics", false, "stock_label"},
		{"Pipeline Metrics", false, "stock_label"},
		{"Margin Analysis", false, "stock_label"},
		{"Churn", false, "no_verb_or_number"},
		{"Customer Journey Maturity Model", false, "no_verb_or_number"},
		{"Churn", true, ""},
		{"Three drivers explain the SMB churn increase", false, ""},
		{"Every headline metric improved in Q3 except SMB churn", false, ""},
		// Sentence case, six words or more: a sentence even when its verb is
		// outside the lexicon.
		{"Enterprise accounts now underwrite the growth plan", false, ""},
		{"Q3 Business Review", false, "stock_label"},
	}
	for _, c := range cases {
		if got := titleNotActionReason(c.title, c.hasTakeaway); got != c.want {
			t.Errorf("titleNotActionReason(%q, takeaway=%v) = %q, want %q", c.title, c.hasTakeaway, got, c.want)
		}
	}
}

// TestTitleNotActionExemptsNavigationAndClosers keeps the review QBR's
// next_steps closer ("Next steps") and agenda slides out of the finding.
func TestTitleNotActionExemptsNavigationAndClosers(t *testing.T) {
	input := storylineDeck(t, `{"slides":[
	 {"layout_id":"title","slide_type":"title","content":[{"placeholder_id":"title","type":"text","text_value":"Q3 Review"}]},
	 {"layout_id":"blank-title","slide_type":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Contents"}],"pattern":{"name":"agenda","values":{"items":["Results","Plan"]}}},
	 {"layout_id":"blank-title","slide_type":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Next steps"}],"pattern":{"name":"next-steps","values":{"actions":[{"action":"Approve budget","owner":"CFO","date":"15 Oct"},{"action":"Hire AEs","owner":"CRO","date":"1 Nov"}]}}},
	 {"layout_id":"title","slide_type":"title","content":[{"placeholder_id":"title","type":"text","text_value":"Thank you"}]}
	]}`)
	if got := collectTitleNotActionFindings(input, nil); len(got) != 0 {
		t.Fatalf("navigation / closer slides drew TITLE_NOT_ACTION: %+v", got)
	}
}

func TestStorylineFindings(t *testing.T) {
	slides := []SlideInput{{SlideType: "title"}}
	for i := 0; i < 5; i++ {
		slides = append(slides, contentSlide("Revenue grew 18% on enterprise renewals"))
	}
	thanks := "Thank you"
	slides = append(slides, SlideInput{SlideType: "title", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &thanks}}})
	input := &PresentationInput{Slides: slides}

	got := collectStorylineFindings(input)
	if f := findingsWithCode(got, patterns.ErrCodeNoExecutiveSummary); len(f) != 1 || f[0].Path != "/slides/1" || f[0].Action != "review" {
		t.Errorf("NO_EXECUTIVE_SUMMARY = %+v, want one review finding on /slides/1", f)
	}
	if f := findingsWithCode(got, patterns.ErrCodeClosingWithoutNextSteps); len(f) != 1 || f[0].Path != "/slides/6/content/0" {
		t.Errorf("CLOSING_WITHOUT_NEXT_STEPS = %+v, want one finding on the closer's title", f)
	}

	// An exec-summary pattern and a next-steps slide clear both.
	input.Slides[1].Pattern = &PatternInput{Name: "exec-summary"}
	input.Slides[5].Pattern = &PatternInput{Name: "next-steps"}
	if got := collectStorylineFindings(input); len(got) != 0 {
		t.Errorf("complete storyline still drew findings: %+v", got)
	}

	// A short deck is not expected to carry an executive summary.
	short := &PresentationInput{Slides: slides[:4]}
	if f := findingsWithCode(collectStorylineFindings(short), patterns.ErrCodeNoExecutiveSummary); len(f) != 0 {
		t.Errorf("four-slide deck drew NO_EXECUTIVE_SUMMARY: %+v", f)
	}
}

func TestSlideTextDense(t *testing.T) {
	seven := []string{"One", "Two", "Three", "Four", "Five", "Six", "Seven"}
	five := []string{"Revenue grew 18%", "Margin reached 61%", "NRR held at 112%", "Churn rose to 4.1%", "Cycle fell to 44 days"}
	long := strings.Repeat("word ", 90)
	title := "Revenue grew 18% on enterprise renewals"
	mk := func(items ...ContentInput) SlideInput {
		return SlideInput{SlideType: "content", Content: append([]ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}, items...)}
	}
	input := &PresentationInput{Slides: []SlideInput{
		mk(ContentInput{PlaceholderID: "body", Type: "bullets", BulletsValue: &seven}),
		mk(ContentInput{PlaceholderID: "body", Type: "bullets", BulletsValue: &five}),
		mk(ContentInput{PlaceholderID: "body", Type: "text", TextValue: &long}),
	}}
	got := collectTextDensityFindings(input, nil)
	if len(got) != 2 || got[0].Path != "/slides/0/content/1" || got[1].Path != "/slides/2/content/1" {
		t.Fatalf("SLIDE_TEXT_DENSE = %+v, want slides 0 (bullets) and 2 (words)", got)
	}
	if got[0].Action != "review" || got[0].Fix == nil || got[0].Fix.Kind != "split_bullets" || got[0].Fix.Params["max_items"] != slideDenseMaxBullets {
		t.Errorf("bullet wall fix = %+v, want split_bullets max_items 6", got[0].Fix)
	}
	if got[1].Fix == nil || got[1].Fix.Kind != "rewrite_field" {
		t.Errorf("word wall fix = %+v, want rewrite_field", got[1].Fix)
	}

	// A read-ahead deck carries up to 110 words.
	input.ViewingMode = "read"
	if f := collectTextDensityFindings(input, nil); len(f) != 1 || f[0].Path != "/slides/0/content/1" {
		t.Errorf("read mode SLIDE_TEXT_DENSE = %+v, want only the seven-bullet slide", f)
	}
}

// TestTakeawayMissingReachesScoreGate pins go-slide-creator-cti7s: the gate's
// require_takeaway_on_charts criterion now sees takeaway_missing for raw
// charts and the data patterns that argue from data, and stands down once a
// takeaway is set.
func TestTakeawayMissingReachesScoreGate(t *testing.T) {
	deck := `{"slides":[
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Revenue"},{"placeholder_id":"body","type":"chart","chart_value":{"type":"bar","data":{"Q1":10,"Q2":20}}}]},
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Revenue mix"}],"pattern":{"name":"chart-insights-split","values":{"chart":{"type":"bar_chart","data":{"categories":["A","B"],"series":[{"name":"S","values":[1,2]}]}},"insights":["A leads"]}}},
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"EBITDA bridge"}],"pattern":{"name":"waterfall-bridge","values":{"columns":[{"label":"Start","value":10,"type":"total"},{"label":"Cost","value":-2,"type":"delta"},{"label":"End","value":8,"type":"total"}]}}},
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Options"}],"pattern":{"name":"table-highlight","values":{"options":["A","B"],"criteria":["Cost","Speed"],"scores":[[1,2],[3,4]]}}}
	]}`
	input := storylineDeck(t, deck)
	got := findingsWithCode(collectFitFindings(input, nil, 12192000, 6858000, nil), patterns.ErrCodeTakeawayMissing)
	if len(got) != 4 {
		t.Fatalf("takeaway_missing on %d slides, want 4: %+v", len(got), got)
	}
	ds := deterministic.ScoreFromFindings(got, len(input.Slides))
	gate := deterministic.EvaluateQualityGate(ds, got, deterministic.DefaultQualityGateCriteria())
	if gate.Passed || !strings.Contains(strings.Join(gate.Reasons, "; "), "require_takeaway_on_charts") {
		t.Fatalf("gate = %+v, want a require_takeaway_on_charts failure", gate)
	}

	for i := range input.Slides {
		input.Slides[i].Takeaway = "Enterprise demand carried the quarter."
	}
	if got := findingsWithCode(collectFitFindings(input, nil, 12192000, 6858000, nil), patterns.ErrCodeTakeawayMissing); len(got) != 0 {
		t.Fatalf("takeaway set but takeaway_missing remains: %+v", got)
	}
}

func TestDeckSourceDefault(t *testing.T) {
	input := storylineDeck(t, `{"source":"Company filings, FY25","slides":[
	 {"layout_id":"title","slide_type":"title","content":[{"placeholder_id":"title","type":"text","text_value":"Cover"}]},
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"ARR reached $4M"}],"pattern":{"name":"kpi-3up","values":[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]}},
	 {"layout_id":"content","source":"CRM export","content":[{"placeholder_id":"title","type":"text","text_value":"Pipeline doubled"}],"pattern":{"name":"kpi-3up","values":[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]}},
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Three pillars hold the plan"},{"placeholder_id":"body","type":"bullets","bullets_value":["a"]}]},
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Revenue grew"},{"placeholder_id":"body","type":"chart","chart_value":{"type":"bar","footnote":"Source: Finance","data":{"Q1":1,"Q2":2}}}]}
	]}`)
	applyDefaults(input)
	wants := []string{"", "Company filings, FY25", "CRM export", "", ""}
	for i, want := range wants {
		if got := input.Slides[i].Source; got != want {
			t.Errorf("slide %d source = %q, want %q", i, got, want)
		}
	}
	if f := findingsWithCode(collectDataWithoutSourceFindings(input), patterns.ErrCodeDataWithoutSource); len(f) != 0 {
		t.Errorf("DATA_WITHOUT_SOURCE after the deck default / chart footnote: %+v", f)
	}

	input.Source = ""
	input.Slides[1].Source = ""
	input.Slides[4].Content[1].ChartValue.Footnote = ""
	f := findingsWithCode(collectDataWithoutSourceFindings(input), patterns.ErrCodeDataWithoutSource)
	if len(f) != 2 || f[0].Action != "review" {
		t.Errorf("DATA_WITHOUT_SOURCE = %+v, want two review findings", f)
	}
}

func TestDeckSpecMetaSourceCompilesToDeckDefault(t *testing.T) {
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "Results", Template: "midnight-blue", Source: "Finance, FY26 close"},
		Slides: []semantic.SlideSpec{{Kind: semantic.KindKPISnapshot, Body: map[string]any{
			"title": "ARR reached $48M and every metric improved",
			"kpis":  []any{map[string]any{"value": "$48M", "label": "ARR"}, map[string]any{"value": "118%", "label": "NRR"}, map[string]any{"value": "41d", "label": "Cycle"}},
		}}},
	}
	input, _, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatal(err)
	}
	applyDefaults(input)
	if input.Source != "Finance, FY26 close" || input.Slides[0].Source != "Finance, FY26 close" {
		t.Fatalf("meta.source did not reach the data slide: deck %q, slide %q", input.Source, input.Slides[0].Source)
	}
}

// TestStorylineGateCalibration is the bead's acceptance pair: the review's weak
// deck (topic titles, unsourced KPIs, seven-bullet walls, no executive
// summary, a "Thank you" closer) fails score_deck's gate on storyline reasons,
// and the review's QBR DeckSpec draws none of them.
func TestStorylineGateCalibration(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(0)}
	score := func(name string) *deterministic.DeckScore {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("testdata", "storyline", name))
		if err != nil {
			t.Fatal(err)
		}
		res, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{"presentation": mustParseJSON(string(data))}))
		if err != nil || res == nil || res.IsError {
			t.Fatalf("%s: score_deck failed: %v %s", name, err, textContent(res))
		}
		var ds deterministic.DeckScore
		if err := json.Unmarshal([]byte(textContent(res)), &ds); err != nil {
			t.Fatal(err)
		}
		return &ds
	}
	storylineReason := func(reasons []string) []string {
		var out []string
		for _, r := range reasons {
			if strings.Contains(r, "TITLE_NOT_ACTION") || strings.Contains(r, "require_storyline") || strings.Contains(r, "require_takeaway_on_charts") {
				out = append(out, r)
			}
		}
		return out
	}

	weak := score("weak_mixed_deck.json")
	if weak.QualityGate == nil || weak.QualityGate.Passed || len(storylineReason(weak.QualityGate.Reasons)) == 0 {
		t.Fatalf("weak deck gate = %+v, want a failure on storyline reasons", weak.QualityGate)
	}
	codes := map[string]bool{}
	for _, s := range weak.PerSlide {
		for _, f := range s.Findings {
			codes[f.Code] = true
		}
	}
	for _, code := range []string{patterns.ErrCodeTitleNotAction, patterns.ErrCodeDataWithoutSource, patterns.ErrCodeNoExecutiveSummary, patterns.ErrCodeClosingWithoutNextSteps, patterns.ErrCodeSlideTextDense, patterns.ErrCodeTakeawayMissing} {
		if !codes[code] {
			t.Errorf("weak deck lacks %s", code)
		}
	}

	good := score("review_qbr_deck.json")
	if r := storylineReason(good.QualityGate.Reasons); len(r) != 0 {
		t.Errorf("review QBR fails storyline criteria: %v", r)
	}
	for _, s := range good.PerSlide {
		for _, f := range s.Findings {
			switch f.Code {
			case patterns.ErrCodeTitleNotAction, patterns.ErrCodeNoExecutiveSummary, patterns.ErrCodeClosingWithoutNextSteps, patterns.ErrCodeSlideTextDense, patterns.ErrCodeTakeawayMissing, patterns.ErrCodeDataWithoutSource:
				t.Errorf("review QBR slide %d draws %s: %s", s.Index, f.Code, f.Message)
			}
		}
	}
}
