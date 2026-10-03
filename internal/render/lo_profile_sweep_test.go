package render

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A profile whose process is gone is removed; one whose process is running,
// one that is still fresh, this process's own, and unrelated directories stay
// (go-slide-creator-12dkn).
func TestSweepStaleLibreOfficeProfiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.Add(-2 * time.Hour)
	ancient := now.Add(-48 * time.Hour)

	mk := func(name string, mod time.Time) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Join(p, "user"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{filepath.Join(p, "user"), p} {
			if err := os.Chtimes(q, mod, mod); err != nil {
				t.Fatal(err)
			}
		}
		return p
	}

	const deadPID, livePID = 111111, 222222
	dead := mk(fmt.Sprintf("json2pptx-lo-%d-aaa", deadPID), old)
	live := mk(fmt.Sprintf("json2pptx-lo-%d-bbb", livePID), old)
	liveAncient := mk(fmt.Sprintf("json2pptx-lo-%d-ccc", livePID), ancient)
	fresh := mk(fmt.Sprintf("json2pptx-lo-%d-ddd", deadPID), now)
	own := mk(fmt.Sprintf("json2pptx-lo-%d-eee", os.Getpid()), old)
	retry := mk("json2pptx-lo-retry-fff", old)
	other := mk("json2pptx-render-cache", old)

	alive := func(pid int) bool { return pid == livePID || pid == os.Getpid() }
	removed, err := sweepStaleProfiles(dir, now, alive)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Errorf("removed %d profiles, want 3 (dead owner, abandoned retry, live pid past the orphan age)", removed)
	}
	for _, p := range []string{dead, liveAncient, retry} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", filepath.Base(p))
		}
	}
	for _, p := range []string{live, fresh, own, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should have been kept: %v", filepath.Base(p), err)
		}
	}
}

func TestSweepStaleLibreOfficeProfilesMissingDir(t *testing.T) {
	removed, err := SweepStaleLibreOfficeProfiles(filepath.Join(t.TempDir(), "absent"))
	if err != nil || removed != 0 {
		t.Errorf("missing dir: removed=%d err=%v, want 0, nil", removed, err)
	}
}

func TestProfilePID(t *testing.T) {
	cases := []struct {
		name string
		pid  int
		ok   bool
	}{
		{"json2pptx-lo-4242-123456", 4242, true},
		{"json2pptx-lo-retry-123456", 0, false},
		{"json2pptx-lo-abc-1", 0, false},
		{"json2pptx-lo-77", 0, false},
	}
	for _, c := range cases {
		pid, ok := profilePID(c.name)
		if pid != c.pid || ok != c.ok {
			t.Errorf("profilePID(%q) = %d, %v; want %d, %v", c.name, pid, ok, c.pid, c.ok)
		}
	}
}
