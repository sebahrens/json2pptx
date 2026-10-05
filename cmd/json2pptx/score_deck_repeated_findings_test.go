package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// matrixDiagramDeck is a two-slide deck whose second slide is a matrix_2x2
// diagram with the given quadrants.
func matrixDiagramDeck(tpl string, quadrants []any) map[string]any {
	return map[string]any{
		"template":        tpl,
		"output_filename": "matrix.pptx",
		"slides": []any{
			map[string]any{"slide_type": "title", "content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Roadmap priorities for the second half"},
			}},
			map[string]any{"slide_type": "diagram", "content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Three quick wins ship before the two major projects start"},
				map[string]any{"placeholder_id": "body", "type": "diagram", "diagram_value": map[string]any{
					"type": "matrix_2x2",
					"alt":  "Two by two matrix of features by effort and customer value",
					"data": map[string]any{"x_label": "Effort", "y_label": "Value", "quadrants": quadrants},
				}},
			}},
		},
	}
}

func matrixQuadrant(position, label string, items ...any) map[string]any {
	q := map[string]any{"label": label, "items": items}
	if position != "" {
		q["position"] = position
	}
	return q
}

// answerFindingsWithCode returns the findings of a tool answer whose code is code,
// whatever namespace the surface puts in front of it.
func answerFindingsWithCode(t *testing.T, findings []any, code string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, f := range findings {
		m, _ := f.(map[string]any)
		if c, _ := m["code"].(string); c == code || strings.HasSuffix(c, "."+code) {
			out = append(out, m)
		}
	}
	return out
}

