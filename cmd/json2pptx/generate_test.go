package main

import (
	"os"
	"strings"
	"testing"
)

// TestGenerateJSONOutputFlagAliases verifies that --json-output-report is a
// non-breaking alias for the deprecated --json-output flag: both names resolve
// to the same JSON result report destination, and setting them to conflicting
// values is rejected with a clear error.
func TestGenerateJSONOutputFlagAliases(t *testing.T) {
	t.Run("resolve", func(t *testing.T) {
		cases := []struct {
			name    string
			oldVal  string
			newVal  string
			oldSet  bool
			newSet  bool
			want    string
			wantErr bool
		}{
			{name: "neither set", want: ""},
			{name: "deprecated only", oldVal: "a.json", oldSet: true, want: "a.json"},
			{name: "preferred only", newVal: "b.json", newSet: true, want: "b.json"},
			{
				name:   "both set identical",
				oldVal: "same.json", newVal: "same.json",
				oldSet: true, newSet: true,
				want: "same.json",
			},
			{
				name:   "both set conflicting",
				oldVal: "a.json", newVal: "b.json",
				oldSet: true, newSet: true,
				wantErr: true,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := resolveJSONOutputReport(tc.oldVal, tc.newVal, tc.oldSet, tc.newSet)
				if tc.wantErr {
					if err == nil {
						t.Fatalf("expected error, got nil (value %q)", got)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Errorf("resolved = %q, want %q", got, tc.want)
				}
			})
		}
	})

	// Drive the real flag wiring in runGenerate: conflicting values for the two
	// aliases must error out before any generation work begins.
	t.Run("conflict via CLI args", func(t *testing.T) {
		saved := os.Args
		defer func() { os.Args = saved }()
		os.Args = []string{
			"json2pptx",
			"--json", "input.json",
			"--json-output", "a.json",
			"--json-output-report", "b.json",
		}
		err := runGenerate()
		if err == nil {
			t.Fatal("expected conflict error, got nil")
		}
		if !strings.Contains(err.Error(), "conflicting values") {
			t.Errorf("error %q does not mention conflicting values", err.Error())
		}
	})
}
