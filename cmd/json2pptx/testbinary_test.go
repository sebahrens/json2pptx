package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// The package's CLI tests shell out to a built json2pptx binary. Building it
// per test cost a full `go build` each time (17 call sites), which added up
// under -race in the CI coverage job (go-slide-creator-s2s53). Build it once
// per test process and share it; tests only execute the binary, never modify
// it.
var (
	sharedBinaryOnce sync.Once
	sharedBinaryDir  string
	sharedBinaryPath string
	sharedBinaryErr  error
	sharedBinaryOut  []byte
)

func TestMain(m *testing.M) {
	// The developer's own installed skill must not put a line on a test's
	// stderr; the skill tests set their own directory.
	_ = os.Setenv(skillCheckEnv, "off")
	flag.Parse()
	if flag.Lookup("test.list").Value.String() == "" && needsExemplarSetup(flag.Lookup("test.run").Value.String()) {
		started := time.Now()
		names := []string{"business-template", "midnight-blue"}
		if testing.Short() {
			names = []string{"business-template", "modern-template"}
		}
		for _, name := range names {
			if _, err := cachedExemplarFindings(name); err != nil {
				fmt.Fprintf(os.Stderr, "exemplar fixture setup: %v\n", err)
				os.Exit(1)
			}
		}
		fmt.Fprintf(os.Stderr, "exemplar fixture setup: %s\n", time.Since(started))
	}
	code := m.Run()
	if sharedBinaryDir != "" {
		_ = os.RemoveAll(sharedBinaryDir)
	}
	os.Exit(code)
}

// sharedTestBinary returns the path of a json2pptx binary built from this
// package, building it on first use.
func sharedTestBinary(t *testing.T) string {
	t.Helper()
	sharedBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "json2pptx-testbin-")
		if err != nil {
			sharedBinaryErr = err
			return
		}
		sharedBinaryDir = dir
		bin := filepath.Join(dir, "json2pptx")
		cmd := exec.Command("go", "build", "-o", bin, ".") //nolint:gosec // controlled args
		cmd.Dir = "."
		sharedBinaryOut, sharedBinaryErr = cmd.CombinedOutput()
		sharedBinaryPath = bin
	})
	if sharedBinaryErr != nil {
		t.Fatalf("failed to build test binary: %v\n%s", sharedBinaryErr, sharedBinaryOut)
	}
	return sharedBinaryPath
}

// Charge shared template/font/exemplar warm-up to process setup, before test
// timing starts. Focused unrelated tests and -list must not pay this cost.
func needsExemplarSetup(pattern string) bool {
	top := strings.Split(pattern, "/")[0]
	re, err := regexp.Compile(top)
	if err != nil {
		return false
	} // testing reports the invalid selection itself.
	for _, name := range []string{"TestExemplarDeckUsesTheContentArea", "TestExemplarsClearTheUnderusedThreshold", "TestShortReducedMatricesCorpus"} {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

func TestExemplarSetupSelection(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		want    bool
	}{
		{"", true}, {"^TestExemplarDeckUsesTheContentArea$", true}, {"^TestExemplarsClearTheUnderusedThreshold$/business-template", true},
		{"^TestSomethingElse$", false}, {"[", false},
	} {
		if got := needsExemplarSetup(tc.pattern); got != tc.want {
			t.Errorf("selection %q=%v want %v", tc.pattern, got, tc.want)
		}
	}
}
