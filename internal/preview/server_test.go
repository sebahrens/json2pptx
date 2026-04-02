package preview

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewServer_Defaults(t *testing.T) {
	s := NewServer(ServerConfig{File: "test.md"})
	if s.cfg.Port != 3333 {
		t.Errorf("default port = %d, want 3333", s.cfg.Port)
	}
	if s.cfg.DPI != 192 {
		t.Errorf("default DPI = %f, want 192", s.cfg.DPI)
	}
}

func TestHandleIndex(t *testing.T) {
	s := NewServer(ServerConfig{File: "/tmp/deck.md"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	s.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Slide Preview") {
		t.Error("response missing 'Slide Preview' title")
	}
	if !strings.Contains(body, "deck.md") {
		t.Error("response missing filename")
	}
}

func TestHandleSlide_NotFound(t *testing.T) {
	s := NewServer(ServerConfig{File: "/tmp/deck.md"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/slide/0.png", nil)
	s.handleSlide(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandleSlide_BadIndex(t *testing.T) {
	s := NewServer(ServerConfig{File: "/tmp/deck.md"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/slide/abc.png", nil)
	s.handleSlide(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestReload_ValidMarkdown(t *testing.T) {
	t.Skip("markdown parsing removed; preview requires JSON mode")
	// Write a test markdown file
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "test.md")
	content := `---
title: Test Deck
template: test
---
# Title Slide

---
# Content
- Bullet one
- Bullet two
`
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(ServerConfig{File: mdPath, DPI: 96})
	s.reload()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.slides) == 0 {
		t.Fatal("expected slides after reload")
	}
	if s.lastErr != "" {
		t.Errorf("unexpected error: %s", s.lastErr)
	}
	if s.generation != 1 {
		t.Errorf("generation = %d, want 1", s.generation)
	}
}

func TestReload_InvalidFile(t *testing.T) {
	s := NewServer(ServerConfig{File: "/nonexistent/file.md"})
	s.reload()

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastErr == "" {
		t.Error("expected error for missing file")
	}
}

func TestSSESubscription(t *testing.T) {
	s := NewServer(ServerConfig{File: "/tmp/test.md"})

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	// Notify
	s.notifySubscribers()

	select {
	case <-ch:
		// OK
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
	}
}
