// Package httpserver provides the HTTP server for the SVG generation API.
package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ahrens/svggen"
	"github.com/ahrens/svggen/internal/safeyaml"
	"golang.org/x/sync/errgroup"
)

// Server represents the SVG generation API HTTP server.
type Server struct {
	mux            *http.ServeMux
	registry       *svggen.Registry
	logger         *slog.Logger
	config         Config
	cache          *svggen.RenderCache
	rateLimiter    *RateLimiter    // Rate limiter instance
	allowedOrigins map[string]bool // CORS allowed origins
	securityConfig SecurityConfig  // Security configuration
}

// Config holds server configuration.
type Config struct {
	// Port is the HTTP server port.
	Port int

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum duration to wait for the next request.
	IdleTimeout time.Duration

	// ShutdownTimeout is the maximum duration to wait for graceful shutdown.
	ShutdownTimeout time.Duration

	// MaxRequestSize is the maximum request body size in bytes.
	MaxRequestSize int64

	// MaxBatchSize limits the number of items in a batch request.
	// If 0, defaults to 100.
	MaxBatchSize int

	// MaxBatchWorkers limits concurrent goroutines in batch rendering.
	// If 0, defaults to runtime.NumCPU() * 2.
	MaxBatchWorkers int

	// Logger is the structured logger to use. If nil, slog.Default() is used.
	Logger *slog.Logger

	// CacheConfig holds cache configuration. If nil, default cache config is used.
	// Set CacheConfig.TTL to 0 to disable caching.
	CacheConfig *svggen.CacheConfig

	// Security holds security configuration.
	// If nil, default security config is used (authentication and rate limiting disabled).
	Security *SecurityConfig
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() Config {
	return Config{
		Port:            3001,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    60 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		MaxRequestSize:  10 * 1024 * 1024, // 10MB
	}
}

// NewServer creates a new SVG generation API server.
// If registry is nil, the default registry is used.
func NewServer(cfg Config, registry *svggen.Registry) *Server {
	if registry == nil {
		registry = svggen.DefaultRegistry()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Initialize cache
	var cache *svggen.RenderCache
	if cfg.CacheConfig != nil {
		if cfg.CacheConfig.TTL > 0 {
			cache = svggen.NewRenderCache(*cfg.CacheConfig)
		}
	} else {
		// Use default cache config
		defaultCfg := svggen.DefaultCacheConfig()
		cache = svggen.NewRenderCache(defaultCfg)
	}

	// Initialize security configuration
	securityCfg := DefaultSecurityConfig()
	if cfg.Security != nil {
		securityCfg = *cfg.Security
	}

	// Build allowed origins map for O(1) lookup
	allowedOrigins := make(map[string]bool)
	for _, origin := range securityCfg.AllowedOrigins {
		allowedOrigins[origin] = true
	}

	// Initialize rate limiter if enabled
	var rateLimiter *RateLimiter
	if securityCfg.RateLimit.Enabled {
		limit := securityCfg.RateLimit.RequestsPerWindow
		if limit <= 0 {
			limit = 100 // Default
		}
		window := securityCfg.RateLimit.WindowDuration
		if window <= 0 {
			window = time.Minute // Default
		}

		var err error
		if len(securityCfg.TrustedProxies) > 0 {
			rateLimiter, err = NewRateLimiterWithTrustedProxies(limit, window, securityCfg.TrustedProxies)
			if err != nil {
				cfg.Logger.Warn("invalid trusted proxies, using default rate limiter", "error", err)
				rateLimiter = NewRateLimiter(limit, window)
			}
		} else {
			rateLimiter = NewRateLimiter(limit, window)
		}

		cfg.Logger.Info("rate limiting enabled",
			"limit", limit,
			"window", window,
		)
	}

	s := &Server{
		mux:            http.NewServeMux(),
		registry:       registry,
		logger:         cfg.Logger,
		config:         cfg,
		cache:          cache,
		rateLimiter:    rateLimiter,
		allowedOrigins: allowedOrigins,
		securityConfig: securityCfg,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes.
// Middleware is applied in order: security headers -> CORS -> auth -> rate limiting -> handler.
func (s *Server) setupRoutes() {
	// Create authentication middleware
	auth := authMiddleware(s.securityConfig.Auth)

	// Helper to wrap handlers with full middleware chain
	withMiddleware := func(h http.Handler) http.Handler {
		// Apply: security headers -> CORS -> auth
		wrapped := securityHeadersMiddleware(corsMiddleware(auth(h), s.allowedOrigins))

		// Apply rate limiting if enabled
		if s.rateLimiter != nil {
			wrapped = RateLimitMiddleware(s.rateLimiter)(wrapped)
		}

		return wrapped
	}

	// Helper for health endpoint (skip auth and rate limiting)
	withHealthMiddleware := func(h http.Handler) http.Handler {
		return securityHeadersMiddleware(corsMiddleware(h, s.allowedOrigins))
	}

	// Health check (no auth or rate limiting - always accessible)
	s.mux.Handle("GET /healthz", withHealthMiddleware(http.HandlerFunc(s.handleHealth)))

	// List available diagram types
	s.mux.Handle("GET /types", withMiddleware(http.HandlerFunc(s.handleTypes)))

	// Render single diagram
	s.mux.Handle("POST /render", withMiddleware(http.HandlerFunc(s.handleRender)))

	// Render batch of diagrams
	s.mux.Handle("POST /render/batch", withMiddleware(http.HandlerFunc(s.handleRenderBatch)))
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add request logging
	start := time.Now()
	lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	s.mux.ServeHTTP(lrw, r)

	s.logger.Info("request completed",
		"method", r.Method,
		"path", r.URL.Path,
		"status", lrw.statusCode,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

// Run starts the HTTP server and blocks until it's shut down.
// It handles graceful shutdown when the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("starting SVG generation API server",
			"port", s.config.Port,
			"read_timeout", s.config.ReadTimeout,
			"write_timeout", s.config.WriteTimeout,
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.logger.Info("shutdown signal received")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	// Stop cache cleanup goroutine
	if s.cache != nil {
		s.cache.Stop()
	}

	s.logger.Info("server stopped")
	return nil
}

// handleHealth handles GET /healthz.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Include cache stats if cache is enabled
	if s.cache != nil {
		stats := s.cache.Stats()
		response["cache"] = map[string]any{
			"enabled":     true,
			"entries":     stats.Entries,
			"hits":        stats.Hits,
			"misses":      stats.Misses,
			"hit_rate":    fmt.Sprintf("%.1f%%", stats.HitRate()),
			"evictions":   stats.Evictions,
			"total_bytes": stats.TotalBytes,
		}
	} else {
		response["cache"] = map[string]any{
			"enabled": false,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// handleTypes handles GET /types.
func (s *Server) handleTypes(w http.ResponseWriter, _ *http.Request) {
	types := s.registry.Types()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"types": types,
		"count": len(types),
	})
}

// RenderRequest is the request body for POST /render.
type RenderRequest = svggen.RequestEnvelope

// RenderResponse is the response body for POST /render.
type RenderResponse struct {
	// Format indicates the actual output format ("svg" or "png").
	Format string `json:"format"`

	// SVG contains the SVG markup (when format is "svg").
	SVG string `json:"svg,omitempty"`

	// PNG contains base64-encoded PNG data (when format is "png").
	PNG string `json:"png,omitempty"`

	// Width is the diagram width in points.
	Width float64 `json:"width"`

	// Height is the diagram height in points.
	Height float64 `json:"height"`

	// Error contains the error message if rendering failed.
	Error string `json:"error,omitempty"`
}

// handleRender handles POST /render.
func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	// Parse request based on content type
	req, err := s.parseRequest(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, parseErrorCode(err), err.Error())
		return
	}

	// Determine requested output format
	requestedFormat := strings.ToLower(req.Output.Format)
	if requestedFormat == "" {
		requestedFormat = "svg"
	}

	// Validate requested format
	if requestedFormat != "svg" && requestedFormat != "png" {
		s.writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			fmt.Sprintf("unsupported output format: %s (supported: svg, png)", requestedFormat))
		return
	}

	// Check cache first
	var result *svggen.RenderResult
	cacheHit := false
	if s.cache != nil {
		if cached := s.cache.Get(req); cached != nil {
			result = cached
			cacheHit = true
		}
	}

	// Render if not cached
	if result == nil {
		var renderErr error
		result, renderErr = svggen.RegistryRenderMultiFormat(s.registry, req, requestedFormat)
		if renderErr != nil {
			s.writeError(w, http.StatusBadRequest, CodeRenderError, renderErr.Error())
			return
		}

		// Store in cache
		if s.cache != nil {
			s.cache.Set(req, result)
		}
	}

	// Build response based on format
	response := RenderResponse{
		Format: result.Format,
		Width:  float64(result.Width),
		Height: float64(result.Height),
	}

	switch result.Format {
	case "png":
		response.PNG = base64.StdEncoding.EncodeToString(result.PNG)
	default:
		response.SVG = result.SVG.String()
	}

	// Add cache header for debugging
	if cacheHit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// BatchRequest is the request body for POST /render/batch.
type BatchRequest struct {
	Requests []RenderRequest `json:"requests" yaml:"requests"`
}

// BatchResponse is the response body for POST /render/batch.
type BatchResponse struct {
	Results []RenderResponse `json:"results"`
	Total   int              `json:"total"`
	Success int              `json:"success"`
	Failed  int              `json:"failed"`
}

// renderSingleRequest renders a single request and returns the response.
// This method handles format validation, caching, and result building.
func (s *Server) renderSingleRequest(req *RenderRequest) RenderResponse {
	// Determine requested output format
	requestedFormat := strings.ToLower(req.Output.Format)
	if requestedFormat == "" {
		requestedFormat = "svg"
	}

	// Validate requested format
	if requestedFormat != "svg" && requestedFormat != "png" {
		return RenderResponse{Error: fmt.Sprintf("unsupported output format: %s", requestedFormat)}
	}

	// Check cache first
	var result *svggen.RenderResult
	if s.cache != nil {
		result = s.cache.Get(req)
	}

	// Render if not cached
	if result == nil {
		var err error
		result, err = svggen.RegistryRenderMultiFormat(s.registry, req, requestedFormat)
		if err != nil {
			return RenderResponse{Error: err.Error()}
		}

		// Store in cache
		if s.cache != nil {
			s.cache.Set(req, result)
		}
	}

	// Build response
	return s.buildRenderResponse(result)
}

// buildRenderResponse converts a RenderResult to a RenderResponse.
func (s *Server) buildRenderResponse(result *svggen.RenderResult) RenderResponse {
	response := RenderResponse{
		Format: result.Format,
		Width:  float64(result.Width),
		Height: float64(result.Height),
	}
	switch result.Format {
	case "png":
		response.PNG = base64.StdEncoding.EncodeToString(result.PNG)
	default:
		response.SVG = result.SVG.String()
	}
	return response
}

// handleRenderBatch handles POST /render/batch.
func (s *Server) handleRenderBatch(w http.ResponseWriter, r *http.Request) {
	// Parse request based on content type
	batch, err := s.parseBatchRequest(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, parseErrorCode(err), err.Error())
		return
	}

	if len(batch.Requests) == 0 {
		s.writeError(w, http.StatusBadRequest, CodeInvalidRequest, "no requests provided")
		return
	}

	maxBatchSize := s.config.MaxBatchSize
	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}
	if len(batch.Requests) > maxBatchSize {
		s.writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			fmt.Sprintf("batch size %d exceeds maximum of %d", len(batch.Requests), maxBatchSize))
		return
	}

	// Determine worker count
	workers := s.config.MaxBatchWorkers
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}

	// Render all requests with bounded concurrency using errgroup
	results := make([]RenderResponse, len(batch.Requests))
	var mu sync.Mutex
	var successCount, failedCount int

	g, _ := errgroup.WithContext(r.Context())
	g.SetLimit(workers)

	for i, req := range batch.Requests {
		reqCopy := req // capture loop variable
		idx := i
		g.Go(func() error {
			resp := s.renderSingleRequest(&reqCopy)
			results[idx] = resp

			mu.Lock()
			if resp.Error != "" {
				failedCount++
			} else {
				successCount++
			}
			mu.Unlock()

			return nil // errors are captured in RenderResponse.Error
		})
	}

	_ = g.Wait() // all goroutines return nil

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BatchResponse{
		Results: results,
		Total:   len(batch.Requests),
		Success: successCount,
		Failed:  failedCount,
	})
}

