package preview

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/sebahrens/json2pptx/internal/data"
	"github.com/sebahrens/json2pptx/internal/pagination"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/themegen"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ServerConfig holds configuration for the preview server.
type ServerConfig struct {
	File      string          // Markdown file path
	Theme     *types.ThemeInfo // Theme from template (nil for defaults)
	Port      int
	DPI       float64 // PNG rendering DPI (default 192)
}

// Server is a live preview HTTP server with hot reload.
type Server struct {
	cfg        ServerConfig
	mu         sync.RWMutex
	slides     []types.SlideDefinition
	pngCache   map[int][]byte // slide index → PNG bytes
	generation int64          // bumped on every reload
	lastErr    string

	// SSE subscribers
	subMu   sync.Mutex
	subs    map[chan struct{}]struct{}
	closeCh chan struct{}
}

// NewServer creates a new preview server.
func NewServer(cfg ServerConfig) *Server {
	if cfg.DPI <= 0 {
		cfg.DPI = 192
	}
	if cfg.Port <= 0 {
		cfg.Port = 3333
	}
	return &Server{
		cfg:     cfg,
		pngCache: make(map[int][]byte),
		subs:    make(map[chan struct{}]struct{}),
		closeCh: make(chan struct{}),
	}
}

// Run starts the server: loads the file, starts watching, and serves HTTP.
func (s *Server) Run() error {
	// Initial load
	s.reload()

	// Start file watcher
	go s.watchFile()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/slide/", s.handleSlide)
	mux.HandleFunc("/events", s.handleSSE)

	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
	slog.Info("preview server listening", "addr", addr, "file", s.cfg.File)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 0, // SSE streams need unlimited write time
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}

// reload reads and parses the markdown file, then re-renders all slides.
func (s *Server) reload() {
	content, err := os.ReadFile(s.cfg.File)
	if err != nil {
		s.setError(fmt.Sprintf("read file: %v", err))
		return
	}

	// Parse JSON presentation definition.
	baseDir := filepath.Dir(s.cfg.File)
	var presentation types.PresentationDefinition
	if err := json.Unmarshal(content, &presentation); err != nil {
		s.setError(fmt.Sprintf("parse: %v", err))
		return
	}

	// Check for fatal parse errors
	for _, pe := range presentation.Errors {
		if pe.Level == types.ErrorLevelError {
			s.setError(pe.Format())
			return
		}
	}

	// Resolve data variables
	if len(presentation.Metadata.Data) > 0 {
		dataCtx, _, err := data.BuildContext(presentation.Metadata.Data, nil, baseDir)
		if err == nil {
			_ = data.ResolveVariables(&presentation, dataCtx)
		}
	}

	// Auto-agenda
	if presentation.Metadata.AutoAgenda {
		pipeline.GenerateAgenda(&presentation)
	}

	// Auto-paginate
	_ = pagination.Paginate(&presentation)

	// Resolve brand_color into a ThemeOverride (if specified)
	if presentation.Metadata.BrandColor != "" {
		if resolved, brandErr := themegen.ResolveBrandColor(presentation.Metadata.BrandColor, presentation.Metadata.ThemeOverride); brandErr == nil {
			presentation.Metadata.ThemeOverride = resolved
		}
	}

	// Apply theme override from frontmatter
	theme := s.cfg.Theme
	if presentation.Metadata.ThemeOverride != nil && theme != nil {
		overridden := theme.ApplyOverride(presentation.Metadata.ThemeOverride)
		theme = &overridden
	}

	// Render all slides to PNG
	newCache := make(map[int][]byte, len(presentation.Slides))
	for i, slide := range presentation.Slides {
		pngBytes, err := RenderSlidePNG(slide, theme, s.cfg.DPI)
		if err != nil {
			slog.Warn("render slide failed", "slide", i, "error", err)
			continue
		}
		newCache[i] = pngBytes
	}

	s.mu.Lock()
	s.slides = presentation.Slides
	s.pngCache = newCache
	s.generation++
	s.lastErr = ""
	s.mu.Unlock()

	slog.Info("reloaded", "slides", len(presentation.Slides), "generation", s.generation)
	s.notifySubscribers()
}

// setError records a parse/render error and notifies subscribers.
func (s *Server) setError(msg string) {
	slog.Error("preview error", "error", msg)
	s.mu.Lock()
	s.lastErr = msg
	s.generation++
	s.mu.Unlock()
	s.notifySubscribers()
}

// watchFile uses fsnotify to watch the markdown file for changes.
func (s *Server) watchFile() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create watcher", "error", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	absPath, _ := filepath.Abs(s.cfg.File)
	dir := filepath.Dir(absPath)
	if err := watcher.Add(dir); err != nil {
		slog.Error("failed to watch directory", "dir", dir, "error", err)
		return
	}

	// Debounce timer — avoid re-parsing on rapid saves
	var debounce *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Only react to the watched file
			eventAbs, _ := filepath.Abs(event.Name)
			if eventAbs != absPath {
				continue
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(200*time.Millisecond, func() {
				s.reload()
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)

		case <-s.closeCh:
			return
		}
	}
}

// --- SSE ---

func (s *Server) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
	return ch
}

func (s *Server) unsubscribe(ch chan struct{}) {
	s.subMu.Lock()
	delete(s.subs, ch)
	s.subMu.Unlock()
}

func (s *Server) notifySubscribers() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// --- HTTP handlers ---

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = indexTemplate.Execute(w, struct{ File string }{File: filepath.Base(s.cfg.File)})
}

func (s *Server) handleSlide(w http.ResponseWriter, r *http.Request) {
	// Parse /slide/0.png
	path := strings.TrimPrefix(r.URL.Path, "/slide/")
	path = strings.TrimSuffix(path, ".png")
	idx, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "invalid slide index", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	pngBytes, ok := s.pngCache[idx]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "slide not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(pngBytes)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	// Send current state immediately
	s.sendReloadEvent(w, flusher)

	for {
		select {
		case <-ch:
			s.sendReloadEvent(w, flusher)
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) sendReloadEvent(w http.ResponseWriter, flusher http.Flusher) {
	s.mu.RLock()
	payload := struct {
		Slides     int    `json:"slides"`
		Generation int64  `json:"generation"`
		Error      string `json:"error,omitempty"`
	}{
		Slides:     len(s.slides),
		Generation: s.generation,
		Error:      s.lastErr,
	}
	s.mu.RUnlock()

	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: reload\ndata: %s\n\n", data)
	flusher.Flush()
}
