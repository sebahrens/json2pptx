package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
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
