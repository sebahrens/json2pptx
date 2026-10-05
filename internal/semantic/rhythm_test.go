package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// rhythmCodes returns the set of rhythm warning codes present in the warnings.
func rhythmCodes(ws []RhythmWarning) map[string]bool {
	out := map[string]bool{}
	for _, w := range ws {
		out[w.Code] = true
	}
	return out
}

// kpiSlide builds a kpi_snapshot spec slide with n KPIs so it normalizes to the
// FamilyKPI / DensityMedium plan.
func kpiSlide(n int) SlideSpec {
	kpis := make([]any, 0, n)
	for i := 0; i < n; i++ {
		kpis = append(kpis, map[string]any{"label": "L", "value": "1"})
	}
	return SlideSpec{Kind: KindKPISnapshot, Body: map[string]any{"title": "Metrics", "kpis": kpis, "takeaway": "x"}}
}

func TestRhythmMonotonyFlagsLongRun(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{kpiSlide(3), kpiSlide(3), kpiSlide(3)},
	}
	ws := Normalize(spec).RhythmWarnings()
	if !rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmMonotony)] {
		t.Fatalf("expected monotony warning for 3 consecutive KPI slides, got %+v", ws)
	}
	// The warning anchors at the run's first slide.
	for _, w := range ws {
		if w.Code == string(diagnostics.CodeSemanticRhythmMonotony) && w.Path != "slides[0]" {
			t.Errorf("monotony path = %q, want slides[0]", w.Path)
		}
	}
}

func TestRhythmMonotonyAllowsTwoInARow(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{kpiSlide(3), kpiSlide(3)},
	}
	if ws := Normalize(spec).RhythmWarnings(); rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmMonotony)] {
		t.Errorf("two consecutive KPI slides must not trip monotony, got %+v", ws)
	}
}

// khzni: appendix back matter is exempt from the run checks, in both the flat
// (an appendix divider) and the structure (sections[].appendix) forms.
func TestRhythmMonotonySkipsAppendix(t *testing.T) {
	monotony := string(diagnostics.CodeSemanticRhythmMonotony)
	divider := func(title string, appendix bool) SlideSpec {
		body := map[string]any{"title": title}
		if appendix {
			body["appendix"] = true
		}
		return SlideSpec{Kind: KindSection, Body: body}
	}
	cases := []struct {
		name string
		spec *DeckSpec
		want bool
	}{
		{"flat appendix: true", &DeckSpec{Slides: []SlideSpec{divider("Detailed data", true), kpiSlide(3), kpiSlide(3), kpiSlide(3)}}, false},
		{"flat Appendix title", &DeckSpec{Slides: []SlideSpec{divider("Appendix", false), kpiSlide(3), kpiSlide(3), kpiSlide(3)}}, false},
		{"flat ordinary chapter", &DeckSpec{Slides: []SlideSpec{divider("Market", false), kpiSlide(3), kpiSlide(3), kpiSlide(3)}}, true},
		{"structure appendix section", &DeckSpec{Structure: &DeckStructure{Sections: []DeckSection{
			{Title: "Body", Slides: []SlideSpec{kpiSlide(3)}},
			{Title: "Supporting data", Appendix: true, Slides: []SlideSpec{kpiSlide(3), kpiSlide(3), kpiSlide(3)}},
		}}}, false},
		{"structure chapter section", &DeckSpec{Structure: &DeckStructure{Sections: []DeckSection{
			{Title: "Body", Slides: []SlideSpec{kpiSlide(3)}},
			{Title: "Supporting data", Slides: []SlideSpec{kpiSlide(3), kpiSlide(3), kpiSlide(3)}},
		}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.spec.Meta = DeckMeta{Title: "Deck"}
			ir := Normalize(tc.spec)
			if got := rhythmCodes(ir.RhythmWarnings())[monotony]; got != tc.want {
				t.Errorf("monotony = %v, want %v", got, tc.want)
			}
			if !tc.want && !ir.Slides[len(ir.Slides)-1].Appendix {
				t.Errorf("last backup slide not marked appendix: %+v", ir.Slides[len(ir.Slides)-1])
			}
		})
	}
}

func TestRhythmMonotonyAllowsStructuredOpening(t *testing.T) {
	cover := SlideSpec{Kind: KindTitle, Body: map[string]any{"title": "Deck"}}
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Structure: &DeckStructure{Cover: &cover, AutoAgenda: true, Sections: []DeckSection{
			{Title: "One", Slides: []SlideSpec{kpiSlide(3)}},
			{Title: "Two", Slides: []SlideSpec{kpiSlide(3)}},
		}},
	}
	if ws := Normalize(spec).RhythmWarnings(); rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmMonotony)] {
		t.Errorf("cover, agenda, and first divider are intentional structure, got %+v", ws)
	}
}

