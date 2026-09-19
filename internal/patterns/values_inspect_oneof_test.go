package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every pattern whose values are a list of cells declares the element as
// oneOf{string shorthand, object}, and the property names live in the object
// branch. The inspector asked the oneOf itself, got no properties, and read
// that as "free-form content — nothing here can be unknown", so a typo INSIDE a
// cell was dropped in silence: the agent that wrote {"bodySize": 9} was told
// the deck was valid and got template defaults (go-slide-creator-4cqh).
func TestInspectReportsUnknownFieldsInsideOneOfElements(t *testing.T) {
	cases := []struct {
		pattern  string
		values   string
		wantPath string
		wantIn   []string
	}{
		{
			pattern:  "kpi-3up",
			values:   `[{"big":"$4.2M","small":"ARR","bogus":"z"},{"big":"117%","small":"NRR"},{"big":"6.2%","small":"Churn"}]`,
			wantPath: "values[0].bogus",
			wantIn:   []string{`"big"`, `"small"`},
		},
		{
			pattern:  "card-grid",
			values:   `{"columns":2,"rows":1,"cells":[{"header":"A","body":"a","bodySize":9},{"header":"B","body":"b"}]}`,
			wantPath: "values.cells[0].bodySize",
			wantIn:   []string{`"body"`, `"header"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			pat, ok := Default().Get(tc.pattern)
			if !ok {
				t.Fatalf("%s is not registered", tc.pattern)
			}
			found := InspectPatternInput(pat, json.RawMessage(tc.values), nil, nil)
			if len(found) != 1 {
				t.Fatalf("expected 1 finding, got %d (%v)", len(found), found)
			}
			if found[0].Path != tc.wantPath {
				t.Errorf("path = %q, want %q", found[0].Path, tc.wantPath)
			}
			if found[0].Code != ErrCodePatternUnknownField {
				t.Errorf("code = %q, want %q", found[0].Code, ErrCodePatternUnknownField)
			}
			// The message must name what the element DOES read, so the agent can
			// pick the right key without opening the schema.
			for _, want := range tc.wantIn {
				if !strings.Contains(found[0].Message, want) {
					t.Errorf("message %q does not name %s", found[0].Message, want)
				}
			}
		})
	}
}

// The shorthands and aliases a pattern tolerates must still pass: they survive
// the decode round trip, so nothing was dropped.
func TestInspectKeepsQuietOnToleratedForms(t *testing.T) {
	pat, _ := Default().Get("kpi-3up")
	quiet := []string{
		`[{"big":"$4.2M","small":"ARR"},{"big":"117%","small":"NRR"},{"big":"6.2%","small":"Churn"}]`,
		`[{"value":"$4.2M","label":"ARR"},"117% | NRR",{"big":"6.2%","small":"Churn"}]`,
		`["$4.2M | ARR","117% | NRR","6.2% | Churn"]`,
	}
	for _, values := range quiet {
		if found := InspectPatternInput(pat, json.RawMessage(values), nil, nil); len(found) != 0 {
			t.Errorf("%s drew %v", values, found)
		}
	}
}

// branchFor is the whole rule; pin it directly.
func TestBranchFor(t *testing.T) {
	pat, _ := Default().Get("kpi-3up")
	root := pat.Schema()
	item := sectionSchema(root, "values").Deref(root).ItemSchema().Deref(root)
	if len(item.OneOfBranches()) == 0 {
		t.Fatal("expected the KPI cell to be a oneOf")
	}
	if got := branchFor(item, map[string]any{"big": "x"}, root); len(got.PropertyNames()) == 0 {
		t.Error("object node resolved to a branch with no properties")
	}
	if got := branchFor(item, "shorthand", root); got.TypeName() != TypeString {
		t.Errorf("string node resolved to %q, want the string branch", got.TypeName())
	}
	// A schema with no branches is returned unchanged, and nil stays nil.
	plain := StringSchema(0)
	if branchFor(plain, "x", root) != plain {
		t.Error("a non-oneOf schema must be returned unchanged")
	}
	if branchFor(nil, "x", root) != nil {
		t.Error("nil schema must stay nil")
	}
}
