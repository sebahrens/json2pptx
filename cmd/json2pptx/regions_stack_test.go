package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// regionsStackGrid is a main_left stack as the compiler writes it: a stat
// region (a nested stat-hero pattern) over a text region under its heading.
func regionsStackGrid(statPct, textPct float64, bullets ...string) *jsonschema.ShapeGridInput {
	paras := make([]string, len(bullets))
	for i, b := range bullets {
		paras[i] = fmt.Sprintf(`{"content":%q,"bullet":true}`, b)
	}
	heading := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`),
		Text: json.RawMessage(`{"paragraphs":[{"content":"Position","bold":true}],"align":"l","vertical_align":"b","inset_top":0,"inset_bottom":0}`)}}
	body := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`),
		Text: json.RawMessage(`{"paragraphs":[` + strings.Join(paras, ",") + `],"align":"l","vertical_align":"t"}`)}}
	text := &jsonschema.GridCellInput{Grid: &jsonschema.ShapeGridInput{Columns: json.RawMessage("1"), RowGap: 4, Rows: []jsonschema.GridRowInput{
		{MinHeight: 20, MaxHeight: 20, Cells: []*jsonschema.GridCellInput{heading}},
		{Cells: []*jsonschema.GridCellInput{body}},
	}}}
	stat := &jsonschema.GridCellInput{Pattern: json.RawMessage(`{"name":"stat-hero","values":{"value":"2.3%","label":"Share of the market"}}`)}
	return &jsonschema.ShapeGridInput{Source: jsonschema.CompilerRegionsStackSource, Columns: json.RawMessage("1"), Rows: []jsonschema.GridRowInput{
		{Height: statPct, Cells: []*jsonschema.GridCellInput{stat}},
		{Height: textPct, Cells: []*jsonschema.GridCellInput{text}},
	}}
}

// A regions stack split its column by shares written without font metrics:
// two short bullets left most of a 60% share blank under them while the
// stat above stayed small, and three two-line bullets missed the same share
// by a few points and were written shrunk (go-slide-creator-18dqh, journey
// f-A12). The text region takes its measured need; the stat takes the rest,
// never less than its readable minimum.
func TestRegionsStackFollowsTheTextRegionsMeasuredNeed(t *testing.T) {
	ctx := patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	bounds := pptx.RectEmu{CX: 340 * 12700, CY: 256 * 12700}

	short := regionsStackGrid(40, 60, "Bossard leads with 9.1%", "Market grows 4% a year")
	rebalanceRegionsStack(short, ctx, bounds)
	if got := short.Rows[1].Height; got >= 50 || got < 25 {
		t.Errorf("two one-line bullets keep %.1f%% of the stack, want their measured need (about a third)", got)
	}
	if sum := short.Rows[0].Height + short.Rows[1].Height; sum < 99.9 || sum > 100.1 {
		t.Errorf("the shares sum to %.1f, want 100", sum)
	}
	measured, ok := measureRegionText(short.Rows[1].Cells[0], 340)
	if !ok {
		t.Fatal("the text region was not measured")
	}
	need := measured.needPt
	if rowPt := short.Rows[1].Height / 100 * (256 - regionsStackDefaultGapPt); rowPt < need || rowPt > need+regionsStackTextSlackPt+2 {
		t.Errorf("the text row is %.0fpt for a measured need of %.0fpt", rowPt, need)
	}

	// Journey f-A12's own bullets: three two-line items miss a 60% share by a
	// few points and take them from the stat.
	long := regionsStackGrid(40, 60,
		"Top three: Bossard 9.1%, Wuerth 7.4%, Fabory 3.8%",
		"Automotive 41%, construction 27%, machinery 22% of revenue",
		"Market grows about 4% a year; did Nordbolt keep pace?")
	rebalanceRegionsStack(long, ctx, bounds)
	if got := long.Rows[1].Height; got <= 60 || got > 65.01 {
		t.Errorf("text that needs a little more than its 60%% holds %.1f%%, want more, up to what the stat leaves (65%%)", got)
	}
	if got := long.Rows[0].Height; got < regionsStackVisualMinPct["stat-hero"]-0.01 {
		t.Errorf("the stat is left %.1f%%, under its %.0f%% minimum", got, regionsStackVisualMinPct["stat-hero"])
	}

	// Text the stat cannot make room for keeps the split its author wrote:
	// the slide holds too much, and the finding belongs on the text.
	tooLong := regionsStackGrid(40, 60,
		"Top three: Bossard 9.1%, Wuerth 7.4%, Fabory 3.8% and a long tail of regional distributors",
		"Automotive 41%, construction 27%, machinery 22% of revenue, with aerospace the only growing niche",
		"Market grows about 4% a year; did Nordbolt keep pace across each of its end markets since 2021?",
		"Pricing power is weakest where the three leaders overlap, which is two thirds of revenue")
	rebalanceRegionsStack(tooLong, ctx, bounds)
	if tooLong.Rows[0].Height != 40 || tooLong.Rows[1].Height != 60 {
		t.Errorf("a stack whose text cannot fit was re-split: %.1f / %.1f", tooLong.Rows[0].Height, tooLong.Rows[1].Height)
	}

	// Any other grid is left alone, as is a stack of two text regions.
	plain := regionsStackGrid(40, 60, "One", "Two")
	plain.Source = jsonschema.CompilerSourcePrefix + "regions"
	rebalanceRegionsStack(plain, ctx, bounds)
	if plain.Rows[0].Height != 40 || plain.Rows[1].Height != 60 {
		t.Errorf("a grid that is not a regions stack was re-split: %.1f / %.1f", plain.Rows[0].Height, plain.Rows[1].Height)
	}
	twoText := regionsStackGrid(40, 60, "One", "Two")
	twoText.Rows[0].Cells[0] = regionsStackGrid(40, 60, "Three").Rows[1].Cells[0]
	rebalanceRegionsStack(twoText, ctx, bounds)
	if twoText.Rows[0].Height != 40 || twoText.Rows[1].Height != 60 {
		t.Errorf("a stack of two text regions was re-split: %.1f / %.1f", twoText.Rows[0].Height, twoText.Rows[1].Height)
	}
}

