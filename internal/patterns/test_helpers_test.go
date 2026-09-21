package patterns

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type invalidShapeCase struct{ name, input string }

func assertInvalidShapeErrors(t *testing.T, tests []invalidShapeCase, example string, decode func(string) error) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decode(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T: %v", err, err)
			}
			if ve.Code != ErrCodeInvalidShape || ve.Fix == nil || ve.Fix.Kind != "reshape_value" {
				t.Fatalf("unexpected validation error contract: %+v", ve)
			}
			if ve.Fix.Params["example"] != example {
				t.Errorf("Fix.Params[example] = %v, want %q", ve.Fix.Params["example"], example)
			}
		})
	}
}

func assertExpandEquivalent(t *testing.T, firstJSON, secondJSON string, expand func(string) (any, error)) {
	t.Helper()
	first, err := expand(firstJSON)
	if err != nil {
		t.Fatalf("expand first form: %v", err)
	}
	second, err := expand(secondJSON)
	if err != nil {
		t.Fatalf("expand second form: %v", err)
	}
	firstOut, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first form: %v", err)
	}
	secondOut, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second form: %v", err)
	}
	if string(firstOut) != string(secondOut) {
		t.Errorf("expand outputs differ.\nfirst: %s\nsecond: %s", firstOut, secondOut)
	}
}

func assertPatternGolden(t *testing.T, value any, goldenPath string) {
	t.Helper()
	got, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden value: %v", err)
	}
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Log("golden file updated")
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("golden mismatch.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func assertDenseGridWarnings(t *testing.T, atBudget, overBudget, sparse, nilValues []string, wantParts ...string) {
	t.Helper()
	if len(atBudget) != 0 {
		t.Fatalf("at budget: %v", atBudget)
	}
	if len(overBudget) != 1 {
		t.Fatalf("over budget warnings = %v, want one", overBudget)
	}
	for _, want := range wantParts {
		if !strings.Contains(overBudget[0], want) {
			t.Errorf("warning %q does not contain %q", overBudget[0], want)
		}
	}
	if len(sparse) != 0 {
		t.Fatalf("sparse grid at schema maximum warned: %v", sparse)
	}
	if nilValues != nil {
		t.Fatalf("nil values: %v", nilValues)
	}
}
