package patterns

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/textfit"
)

func TestSCQASummarySharedRowBudget(t *testing.T) {
	p := &scqaSummary{}
	v := validSCQAValues()
	v.Questions = []string{strings.Repeat("Q", 106), strings.Repeat("Q", 106)}
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "questions") || !strings.Contains(got[0], "about 105 characters per bullet") {
		t.Fatalf("two dense bullets warning: %v", got)
	}
	v.Questions = []string{strings.Repeat("Q", 105), strings.Repeat("Q", 105), strings.Repeat("Q", 105)}
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("three measured-length bullets should fit: %v", got)
	}
	// The 2-bullet single-bullet budget (210) equals the row total (2 x 105),
	// so a short neighbour spends part of it.
	v.Questions = []string{strings.Repeat("Q", 205), "Brief"}
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("one long bullet with a short neighbor should fit: %v", got)
	}
	v.Questions = []string{strings.Repeat("Q", 106), "Brief", "Brief"}
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "questions[0]") {
		t.Fatalf("one long bullet among three should warn: %v", got)
	}
	// Four-bullet rows hold no readable copy.
	v.Questions = []string{"Brief", "Brief", "Brief", "Brief"}
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "at most 3 readable bullets") {
		t.Fatalf("four bullets should warn: %v", got)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
}

func TestSCQASummary_Registration(t *testing.T) {
	p, ok := Default().Get("scqa-summary")
	if !ok {
		t.Fatal("expected scqa-summary to be registered in default registry")
	}
	if p.Name() != "scqa-summary" {
		t.Errorf("Name() = %q, want %q", p.Name(), "scqa-summary")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
	if !strings.Contains(p.UseWhen(), "prefer") {
		t.Errorf("UseWhen() should contain contrastive language: %q", p.UseWhen())
	}
}

func validSCQAValues() *SCQASummaryValues {
	return &SCQASummaryValues{
		Situation:    SCQAText{"Cloud spend grew 38% YoY in FY25."},
		Complication: SCQAText{"Margin eroded as workloads scaled."},
		Questions:    []string{"What drives the spend?", "Where can we recover margin?"},
		Answer:       []string{"Right-size top workloads.", "Stand up FinOps governance."},
	}
}

func TestSCQASummary_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	if err := p.Validate(validSCQAValues(), nil, nil); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestSCQASummary_Validate_RequiredFields(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	cases := []struct {
		name    string
		mutate  func(*SCQASummaryValues)
		wantErr string
	}{
		{
			"empty_situation",
			func(v *SCQASummaryValues) { v.Situation = SCQAText{} },
			"at least 1 items",
		},
		{
			"empty_complication",
			func(v *SCQASummaryValues) { v.Complication = SCQAText{} },
			"at least 1 items",
		},
		{
			"empty_questions",
			func(v *SCQASummaryValues) { v.Questions = nil },
			"at least 1 items",
		},
		{
			"empty_answer",
			func(v *SCQASummaryValues) { v.Answer = nil },
			"at least 1 items",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := validSCQAValues()
			tc.mutate(v)
			err := p.Validate(v, nil, nil)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestSCQASummary_Validate_TooManyItems(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Answer = []string{"a", "b", "c", "d", "e"}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for more than 4 answer items")
	}
	if !strings.Contains(err.Error(), "at most 4") {
		t.Errorf("expected max_items error, got: %v", err)
	}
}

func TestSCQASummary_Validate_ItemTooLong(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Questions[0] = strings.Repeat("x", 241)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for too-long item")
	}
	if !strings.Contains(err.Error(), "exceeds maxLength 240") {
		t.Errorf("expected max_length error, got: %v", err)
	}
}

func TestSCQASummary_Validate_EmptyItem(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Answer = []string{"Real answer", ""}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for empty item")
	}
	if !strings.Contains(err.Error(), "answer[1] is required") {
		t.Errorf("expected required error for empty item, got: %v", err)
	}
}

func TestSCQASummary_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	overrides := map[int]any{99: &SCQASummaryCellOverride{AccentBar: true}}
	err := p.Validate(validSCQAValues(), nil, overrides)
	if err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected out_of_range error, got: %v", err)
	}
}

