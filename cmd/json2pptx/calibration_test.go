package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// The calibration corpus (go-slide-creator-q7ar).
//
// testdata/calibration holds 16 decks — 3 built to be good, 3 mediocre, 10 with
// one dominant defect each — and blind_grades.json, the 0-100 grades a human
// gave their RENDERS before seeing any tool output. It is the regression fixture
// the bead asked for: score changes are measured against ranked decks rather
// than against unit tests of individual codes, so a change that improves one
// finding while flattening the ranking is visible here.
//
// Baseline before the work: Spearman +0.39 against the human ranking and 10 of
// 13 defect decks passing the quality gate — including a deck whose every slide
// reads "Lorem ipsum" / "Click to add title" / "XX%".

const (
	// calibrationMinSpearman is the floor for rank agreement with the blind
	// human grades. The measured value is ~0.75; a tighter score model reached
	// 0.83 but cost six bundled example decks as false gate failures, and not
	// blocking good decks matters more than the last 0.08 of correlation.
	calibrationMinSpearman = 0.70
	// calibrationSpecSuffix marks decks authored in the older DeckSpec shape.
	// score_deck cannot grade a DeckSpec — it reports that the slides carry no
	// content it recognises — so they are excluded from the gate assertions.
	calibrationSpecMarker = "_spec_"
)

// staticPathMisses are the decks this test's STATIC pass (collectFitFindings)
// does not catch, with the signal that does. score_deck itself adds render-time
// findings and, when configured, a vision pass; the test measures the static
// path because that is the layer the scoring model lives in.
var staticPathMisses = map[string]string{
	"B05_low_contrast_grid": "its defect is contrast the engine auto-fixes, so the static path sees an advisory; the render is what the human graded at 20, and vision is the signal for it",
	"B06_table_9_columns":   "static sees density_exceeded at review; the render pass truncates the table and raises a P0, so full score_deck fails it (verified: 1 P0, gate failed)",
	"B07_wrong_pattern":     "its defect is semantic — KPIs drawn as a timeline, a timeline drawn as a 2x2 — and every slide is structurally sound, so no geometric check sees it. It used to fail the gate on pattern_overcrowded, but that finding fired on every conforming timeline and 2x2 alike (go-slide-creator-wrsb); catching one bad deck by accident is not a signal. go-slide-creator-h339i is the real check",
}

type calibrationResult struct {
	name  string
	grade int
	score int
	gate  bool
	spec  bool
}

// TestCalibrationRanking is the acceptance test for the deck-quality signal: the
// gate must fail defective decks and pass good ones, and the score must track
// the human ranking rather than the count of codes that happen to exist.
func TestCalibrationRanking(t *testing.T) {
	results := scoreCalibrationCorpus(t)
	if len(results) < 16 {
		t.Fatalf("calibration corpus has %d decks, want 16", len(results))
	}

	grades := make([]float64, 0, len(results))
	scores := make([]float64, 0, len(results))
	for _, r := range results {
		grades = append(grades, float64(r.grade))
		scores = append(scores, float64(r.score))
	}
	rho := spearman(grades, scores)
	t.Logf("Spearman(score, blind human grade) = %.3f over %d decks", rho, len(results))
	if rho < calibrationMinSpearman {
		t.Errorf("score no longer tracks deck quality: Spearman %.3f < %.2f\n%s", rho, calibrationMinSpearman, formatCalibration(results))
	}

	for _, r := range results {
		if r.spec {
			continue // not gradable through the presentation path
		}
		good := strings.HasPrefix(r.name, "G")
		switch {
		case good && !r.gate:
			t.Errorf("%s is a good deck (human %d) but the gate failed it — false blocks are worse than misses", r.name, r.grade)
		case !good && r.gate:
			if why, known := staticPathMisses[r.name]; known {
				t.Logf("known static-path miss: %s (human %d, score %d) — %s", r.name, r.grade, r.score, why)
				continue
			}
			t.Errorf("%s is a defect deck (human %d) but the gate passed it at score %d", r.name, r.grade, r.score)
		}
	}
	t.Log("\n" + formatCalibration(results))
}

