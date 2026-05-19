package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestPrintDoubleDashUsage_LongFlag verifies long flag names render with the
// canonical -- prefix, matching the standardized GNU style.
func TestPrintDoubleDashUsage_LongFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("templates-dir", "./templates", "Directory containing templates")
	fs.Bool("dry-run", false, "Validate without generating output")

	var buf bytes.Buffer
	fs.SetOutput(&buf)
	printDoubleDashUsage(fs)
	got := buf.String()

	for _, want := range []string{"--templates-dir", "--dry-run", "Directory containing templates", `(default "./templates")`} {
		if !strings.Contains(got, want) {
			t.Errorf("usage output missing %q\n--- output ---\n%s", want, got)
		}
	}
	// Long flags must not appear with single-dash leading form in the help text.
	if strings.Contains(got, "  -templates-dir") {
		t.Errorf("expected double-dash prefix for long flag, got:\n%s", got)
	}
}

// TestPrintDoubleDashUsage_ShortFlag verifies single-letter flags keep the
// single-dash prefix (GNU short-flag convention).
func TestPrintDoubleDashUsage_ShortFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("n", false, "Shorthand for --dry-run")

	var buf bytes.Buffer
	fs.SetOutput(&buf)
	printDoubleDashUsage(fs)
	got := buf.String()

	if !strings.Contains(got, "-n") {
		t.Errorf("expected -n in output, got:\n%s", got)
	}
	if strings.Contains(got, "--n\t") || strings.Contains(got, "--n ") {
		t.Errorf("single-letter flag should keep -x form, got:\n%s", got)
	}
}

// TestCLIAcceptsBothFlagStyles is the back-compat guarantee: every long flag
// must parse whether the caller writes --flag (canonical) or -flag (legacy).
// We exercise this via a real CLI invocation against `resolve-theme`, which
// touches templates-dir + template + variation in a known-error path.
func TestCLIAcceptsBothFlagStyles(t *testing.T) {
	binary := buildTestBinary(t)

	cases := []struct {
		name string
		args []string
	}{
		{
			name: "double-dash",
			args: []string{"resolve-theme", "--template", "midnight-blue", "--templates-dir", "../../templates", "--variation", "no-such-variation"},
		},
		{
			name: "single-dash-legacy",
			args: []string{"resolve-theme", "-template", "midnight-blue", "-templates-dir", "../../templates", "-variation", "no-such-variation"},
		},
		{
			name: "mixed",
			args: []string{"resolve-theme", "--template", "midnight-blue", "-templates-dir", "../../templates", "--variation", "no-such-variation"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binary, tc.args...) //nolint:gosec
			cmd.Env = append(os.Environ(), "HOME=/tmp")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected error for unknown variation, got success: %s", string(out))
			}
			// Both flag styles must reach the variation handler (which then
			// rejects the unknown name). If parsing failed, we'd see "flag
			// provided but not defined" or similar instead.
			if !strings.Contains(string(out), "unknown variation") {
				t.Errorf("expected 'unknown variation' in error (proves flag parsed), got: %s", string(out))
			}
		})
	}
}
