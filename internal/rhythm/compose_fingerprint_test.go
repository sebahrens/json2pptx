package rhythm

import (
	"strings"
	"testing"
)

// go-slide-creator-dzv7d. Every compose envelope was fingerprinted as the one
// family "compose", so three structurally different composed slides read as a
// three-slide run and drew break_run advice that pushed agents back to
// singleton patterns.

func composeSlide(dir string, regions ...ComposeRegion) Slide {
	return Slide{HasCompose: true, Compose: &Compose{Direction: dir, Regions: regions}}
}

func region(visual string, sizePct float64) ComposeRegion {
	return ComposeRegion{Visual: visual, SizePct: sizePct}
}

func TestComposeFingerprintSeparatesStructures(t *testing.T) {
	base := composeSlide("vertical", region("kpi-3up", 0), region("pull-quote", 0))
	different := map[string]Slide{
		"direction":       composeSlide("horizontal", region("kpi-3up", 0), region("pull-quote", 0)),
		"region family":   composeSlide("vertical", region("kpi-3up", 0), region("process-flow", 0)),
		"dominant region": composeSlide("vertical", region("kpi-3up", 70), region("pull-quote", 30)),
		"region count":    composeSlide("vertical", region("kpi-3up", 0), region("pull-quote", 0), region("chart", 0)),
		"nesting": composeSlide("vertical", region("kpi-3up", 0), ComposeRegion{Nested: &Compose{
			Direction: "horizontal", Regions: []ComposeRegion{region("pull-quote", 0), region("chart", 0)},
		}}),
		"chart vs diagram": composeSlide("vertical", region("kpi-3up", 0), region("chart", 0)),
	}
	baseKey := FingerprintKey(base)
	if !strings.HasPrefix(fingerprint(0, base).Pattern, "compose:") {
		t.Errorf("composed fingerprint %q should still identify the surface as compose", fingerprint(0, base).Pattern)
	}
	seen := map[string]string{baseKey: "base"}
	for name, s := range different {
		key := FingerprintKey(s)
		if prev, dup := seen[key]; dup {
			t.Errorf("%s shares fingerprint %q with %s", name, key, prev)
		}
		seen[key] = name
	}
}

func TestComposeFingerprintIgnoresTrivialDifferences(t *testing.T) {
	base := composeSlide("vertical", region("kpi-3up", 0), region("pull-quote", 0))
	same := map[string]Slide{
		"reordered regions":     composeSlide("vertical", region("pull-quote", 0), region("kpi-3up", 0)),
		"55/45 share":           composeSlide("vertical", region("kpi-3up", 55), region("pull-quote", 45)),
		"same family variant":   composeSlide("vertical", region("kpi-4up", 0), region("pull-quote", 0)),
		"implicit remainder":    composeSlide("vertical", region("kpi-3up", 52), region("pull-quote", 0)),
		"unnormalised explicit": composeSlide("vertical", region("kpi-3up", 30), region("pull-quote", 30)),
	}
	want := FingerprintKey(base)
	for name, s := range same {
		if got := FingerprintKey(s); got != want {
			t.Errorf("%s: fingerprint %q, want %q (trivial change defeated monotony detection)", name, got, want)
		}
	}
	// The dominant region keeps its identity when the order is swapped.
	a := composeSlide("horizontal", region("chart", 65), region("labeled-rows", 35))
	b := composeSlide("horizontal", region("labeled-rows", 30), region("chart", 70))
	if FingerprintKey(a) != FingerprintKey(b) {
		t.Errorf("same dominant chart split reads differently: %q vs %q", FingerprintKey(a), FingerprintKey(b))
	}
}

func TestComposeRunsOnlyForEquivalentStructures(t *testing.T) {
	diverse := []Slide{
		composeSlide("vertical", region("stylish-panels", 0), region("pull-quote", 0)),
		composeSlide("horizontal", region("kpi-3up", 0), region("process-flow", 0)),
		composeSlide("vertical", region("pull-quote", 0), region("kpi-3up", 0)),
	}
	r := Analyze(diverse)
	if len(r.Aggregates.PatternRuns) != 0 || r.Aggregates.LongestRun != 0 {
		t.Errorf("distinct compositions formed a run: %+v", r.Aggregates.PatternRuns)
	}
	for _, rec := range r.Recommendations {
		if rec.Code == CodeBreakRun {
			t.Errorf("phantom break_run on distinct compositions: %+v", rec)
		}
	}

	repeated := []Slide{
		composeSlide("vertical", region("kpi-3up", 0), region("pull-quote", 0)),
		composeSlide("vertical", region("pull-quote", 45), region("kpi-4up", 55)),
		composeSlide("vertical", region("kpi-5up", 0), region("pull-quote", 0)),
	}
	r = Analyze(repeated)
	if r.Aggregates.LongestRun != 3 {
		t.Fatalf("repeated equivalent compositions: longest_run = %d, want 3 (runs %+v)", r.Aggregates.LongestRun, r.Aggregates.PatternRuns)
	}
	var rec *Recommendation
	for i := range r.Recommendations {
		if r.Recommendations[i].Code == CodeBreakRun {
			rec = &r.Recommendations[i]
		}
	}
	if rec == nil {
		t.Fatalf("no break_run for three equivalent compositions: %+v", r.Recommendations)
	}
	if !strings.Contains(rec.Message, "compose:v[") {
		t.Errorf("break_run message should name the composed structure: %q", rec.Message)
	}
	if len(rec.RecommendedBreak) == 0 {
		t.Fatal("break_run carries no alternatives")
	}
	for _, name := range rec.RecommendedBreak {
		if fam := visualFamily(name); fam == "kpi" || fam == "pull-quote" {
			t.Errorf("break suggestion %q repeats a region already on every slide of the run", name)
		}
	}
}

func TestComposeWithoutStructureKeepsLegacyFamily(t *testing.T) {
	if got := fingerprint(0, Slide{HasCompose: true}).Pattern; got != "compose" {
		t.Errorf("compose slide without a projected structure = %q, want compose", got)
	}
}