// go-slide-creator-t3k06: the preflight reports a diagram note at the field it
// concerns and the render reports it again at the content item, so every
// surface that joins the two listed the note twice, and score_deck charged it
// twice. Two quadrants of this matrix name a position that is none: each
// surface lists two notes, and the slide pays for two.
func TestDiagramNoteIsListedOnceOnEverySurface(t *testing.T) {
	const code = "diagram.quadrant_position_defaulted"
	mc := testMCPConfig(t)
	deck := func() map[string]any {
		return matrixDiagramDeck("midnight-blue", []any{
			matrixQuadrant("diagonal", "Quick Wins", "Dark Mode", "Export PDF"),
			matrixQuadrant("top-right", "Major Projects", "AI Assistant"),
			matrixQuadrant("bottom-left", "Fill-Ins", "Emoji Support"),
			matrixQuadrant("sideways", "Time Sinks", "Legacy Import"),
		})
	}

	t.Run("score_deck", func(t *testing.T) {
		var out struct {
			PerSlide []struct {
				Score    int `json:"score"`
				Findings []struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"findings"`
			} `json:"per_slide"`
			Summary struct {
				TopCodes []struct {
					Code  string `json:"code"`
					Count int    `json:"count"`
				} `json:"top_codes"`
			} `json:"summary"`
		}
		res, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{"presentation": deck()}))
		if err != nil || res.IsError {
			t.Fatalf("score_deck: %v %s", err, resultText(res))
		}
		structuredInto(t, res.StructuredContent, &out)
		if len(out.PerSlide) != 2 {
			t.Fatalf("per_slide = %+v", out.PerSlide)
		}
		notes := 0
		for _, f := range out.PerSlide[1].Findings {
			if f.Code == code {
				notes++
			}
		}
		if notes != 2 {
			t.Errorf("slide 2 lists the note %d times, want once per mispositioned quadrant (2): %+v", notes, out.PerSlide[1].Findings)
		}
		if want := 100 - 5*len(out.PerSlide[1].Findings); out.PerSlide[1].Score != want || notes != len(out.PerSlide[1].Findings) {
			t.Errorf("slide 2 scores %d with %+v, want %d from the two notes alone", out.PerSlide[1].Score, out.PerSlide[1].Findings, 90)
		}
		for _, tc := range out.Summary.TopCodes {
			if tc.Code == code && tc.Count != 2 {
				t.Errorf("top_codes counts the note %d times, want 2", tc.Count)
			}
		}
	})

	for name, call := range map[string]func(map[string]any) map[string]any{
		"validate_input": func(args map[string]any) map[string]any {
			res, err := mc.handleValidate(context.Background(), makeRequest(args))
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
				t.Fatalf("validate_input answer is not JSON: %v", err)
			}
			return doc
		},
		"generate_presentation": func(args map[string]any) map[string]any {
			res, err := mc.handleGenerate(context.Background(), makeRequest(args))
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(textContent(res)), &doc); err != nil {
				t.Fatalf("generate_presentation answer is not JSON: %v", err)
			}
			return doc
		},
	} {
		t.Run(name, func(t *testing.T) {
			doc := call(map[string]any{"presentation": deck(), "fit_report": true, "verbose_fit": true})
			notes := answerFindingsWithCode(t, answerFindings(t, doc), code)
			if len(notes) != 2 {
				t.Fatalf("%d notes, want 2: %+v", len(notes), notes)
			}
			for _, n := range notes {
				raw, _ := json.Marshal(n)
				if !strings.Contains(string(raw), "/diagram_value/data/quadrants/") {
					t.Errorf("the note kept is not the one at the quadrant's field: %s", raw)
				}
			}
		})
	}
}

// A matrix whose quadrants name no position is read in list order. It is a
// documented way to author one: nothing is reported and the slide keeps its
// hundred points.
func TestMatrixInListOrderCostsNothing(t *testing.T) {
	mc := testMCPConfig(t)
	deck := matrixDiagramDeck("midnight-blue", []any{
		matrixQuadrant("", "Quick Wins", "Dark Mode", "Export PDF"),
		matrixQuadrant("", "Major Projects", "AI Assistant"),
		matrixQuadrant("", "Fill-Ins", "Emoji Support"),
		matrixQuadrant("", "Time Sinks", "Legacy Import"),
	})
	got := scoreDeckFor(t, mc, map[string]any{"presentation": deck})
	if len(got.PerSlide) != 2 || got.PerSlide[1].Score != 100 || len(got.PerSlide[1].Findings) != 0 {
		t.Errorf("list-order matrix slide = %+v, want 100 and no findings", got.PerSlide)
	}
}

// The score CLI resolves relative asset paths against the JSON file's
// directory, as generate and validate do.
func TestScoreCLIArgsResolveAssetsBesideTheDeck(t *testing.T) {
	deckPath := filepath.Join("..", "..", "examples", "exhibit-callouts.json")
	presentation, err := readJSONObject(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	args := scoreCLIArgs(deckPath, presentation, "deterministic", "midnight-blue")
	wantDir, _ := filepath.Abs(filepath.Dir(deckPath))
	if args["base_dir"] != wantDir {
		t.Fatalf("base_dir = %v, want the deck's directory %s", args["base_dir"], wantDir)
	}
	res, err := testMCPConfig(t).handleScoreDeck(context.Background(), makeRequest(args))
	if err != nil || res.IsError {
		t.Fatalf("score of a deck with images beside it: %v %s", err, resultText(res))
	}
	if _, has := scoreCLIArgs("-", presentation, "deterministic", "")["base_dir"]; has {
		t.Error("stdin has no directory: base_dir must be left to the working directory")
	}
}

// shortScoreAuditDecks are the decks -short scores: the two that listed a
// finding twice before go-slide-creator-t3k06 (a diagram note at two paths, a
// template-size note at one), each on the template that showed it.
var shortScoreAuditDecks = map[string]string{
	"diagrams/matrix_2x2.json": "midnight-blue",
	"basic-deck.json":          "modern-template",
}

// TestExampleCorpusScoreListsEachFindingOnce scores every example deck and
// holds the list score_deck computes from to one rule: no finding is there
// twice — the same code and message at one path, or at a path and one of its
// ancestors — and the answer lists exactly that list. The corpus job scores
// every deck on midnight-blue, modern-template and the local p-style when
// present; -short scores shortScoreAuditDecks.
func TestExampleCorpusScoreListsEachFindingOnce(t *testing.T) {
	mc := testMCPConfig(t)
	root, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, glob := range []string{"*.json", filepath.Join("diagrams", "*.json")} {
		m, err := filepath.Glob(filepath.Join(root, glob))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	templates := []string{"midnight-blue", "modern-template"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	decks := 0
	for _, file := range files {
		rel, _ := filepath.Rel(root, file)
		rel = filepath.ToSlash(rel)
		on := templates
		if testing.Short() {
			tmpl, ok := shortScoreAuditDecks[rel]
			if !ok {
				continue
			}
			on = []string{tmpl}
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		decks++
		for _, tmpl := range on {
			var deck map[string]any
			if err := json.Unmarshal(data, &deck); err != nil {
				t.Fatal(err)
			}
			deck["template"] = tmpl
			delete(deck, "template_path")
			args := map[string]any{"presentation": deck, "base_dir": filepath.Dir(file)}
			name := rel + " on " + tmpl

			ev, errResult := mc.collectScoreDeckEvidence(context.Background(), makeRequest(args))
			if errResult != nil {
				t.Errorf("%s: not scored: %s", name, resultText(errResult))
				continue
			}
			assertNoRepeatedFinding(t, name, ev.findings)

			got := scoreDeckFor(t, mc, args)
			listed := len(got.DeckFindings)
			for _, ss := range got.PerSlide {
				listed += len(ss.Findings)
			}
			if listed != len(ev.findings) {
				t.Errorf("%s: the answer lists %d findings, the deck has %d", name, listed, len(ev.findings))
			}
		}
	}
	if len(files) == 0 || (testing.Short() && decks != len(shortScoreAuditDecks)) {
		t.Errorf("scored %d of %d decks: one of the -short decks was renamed or removed", decks, len(files))
	}
}

// assertNoRepeatedFinding fails for each pair of findings that state one fact:
// the same code and message at one path, or at a path and its ancestor.
func assertNoRepeatedFinding(t *testing.T, name string, findings []patterns.FitFinding) {
	t.Helper()
	related := func(a, b string) bool {
		return a == b || (a != "" && strings.HasPrefix(b, a+"/")) || (b != "" && strings.HasPrefix(a, b+"/"))
	}
	for i, a := range findings {
		for _, b := range findings[i+1:] {
			if a.Code == b.Code && a.Message == b.Message && related(a.Path, b.Path) {
				t.Errorf("%s: slide %d carries %s twice (%s and %s): %s",
					name, slidepath.SlideIndex(a.Path)+1, a.Code, a.Path, b.Path, a.Message)
			}
		}
	}
}
