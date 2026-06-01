package semantic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// findAt reports whether ds contains a diagnostic with the given code at the
// given path, and returns it.
func findAt(ds []diagnostics.Diagnostic, code, path string) (diagnostics.Diagnostic, bool) {
	for _, d := range ds {
		if d.Code == code && d.Path == path {
			return d, true
		}
	}
	return diagnostics.Diagnostic{}, false
}

func hasCode(ds []diagnostics.Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestValidateCleanFixtureHasNoErrors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "board_update.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ds := Check("board_update.yaml", data, StrictnessWarn)
	if diagnostics.HasErrors(ds) {
		t.Fatalf("clean fixture produced error diagnostics: %v", ds)
	}
}

func TestValidateMissingTitleIsRequired(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{Archetype: ArchetypeBoardUpdate},
		Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Hi"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticRequired, "meta.title")
	if !ok {
		t.Fatalf("expected SEMANTIC_REQUIRED at meta.title, got %v", ds)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("meta.title severity = %q, want error", d.Severity)
	}
}

// TestValidateMissingSlidesIsRequired is the regression guard for
// go-slide-creator-444x: a deck with a title but no slides field (or an explicit
// empty array) must fail fast with a blocking error at slides rather than
// validating clean and compiling to a zero-slide deck.
func TestValidateMissingSlidesIsRequired(t *testing.T) {
	cases := []struct {
		name string
		spec *DeckSpec
	}{
		{"nil slides", &DeckSpec{Meta: DeckMeta{Title: "Empty Deck"}}},
		{"empty slices", &DeckSpec{Meta: DeckMeta{Title: "Empty Deck"}, Slides: []SlideSpec{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := Validate(tc.spec, StrictnessWarn)
			d, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides")
			if !ok {
				t.Fatalf("expected SEMANTIC_REQUIRED at slides, got %v", ds)
			}
			if d.Severity != diagnostics.SeverityError {
				t.Errorf("slides severity = %q, want error", d.Severity)
			}
		})
	}
}

// TestCheckMissingSlidesViaParse mirrors the bug's reproduction: a YAML doc with
// only meta must surface the blocking slides finding through the full
// parse+validate path that both the CLI and MCP surfaces share.
func TestCheckMissingSlidesViaParse(t *testing.T) {
	const doc = "meta:\n  title: Empty Deck\n"
	ds := Check("no-slides.yaml", []byte(doc), StrictnessWarn)
	if !diagnostics.HasErrors(ds) {
		t.Fatalf("expected blocking errors for a deck with no slides, got %v", ds)
	}
	if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides"); !ok {
		t.Fatalf("expected SEMANTIC_REQUIRED at slides, got %v", ds)
	}
}

