package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// labelledDeck is one fixture of testdata/quality_labelled: a DeckSpec, the
// template it is judged on, and a human label per slide (0-based index).
type labelledDeck struct {
	About    string                   `json:"about"`
	Template string                   `json:"template"`
	Labels   map[string]labelledSlide `json:"labels"`
	Spec     map[string]any           `json:"spec"`
}

type labelledSlide struct {
	// Label is "weak" (an expert would send the slide back: it must score
	// below the gate's min_score and carry each of Codes), "good" (it must
	// score at or above min_score and carry no composition-fault finding) or
	// "label_title" (a topic-label title: it must carry TITLE_NOT_ACTION and
	// lose points, without falling below the gate on that alone).
	Label    string   `json:"label"`
	Codes    []string `json:"codes"`
	Evidence string   `json:"evidence"`
}

func loadLabelledDeck(t *testing.T, name string) labelledDeck {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "quality_labelled", name))
	if err != nil {
		t.Fatal(err)
	}
	var d labelledDeck
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return d
}

// compositionFaultCodes are the findings that mark a slide as visibly weak.
var compositionFaultCodes = func() []string {
	codes := make([]string, 0, len(deterministic.CompositionFaultWeight))
	for code := range deterministic.CompositionFaultWeight {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}()

// TestLabelledSlidesScoreOnTheRightSideOfTheGate is the go-slide-creator-wwmod
// acceptance test. The weak slides are the ones the 2026-10-03 agent journeys
// photographed at score 100 (c-repair C15 / C17, a-coldstart A13, b-discovery
// B15, d-cli A15, e-revise E15), rebuilt from the journeys' call logs; the
// good slides are the same content authored the way an expert would accept.
// Every weak slide must score below the quality gate's min_score and name its
// fault; every good slide must stay above it with no composition fault.
func TestLabelledSlidesScoreOnTheRightSideOfTheGate(t *testing.T) {
	gate := deterministic.DefaultQualityGateMinScore
	cases := []struct {
		file string
		// wantReady is whether the deck as a whole passes the deterministic
		// gate.
		wantReady bool
	}{
		{"journey-weak.json", false},
		{"matrix-weak.json", true}, // one weak slide of three: reported, not blocking
		{"good.json", true},
	}
	mc := refusalTestConfig(t)
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			deck := loadLabelledDeck(t, tc.file)
			v := deckSpecVerdicts(t, mc, map[string]any{"spec": deck.Spec, "template": deck.Template})
			assertFindingParity(t, tc.file, v)
			if !v.Render.OK || v.Render.Quality == nil {
				t.Fatalf("render failed: %s %+v", v.Render.Error, v.Render.Diagnostics)
			}
			ready := v.Render.DeterministicReady != nil && *v.Render.DeterministicReady
			if ready != tc.wantReady {
				t.Errorf("deterministic_ready = %v, want %v (score %.0f, reasons %v)", ready, tc.wantReady, v.Render.Quality.Score, v.Render.DeterministicBlockingReasons)
			}
			scores := map[int]SlideQuality{}
			for _, s := range v.Render.Quality.SlideScores {
				scores[s.SlideNumber-1] = s
			}
			for key, want := range deck.Labels {
				idx, err := strconv.Atoi(key)
				if err != nil {
					t.Fatalf("label key %q: %v", key, err)
				}
				got, ok := scores[idx]
				if !ok {
					t.Errorf("slide %d has no score", idx)
					continue
				}
				issues := map[string]bool{}
				for _, code := range got.Issues {
					issues[code] = true
				}
				for _, code := range want.Codes {
					if !issues[code] {
						t.Errorf("slide %d (%s): finding %s missing, got %v — %s", idx, want.Label, code, got.Issues, want.Evidence)
					}
				}
				switch want.Label {
				case "weak":
					if got.Score >= float64(gate) {
						t.Errorf("slide %d is labelled weak but scores %.0f (gate %d), issues %v — %s", idx, got.Score, gate, got.Issues, want.Evidence)
					}
				case "good":
					if got.Score < float64(gate) {
						t.Errorf("slide %d is labelled good but scores %.0f (gate %d), issues %v", idx, got.Score, gate, got.Issues)
					}
					for _, code := range compositionFaultCodes {
						if issues[code] {
							t.Errorf("slide %d is labelled good but carries %s", idx, code)
						}
					}
				case "label_title":
					if got.Score >= 100 || got.Score < float64(gate) {
						t.Errorf("slide %d (label title): score %.0f, want below 100 and at or above the gate %d", idx, got.Score, gate)
					}
				default:
					t.Errorf("slide %d: unknown label %q", idx, want.Label)
				}
			}
		})
	}
}

