package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

type scoredDeck struct {
	OverallScore int `json:"overall_score"`
	PerSlide     []struct {
		Index    int `json:"index"`
		Score    int `json:"score"`
		Findings []struct {
			Code string `json:"code"`
		} `json:"findings"`
	} `json:"per_slide"`
	DeckFindings []struct {
		Code     string         `json:"code"`
		Severity string         `json:"severity"`
		Message  string         `json:"message"`
		Path     string         `json:"path"`
		Slides   []int          `json:"slides"`
		Points   int            `json:"points"`
		Fix      map[string]any `json:"fix"`
	} `json:"deck_findings"`
	Summary struct {
		TopCodes []struct {
			Code  string `json:"code"`
			Count int    `json:"count"`
		} `json:"top_codes"`
	} `json:"summary"`
	QualityGate struct {
		Passed bool `json:"passed"`
	} `json:"quality_gate"`
}

func scoreDeckFor(t *testing.T, mc *mcpConfig, args map[string]any) scoredDeck {
	t.Helper()
	res, err := mc.handleScoreDeck(context.Background(), makeRequest(args))
	if err != nil || res.IsError {
		t.Fatalf("score_deck: %v %s", err, resultText(res))
	}
	var out scoredDeck
	structuredInto(t, res.StructuredContent, &out)
	return out
}

// go-slide-creator-qhgm8: score_deck dropped every finding whose path is not
// under /slides/N. A deck whose footer line is cut on every slide reported
// CHROME_TRUNCATED from validate and generate, while score_deck listed nothing
// and charged nothing — persona f read its score as nothing to fix. The
// finding is now in deck_findings once (preflight and render report the same
// line), in top_codes, and costs its five points on overall_score.
func TestScoreDeckListsDeckLevelFindings(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(24 * time.Hour)}
	deck := chromeTruncatedDeck("midnight-blue")

	// The same deck with a footer line that fits is the baseline.
	fits := chromeTruncatedDeck("midnight-blue")
	fits["chrome"] = map[string]any{"client_name": "Meridian", "page_numbers": map[string]any{"enabled": true}}
	base := scoreDeckFor(t, mc, map[string]any{"presentation": fits})
	if len(base.DeckFindings) != 0 {
		t.Fatalf("baseline deck has deck-level findings: %+v", base.DeckFindings)
	}

	got := scoreDeckFor(t, mc, map[string]any{"presentation": deck})
	if len(got.DeckFindings) == 0 {
		t.Fatal("CHROME_TRUNCATED is missing from deck_findings")
	}
	messages := map[string]bool{}
	for _, df := range got.DeckFindings {
		if df.Code != patterns.ErrCodeChromeTruncated {
			t.Errorf("unexpected deck-level finding %s: %s", df.Code, df.Message)
			continue
		}
		if df.Path != "/chrome" || df.Points != 5 || df.Severity != "info" || df.Fix["kind"] != "rewrite_field" {
			t.Errorf("deck finding = %+v", df)
		}
		if len(df.Slides) == 0 {
			t.Errorf("deck finding names no slides: %+v", df)
		}
		for _, s := range df.Slides {
			if s < 0 || s >= len(got.PerSlide) {
				t.Errorf("slides = %v: not per_slide indices", df.Slides)
			}
		}
		if messages[df.Message] {
			t.Errorf("the same footer finding is listed twice: %q", df.Message)
		}
		messages[df.Message] = true
	}
	n := len(got.DeckFindings)
	if want := base.OverallScore - 5*n; got.OverallScore != want {
		t.Errorf("overall_score = %d, want the baseline %d less 5 for each of %d deck-level findings", got.OverallScore, base.OverallScore, n)
	}
	counted := 0
	for _, c := range got.Summary.TopCodes {
		if c.Code == patterns.ErrCodeChromeTruncated {
			counted = c.Count
		}
	}
	if counted != n {
		t.Errorf("top_codes counts CHROME_TRUNCATED %d times, want %d", counted, n)
	}
	for _, s := range got.PerSlide {
		for _, f := range s.Findings {
			if f.Code == patterns.ErrCodeChromeTruncated {
				t.Errorf("slide %d carries the deck-level footer finding", s.Index)
			}
		}
		if s.Score != base.PerSlide[s.Index].Score {
			t.Errorf("slide %d scores %d, baseline %d: the footer finding must not move a slide's score", s.Index, s.Score, base.PerSlide[s.Index].Score)
		}
	}
	if got.QualityGate.Passed != base.QualityGate.Passed {
		t.Errorf("quality gate passed = %v, baseline %v: a review-level footer finding does not decide the gate", got.QualityGate.Passed, base.QualityGate.Passed)
	}

	// slide_indices renders a subset, renumbered from 1. Its render-side
	// footer finding would cite the subset's slide numbers and so not fold
	// into the preflight one: the deck-level list must be the whole-deck
	// findings that name a scored slide, word for word, and nothing else.
	named := map[int][]string{}
	for _, df := range got.DeckFindings {
		for _, s := range df.Slides {
			named[s] = append(named[s], df.Message)
		}
	}
	// Two content slides that are not neighbours: rendered alone they are
	// "slides 1-2", which no slide of the deck is.
	pair := scoreDeckFor(t, mc, map[string]any{"presentation": deck, "slide_indices": []any{float64(1), float64(3)}})
	wantPair := map[string]bool{}
	for _, idx := range []int{1, 3} {
		for _, m := range named[idx] {
			wantPair[m] = true
		}
	}
	if len(pair.DeckFindings) != len(wantPair) {
		raw, _ := json.Marshal(pair.DeckFindings)
		t.Errorf("slide_indices [1,3]: %d deck-level findings, want %d: %s", len(pair.DeckFindings), len(wantPair), raw)
	}
	for _, df := range pair.DeckFindings {
		if !wantPair[df.Message] {
			t.Errorf("slide_indices [1,3]: finding cites the subset's numbering: %q", df.Message)
		}
	}
	if want := 5 * len(wantPair); len(pair.PerSlide) == 2 && (pair.PerSlide[0].Score+pair.PerSlide[1].Score)/2-pair.OverallScore != want {
		t.Errorf("slide_indices [1,3]: overall %d against slide scores %d and %d: the footer finding must cost %d once", pair.OverallScore, pair.PerSlide[0].Score, pair.PerSlide[1].Score, want)
	}

	for idx := range got.PerSlide {
		sub := scoreDeckFor(t, mc, map[string]any{"presentation": deck, "slide_indices": []any{float64(idx)}})
		if len(sub.DeckFindings) != len(named[idx]) {
			raw, _ := json.Marshal(sub.DeckFindings)
			t.Errorf("slide_indices [%d]: %d deck-level findings, want the %d that name the slide: %s", idx, len(sub.DeckFindings), len(named[idx]), raw)
			continue
		}
		for _, s := range sub.PerSlide {
			for _, f := range s.Findings {
				if f.Code == patterns.ErrCodeChromeTruncated {
					t.Errorf("slide_indices [%d]: the subset render's footer finding was charged to slide %d", idx, s.Index)
				}
			}
		}
		for _, df := range sub.DeckFindings {
			if !messages[df.Message] {
				t.Errorf("slide_indices [%d]: finding cites the subset's numbering: %q", idx, df.Message)
			}
		}
	}
}