func TestValidateUnknownKindAndArchetype(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck", Archetype: Archetype("nope")},
		Slides: []SlideSpec{{Kind: SlideKind("bogus"), Body: map[string]any{"title": "x"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticUnknownArchetype, "meta.archetype"); !ok {
		t.Errorf("expected SEMANTIC_UNKNOWN_ARCHETYPE at meta.archetype, got %v", ds)
	}
	if _, ok := findAt(ds, diagnostics.CodeSemanticUnknownKind, "slides[0].kind"); !ok {
		t.Errorf("expected SEMANTIC_UNKNOWN_KIND at slides[0].kind, got %v", ds)
	}
}

// TestCheckBlankKindEmitsSingleMissingFinding guards against the regression
// where a present-but-blank kind ("" or whitespace) was reported twice: the
// parse pass emitted "unknown slide kind \"\"" while validateSlide separately
// emitted "missing the required \"kind\" field", two findings at the same code
// and path with contradictory messages. A blank kind must now yield exactly one
// canonical missing-kind finding.
func TestCheckBlankKindEmitsSingleMissingFinding(t *testing.T) {
	for _, kind := range []string{"", "   ", "\t"} {
		src := fmt.Sprintf(`{"meta":{"title":"X"},"slides":[{"kind":%q,"title":"x"}]}`, kind)
		ds := Check("spec.json", []byte(src), StrictnessWarn)
		var atKind []diagnostics.Diagnostic
		for _, d := range ds {
			if d.Path == "slides[0].kind" {
				atKind = append(atKind, d)
			}
		}
		if len(atKind) != 1 {
			t.Fatalf("kind=%q: expected exactly 1 finding at slides[0].kind, got %d: %v", kind, len(atKind), atKind)
		}
		d := atKind[0]
		if d.Code != diagnostics.CodeSemanticUnknownKind {
			t.Errorf("kind=%q: code = %q, want %q", kind, d.Code, diagnostics.CodeSemanticUnknownKind)
		}
		if !strings.Contains(d.Message, "missing the required") {
			t.Errorf("kind=%q: message = %q, want the canonical missing-kind message", kind, d.Message)
		}
	}
}

func TestValidateRequiredPayloadField(t *testing.T) {
	// kpi_snapshot requires "kpis".
	spec := &DeckSpec{
		Meta:   DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{"title": "Metrics"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[0].kpis"); !ok {
		t.Fatalf("expected SEMANTIC_REQUIRED at slides[0].kpis, got %v", ds)
	}
}

// TestValidateRequiredFieldAlias is the regression guard for
// go-slide-creator-i2p4: the kpi_snapshot compiler reads "metrics" as an alias
// for "kpis", so a spec using only "metrics" must validate (and compile), not be
// blocked by the required-field gate. The validator, schema, and discovery must
// agree the alias is reachable.
func TestValidateRequiredFieldAlias(t *testing.T) {
	cells := []any{
		map[string]any{"value": "42%", "label": "Win rate"},
		map[string]any{"value": "$1.2M", "label": "ARR"},
		map[string]any{"value": "12", "label": "Logos"},
	}

	t.Run("metrics alias satisfies the required-one-of gate", func(t *testing.T) {
		spec := &DeckSpec{
			Meta: DeckMeta{Title: "Deck"},
			Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{
				"title":    "Metrics",
				"metrics":  cells,
				"takeaway": "Numbers that matter.",
			}}},
		}
		ds := Validate(spec, StrictnessStrict)
		if hasErrorAt(ds, "slides[0].kpis") {
			t.Fatalf("metrics alias must not trigger a missing-kpis error, got %v", ds)
		}
	})

	t.Run("a kpi_snapshot with neither kpis nor metrics is still blocked", func(t *testing.T) {
		spec := &DeckSpec{
			Meta:   DeckMeta{Title: "Deck"},
			Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{"title": "Metrics"}}},
		}
		ds := Validate(spec, StrictnessWarn)
		if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[0].kpis"); !ok {
			t.Fatalf("expected SEMANTIC_REQUIRED at slides[0].kpis when both kpis and metrics are absent, got %v", ds)
		}
	})

	t.Run("all-blank metrics is blocked, not silently dropped", func(t *testing.T) {
		spec := &DeckSpec{
			Meta: DeckMeta{Title: "Deck"},
			Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{
				"title":   "Metrics",
				"metrics": []any{map[string]any{}, map[string]any{}},
			}}},
		}
		ds := Validate(spec, StrictnessWarn)
		if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[0].metrics"); !ok {
			t.Fatalf("expected blocking SEMANTIC_REQUIRED at slides[0].metrics for all-blank metrics, got %v", ds)
		}
	})
}