func TestSCQASummary_Validate_CellOverrideBadKey(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	overrides := map[int]any{
		0: &struct {
			BadKey string `json:"bad_key"`
		}{BadKey: "nope"},
	}
	err := p.Validate(validSCQAValues(), nil, overrides)
	if err == nil {
		t.Fatal("expected validation error for bad cell override key")
	}
	if !strings.Contains(err.Error(), `unknown key "bad_key"`) {
		t.Errorf("expected unknown_key error, got: %v", err)
	}
}

func TestSCQASummary_UnmarshalJSON_StringForm(t *testing.T) {
	raw := []byte(`{
        "situation": "Single-paragraph situation.",
        "complication": "Single-paragraph complication.",
        "questions": ["Q1", "Q2"],
        "answer": ["A1", "A2"]
    }`)
	var v SCQASummaryValues
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(v.Situation) != 1 || v.Situation[0] != "Single-paragraph situation." {
		t.Errorf("situation = %#v, want single-item list", v.Situation)
	}
	if len(v.Complication) != 1 || v.Complication[0] != "Single-paragraph complication." {
		t.Errorf("complication = %#v, want single-item list", v.Complication)
	}
}

func TestSCQASummary_UnmarshalJSON_ArrayForm(t *testing.T) {
	raw := []byte(`{
        "situation": ["s1", "s2"],
        "complication": ["c1"],
        "questions": ["Q1"],
        "answer": ["A1"]
    }`)
	var v SCQASummaryValues
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(v.Situation) != 2 || v.Situation[0] != "s1" || v.Situation[1] != "s2" {
		t.Errorf("situation = %#v, want [s1, s2]", v.Situation)
	}
	if len(v.Complication) != 1 || v.Complication[0] != "c1" {
		t.Errorf("complication = %#v, want [c1]", v.Complication)
	}
}

func TestSCQASummary_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 4 {
		t.Fatalf("expected 4 rows, got %d", got)
	}
	for i, row := range grid.Rows {
		if got := len(row.Cells); got != 2 {
			t.Errorf("row[%d]: expected 2 cells, got %d", i, got)
		}
	}
	// Columns spec is [1, 4] so left column is ~20%.
	if string(grid.Columns) != "[1, 4]" {
		t.Errorf("columns = %s, want [1, 4]", string(grid.Columns))
	}
}