// TestCalibrationDefectClassesAreDetected pins the specific defect each deck was
// built to carry, so a regression shows up as "we stopped seeing lorem ipsum"
// rather than as a small score drift.
func TestCalibrationDefectClassesAreDetected(t *testing.T) {
	want := map[string]string{
		"B03_near_empty":          patterns.ErrCodeSlideNearlyEmpty,
		"B04_chart_14_categories": patterns.ErrCodeChartOverloaded,
		"B08_lorem_placeholder":   patterns.ErrCodeWeakContent,
		"B09_same_layout_x8":      patterns.ErrCodeDeckMonotony,
		"B10_titleless":           patterns.ErrCodeMissingTitle,
	}
	for name, code := range want {
		t.Run(name, func(t *testing.T) {
			input, layouts, w, h := loadCalibrationDeckWithTemplate(t, name)
			findings := collectFitFindings(input, layouts, w, h, nil)
			for _, f := range findings {
				if f.Code == code {
					return
				}
			}
			codes := map[string]int{}
			for _, f := range findings {
				codes[f.Code]++
			}
			t.Errorf("%s no longer emits %s; got %v", name, code, codes)
		})
	}
}

func scoreCalibrationCorpus(t *testing.T) []calibrationResult {
	t.Helper()
	gradeData, err := os.ReadFile(filepath.Join("testdata", "calibration", "blind_grades.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(gradeData, &raw); err != nil {
		t.Fatal(err)
	}

	var out []calibrationResult
	for name, v := range raw {
		grade, ok := v.(float64)
		if !ok {
			continue // the "_note" field
		}
		input, layouts, w, h := loadCalibrationDeckWithTemplate(t, name)
		findings := collectFitFindings(input, layouts, w, h, nil)
		ds := deterministic.ScoreFromFindings(findings, len(input.Slides))
		gate := deterministic.EvaluateQualityGate(ds, findings, deterministic.DefaultQualityGateCriteria())
		out = append(out, calibrationResult{
			name:  name,
			grade: int(grade),
			score: ds.OverallScore,
			gate:  gate.Passed,
			spec:  strings.Contains(name, calibrationSpecMarker),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].grade > out[j].grade })
	return out
}

// loadCalibrationDeckWithTemplate loads a calibration deck and resolves its
// template geometry, so the detectors measure against the bounds the deck will
// actually render into (table fit in particular needs it).
func loadCalibrationDeckWithTemplate(t *testing.T, name string) (*PresentationInput, []types.LayoutMetadata, int64, int64) {
	t.Helper()
	input := loadCalibrationDeck(t, name)
	if input.Template == "" {
		return input, nil, 12192000, 6858000
	}
	reader, err := template.OpenTemplate(filepath.Join("..", "..", "templates", input.Template+".pptx"))
	if err != nil {
		return input, nil, 12192000, 6858000
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return input, nil, 12192000, 6858000
	}
	w, h := template.ParseSlideDimensions(reader)
	return input, layouts, w, h
}

func loadCalibrationDeck(t *testing.T, name string) *PresentationInput {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "calibration", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	applyDefaults(&input)
	return &input
}

func formatCalibration(results []calibrationResult) string {
	var b strings.Builder
	b.WriteString("deck                             human  score  gate\n")
	for _, r := range results {
		gate := "FAIL"
		if r.gate {
			gate = "pass"
		}
		b.WriteString(strings.TrimRight(r.name, " "))
		for i := len(r.name); i < 33; i++ {
			b.WriteByte(' ')
		}
		b.WriteString(padInt(r.grade, 5) + padInt(r.score, 7) + "  " + gate + "\n")
	}
	return b.String()
}

func padInt(v, width int) string {
	s := ""
	for n := v; ; n /= 10 {
		s = string(rune('0'+n%10)) + s
		if n < 10 {
			break
		}
	}
	for len(s) < width {
		s = " " + s
	}
	return s
}

// spearman returns the rank correlation of two equal-length samples.
func spearman(xs, ys []float64) float64 {
	n := len(xs)
	if n < 2 || len(ys) != n {
		return 0
	}
	rx, ry := ranks(xs), ranks(ys)
	var d2 float64
	for i := 0; i < n; i++ {
		d := rx[i] - ry[i]
		d2 += d * d
	}
	return 1 - 6*d2/(float64(n)*(float64(n)*float64(n)-1))
}

// ranks returns 1-based ranks, averaging ties.
func ranks(v []float64) []float64 {
	n := len(v)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return v[idx[a]] < v[idx[b]] })
	out := make([]float64, n)
	for i := 0; i < n; {
		j := i
		for j+1 < n && v[idx[j+1]] == v[idx[i]] {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			out[idx[k]] = avg
		}
		i = j + 1
	}
	return out
}