// TestValidateRawEscapeHatchStructure is the regression guard for
// go-slide-creator-8kor: a raw_json2pptx slide whose "slide" payload is an
// object but not a valid, renderable raw json2pptx slide must fail fast at
// validate (and therefore compile) rather than passing the lenient
// required-field check and compiling to an empty slide. Unknown raw fields must
// be reported instead of silently dropped.
func TestValidateRawEscapeHatchStructure(t *testing.T) {
	rawSpec := func(payload any) *DeckSpec {
		return &DeckSpec{
			Meta: DeckMeta{Title: "Raw"},
			Slides: []SlideSpec{
				{Kind: KindTitle, Body: map[string]any{"title": "Raw"}},
				{Kind: KindRawJSON2pptx, Body: map[string]any{"slide": payload}},
			},
		}
	}

	t.Run("unknown raw field is rejected, not dropped", func(t *testing.T) {
		ds := Validate(rawSpec(map[string]any{"foo": "bar"}), StrictnessWarn)
		if _, ok := findAt(ds, diagnostics.CodeSemanticUnknownField, "slides[1].slide"); !ok {
			t.Fatalf("expected SEMANTIC_UNKNOWN_FIELD at slides[1].slide for unknown raw field, got %v", ds)
		}
	})

	t.Run("missing slide_type and layout_id is blocked", func(t *testing.T) {
		ds := Validate(rawSpec(map[string]any{
			"content": []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hi"}},
		}), StrictnessWarn)
		if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[1].slide"); !ok {
			t.Fatalf("expected SEMANTIC_REQUIRED at slides[1].slide for missing slide_type/layout_id, got %v", ds)
		}
	})

	t.Run("slide_type with no renderable content is blocked", func(t *testing.T) {
		ds := Validate(rawSpec(map[string]any{"slide_type": "content"}), StrictnessWarn)
		if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[1].slide"); !ok {
			t.Fatalf("expected SEMANTIC_REQUIRED at slides[1].slide for empty content slide, got %v", ds)
		}
	})

	t.Run("non-object slide payload is blocked", func(t *testing.T) {
		ds := Validate(rawSpec("just a string"), StrictnessWarn)
		if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[1].slide"); !ok {
			t.Fatalf("expected SEMANTIC_REQUIRED at slides[1].slide for non-object payload, got %v", ds)
		}
	})

	t.Run("valid content slide passes", func(t *testing.T) {
		ds := Validate(rawSpec(map[string]any{
			"slide_type": "content",
			"content":    []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Hi"}},
		}), StrictnessWarn)
		if hasCode(ds, diagnostics.CodeSemanticUnknownField) || hasErrorAt(ds, "slides[1].slide") {
			t.Fatalf("valid raw content slide should not be flagged, got %v", ds)
		}
	})

	t.Run("blank canvas is exempt from the content requirement", func(t *testing.T) {
		ds := Validate(rawSpec(map[string]any{"slide_type": "blank", "eyebrow": "RAW"}), StrictnessWarn)
		if hasErrorAt(ds, "slides[1].slide") {
			t.Fatalf("blank raw slide should be allowed without content, got %v", ds)
		}
	})
}

// hasErrorAt reports whether ds carries an error-severity diagnostic at path.
func hasErrorAt(ds []diagnostics.Diagnostic, path string) bool {
	for _, d := range ds {
		if d.Path == path && d.Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}

func TestValidateKPIDensity(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{
			"title":    "Metrics",
			"kpis":     []any{map[string]any{"label": "A", "value": "1"}},
			"takeaway": "One number that matters.",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticDensity, "slides[0].kpis")
	if !ok {
		t.Fatalf("expected SEMANTIC_DENSITY at slides[0].kpis, got %v", ds)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("density severity = %q, want warning under warn strictness", d.Severity)
	}
}

func TestValidateChartSeriesAdvisory(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
			"title":   "Trend",
			"chart":   map[string]any{"type": "bar", "series": []any{}},
			"insight": "It goes up.",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticDensity, "slides[0].chart.series")
	if !ok {
		t.Fatalf("expected SEMANTIC_DENSITY at slides[0].chart.series, got %v", ds)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("chart series severity = %q, want warning under warn strictness", d.Severity)
	}
	// And under off it must be suppressed (advisory).
	if hasCode(Validate(spec, StrictnessOff), diagnostics.CodeSemanticDensity) {
		t.Error("advisory chart-series finding must be suppressed under off")
	}
}