// The labels are pentagon tabs pointing into their rows: Situation /
// Complication / Questions in the accent's Lighter 80% swatch with ink measured
// on it, the Answer — the one emphasis — in the solid accent.
func TestSCQASummary_Expand_LabelCellsAreAccentTabs(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	for _, tc := range []struct {
		name string
		ctx  ExpandContext
	}{{"no theme", ExpandContext{}}, {"themed", fullThemeCtx()}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx
			grid, err := p.Expand(ctx, validSCQAValues(), nil, nil)
			if err != nil {
				t.Fatalf("Expand failed: %v", err)
			}
			if len(grid.Rows) != 4 {
				t.Fatalf("rows = %d, want 4", len(grid.Rows))
			}
			const wantTab = `{"color":"accent1","lumMod":20000,"lumOff":80000}`
			tabTone := tonalContent(ctx, "accent1")
			if string(tabTone.fillJSON()) != wantTab {
				t.Fatalf("tonalContent(accent1) = %s, want the Lighter 80%% swatch", tabTone.fillJSON())
			}
			tabInk := tonalInk(ctx, tabTone)
			answerTone, answerInk := tonalEmphasis(ctx, "accent1")
			if answerTone.Color != "accent1" || answerTone.LumOff != 0 {
				t.Fatalf("tonalEmphasis(accent1) = %+v, want the solid accent (shaded at most)", answerTone)
			}
			if tabInk == answerInk {
				t.Fatalf("fixture: tab ink and answer ink are both %q; the pale tab and the solid tab should take opposite inks", tabInk)
			}

			// The expected point: 12pt deep on a tab labelW wide and an equal
			// share of the height tall (no row of this fixture is floored).
			var cols []float64
			if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 2 {
				t.Fatalf("columns %s: %v", grid.Columns, err)
			}
			areaW, areaH := sizingAreaPt(ctx)
			labelW, _ := scqaColumnWidthsPt(ctx, areaW, cols[0])
			rowH := (areaH - 3*ctx.Gap(scqaRowGapPt)) / 4
			wantAdj := swimlanePointAdj(12, labelW, rowH)
			if wantAdj <= 0 || wantAdj > 50000 {
				t.Fatalf("expected adj = %d out of range", wantAdj)
			}

			labels := []string{"Situation", "Complication", "Questions", "Answer"}
			for i, row := range grid.Rows {
				if row.Flex != 0 {
					t.Fatalf("fixture drifted: row %d is floored at its need; the equal-share point depth no longer applies", i)
				}
				labelCell := row.Cells[0]
				wantFill, wantInk := wantTab, tabInk
				if i == 3 {
					wantFill, wantInk = string(answerTone.fillJSON()), answerInk
				}
				if labelCell.Shape.Geometry != "homePlate" {
					t.Errorf("row[%d] label geometry = %q, want homePlate", i, labelCell.Shape.Geometry)
				}
				if got := string(labelCell.Shape.Fill); got != wantFill {
					t.Errorf("row[%d] label fill = %s, want %s", i, got, wantFill)
				}
				if got := string(labelCell.Shape.Line); got != `"none"` {
					t.Errorf("row[%d] label line = %s, want none", i, got)
				}
				if labelCell.AccentBar != nil {
					t.Errorf("row[%d] label carries an accent bar: %+v", i, labelCell.AccentBar)
				}
				if got, ok := labelCell.Shape.Adjustments["adj"]; !ok || got != wantAdj || len(labelCell.Shape.Adjustments) != 1 {
					t.Errorf("row[%d] adjustments = %v, want adj=%d (a 12pt point)", i, labelCell.Shape.Adjustments, wantAdj)
				}
				var obj scqaTextObj
				if err := json.Unmarshal(labelCell.Shape.Text, &obj); err != nil {
					t.Fatalf("row[%d] label text: %v", i, err)
				}
				if len(obj.Paragraphs) != 1 || obj.Paragraphs[0].Content != labels[i] || !obj.Paragraphs[0].Bold || obj.Paragraphs[0].Color != wantInk {
					t.Errorf("row[%d] label text = %+v, want bold %s %q", i, obj.Paragraphs, wantInk, labels[i])
				}
				if got := string(labelCell.Shape.Text); got != string(buildSCQALabelText(labels[i], obj.Paragraphs[0].Size, wantInk)) {
					t.Errorf("row[%d] label text = %s, want buildSCQALabelText in %s", i, got, wantInk)
				}
			}
			if got := string(grid.Rows[3].Cells[0].Shape.Fill); tc.name == "no theme" && got != `"accent1"` {
				t.Errorf("Answer tab fill = %s, want the solid accent1", got)
			}
		})
	}
}