// bodySlides alternates two kinds so monotony does not fire; none is a section.
func bodySlides(n int) []SlideSpec {
	slides := make([]SlideSpec, 0, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			slides = append(slides, kpiSlide(3))
		} else {
			slides = append(slides, SlideSpec{Kind: KindExecutiveSummary, Body: map[string]any{
				"title": "Summary", "points": []any{"a", "b", "c"}, "takeaway": "t",
			}})
		}
	}
	return slides
}

// go-slide-creator-th6o9 (f-A8): two tables, the team grid and the next-steps
// closer were "4 consecutive text slides". A table, a card grid and numbered
// action rows are three looks, none of them prose.
func TestRhythmTableCardsAndNextStepsAreNotText(t *testing.T) {
	monotony := string(diagnostics.CodeSemanticRhythmMonotony)
	table := func(title string) SlideSpec {
		return SlideSpec{Kind: KindTable, Body: map[string]any{"title": title}}
	}
	tail := []SlideSpec{
		table("Fees by workstream"), table("Fees by phase"),
		{Kind: KindTeam, Body: map[string]any{"title": "The team"}},
		{Kind: KindNextSteps, Body: map[string]any{"title": "Next steps"}},
	}
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: append([]SlideSpec{kpiSlide(3)}, tail...)}
	ir := Normalize(spec)
	if ws := ir.RhythmWarnings(); rhythmCodes(ws)[monotony] {
		t.Errorf("two tables, a team grid and next steps are not one run, got %+v", ws)
	}
	families := map[VisualFamily]bool{}
	for _, s := range ir.Slides[1:] {
		if s.Visual.Family == FamilyText {
			t.Errorf("%s still counts as text", s.Kind)
		}
		families[s.Visual.Family] = true
	}
	if len(families) != 3 {
		t.Errorf("table, team and next_steps should be three families, got %v", families)
	}

	// Three tables in a row are still a run, and the message says of what.
	spec.Slides = []SlideSpec{kpiSlide(3), table("A"), table("B"), table("C")}
	found := false
	for _, w := range Normalize(spec).RhythmWarnings() {
		if w.Code == monotony {
			found = strings.Contains(w.Message, "3 consecutive table slides")
		}
	}
	if !found {
		t.Errorf("three tables in a row should read as 3 consecutive table slides, got %+v", Normalize(spec).RhythmWarnings())
	}
}

// go-slide-creator-th6o9 (h-A12, f-A8): a 12-slide proposal or steering case
// whose brief dictated its slides was told on every call to add section
// dividers it has no room for. The advice starts at 13 body slides.
func TestRhythmSectioningLeavesTwelveSlideDecksAlone(t *testing.T) {
	sectioning := string(diagnostics.CodeSemanticRhythmSectioning)
	deck := func(body int) *DeckSpec {
		slides := append([]SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Deck"}}}, bodySlides(body)...)
		return &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: slides}
	}
	for body := 9; body <= 12; body++ {
		if ws := Normalize(deck(body)).RhythmWarnings(); rhythmCodes(ws)[sectioning] {
			t.Errorf("a deck with %d body slides is one chapter, got %+v", body, ws)
		}
	}
	if ws := Normalize(deck(13)).RhythmWarnings(); !rhythmCodes(ws)[sectioning] {
		t.Errorf("a deck with 13 body slides and no dividers should be asked for chapters, got %+v", ws)
	}
}

