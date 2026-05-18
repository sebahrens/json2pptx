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
