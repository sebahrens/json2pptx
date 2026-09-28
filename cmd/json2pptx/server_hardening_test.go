package main

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/config"
)

// go-slide-creator-tcxsq: config allows read_timeout: 0, which used to leave
// header reads unbounded (slowloris, gosec G112).
func TestAPIServerAlwaysBoundsHeaderReads(t *testing.T) {
	srv := newAPIHTTPServer(config.ServerConfig{Port: 8080}, http.NotFoundHandler())
	if srv.ReadHeaderTimeout <= 0 || srv.ReadHeaderTimeout > 30*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v with read_timeout 0, want a bounded header timeout", srv.ReadHeaderTimeout)
	}
}

// outputPathLocks used to keep one mutex per distinct path forever.
func TestOutputPathLocksAreReleased(t *testing.T) {
	before := outputPathLockCount()
	dir := t.TempDir()
	for i := 0; i < 100; i++ {
		lockOutputPath(filepath.Join(dir, fmt.Sprintf("deck-%d.pptx", i)))()
	}
	if got := outputPathLockCount(); got != before {
		t.Fatalf("lock entries grew from %d to %d after 100 distinct paths", before, got)
	}

	// Contended path: the entry survives while anyone holds or waits, and
	// the holders are still serialized.
	path := filepath.Join(dir, "shared.pptx")
	var mu sync.Mutex
	inside := 0
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := lockOutputPath(path)
			mu.Lock()
			inside++
			if inside != 1 {
				t.Errorf("%d holders inside the same output-path lock", inside)
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
			unlock()
		}()
	}
	wg.Wait()
	if got := outputPathLockCount(); got != before {
		t.Fatalf("lock entries = %d after contended use, want %d", got, before)
	}
}

// deterministicGates used to be a never-evicting sync.Map. It is bounded,
// and a full store sheds passing gates before blocking ones: losing a
// blocking record would let a deck with P0 content be called complete.
func TestDeterministicGatesAreBounded(t *testing.T) {
	g := &deterministicGates
	g.mu.Lock()
	saved, savedNow := g.entries, g.now
	g.entries = make(map[string]deterministicGateEntry)
	clock := time.Unix(1_700_000_000, 0)
	g.now = func() time.Time { return clock }
	g.mu.Unlock()
	t.Cleanup(func() {
		g.mu.Lock()
		g.entries, g.now = saved, savedNow
		g.mu.Unlock()
	})

	storeDeterministicGate("blocked", []string{"P0 content"})
	for i := 0; i < maxDeterministicGates*2; i++ {
		clock = clock.Add(time.Second)
		storeDeterministicGate(fmt.Sprintf("pass-%d", i), nil)
	}
	g.mu.Lock()
	n := len(g.entries)
	g.mu.Unlock()
	if n > maxDeterministicGates {
		t.Fatalf("gate store holds %d entries, cap is %d", n, maxDeterministicGates)
	}
	if reasons, known := lookupDeterministicGate("blocked"); !known || len(reasons) != 1 {
		t.Fatal("a blocking gate was evicted while passing gates remained")
	}
	if _, known := lookupDeterministicGate(fmt.Sprintf("pass-%d", maxDeterministicGates*2-1)); !known {
		t.Fatal("the newest gate is missing")
	}

	clock = clock.Add(deterministicGateTTL + time.Minute)
	if _, known := lookupDeterministicGate("blocked"); known {
		t.Fatal("an expired gate is still reported")
	}
}

// Malformed optional structured arguments used to be dropped by
// `_ = json.Unmarshal`, so the tool silently answered a different question.
func TestMalformedOptionalArgsAreInvalidParameter(t *testing.T) {
	mc := testMCPConfig(t)
	cases := []struct {
		tool string
		call func(map[string]any) (string, bool)
		args map[string]any
		name string
	}{
		{"recommend_visual", func(a map[string]any) (string, bool) {
			r, _ := mc.handleRecommendVisual(context.Background(), makeRequest(a))
			return textContent(r), r.IsError
		}, map[string]any{"intent": "show three KPIs", "candidates": "kpi-3up"}, "candidates"},
		{"recommend_visual", func(a map[string]any) (string, bool) {
			r, _ := mc.handleRecommendVisual(context.Background(), makeRequest(a))
			return textContent(r), r.IsError
		}, map[string]any{"intent": "show three KPIs", "content_hints": map[string]any{"item_count": "three"}}, "content_hints"},
		{"recommend_pattern", func(a map[string]any) (string, bool) {
			r, _ := mc.handleRecommendPattern(context.Background(), makeRequest(a))
			return textContent(r), r.IsError
		}, map[string]any{"intent": "show three KPIs", "recent_patterns": 7}, "recent_patterns"},
		{"plan_deck", func(a map[string]any) (string, bool) {
			r, _ := mc.handlePlanDeck(context.Background(), makeRequest(a))
			return textContent(r), r.IsError
		}, map[string]any{"brief": "Quarterly business review for the board", "must_include": map[string]any{"a": 1}}, "must_include"},
	}
	for _, tc := range cases {
		text, isErr := tc.call(tc.args)
		if !isErr || !strings.Contains(text, "INVALID_PARAMETER") || !strings.Contains(text, tc.name) {
			t.Errorf("%s with malformed %s: want INVALID_PARAMETER naming it, got isError=%v %.400s", tc.tool, tc.name, isErr, text)
		}
	}
}
