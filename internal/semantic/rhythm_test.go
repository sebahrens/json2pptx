package semantic

import (
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

func TestRhythmSectioningFlagsLongUnbrokenDeck(t *testing.T) {
	slides := make([]SlideSpec, 0, 9)
	for i := 0; i < 9; i++ {
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
		t.Fatalf("expected sectioning warning for a 9-slide deck with no sections, got %+v", ws)
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
