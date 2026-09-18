package main

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

// assertScore100 fails unless v is on the shared 0-100 scale.
func assertScore100(t *testing.T, label string, v float64) {
	t.Helper()
	if v < 0 || v > 100 {
		t.Errorf("%s = %v, want within [0, 100]", label, v)
	}
}

// assertBasis fails unless got is the wanted member of the basis enum.
func assertBasis(t *testing.T, label, got, want string) {
	t.Helper()
	if !slices.Contains(scoreBases, got) {
		t.Errorf("%s basis %q not in enum %v", label, got, scoreBases)
	}
	if got != want {
		t.Errorf("%s basis = %q, want %q", label, got, want)
	}
}

// TestScoreScaleUnified is the go-slide-creator-n1t7 acceptance test: every
// deck-quality score (generate/render_deck_spec quality, score_deck, and the
// auto_repair/make_deck final_score) is 0-100 and carries a basis from the
// closed enum input|structural|rendered.
func TestScoreScaleUnified(t *testing.T) {
	if want := []string{"input", "structural", "rendered"}; !slices.Equal(scoreBases, want) {
		t.Fatalf("scoreBases = %v, want %v", scoreBases, want)
	}

	t.Run("input_quality", func(t *testing.T) {
		deck := []SlideInput{
			{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Good title")}}},
			{},
		}
		q := computeQualityScore(deck, []string{"w"})
		assertScore100(t, "quality.score", q.Score)
		assertBasis(t, "quality", q.Basis, scoreBasisInput)
		if q.Score <= 1 {
			t.Errorf("quality.score = %v looks like the old 0-1 scale", q.Score)
		}
		for _, s := range q.SlideScores {
			assertScore100(t, "quality.slide_scores[].score", s.Score)
		}
		empty := computeQualityScore(nil, nil)
		assertBasis(t, "empty quality", empty.Basis, scoreBasisInput)
	})

	t.Run("score_deck", func(t *testing.T) {
		mc := repairMC(t)
		res, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(autoRepairDeck(1)),
		}))
		if err != nil || res.IsError {
			t.Fatalf("score_deck failed: %v %s", err, textContent(res))
		}
		var out struct {
			OverallScore float64 `json:"overall_score"`
			Basis        string  `json:"basis"`
		}
		if err := json.Unmarshal([]byte(textContent(res)), &out); err != nil {
			t.Fatal(err)
		}
		assertScore100(t, "score_deck.overall_score", out.OverallScore)
		assertBasis(t, "score_deck", out.Basis, scoreBasisStructural)
	})

	t.Run("auto_repair", func(t *testing.T) {
		mc := repairMC(t)
		res, err := mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
			"presentation":    mustParseJSON(autoRepairDeck(1)),
			"max_passes":      float64(1),
			"output_filename": "score_basis_test.pptx",
		}))
		if err != nil || res.IsError {
			t.Fatalf("auto_repair failed: %v %s", err, textContent(res))
		}
		var out autoRepairOutput
		if err := json.Unmarshal([]byte(textContent(res)), &out); err != nil {
			t.Fatal(err)
		}
		assertScore100(t, "auto_repair.final_score", float64(out.FinalScore))
		assertBasis(t, "auto_repair", out.ScoreBasis, scoreBasisStructural)
		md := makeDeckOutputFromLoop(&out, nil)
		assertBasis(t, "make_deck", md.ScoreBasis, scoreBasisStructural)
	})

	t.Run("output_schemas_declare_basis", func(t *testing.T) {
		for name, schema := range map[string]json.RawMessage{
			"score_deck":  outputSchemaScoreDeck,
			"auto_repair": outputSchemaAutoRepair,
			"make_deck":   outputSchemaMakeDeck,
		} {
			var s struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(schema, &s); err != nil {
				t.Fatalf("%s schema: %v", name, err)
			}
			key := "score_basis"
			if name == "score_deck" {
				key = "basis"
			}
			enum := s.Properties[key].Enum
			if len(enum) == 0 || !slices.Contains(scoreBases, enum[0]) {
				t.Errorf("%s schema %s enum = %v, want a member of %v", name, key, enum, scoreBases)
			}
		}
		var q struct {
			Properties map[string]struct {
				Enum    []string `json:"enum"`
				Maximum float64  `json:"maximum"`
			} `json:"properties"`
		}
		if err := json.Unmarshal([]byte(qualityScoreSchema), &q); err != nil {
			t.Fatal(err)
		}
		if q.Properties["score"].Maximum != 100 || !slices.Equal(q.Properties["basis"].Enum, []string{scoreBasisInput}) {
			t.Errorf("quality_score schema = %+v", q.Properties)
		}
	})
}