// The evidence slide of A13 / A15 / B15 — a row of cards or KPIs in the top
// of the content area over an empty lower half, hung from the body line — is
// measured on where the ink is. The same block centred, or stretched down the
// area, is balanced whatever pattern drew it.
func TestVerticalImbalanceMeasuresInkPosition(t *testing.T) {
	safe := pptx.RectEmu{X: 457200, Y: 1600200, CX: 11277600, CY: 4572000}
	bodyLine := safe.Y
	slide := &SlideInput{}
	block := func(yFrac, hFrac float64) []pptx.RectEmu {
		return []pptx.RectEmu{{X: safe.X, Y: safe.Y + int64(yFrac*float64(safe.CY)), CX: safe.CX, CY: int64(hFrac * float64(safe.CY))}}
	}
	cases := []struct {
		name string
		ink  []pptx.RectEmu
		want bool
	}{
		{"hung from the body line, lower 70% empty", block(0, 0.30), true},
		{"hung from the body line, lower 45% empty", block(0, 0.55), true},
		{"hung from the body line, lower 35% empty", block(0, 0.65), false},
		{"centred thin band", block(0.35, 0.30), false},
		{"stretched down the area", block(0, 0.95), false},
		{"block at the top, callout at the bottom, 55% empty between",
			append(block(0, 0.30), block(0.85, 0.15)...), true},
		{"block at the top, callout at the bottom, 30% empty between",
			append(block(0, 0.55), block(0.85, 0.15)...), false},
	}
	for _, c := range cases {
		f := checkVerticalImbalance(c.ink, safe, slide, 0, "", bodyLine, true)
		if (f != nil) != c.want {
			t.Errorf("%s: finding = %v, want reported=%v", c.name, f, c.want)
		}
		if f != nil && f.Code != patterns.ErrCodeVerticalImbalance {
			t.Errorf("%s: code %s", c.name, f.Code)
		}
	}
}

// HORIZONTAL_IMBALANCE is measured the same way: rows whose text ends
// mid-slide fill the left half only; the same rows written across the width
// do not.
func TestHorizontalImbalanceMeasuresInkPosition(t *testing.T) {
	safe := pptx.RectEmu{X: 457200, Y: 1600200, CX: 11277600, CY: 4572000}
	slide := &SlideInput{}
	span := func(xFrac, wFrac float64) []pptx.RectEmu {
		return []pptx.RectEmu{{X: safe.X + int64(xFrac*float64(safe.CX)), Y: safe.Y, CX: int64(wFrac * float64(safe.CX)), CY: safe.CY}}
	}
	cases := []struct {
		name string
		ink  []pptx.RectEmu
		want bool
	}{
		{"left half only", span(0, 0.50), true},
		{"left 65%", span(0, 0.65), false},
		{"full width", span(0, 1), false},
		{"centred narrow block", span(0.30, 0.40), false},
		{"right half only", span(0.55, 0.45), true},
	}
	for _, c := range cases {
		f := checkHorizontalImbalance(c.ink, safe, slide, 0, "")
		if (f != nil) != c.want {
			t.Errorf("%s: finding = %v, want reported=%v", c.name, f, c.want)
		}
	}
}

// Composition faults cost a slide enough to put it below the gate, count as
// problem slides, and do not block by themselves.
func TestCompositionFaultsScoreBelowGateWithoutBlocking(t *testing.T) {
	criteria := deterministic.DefaultQualityGateCriteria()
	for _, code := range compositionFaultCodes {
		f := patterns.FitFinding{ValidationError: patterns.ValidationError{Path: "/slides/0", Code: code}, Action: "review"}
		ds := deterministic.ScoreFromFindings([]patterns.FitFinding{f}, 4)
		if ds.PerSlide[0].Score >= criteria.MinScore {
			t.Errorf("%s: slide score %d is not below min_score %d", code, ds.PerSlide[0].Score, criteria.MinScore)
		}
		if deterministic.BlocksGate(f, criteria) {
			t.Errorf("%s blocks the gate by itself; it is an advisory that costs points", code)
		}
		if gate := deterministic.EvaluateQualityGate(ds, []patterns.FitFinding{f}, criteria); !gate.Passed {
			t.Errorf("%s: one weak slide of four fails the gate: %v", code, gate.Reasons)
		}
	}
	// The text faults count toward the problem-slide share: three slides of
	// fragment columns in a deck of six are a weak deck.
	var many []patterns.FitFinding
	for i := 0; i < 3; i++ {
		many = append(many, patterns.FitFinding{ValidationError: patterns.ValidationError{Path: "/slides/" + strconv.Itoa(i), Code: patterns.ErrCodeTextWrapsNarrow}, Action: "review"})
	}
	ds := deterministic.ScoreFromFindings(many, 6)
	if ds.Summary.ProblemSlidesCount != 3 {
		t.Errorf("problem_slides_count = %d, want 3", ds.Summary.ProblemSlidesCount)
	}
	if gate := deterministic.EvaluateQualityGate(ds, many, criteria); gate.Passed {
		t.Errorf("three weak slides of six passed the gate at score %d", ds.OverallScore)
	}
	// A deck where every slide is lopsided fails the score floor.
	var lopsided []patterns.FitFinding
	for i := 0; i < 4; i++ {
		lopsided = append(lopsided, patterns.FitFinding{ValidationError: patterns.ValidationError{Path: "/slides/" + strconv.Itoa(i), Code: patterns.ErrCodeVerticalImbalance}, Action: "review"})
	}
	ds = deterministic.ScoreFromFindings(lopsided, 4)
	if gate := deterministic.EvaluateQualityGate(ds, lopsided, criteria); gate.Passed {
		t.Errorf("a deck of four lopsided slides passed the gate at score %d", ds.OverallScore)
	}
}
