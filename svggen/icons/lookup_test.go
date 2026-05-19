package icons

import (
	"strings"
	"testing"
)

func TestLookup_OutlineDefault(t *testing.T) {
	data, err := Lookup("chart-pie")
	if err != nil {
		t.Fatalf("Lookup(chart-pie) error: %v", err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Error("expected SVG content")
	}
}

func TestLookup_OutlineExplicit(t *testing.T) {
	data, err := Lookup("outline:chart-pie")
	if err != nil {
		t.Fatalf("Lookup(outline:chart-pie) error: %v", err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Error("expected SVG content")
	}
}

func TestLookup_Filled(t *testing.T) {
	data, err := Lookup("filled:chart-pie")
	if err != nil {
		t.Fatalf("Lookup(filled:chart-pie) error: %v", err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Error("expected SVG content")
	}
}

func TestLookup_NotFound(t *testing.T) {
	_, err := Lookup("nonexistent-icon-xyz")
	if err == nil {
		t.Error("expected error for nonexistent icon")
	}
}

func TestExists(t *testing.T) {
	if !Exists("chart-pie") {
		t.Error("expected chart-pie to exist")
	}
	if !Exists("filled:chart-pie") {
		t.Error("expected filled:chart-pie to exist")
	}
	if Exists("nonexistent-icon-xyz") {
		t.Error("expected nonexistent icon to not exist")
	}
}

func TestList_Outline(t *testing.T) {
	names, err := List("outline")
	if err != nil {
		t.Fatalf("List(outline) error: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected non-empty outline icon list")
	}
	// Spot-check a known icon
	found := false
	for _, n := range names {
		if n == "chart-pie" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected chart-pie in outline list")
	}
}

func TestList_Filled(t *testing.T) {
	names, err := List("filled")
	if err != nil {
		t.Fatalf("List(filled) error: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected non-empty filled icon list")
	}
}

func TestList_InvalidSet(t *testing.T) {
	_, err := List("bogus")
	if err == nil {
		t.Error("expected error for invalid set")
	}
}

func TestSuggest_TypoBareName(t *testing.T) {
	got := Suggest("chart-pi", 3)
	if len(got) == 0 {
		t.Fatal("expected suggestions for typo'd bare name")
	}
	if got[0] != "chart-pie" {
		t.Errorf("expected first suggestion 'chart-pie' (bare, distance 1), got %q (full list %v)", got[0], got)
	}
}

func TestSuggest_QualifiedFilledTypo(t *testing.T) {
	got := Suggest("filled:chart-pi", 3)
	if len(got) == 0 {
		t.Fatal("expected suggestions for qualified typo")
	}
	if got[0] != "filled:chart-pie" {
		t.Errorf("expected first suggestion 'filled:chart-pie', got %q (full list %v)", got[0], got)
	}
}

func TestSuggest_UnknownSetPrefix(t *testing.T) {
	// 'filed' is a typo of 'filled' — the set prefix is unrecognized. Suggest
	// should still find the closest match across both sets, qualified.
	got := Suggest("filed:rocket", 5)
	if len(got) == 0 {
		t.Fatal("expected suggestions for unknown set prefix")
	}
	if !strings.Contains(got[0], "rocket") {
		t.Errorf("expected first suggestion to contain 'rocket', got %q (full list %v)", got[0], got)
	}
	if !strings.Contains(got[0], ":") {
		t.Errorf("expected qualified suggestion (set:name), got bare %q", got[0])
	}
}

func TestSuggest_NoCandidatesWhenFarFromAnything(t *testing.T) {
	got := Suggest("xyzzy-no-icon-name-like-this-exists", 3)
	if got != nil {
		t.Errorf("expected nil suggestions for far-off input, got %v", got)
	}
}

func TestSuggest_BoundedByMaxResults(t *testing.T) {
	got := Suggest("a", 2)
	if len(got) > 2 {
		t.Errorf("expected at most 2 suggestions, got %d: %v", len(got), got)
	}
}

func TestParseName(t *testing.T) {
	tests := []struct {
		input    string
		wantSet  string
		wantBase string
	}{
		{"chart-pie", "outline", "chart-pie"},
		{"outline:chart-pie", "outline", "chart-pie"},
		{"filled:chart-pie", "filled", "chart-pie"},
		{"  filled:alert  ", "filled", "alert"},
	}
	for _, tt := range tests {
		set, base := parseName(tt.input)
		if set != tt.wantSet || base != tt.wantBase {
			t.Errorf("parseName(%q) = (%q, %q), want (%q, %q)",
				tt.input, set, base, tt.wantSet, tt.wantBase)
		}
	}
}