// regionsStackSpec is journey f-A12's slide: a chart beside a stat over three
// two-line bullets, the stack split 40 / 60 by its author.
const regionsStackSpec = `{"meta":{"template":"%s","title":"Stack"},"slides":[{"kind":"regions","arrangement":"main_left",
"title":"The market adds EUR 1.2bn by 2028, but Nordbolt holds only 2.3%% of it",
"takeaway":"Growth is in the market, not yet in Nordbolt's share.",
"source":"Market model desk research 2026; Nordbolt information memorandum",
"regions":[
{"kind":"chart","size_pct":60,"heading":"European industrial fastener market","unit":"EUR bn","chart":{"type":"bar","data":{"categories":["2021","2025","2028F"],"series":[{"name":"Market size","values":[8.1,9.4,10.6]}]}}},
{"kind":"stat","size_pct":40,"value":"2.3%%","label":"Nordbolt share of the European market","context":"Number six in Europe"},
{"kind":"text","size_pct":60,"heading":"Nordbolt's position","bullets":["Top three: Bossard 9.1%%, Wuerth 7.4%%, Fabory 3.8%%","Automotive 41%%, construction 27%%, machinery 22%% of revenue","Market grows about 4%% a year; did Nordbolt keep pace?"]}]}]}`

// The same slide validated on p-style and was refused on midnight-blue, where
// the bullets were written at 11.5pt in their 60% of a shorter content area.
func TestRegionsStackTextIsNotRefusedForAShareItCanTake(t *testing.T) {
	mc := refusalTestConfig(t)
	res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": decodeSpecObject(t, fmt.Sprintf(regionsStackSpec, "midnight-blue"))}))
	if err != nil {
		t.Fatal(err)
	}
	var env deckSpecEnvelopeResponse
	structuredInto(t, res.StructuredContent, &env)
	for _, f := range env.Findings {
		if f.Code == "INPUT.TEXT_BELOW_READABLE_MIN" {
			t.Errorf("the stack's text region is still refused: %s", f.Message)
		}
	}
	if !env.OK {
		t.Errorf("validate does not approve the slide: %+v", env.Findings)
	}
}