// TestValidateChartInsightOverCap is the regression guard for go-slide-creator-hadk:
// a chart_insight with more than 6 insights validates cleanly today but compile
// degrades to a bullet list and drops the chart. Validation must flag the
// over-cap count (warning under warn, error under strict) so an agent can split
// or shorten before shipping a slide that loses its chart.
func TestValidateChartInsightOverCap(t *testing.T) {
	insights := func(n int) []any {
		items := make([]any, n)
		for i := range items {
			items[i] = fmt.Sprintf("insight %d", i+1)
		}
		return items
	}
	newSpec := func() *DeckSpec {
		return &DeckSpec{
			Meta: DeckMeta{Title: "Deck"},
			Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
				"title":    "Revenue",
				"takeaway": "It grew.",
				"chart": map[string]any{
					"type": "bar_chart",
					"data": map[string]any{"series": []any{map[string]any{"name": "Rev", "values": []any{1, 2}}}},
				},
				"insights": insights(7),
			}}},
		}
	}

	warnDS := Validate(newSpec(), StrictnessWarn)
	d, ok := findAt(warnDS, diagnostics.CodeSemanticDensity, "slides[0].insights")
	if !ok {
		t.Fatalf("expected SEMANTIC_DENSITY (over-cap) at slides[0].insights, got %v", warnDS)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("warn severity = %q, want warning", d.Severity)
	}

	strictDS := Validate(newSpec(), StrictnessStrict)
	d, ok = findAt(strictDS, diagnostics.CodeSemanticDensity, "slides[0].insights")
	if !ok {
		t.Fatalf("expected over-cap finding under strict, got %v", strictDS)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("strict severity = %q, want error", d.Severity)
	}

	// A chart_insight at the cap must NOT trip the over-cap advisory.
	atCap := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
			"title":    "Revenue",
			"takeaway": "It grew.",
			"insights": insights(6),
		}}},
	}
	if _, ok := findAt(Validate(atCap, StrictnessWarn), diagnostics.CodeSemanticDensity, "slides[0].insights"); ok {
		t.Errorf("chart_insight at the insight cap must not emit SEMANTIC_DENSITY at .insights")
	}
}

func TestValidateComparisonBalance(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindComparison, Body: map[string]any{
			"title":    "A vs B",
			"takeaway": "Pick A.",
			"columns": []any{
				map[string]any{"header": "A", "items": []any{"x", "y"}},
				map[string]any{"header": "B", "items": []any{"z"}},
			},
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticDensity, "slides[0].columns"); !ok {
		t.Fatalf("expected SEMANTIC_DENSITY (unbalanced) at slides[0].columns, got %v", ds)
	}
}

// TestValidateComparisonOverCap is the regression guard for go-slide-creator-wzd4:
// two balanced columns with more rows than the comparison-2col cap validate
// cleanly today, but compile degrades to a bullet list. Validation must flag the
// over-cap count (warning under warn, error under strict) so an agent can split
// or shorten before shipping a degraded slide.
func TestValidateComparisonOverCap(t *testing.T) {
	rows := func(n int) []any {
		items := make([]any, n)
		for i := range items {
			items[i] = fmt.Sprintf("row %d", i+1)
		}
		return items
	}
	newSpec := func() *DeckSpec {
		return &DeckSpec{
			Meta: DeckMeta{Title: "Deck"},
			Slides: []SlideSpec{{Kind: KindComparison, Body: map[string]any{
				"title":    "A vs B",
				"takeaway": "Pick A.",
				"columns": []any{
					map[string]any{"header": "A", "items": rows(11)},
					map[string]any{"header": "B", "items": rows(11)},
				},
			}}},
		}
	}

	warnDS := Validate(newSpec(), StrictnessWarn)
	d, ok := findAt(warnDS, diagnostics.CodeSemanticDensity, "slides[0].columns")
	if !ok {
		t.Fatalf("expected SEMANTIC_DENSITY (over-cap) at slides[0].columns, got %v", warnDS)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("warn severity = %q, want warning", d.Severity)
	}

	strictDS := Validate(newSpec(), StrictnessStrict)
	d, ok = findAt(strictDS, diagnostics.CodeSemanticDensity, "slides[0].columns")
	if !ok {
		t.Fatalf("expected over-cap finding under strict, got %v", strictDS)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("strict severity = %q, want error", d.Severity)
	}

	// A balanced comparison at the cap must NOT trip the over-cap advisory.
	atCap := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindComparison, Body: map[string]any{
			"title":    "A vs B",
			"takeaway": "Pick A.",
			"columns": []any{
				map[string]any{"header": "A", "items": rows(10)},
				map[string]any{"header": "B", "items": rows(10)},
			},
		}}},
	}
	if hasCode(Validate(atCap, StrictnessWarn), diagnostics.CodeSemanticDensity) {
		t.Errorf("balanced comparison at the row cap must not emit SEMANTIC_DENSITY")
	}
}

