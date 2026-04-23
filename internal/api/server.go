package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/types"
)

// requestIDKey is the context key for storing the request ID.
type requestIDKey struct{}

// generateRequestID creates a random 8-byte hex request ID.
func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Server represents the HTTP API server.
type Server struct {
	mux              *http.ServeMux
	convertService   *ConvertService
	templateService  *TemplateService
	healthHandler    *HealthHandler
	patternsHandler  *PatternsHandler
	logger           *slog.Logger
}

// ServerConfig holds configuration for creating a server.
type ServerConfig struct {
	TemplatesDir     string
	OutputDir        string
	Cache            types.TemplateCache
	Logger           *slog.Logger
	StrictValidation bool // If true, fail on template metadata warnings; if false, log and continue.
	Version          string
	CommitSHA        string
	BuildTime        string
}

// NewServer creates a new API server with all handlers configured.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	templateService := NewTemplateService(cfg.TemplatesDir, cfg.Cache, cfg.StrictValidation)
	conversionPipeline := pipeline.NewPipeline()
	convertService := NewConvertService(cfg.TemplatesDir, cfg.OutputDir, templateService, conversionPipeline)
	healthHandler := NewHealthHandler(cfg.Logger, HealthConfig{
		Version:   cfg.Version,
		CommitSHA: cfg.CommitSHA,
		BuildTime: cfg.BuildTime,
	})
	patternsHandler := NewPatternsHandler(patterns.Default())

	s := &Server{
		mux:              http.NewServeMux(),
		convertService:   convertService,
		templateService:  templateService,
		healthHandler:    healthHandler,
		patternsHandler:  patternsHandler,
		logger:           cfg.Logger,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes.
func (s *Server) setupRoutes() {
	s.mux.Handle("GET /api/v1/health", s.healthHandler)
	s.mux.Handle("GET /api/v1/templates", s.templateService.ListTemplatesHandler())
	s.mux.Handle("GET /api/v1/templates/{name}", s.templateService.GetTemplateDetailsHandler())
	s.mux.Handle("GET /api/v1/slide-types", SlideTypesHandler())
	s.mux.Handle("POST /api/v1/convert", s.convertService.ConvertHandler())
	s.mux.Handle("GET /api/v1/download/{filename}", s.convertService.DownloadHandler())
	s.mux.Handle("GET /api/v1/patterns", s.patternsHandler.ListHandler())
	s.mux.Handle("GET /api/v1/patterns/{name}", s.patternsHandler.ShowHandler())
	s.mux.Handle("POST /api/v1/patterns/{name}/validate", s.patternsHandler.ValidateHandler())
	s.mux.Handle("POST /api/v1/patterns/{name}/expand", s.patternsHandler.ExpandHandler())
}

// ServeHTTP implements http.Handler with request ID injection, security headers, and panic recovery.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

	// Generate or adopt an incoming request ID
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = generateRequestID()
	}

	// Inject into context and response header
	ctx := context.WithValue(r.Context(), requestIDKey{}, reqID)
	r = r.WithContext(ctx)
	w.Header().Set("X-Request-ID", reqID)

	defer func() {
		if err := recover(); err != nil {
			stack := debug.Stack()
			s.logger.Error("panic recovered in HTTP handler",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", reqID,
				"stack", string(stack),
			)
			apierrors.WriteInternalError(w, "PANIC_RECOVERED", "Internal server error", nil)
		}
	}()
	s.mux.ServeHTTP(w, r)
}