// An explicit meta.chrome.tracker: false declines chapters at any length; a
// chrome block that says nothing about the tracker does not.
func TestRhythmSectioningDeclinedByTrackerFalse(t *testing.T) {
	sectioning := string(diagnostics.CodeSemanticRhythmSectioning)
	doc := func(chrome string) []byte {
		var b strings.Builder
		b.WriteString(`{"meta": {"title": "Deck", "chrome": ` + chrome + `}, "slides": [`)
		for i := 0; i < 15; i++ {
			if i > 0 {
				b.WriteString(",")
			}
			if i%2 == 0 {
				b.WriteString(`{"kind": "kpi_snapshot", "title": "Metrics", "takeaway": "x", "kpis": [{"label": "L", "value": "1"}, {"label": "M", "value": "2"}]}`)
			} else {
				b.WriteString(`{"kind": "executive_summary", "title": "Summary", "points": ["a", "b", "c"], "takeaway": "t"}`)
			}
		}
		b.WriteString("]}")
		return []byte(b.String())
	}
	for chrome, want := range map[string]bool{
		`{"client_name": "Atlas", "tracker": false}`: false,
		`{"client_name": "Atlas"}`:                   true,
	} {
		spec, ds := ParseJSON(doc(chrome))
		if ds.HasErrors() {
			t.Fatalf("parse %s: %+v", chrome, ds)
		}
		if got := rhythmCodes(Normalize(spec).RhythmWarnings())[sectioning]; got != want {
			t.Errorf("chrome %s: sectioning advice = %v, want %v", chrome, got, want)
		}
	}
	yamlSpec, ds := ParseYAML([]byte(strings.ReplaceAll(string(doc(`{"tracker": false}`)), "\n", "")))
	if ds.HasErrors() {
		t.Fatalf("parse yaml: %+v", ds)
	}
	if rhythmCodes(Normalize(yamlSpec).RhythmWarnings())[sectioning] {
		t.Error("tracker: false in a YAML spec should decline the sectioning advice too")
	}
}

// go-slide-creator-th6o9 (i-A12): the density advice named neither the run
// nor a place to break it.
func TestRhythmDensityNamesTheRunAndWhereToBreakIt(t *testing.T) {
	heavy := func(title string) SlideSpec {
		return SlideSpec{Kind: KindOptionMatrix, Body: map[string]any{"title": title}}
	}
	framework := func(title string) SlideSpec {
		return SlideSpec{Kind: KindFramework, Body: map[string]any{"title": title}}
	}
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{
		kpiSlide(3), heavy("Board"), {Kind: KindTable, Body: map[string]any{"title": "Register"}}, framework("Trend"), heavy("Options"), kpiSlide(3),
	}}
	ir := Normalize(spec)
	var got *RhythmWarning
	for _, w := range ir.RhythmWarnings() {
		if w.Code == string(diagnostics.CodeSemanticRhythmDensity) {
			w := w
			got = &w
		}
	}
	if got == nil {
		t.Fatalf("expected a density warning for four dense slides in a row, got %+v", ir.RhythmWarnings())
	}
	if got.Path != "slides[1]" || len(got.Run) != 4 || got.Run[0] != "slides[1]" || got.Run[3] != "slides[4]" {
		t.Errorf("run = %v at %q, want slides[1]..slides[4]", got.Run, got.Path)
	}
	if got.InsertBefore != "slides[3]" {
		t.Errorf("insert_before = %q, want slides[3]: the first place the run exceeds two", got.InsertBefore)
	}
	for _, want := range []string{"4 consecutive dense slides", "slides[1] to slides[4]", "before slides[3]"} {
		if !strings.Contains(got.Message, want) {
			t.Errorf("message %q should contain %q", got.Message, want)
		}
	}
	for _, d := range rhythmDiagnostics(ir, StrictnessWarn) {
		if d.Code == string(diagnostics.CodeSemanticRhythmDensity) {
			if d.Details["insert_before"] != "slides[3]" || d.Details["run"] == nil {
				t.Errorf("diagnostic details = %+v, want run and insert_before", d.Details)
			}
		}
	}
}

func TestRhythmSectioningFlagsLongUnbrokenDeck(t *testing.T) {
	slides := make([]SlideSpec, 0, 13)
	for i := 0; i < 13; i++ {
		// Alternate kinds so monotony does not also fire; none are sections.
		if i%2 == 0 {
			slides = append(slides, kpiSlide(3))
		} else {
			slides = append(slides, SlideSpec{Kind: KindExecutiveSummary, Body: map[string]any{
				"title": "Summary", "points": []any{"a", "b", "c"}, "takeaway": "t",
			}})
		}
	}
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: slides}
	ws := Normalize(spec).RhythmWarnings()
	if !rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmSectioning)] {
		t.Fatalf("expected sectioning warning for a 13-body-slide deck with no sections, got %+v", ws)
	}
}