func TestValidateTakeawayRequiredAndStrictness(t *testing.T) {
	newSpec := func() *DeckSpec {
		return &DeckSpec{
			Meta: DeckMeta{Title: "Deck"},
			Slides: []SlideSpec{{Kind: KindProcess, Body: map[string]any{
				"title": "How it works",
				"steps": []any{"one", "two"},
			}}},
		}
	}

	warnDS := Validate(newSpec(), StrictnessWarn)
	d, ok := findAt(warnDS, diagnostics.CodeSemanticTakeawayRequired, "slides[0].takeaway")
	if !ok {
		t.Fatalf("expected SEMANTIC_TAKEAWAY_REQUIRED at slides[0].takeaway, got %v", warnDS)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("warn severity = %q, want warning", d.Severity)
	}

	strictDS := Validate(newSpec(), StrictnessStrict)
	d, ok = findAt(strictDS, diagnostics.CodeSemanticTakeawayRequired, "slides[0].takeaway")
	if !ok {
		t.Fatalf("expected takeaway finding under strict, got %v", strictDS)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("strict severity = %q, want error", d.Severity)
	}

	offDS := Validate(newSpec(), StrictnessOff)
	if hasCode(offDS, diagnostics.CodeSemanticTakeawayRequired) {
		t.Errorf("advisory takeaway finding must be suppressed under off, got %v", offDS)
	}
}

