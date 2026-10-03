package render

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Stale LibreOffice profile sweep (go-slide-creator-12dkn).
//
// Each process that renders creates one private LibreOffice profile,
// json2pptx-lo-<pid>-<random>, and removes it at exit
// (CleanupLibreOfficeProfile). A process that is killed, crashes, or is a test
// binary never reaches that call, so its profile stays: one machine had 2,465
// of them (617 MB) in TMPDIR. Nothing ever looked at another process's
// profile, so the directory only grew.
//
// The first render of a process now also removes the profiles of processes
// that are gone. A profile is stale when the process whose pid is in its name
// is no longer running and the directory has not been touched for
// staleProfileMinAge; the age guard keeps a profile whose owner is still
// starting up, and covers platforms where liveness cannot be probed.

const (
	loProfilePrefix      = "json2pptx-lo-"
	loRetryProfilePrefix = "json2pptx-lo-retry-"
	// staleProfileMinAge is how long a profile must have been untouched before
	// it can be swept. LibreOffice rewrites files in its profile on every
	// conversion, so a profile in use is never this old.
	staleProfileMinAge = 30 * time.Minute
	// staleProfileOrphanAge is the age past which a profile is removed even
	// when liveness cannot be established (pid reuse, no probe on this OS).
	staleProfileOrphanAge = 24 * time.Hour
)

var staleProfileSweepOnce sync.Once

// sweepStaleProfilesOnce runs the stale-profile sweep at most once per
// process, in the background: it must not delay the render that triggered it.
func sweepStaleProfilesOnce() {
	staleProfileSweepOnce.Do(func() {
		go func() { _, _ = SweepStaleLibreOfficeProfiles(os.TempDir()) }()
	})
}

// SweepStaleLibreOfficeProfiles removes the private LibreOffice profiles in
// dir that belong to processes which are no longer running. It returns how
// many directories it removed. The calling process's own profile is never
// touched.
func SweepStaleLibreOfficeProfiles(dir string) (int, error) {
	return sweepStaleProfiles(dir, time.Now(), processAlive)
}

func sweepStaleProfiles(dir string, now time.Time, alive func(pid int) bool) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	self := os.Getpid()
	removed := 0
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !strings.HasPrefix(name, loProfilePrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		age := now.Sub(newestModTime(filepath.Join(dir, name), info.ModTime()))
		if age < staleProfileMinAge {
			continue
		}
		pid, hasPID := profilePID(name)
		if hasPID && pid == self {
			continue
		}
		// A retry profile carries no pid and is removed by the call that made
		// it; one still here after the minimum age was abandoned.
		if hasPID && age < staleProfileOrphanAge && alive(pid) {
			continue
		}
		if os.RemoveAll(filepath.Join(dir, name)) == nil {
			removed++
		}
	}
	return removed, nil
}

// profilePID extracts the owning pid from a profile directory name
// (json2pptx-lo-<pid>-<random>). Retry profiles (json2pptx-lo-retry-<random>)
// have none.
func profilePID(name string) (int, bool) {
	if strings.HasPrefix(name, loRetryProfilePrefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(name, loProfilePrefix)
	pidPart, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	pid, err := strconv.Atoi(pidPart)
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// newestModTime returns the most recent modification time among a profile
// directory and its immediate children. LibreOffice writes below the top
// level, which does not bump the top-level mtime.
func newestModTime(path string, top time.Time) time.Time {
	newest := top
	entries, err := os.ReadDir(path)
	if err != nil {
		return newest
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}