// A deck plan_deck builds at its default budget — cover, ten body slides,
// closing, and no dividers — is one chapter (go-slide-creator-n83ml).
func TestRhythmSectioningIgnoresChromeAtDefaultBudget(t *testing.T) {
	slides := []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Deck"}}}
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			slides = append(slides, kpiSlide(3))
		} else {
			slides = append(slides, SlideSpec{Kind: KindExecutiveSummary, Body: map[string]any{
				"title": "Summary", "points": []any{"a", "b", "c"}, "takeaway": "t",
			}})
		}
	}
	slides = append(slides, SlideSpec{Kind: KindClosing, Body: map[string]any{"title": "Thanks"}})
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: slides}
	if ws := Normalize(spec).RhythmWarnings(); rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmSectioning)] {
		t.Errorf("a 12-slide deck with 10 body slides must not trip sectioning, got %+v", ws)
	}
}

func TestRhythmSectioningSatisfiedBySectionSlide(t *testing.T) {
	slides := make([]SlideSpec, 0, 9)
	slides = append(slides, SlideSpec{Kind: KindSection, Body: map[string]any{"title": "Part One"}})
	for i := 0; i < 8; i++ {
		slides = append(slides, SlideSpec{Kind: KindExecutiveSummary, Body: map[string]any{
			"title": "Summary", "points": []any{"a", "b", "c"}, "takeaway": "t",
		}})
	}
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: slides}
	if ws := Normalize(spec).RhythmWarnings(); rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmSectioning)] {
		t.Errorf("a deck with a section slide must not trip sectioning, got %+v", ws)
	}
}

func TestRhythmSynthesisFlagsExecutiveDeckWithoutSynthesis(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Board", Archetype: ArchetypeBoardUpdate},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Board"}},
			kpiSlide(3),
			{Kind: KindClosing, Body: map[string]any{"title": "Questions?"}},
		},
	}
	ws := Normalize(spec).RhythmWarnings()
	if !rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmSynthesis)] {
		t.Fatalf("executive deck without summary/decision should warn, got %+v", ws)
	}
}

func TestRhythmSynthesisSatisfiedByExecutiveSummary(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Board", Archetype: ArchetypeBoardUpdate},
		Slides: []SlideSpec{
			{Kind: KindExecutiveSummary, Body: map[string]any{"title": "Summary", "points": []any{"a", "b", "c"}, "takeaway": "t"}},
			kpiSlide(3),
		},
	}
	if ws := Normalize(spec).RhythmWarnings(); rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmSynthesis)] {
		t.Errorf("executive deck with a summary must not trip synthesis, got %+v", ws)
	}
}

func TestRhythmSynthesisSkippedForNonExecutiveArchetype(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Pitch", Archetype: ArchetypeSalesPitch},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Pitch"}},
			kpiSlide(3),
		},
	}
	if ws := Normalize(spec).RhythmWarnings(); rhythmCodes(ws)[string(diagnostics.CodeSemanticRhythmSynthesis)] {
		t.Errorf("non-executive archetype must not trip synthesis, got %+v", ws)
	}
}

func TestRhythmDiagnosticsStrictness(t *testing.T) {
	ir := Normalize(&DeckSpec{
		Meta:   DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{kpiSlide(3), kpiSlide(3), kpiSlide(3)},
	})

	if ds := rhythmDiagnostics(ir, StrictnessOff); len(ds) != 0 {
		t.Errorf("rhythm diagnostics must be suppressed under off, got %+v", ds)
	}

	warnDS := rhythmDiagnostics(ir, StrictnessWarn)
	if len(warnDS) == 0 {
		t.Fatal("expected rhythm diagnostics under warn")
	}
	for _, d := range warnDS {
		if d.Severity != diagnostics.SeverityWarning {
			t.Errorf("warn severity = %q, want warning", d.Severity)
		}
	}

	for _, d := range rhythmDiagnostics(ir, StrictnessStrict) {
		if d.Severity != diagnostics.SeverityError {
			t.Errorf("strict severity = %q, want error", d.Severity)
		}
	}
}