func TestValidateWeakContent(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindSection, Body: map[string]any{
			"title": "TBD",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	if _, ok := findAt(ds, diagnostics.CodeSemanticWeakContent, "slides[0].title"); !ok {
		t.Fatalf("expected SEMANTIC_WEAK_CONTENT at slides[0].title, got %v", ds)
	}
}

// TestWeakMarkerWordBoundary is the regression guard for go-slide-creator-j572:
// the short alpha markers (tbd/todo/fixme) must match only as whole tokens so
// ordinary words that merely embed them ("Mastodon" -> "todo") do not trip the
// placeholder detector, while genuine placeholders still do.
func TestWeakMarkerWordBoundary(t *testing.T) {
	clean := []string{
		"Mastodon Migration Strategy",
		"Automated Backups Roadmap",
		"subtotal",
		"fixmestreadymarket", // embedded fixme
		"tbdesign",           // embedded tbd
	}
	for _, s := range clean {
		if m := weakMarker(s); m != "" {
			t.Errorf("weakMarker(%q) = %q, want no marker", s, m)
		}
	}

	flagged := map[string]string{
		"TODO: fill this in":  "todo",
		"status is still TBD": "tbd",
		"(fixme) wording":     "fixme",
		"lorem ipsum dolor":   "lorem ipsum",
		"__fill__":            "__fill__",
		"placeholder text":    "placeholder",
	}
	for s, want := range flagged {
		if m := weakMarker(s); m != want {
			t.Errorf("weakMarker(%q) = %q, want %q", s, m, want)
		}
	}
}

// TestValidateBlankListEntriesBlock is the regression guard for go-slide-creator-qald:
// a required content-bearing list whose entries are all blank passes the raw
// presence/length gate but compiles to a title-only slide. Validation must count
// usable (post-extraction) entries and fail fast with a blocking error so the
// disagreement between validate and compile cannot ship an empty slide.
func TestValidateBlankListEntriesBlock(t *testing.T) {
	cases := []struct {
		name string
		kind SlideKind
		body map[string]any
		path string
	}{
		{
			name: "process all-blank steps",
			kind: KindProcess,
			body: map[string]any{"title": "How", "steps": []any{"", "  ", "\t"}, "takeaway": "x"},
			path: "slides[0].steps",
		},
		{
			name: "roadmap nameless phases",
			kind: KindRoadmap,
			body: map[string]any{"title": "Plan", "phases": []any{
				map[string]any{"description": "no name"}, map[string]any{"date": "Q1"},
			}, "takeaway": "x"},
			path: "slides[0].phases",
		},
		{
			name: "kpi cells without number or caption",
			kind: KindKPISnapshot,
			body: map[string]any{"title": "Metrics", "kpis": []any{
				map[string]any{"note": "n/a"}, map[string]any{},
			}, "takeaway": "x"},
			path: "slides[0].kpis",
		},
		{
			name: "comparison columns all blank",
			kind: KindComparison,
			body: map[string]any{"title": "A vs B", "columns": []any{
				map[string]any{"items": []any{""}}, map[string]any{},
			}, "takeaway": "x"},
			path: "slides[0].columns",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{{Kind: tc.kind, Body: tc.body}}}
			ds := Validate(spec, StrictnessWarn)
			d, ok := findAt(ds, diagnostics.CodeSemanticRequired, tc.path)
			if !ok {
				t.Fatalf("expected SEMANTIC_REQUIRED at %s for all-blank entries, got %v", tc.path, ds)
			}
			if d.Severity != diagnostics.SeverityError {
				t.Errorf("severity = %q, want error (content-drop must block even under warn)", d.Severity)
			}
			// Blocking even under off: this is a missing-content condition, not an
			// advisory the strictness policy may suppress.
			if !diagnostics.HasErrors(Validate(spec, StrictnessOff)) {
				t.Errorf("all-blank required content must block under off, got clean validate")
			}
		})
	}
}

// TestValidateUsableDensityMatchesCompile guards that density advisories count
// the entries compile will render, not raw list length: a process with two real
// steps padded by blanks compiles to a degraded bullet list, so validation must
// see two usable steps (below the 3–8 process-flow range), not five.
func TestValidateUsableDensityMatchesCompile(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindProcess, Body: map[string]any{
			"title":    "How it works",
			"steps":    []any{"one", "two", "", "  ", "\t"},
			"takeaway": "x",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticDensity, "slides[0].steps")
	if !ok {
		t.Fatalf("expected SEMANTIC_DENSITY at slides[0].steps for 2 usable steps, got %v", ds)
	}
	if !strings.Contains(d.Message, "2 usable steps") {
		t.Errorf("density message = %q, want it to report 2 usable steps", d.Message)
	}
	// Two real steps are still usable content, so it must not be a blocking error.
	if hasCode(ds, diagnostics.CodeSemanticRequired) {
		t.Errorf("two usable steps must not trip the zero-usable blocking error, got %v", ds)
	}
}

func TestValidateWrapsIntoEnvelope(t *testing.T) {
	spec := &DeckSpec{
		Meta:   DeckMeta{}, // missing title -> error
		Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Hi"}}},
	}
	ds := Validate(spec, StrictnessWarn)
	env := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "semantic validate"}, ds)
	if env.OK {
		t.Error("envelope OK should be false when a required field is missing")
	}
	if len(env.Findings) == 0 {
		t.Fatal("envelope should carry findings")
	}
	if env.Findings[0].Category != diagnostics.NamespaceInput {
		t.Errorf("finding category = %q, want INPUT", env.Findings[0].Category)
	}
}