// Situation / Complication / Questions content stays open; the Answer's
// content is a pale band (the accent's Lighter 90% swatch) led by its solid
// tab.
func TestSCQASummary_Expand_ContentCellsOpenExceptAnswerBand(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	for _, ctx := range []ExpandContext{{}, fullThemeCtx()} {
		grid, err := p.Expand(ctx, validSCQAValues(), nil, nil)
		if err != nil {
			t.Fatalf("Expand failed: %v", err)
		}
		if len(grid.Rows) != 4 {
			t.Fatalf("rows = %d, want 4", len(grid.Rows))
		}
		const wantBand = `{"color":"accent1","lumMod":10000,"lumOff":90000}`
		if got := string(tonalRung(ctx, "accent1", 90).fillJSON()); got != wantBand {
			t.Fatalf("tonalRung(accent1, 90) = %s, want %s", got, wantBand)
		}
		for i, row := range grid.Rows {
			contentCell := row.Cells[1]
			want := `"none"`
			if i == 3 {
				want = wantBand
			}
			if got := string(contentCell.Shape.Fill); got != want {
				t.Errorf("row[%d] content fill = %s, want %s", i, got, want)
			}
			if contentCell.Shape.Geometry != "rect" || len(contentCell.Shape.Adjustments) != 0 {
				t.Errorf("row[%d] content cell = %q %v, want a plain rect", i, contentCell.Shape.Geometry, contentCell.Shape.Adjustments)
			}
			if got := string(contentCell.Shape.Line); got != `"none"` {
				t.Errorf("row[%d] content line = %s, want none", i, got)
			}
		}
	}
}

func TestSCQASummary_Expand_BulletsRenderForMultiItem(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Answer = []string{"First answer.", "Second answer."}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Row 3 is Answer; cell 1 is the content cell.
	contentText := string(grid.Rows[3].Cells[1].Shape.Text)
	// Real bullets (go-slide-creator-zieyk): a bullet field, never a typed glyph.
	if !strings.Contains(contentText, `"content":"First answer.","size":12,"color":"dk1","align":"l","bullet":true`) {
		t.Errorf("expected a real bullet on multi-item content, got: %s", contentText)
	}
	if !strings.Contains(contentText, `"content":"Second answer.","size":12,"color":"dk1","align":"l","bullet":true`) {
		t.Errorf("expected a real bullet on second item, got: %s", contentText)
	}
	if strings.Contains(contentText, "• ") {
		t.Errorf("literal bullet glyph in content: %s", contentText)
	}
}

func TestSCQASummary_Expand_NoBulletForSingleItem(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// validSCQAValues has a single-item situation/complication; should not be bulleted.
	situationText := string(grid.Rows[0].Cells[1].Shape.Text)
	if strings.Contains(situationText, "•") {
		t.Errorf("single-item situation should not have bullet prefix, got: %s", situationText)
	}
}

func TestSCQASummary_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	ovr := &SCQASummaryOverrides{Accent: "accent3"}
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), ovr, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// The override moves every accent-derived fill: the three pale tabs, the
	// solid Answer tab and the Answer band.
	for i := 0; i < 3; i++ {
		if got := string(grid.Rows[i].Cells[0].Shape.Fill); got != `{"color":"accent3","lumMod":20000,"lumOff":80000}` {
			t.Errorf("row[%d] label fill = %s, want accent3's Lighter 80%% swatch", i, got)
		}
		if got := string(grid.Rows[i].Cells[1].Shape.Fill); got != `"none"` {
			t.Errorf("row[%d] content fill = %s, want none", i, got)
		}
	}
	var fill string
	if err := json.Unmarshal(grid.Rows[3].Cells[0].Shape.Fill, &fill); err != nil {
		t.Fatalf("Answer label fill unmarshal: %v", err)
	}
	if fill != "accent3" {
		t.Errorf("Answer label fill = %q, want accent3", fill)
	}
	if got := string(grid.Rows[3].Cells[1].Shape.Fill); got != `{"color":"accent3","lumMod":10000,"lumOff":90000}` {
		t.Errorf("Answer content fill = %s, want accent3's Lighter 90%% swatch", got)
	}
	for i, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if strings.Contains(string(cell.Shape.Fill), "accent1") {
				t.Errorf("row[%d] cell %d fill = %s still uses the default accent", i, ci, cell.Shape.Fill)
			}
		}
	}
}

