package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A string threshold — the natural RAG rule — used to abort the WHOLE deck at
// parse time with a Go type error that named TableCellInput, pointed at no
// path, and offered no fix (go-slide-creator-6hlu).
func TestTableCellAcceptsAnyThresholdShape(t *testing.T) {
	cases := map[string]string{
		"string":     `{"content":"On track","conditional":{"rule":"equals","threshold":"On track","fill":"accent3"}}`,
		"number":     `{"content":"95","conditional":{"rule":"gte","threshold":90,"fill":"accent6"}}`,
		"range":      `{"content":"3","conditional":{"rule":"between","threshold":[0,5],"fill":"accent4"}}`,
		"no operand": `{"content":"3","conditional":{"rule":"positive","fill":"accent6"}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var cell TableCellInput
			if err := json.Unmarshal([]byte(raw), &cell); err != nil {
				t.Fatalf("parse failed, which aborts the entire deck: %v", err)
			}
			if cell.Conditional == nil {
				t.Fatal("conditional was dropped")
			}
			if _, ok := cell.Conditional.ThresholdValue(); !ok {
				t.Error("threshold shape was rejected")
			}
		})
	}
}

// An object threshold is a shape no rule can use; it is reported rather than
// aborting the parse.
func TestTableCellObjectThresholdIsReportedNotFatal(t *testing.T) {
	var cell TableCellInput
	if err := json.Unmarshal([]byte(`{"content":"x","conditional":{"rule":"equals","threshold":{"a":1}}}`), &cell); err != nil {
		t.Fatalf("parse should survive an unusable threshold: %v", err)
	}
	if _, ok := cell.Conditional.ThresholdValue(); ok {
		t.Error("an object threshold should be reported as unusable")
	}
}

// conditionalTable builds a one-table deck input around a single cell's
// conditional block.
func conditionalTable(cell string) *TableInput {
	var table TableInput
	raw := `{"headers":["A","B"],"rows":[["x",` + cell + `]]}`
	if err := json.Unmarshal([]byte(raw), &table); err != nil {
		panic(err)
	}
	return &table
}

func TestConditionalRuleDiagnostics(t *testing.T) {
	cases := []struct {
		name     string
		cell     string
		wantPath string
		wantIn   []string
	}{
		{
			name:     "unknown rule names the vocabulary and the likely typo",
			cell:     `{"content":"On track","conditional":{"rule":"eqals","threshold":"On track","fill":"accent3"}}`,
			wantPath: "rows[0][1].conditional.rule",
			wantIn:   []string{`"eqals"`, `did you mean "equals"`, "always, positive"},
		},
		{
			name:     "a numeric rule with a text threshold",
			cell:     `{"content":"5","conditional":{"rule":"gte","threshold":"soon","fill":"accent3"}}`,
			wantPath: "rows[0][1].conditional.threshold",
			wantIn:   []string{"compares numbers", `"soon"`},
		},
		{
			name:     "between without a pair",
			cell:     `{"content":"5","conditional":{"rule":"between","threshold":3,"fill":"accent3"}}`,
			wantPath: "rows[0][1].conditional.threshold",
			wantIn:   []string{"two-number range"},
		},
		{
			name:     "contains with no threshold",
			cell:     `{"content":"5","conditional":{"rule":"contains","fill":"accent3"}}`,
			wantPath: "rows[0][1].conditional.threshold",
			wantIn:   []string{"compare the cell against"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := conditionalRuleDiagnostics(conditionalTable(tc.cell), "/slides/0/content/1", 0)
			if len(diags) != 1 {
				t.Fatalf("expected 1 diagnostic, got %d (%+v)", len(diags), diags)
			}
			d := diags[0]
			if !strings.HasSuffix(d.Path, tc.wantPath) {
				t.Errorf("path = %q, want it to end in %q", d.Path, tc.wantPath)
			}
			for _, want := range tc.wantIn {
				if !strings.Contains(d.Message, want) {
					t.Errorf("message %q does not contain %q", d.Message, want)
				}
			}
			if d.Fix == nil {
				t.Error("a diagnostic an agent must act on carries no fix")
			}
		})
	}
}

// The rules an author actually writes draw nothing.
func TestConditionalRuleDiagnosticsAcceptValidRules(t *testing.T) {
	valid := []string{
		`{"content":"On track","conditional":{"rule":"equals","threshold":"On track","fill":"accent3"}}`,
		`{"content":"On track","conditional":{"rule":"contains","threshold":"track","fill":"accent3"}}`,
		`{"content":"95","conditional":{"rule":"gte","threshold":90,"fill":"accent6"}}`,
		`{"content":"95","conditional":{"rule":"gte","threshold":"90","fill":"accent6"}}`,
		`{"content":"3","conditional":{"rule":"between","threshold":[0,5],"fill":"accent4"}}`,
		`{"content":"+4%","conditional":{"rule":"positive","fill":"accent6"}}`,
		`{"content":"highlighted","conditional":{"fill":"accent3"}}`,
	}
	for _, cell := range valid {
		if d := conditionalRuleDiagnostics(conditionalTable(cell), "/slides/0/content/1", 0); len(d) != 0 {
			t.Errorf("%s drew %+v", cell, d)
		}
	}
}