// TestValidateWrongTypedScalarFieldFlagged is the regression guard for
// go-slide-creator-88g8 repro 1: a numeric title is silently dropped by the
// compiler (strField reads only strings), so without a finding the content
// vanishes behind a green validate gate. Validation must emit SEMANTIC_FIELD_TYPE
// at the field path, as a warning under warn and an error under strict.
func TestValidateWrongTypedScalarFieldFlagged(t *testing.T) {
	const doc = `{"meta":{"title":"X"},"slides":[{"kind":"executive_summary","title":12345,"points":["a","b","c"]}]}`

	warn := Check("t9.json", []byte(doc), StrictnessWarn)
	d, ok := findAt(warn, diagnostics.CodeSemanticFieldType, "slides[0].title")
	if !ok {
		t.Fatalf("expected SEMANTIC_FIELD_TYPE at slides[0].title for a numeric title, got %v", warn)
	}
	if d.Severity != diagnostics.SeverityWarning {
		t.Errorf("under warn, field-type severity = %q, want warning", d.Severity)
	}

	strict := Check("t9.json", []byte(doc), StrictnessStrict)
	d, ok = findAt(strict, diagnostics.CodeSemanticFieldType, "slides[0].title")
	if !ok {
		t.Fatalf("expected SEMANTIC_FIELD_TYPE at slides[0].title under strict, got %v", strict)
	}
	if d.Severity != diagnostics.SeverityError {
		t.Errorf("under strict, field-type severity = %q, want error", d.Severity)
	}
}

// TestValidateWrongTypedListFieldFlagged is the regression guard for
// go-slide-creator-88g8 repro 3: points supplied as a bare string (not an array)
// is silently skipped by stringList, dropping the content. Validation must emit
// SEMANTIC_FIELD_TYPE at slides[0].points naming the expected array type.
func TestValidateWrongTypedListFieldFlagged(t *testing.T) {
	const doc = `{"meta":{"title":"X"},"slides":[{"kind":"executive_summary","title":"S","points":"one point not an array","takeaway":"t"}]}`
	ds := Check("t11.json", []byte(doc), StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticFieldType, "slides[0].points")
	if !ok {
		t.Fatalf("expected SEMANTIC_FIELD_TYPE at slides[0].points for a string-valued points, got %v", ds)
	}
	if !strings.Contains(d.Message, "an array") {
		t.Errorf("field-type message = %q, want it to name the expected array type", d.Message)
	}
}

// TestValidateWrongTypedListElementsBlocksEmptySlide is the regression guard for
// go-slide-creator-88g8 repro 2: kpis given as numbers clear the raw presence and
// array-shape gates, but every element extracts to zero usable content
// (kpiCells reads only string subfields), so the slide compiles to a blank
// content:null. The usable-content gate must turn this into a blocking error
// rather than letting it pass validation as a renderable slide.
func TestValidateWrongTypedListElementsBlocksEmptySlide(t *testing.T) {
	const doc = `{"meta":{"title":"X"},"slides":[{"kind":"kpi_snapshot","kpis":[42,99,7]}]}`
	ds := Check("t10.json", []byte(doc), StrictnessWarn)
	if !diagnostics.HasErrors(ds) {
		t.Fatalf("a kpi_snapshot whose kpis extract to zero usable content must not validate clean, got %v", ds)
	}
	if _, ok := findAt(ds, diagnostics.CodeSemanticRequired, "slides[0].kpis"); !ok {
		t.Fatalf("expected blocking SEMANTIC_REQUIRED at slides[0].kpis for numeric kpis, got %v", ds)
	}
}

func TestToDiagnosticsBridgesParseFindings(t *testing.T) {
	src := "slides:\n  - kind: bogus_kind\n    title: Oops\n"
	_, parseDiags := ParseYAML([]byte(src))
	ds := parseDiags.ToDiagnostics()
	if _, ok := findAt(ds, diagnostics.CodeSemanticUnknownKind, "slides[0].kind"); !ok {
		t.Fatalf("expected bridged SEMANTIC_UNKNOWN_KIND at slides[0].kind, got %v", ds)
	}
}