func TestSCQASummary_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	overrides := map[int]any{
		0: &SCQASummaryCellOverride{AccentBar: true},
		7: &SCQASummaryCellOverride{AccentBar: true},
	}
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, overrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Cells[0].AccentBar == nil {
		t.Error("expected cell[0] (Situation label) to have an accent bar")
	}
	if grid.Rows[3].Cells[1].AccentBar == nil {
		t.Error("expected cell[7] (Answer content) to have an accent bar")
	}
	if grid.Rows[0].Cells[1].AccentBar != nil {
		t.Error("cell[1] should not have an accent bar")
	}
}

func TestSCQASummary_Schema(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	s := p.Schema()
	if s == nil {
		t.Fatal("Schema() returned nil")
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("Schema marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Schema unmarshal: %v", err)
	}
	if m["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("missing $schema draft 2020-12")
	}
	if m["type"] != "object" {
		t.Errorf("root type = %v, want object", m["type"])
	}
}

func TestSCQASummary_Taxonomy(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	tax := p.Taxonomy()
	if tax.Category == "" {
		t.Error("Category should be set")
	}
	if tax.DensityClass == "" {
		t.Error("DensityClass should be set")
	}
	if tax.AccentWeight == "" {
		t.Error("AccentWeight should be set")
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("NarrativeRole should be non-empty")
	}
	if len(tax.PairsWith) == 0 {
		t.Error("PairsWith should be non-empty")
	}
}

func TestSCQASummary_Golden_Default(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	assertPatternGolden(t, grid, filepath.Join("testdata", "scqa-summary", "default.golden.json"))
}

// A row label is one word ("Complication") that must never break mid-word. On
// abstract the body face is Tenorite, which the measurer only substitutes, and
// LibreOffice drew "Complicatio / n" at 20pt in the 1/5 label column: the label
// was measured edge-to-edge in Liberation Sans. Every label word now fits the
// atomic-token width (80% of the text width for a substituted face), the label
// stepping down 1pt at a time (go-slide-creator-n1muf).
func TestSCQASummaryLabelsNeverBreakMidWord(t *testing.T) {
	areas := []struct {
		name string
		w, h float64
	}{{"abstract", 687, 294}, {"modern", 851, 311}, {"warm-coral", 828, 349}, {"p-style", 899, 360}}
	for _, font := range []string{"Tenorite", "Segoe UI", "Arial", ""} {
		for _, a := range areas {
			for _, header := range []float64{0, 28} {
				ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(a.w * 12700), Height: int64(a.h * 12700)}}
				ctx.Theme.BodyFont = font
				var ovr any
				if header > 0 {
					ovr = &SCQASummaryOverrides{HeaderSize: header}
				}
				grid, err := (&scqaSummary{}).Expand(ctx, validSCQAValues(), ovr, nil)
				if err != nil {
					t.Fatal(err)
				}
				var cols []float64
				if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 2 {
					t.Fatalf("columns %s: %v", grid.Columns, err)
				}
				labelW := (a.w-scqaColGapPt)*cols[0]/(cols[0]+cols[1]) - 2*defaultShapeInsetLRPt
				w := textfit.AtomicTokenWidthPt(font, labelW)
				for _, row := range grid.Rows {
					var obj scqaTextObj
					if err := json.Unmarshal(row.Cells[0].Shape.Text, &obj); err != nil {
						t.Fatal(err)
					}
					p := obj.Paragraphs[0]
					if p.Size < 12 {
						t.Errorf("%q/%s/header %.0f: label %q at %.0fpt, below the 12pt floor", font, a.name, header, p.Content, p.Size)
					}
					if lines := measuredLines(p.Content, font, true, p.Size, w); lines > 1 {
						t.Errorf("%q/%s/header %.0f: label %q at %.0fpt wraps to %d lines in %.0fpt", font, a.name, header, p.Content, p.Size, lines, w)
					}
				}
			}
		}
	}
}