// parseRequest parses a single render request from the HTTP request body.
// Supports both JSON and YAML content types.
// Uses safeyaml for YAML parsing to protect against billion laughs and other attacks.
func (s *Server) parseRequest(r *http.Request) (*svggen.RequestEnvelope, error) {
	contentType := r.Header.Get("Content-Type")

	// Limit request body size
	r.Body = http.MaxBytesReader(nil, r.Body, s.config.MaxRequestSize)

	var req svggen.RequestEnvelope

	if strings.Contains(contentType, "yaml") || strings.Contains(contentType, "x-yaml") {
		// Read body with size limit already enforced by MaxBytesReader
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		// Use safeyaml for protected parsing
		if err := safeyaml.Unmarshal(data, &req); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	} else {
		// Default to JSON
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	return &req, nil
}

// parseBatchRequest parses a batch render request from the HTTP request body.
// Uses safeyaml for YAML parsing to protect against billion laughs and other attacks.
func (s *Server) parseBatchRequest(r *http.Request) (*BatchRequest, error) {
	contentType := r.Header.Get("Content-Type")

	// Limit request body size
	r.Body = http.MaxBytesReader(nil, r.Body, s.config.MaxRequestSize)

	var batch BatchRequest

	if strings.Contains(contentType, "yaml") || strings.Contains(contentType, "x-yaml") {
		// Read body with size limit already enforced by MaxBytesReader
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		// Use safeyaml for protected parsing
		if err := safeyaml.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	} else {
		// Default to JSON
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&batch); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	return &batch, nil
}

// writeError writes an error response using the standard error format.
func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	writeErrorResponse(w, status, code, message, nil)
}

// parseErrorCode determines the appropriate error code for a parse error.
func parseErrorCode(err error) string {
	// Check for validation errors first
	if svggen.IsValidationError(err) {
		return CodeInvalidRequest
	}

	// Check error message for JSON/YAML parse failures
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "invalid json"):
		return CodeInvalidJSON
	case strings.Contains(msg, "invalid yaml"):
		return CodeInvalidYAML
	default:
		return CodeInvalidRequest
	}
}

// loggingResponseWriter wraps http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
