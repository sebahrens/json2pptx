package semantic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-oh3qr: meta.waivers names storyline findings a deck waives
// on purpose, each with a reason. It is parsed from YAML and JSON, carried on
// the IR, described by the schema, and validated: only storyline codes, one
// waiver per code, and never without a reason.
func TestMetaWaiversParseValidateAndReachTheIR(t *testing.T) {
	yamlSpec := `meta:
  title: Seed round
  archetype: sales_pitch
  waivers:
    - code: CLOSING_WITHOUT_NEXT_STEPS
      reason: The brief fixes seven slides; the ask is made verbally.
slides:
  - kind: title
    title: Seed round
`
	spec, diags := Parse("deck.yaml", []byte(yamlSpec))
	if diags.HasErrors() {
		t.Fatalf("parse: %+v", diags)
	}
	if len(spec.Meta.Waivers) != 1 || spec.Meta.Waivers[0].Code != "CLOSING_WITHOUT_NEXT_STEPS" ||
		!strings.HasPrefix(spec.Meta.Waivers[0].Reason, "The brief fixes") {
		t.Fatalf("waivers = %+v", spec.Meta.Waivers)
	}
	for _, d := range Validate(spec, StrictnessStrict) {
		if strings.HasPrefix(d.Path, "meta.waivers") {
			t.Errorf("valid waiver rejected: %+v", d)
		}
	}
	ir := Normalize(spec)
	if len(ir.Waivers) != 1 || ir.Waivers[0] != spec.Meta.Waivers[0] {
		t.Errorf("IR waivers = %+v", ir.Waivers)
	}
	if ir.Executive {
		t.Error("sales_pitch must not be an executive archetype")
	}

	jsonSpec := `{"meta":{"title":"Seed round","waivers":[{"code":"NO_EXECUTIVE_SUMMARY","reason":"Pitch format."}]},"slides":[{"kind":"title","title":"Seed round"}]}`
	if spec, diags = Parse("deck.json", []byte(jsonSpec)); diags.HasErrors() || len(spec.Meta.Waivers) != 1 {
		t.Fatalf("JSON waivers: %+v %+v", spec, diags)
	}
}

func TestMetaWaiversRejectWhatCannotBeWaived(t *testing.T) {
	cases := map[string]struct {
		waivers  []FindingWaiver
		wantPath string
		wantText string
	}{
		"fit finding": {
			waivers:  []FindingWaiver{{Code: "BODY_TOO_LONG", Reason: "We like long text."}},
			wantPath: "meta.waivers[0].code", wantText: "cannot be waived",
		},
		"unknown code": {
			waivers:  []FindingWaiver{{Code: "NOT_A_CODE", Reason: "Because."}},
			wantPath: "meta.waivers[0].code", wantText: "NO_EXECUTIVE_SUMMARY",
		},
		"no reason": {
			waivers:  []FindingWaiver{{Code: "NO_EXECUTIVE_SUMMARY", Reason: "  "}},
			wantPath: "meta.waivers[0].reason", wantText: "needs a reason",
		},
		"duplicate": {
			waivers: []FindingWaiver{
				{Code: "TITLE_NOT_ACTION", Reason: "Reference deck."},
				{Code: "TITLE_NOT_ACTION", Reason: "Reference deck, again."},
			},
			wantPath: "meta.waivers[1].code", wantText: "more than once",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			spec := &DeckSpec{
				Meta:   DeckMeta{Title: "Deck", Waivers: c.waivers},
				Slides: []SlideSpec{{Kind: KindTitle, Body: map[string]any{"title": "Deck"}}},
			}
			found := false
			for _, d := range Validate(spec, StrictnessWarn) {
				if d.Path == c.wantPath && d.Severity == diagnostics.SeverityError && strings.Contains(d.Message, c.wantText) {
					found = true
				}
			}
			if !found {
				t.Errorf("no blocking finding at %s containing %q: %+v", c.wantPath, c.wantText, Validate(spec, StrictnessWarn))
			}
		})
	}
}

// The waivable codes are exactly the storyline rules, and the schema's enum is
// derived from the same list.
func TestWaivableCodesAreStorylineRulesAndInTheSchema(t *testing.T) {
	want := []string{"CLOSING_WITHOUT_NEXT_STEPS", "NO_EXECUTIVE_SUMMARY", "TITLE_NOT_ACTION", "takeaway_missing"}
	got := WaivableFindingCodes()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("waivable codes = %v, want %v", got, want)
	}
	if code, ok := CanonicalWaivableCode(" TAKEAWAY_MISSING "); !ok || code != "takeaway_missing" {
		t.Errorf("waiver codes must match without regard to case: %q %v", code, ok)
	}
	for _, code := range []string{"BODY_TOO_LONG", "TEXT_BELOW_READABLE_MIN", "SLIDE_NEARLY_EMPTY", ""} {
		if IsWaivableFindingCode(code) {
			t.Errorf("%q must not be waivable", code)
		}
	}
	raw, err := json.Marshal(deckMetaSchema())
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Items struct {
				Required   []string `json:"required"`
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	waivers := schema.Properties["waivers"].Items
	if strings.Join(waivers.Properties["code"].Enum, ",") != strings.Join(want, ",") {
		t.Errorf("schema waiver code enum = %v", waivers.Properties["code"].Enum)
	}
	if strings.Join(waivers.Required, ",") != "code,reason" {
		t.Errorf("schema waiver required = %v", waivers.Required)
	}
}