func TestRhythmWarningsEmptyDeck(t *testing.T) {
	if ws := Normalize(nil).RhythmWarnings(); len(ws) != 0 {
		t.Errorf("empty deck should have no rhythm warnings, got %+v", ws)
	}
}

func TestMarketAnalysisWarnsWithoutEvidenceVisual(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Market", Archetype: ArchetypeMarketAnalysis},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Market"}},
			{Kind: KindSection, Body: map[string]any{"title": "Context"}},
			{Kind: KindExecutiveSummary, Body: map[string]any{"title": "Summary", "points": []any{"a", "b", "c"}}},
			{Kind: KindComparison, Body: map[string]any{"title": "Choices", "columns": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}}},
			{Kind: KindExecutiveSummary, Body: map[string]any{"title": "Signals", "points": []any{"a", "b", "c"}}},
			{Kind: KindComparison, Body: map[string]any{"title": "Response", "columns": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}}},
			{Kind: KindClosing, Body: map[string]any{"title": "Close"}},
		},
	}
	ir := Normalize(spec)
	codes := rhythmCodes(ir.RhythmWarnings())
	if !codes[string(diagnostics.CodeSemanticEvidenceVisualMissing)] {
		t.Fatalf("expected evidence warning, got %+v", ir.RhythmWarnings())
	}
	if !codes[string(diagnostics.CodeSemanticVisualFamilyNarrow)] {
		t.Fatalf("expected family-breadth warning, got %+v", ir.RhythmWarnings())
	}
	if ir.Rhythm.DistinctFamilyCount != 2 || ir.Rhythm.EvidenceFamilyCount != 0 {
		t.Errorf("rhythm counts = %+v, want distinct=2 evidence=0", ir.Rhythm)
	}
}

func TestMarketAnalysisEvidenceAndBreadthClearWarnings(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Market", Archetype: ArchetypeMarketAnalysis},
		Slides: []SlideSpec{
			{Kind: KindTitle, Body: map[string]any{"title": "Market"}},
			{Kind: KindExecutiveSummary, Body: map[string]any{"title": "Summary", "points": []any{"a", "b", "c"}}},
			{Kind: KindComparison, Body: map[string]any{"title": "Choices", "columns": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}}},
			kpiSlide(3),
			{Kind: KindProcess, Body: map[string]any{"title": "Method", "steps": []any{"a", "b", "c"}}},
			{Kind: KindTimeline, Body: map[string]any{"title": "History", "milestones": []any{"a", "b", "c"}}},
			{Kind: KindClosing, Body: map[string]any{"title": "Close"}},
		},
	}
	ir := Normalize(spec)
	codes := rhythmCodes(ir.RhythmWarnings())
	if codes[string(diagnostics.CodeSemanticEvidenceVisualMissing)] || codes[string(diagnostics.CodeSemanticVisualFamilyNarrow)] {
		t.Fatalf("varied evidence deck should pass breadth checks, got %+v", ir.RhythmWarnings())
	}
	if ir.Rhythm.DistinctFamilyCount < 3 || ir.Rhythm.EvidenceFamilyCount != 1 {
		t.Errorf("rhythm counts = %+v", ir.Rhythm)
	}
	explained := ir.Explain()
	if explained.Rhythm.DistinctFamilyCount != ir.Rhythm.DistinctFamilyCount || explained.Rhythm.EvidenceFamilyCount != 1 {
		t.Errorf("explain dropped family counts: %+v", explained.Rhythm)
	}
}

func TestEvidenceWarningsAvoidShortAndNonEvidenceDecks(t *testing.T) {
	short := Normalize(&DeckSpec{Meta: DeckMeta{Title: "Status"}, Slides: []SlideSpec{
		{Kind: KindTitle, Body: map[string]any{"title": "Status"}},
		{Kind: KindExecutiveSummary, Body: map[string]any{"title": "Update", "points": []any{"a", "b", "c"}}},
		{Kind: KindClosing, Body: map[string]any{"title": "Close"}},
	}})
	if codes := rhythmCodes(short.RhythmWarnings()); codes[string(diagnostics.CodeSemanticEvidenceVisualMissing)] || codes[string(diagnostics.CodeSemanticVisualFamilyNarrow)] {
		t.Fatalf("short status note false-positive: %+v", short.RhythmWarnings())
	}

	strategy := Normalize(&DeckSpec{Meta: DeckMeta{Title: "Strategy", Archetype: ArchetypeStrategyProposal}, Slides: []SlideSpec{
		{Kind: KindTitle, Body: map[string]any{"title": "Strategy"}},
		{Kind: KindExecutiveSummary, Body: map[string]any{"title": "Case", "points": []any{"a", "b", "c"}}},
		{Kind: KindComparison, Body: map[string]any{"title": "Options", "columns": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}}},
		{Kind: KindProcess, Body: map[string]any{"title": "Operating model", "steps": []any{"a", "b", "c"}}},
		{Kind: KindRoadmap, Body: map[string]any{"title": "Plan", "phases": []any{"a", "b", "c"}}},
		{Kind: KindDecision, Body: map[string]any{"title": "Decision", "recommendation": "Proceed"}},
		{Kind: KindClosing, Body: map[string]any{"title": "Close"}},
	}})
	if codes := rhythmCodes(strategy.RhythmWarnings()); codes[string(diagnostics.CodeSemanticEvidenceVisualMissing)] || codes[string(diagnostics.CodeSemanticVisualFamilyNarrow)] {
		t.Fatalf("varied strategy proposal false-positive: %+v", strategy.RhythmWarnings())
	}
}

func TestDefaultsForArchetypes(t *testing.T) {
	if d := DefaultsFor(ArchetypeBoardUpdate); d.Template != "midnight-blue" || !d.Executive {
		t.Errorf("board_update defaults = %+v, want midnight-blue/executive", d)
	}
	if d := DefaultsFor(ArchetypeSalesPitch); d.Template != "warm-coral" || d.Executive {
		t.Errorf("sales_pitch defaults = %+v, want warm-coral/non-executive", d)
	}
	// Every registered archetype must carry a template default and be deterministic.
	for _, a := range AllArchetypes() {
		if DefaultsFor(a).Template == "" {
			t.Errorf("archetype %q has no default template", a)
		}
	}
	if d := DefaultsFor(Archetype("unknown")); d.Template != "" || d.Executive {
		t.Errorf("unknown archetype defaults = %+v, want zero value", d)
	}
}

func TestArchetypeTemplateFillsWhenUnpinned(t *testing.T) {
	// No template pinned + an archetype with a default → explanation shows it.
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Pitch", Archetype: ArchetypeSalesPitch},
		Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Pitch"}}},
	}
	if got := ExplainSpec(spec).Template; got != "warm-coral" {
		t.Errorf("explained template = %q, want warm-coral (archetype default)", got)
	}

	// A pinned template wins over the archetype default.
	spec.Meta.Template = "forest-green"
	if got := ExplainSpec(spec).Template; got != "forest-green" {
		t.Errorf("explained template = %q, want forest-green (spec pin wins)", got)
	}
}

func TestCompileTemplatePrecedence(t *testing.T) {
	base := func() *DeckSpec {
		return &DeckSpec{
			Meta:   DeckMeta{Title: "Pitch", Archetype: ArchetypeSalesPitch},
			Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Pitch"}}},
		}
	}

	// Archetype default applies when neither spec nor caller pins a template.
	in, _, err := Compile(base(), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if in.Template != "warm-coral" {
		t.Errorf("Template = %q, want warm-coral (archetype default)", in.Template)
	}

	// The caller default beats the archetype default.
	in, _, err = Compile(base(), CompileOptions{DefaultTemplate: "midnight-blue"})
	if err != nil {
		t.Fatal(err)
	}
	if in.Template != "midnight-blue" {
		t.Errorf("Template = %q, want midnight-blue (caller default beats archetype)", in.Template)
	}

	// The spec pin beats everything.
	spec := base()
	spec.Meta.Template = "forest-green"
	in, _, err = Compile(spec, CompileOptions{DefaultTemplate: "midnight-blue"})
	if err != nil {
		t.Fatal(err)
	}
	if in.Template != "forest-green" {
		t.Errorf("Template = %q, want forest-green (spec pin wins)", in.Template)
	}
}
