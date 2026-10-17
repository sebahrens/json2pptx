package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/types"
)

// requestIDKey is the context key for storing the request ID.
type requestIDKey struct{}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// generateRequestID creates a random 8-byte hex request ID.
func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Server represents the HTTP API server.
type Server struct {
	mux             *http.ServeMux
	convertService  *ConvertService
	templateService *TemplateService
	healthHandler   *HealthHandler
	patternsHandler *PatternsHandler
	semanticRender  http.HandlerFunc
	// semanticFindings shapes the findings of semantic validate / compile.
	semanticFindings   SemanticFindingsFinisher
	logger             *slog.Logger
	apiToken           string
	corsAllowedOrigins []string
	serverConfig       ServerConfig
}

// ServerConfig holds configuration for creating a server.
type ServerConfig struct {
	APIToken           string
	CORSAllowedOrigins []string
	SchemaVersion      string
	SchemaFingerprint  string
	TemplatesDir       string
	OutputDir          string
	Cache              types.TemplateCache
	Logger             *slog.Logger
	StrictValidation   bool // If true, fail on template metadata warnings; if false, log and continue.
	Version            string
	CommitSHA          string
	BuildTime          string
	// FileRetention is the lifetime of generated download files, used to
	// compute the expires_at advertised in file responses so it matches the
	// OutputCleaner policy. Zero/negative falls back to DefaultFileRetention.
	FileRetention time.Duration
	// AllowedImagePaths restricts local image paths referenced by decks to
	// these roots (config Images.AllowedBasePaths / ALLOWED_IMAGE_PATHS).
	// Empty means unrestricted apart from the traversal check.
	AllowedImagePaths []string
	// MaxSlidesPerRequest caps slides per /convert request (0 = default).
	MaxSlidesPerRequest int
	// MaxConcurrentConverts bounds concurrent /convert requests (0 = default).
	// Semantic renders share the same bound.
	MaxConcurrentConverts int
	// SemanticRenderer drives POST /api/v1/semantic/render. It is the shared
	// presentation runner (RunPresentation and its semantic compile / asset /
	// diagnostic glue), which lives in cmd/json2pptx and is injected here so
	// internal/api does not import cmd/. It must be built from the same
	// runtime configuration the server runs with (templates dir, SVG
	// strategy, ALLOWED_IMAGE_PATHS). Nil leaves the endpoint answering 501.
	SemanticRenderer  SemanticRenderer
	SemanticValidator SemanticValidator
	SemanticKinds     SemanticKindsProvider
	// SemanticFindings gives the findings of the semantic validate and compile
	// endpoints the address and remedy form the DeckSpec tools use (path as a
	// JSON Pointer into the spec, slide_number, blocking). It lives in
	// cmd/json2pptx with the rest of that shaping. Nil leaves the findings as
	// the envelope builder made them.
	SemanticFindings SemanticFindingsFinisher
}

// NewServer creates a new API server with all handlers configured.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	templateService := NewTemplateService(cfg.TemplatesDir, cfg.Cache, cfg.StrictValidation)
	conversionPipeline := pipeline.NewPipeline()
	convertService := NewConvertService(cfg.TemplatesDir, cfg.OutputDir, templateService, conversionPipeline)
	convertService.SetFileRetention(cfg.FileRetention)
	convertService.SetAllowedImagePaths(cfg.AllowedImagePaths)
	convertService.SetLimits(cfg.MaxSlidesPerRequest, cfg.MaxConcurrentConverts)
	healthHandler := NewHealthHandler(cfg.Logger, HealthConfig{
		Version:           cfg.Version,
		SchemaVersion:     cfg.SchemaVersion,
		SchemaFingerprint: cfg.SchemaFingerprint,
		CommitSHA:         cfg.CommitSHA,
		BuildTime:         cfg.BuildTime,
	})
	patternsHandler := NewPatternsHandler(patterns.Default())

	s := &Server{
		mux:             http.NewServeMux(),
		convertService:  convertService,
		templateService: templateService,
		healthHandler:   healthHandler,
		patternsHandler: patternsHandler,
		semanticRender: SemanticRenderHandler(cfg.SemanticRenderer, cfg.OutputDir,
			cfg.FileRetention, convertService.convertSem),
		semanticFindings:   cfg.SemanticFindings,
		logger:             cfg.Logger,
		apiToken:           cfg.APIToken,
		corsAllowedOrigins: append([]string(nil), cfg.CORSAllowedOrigins...),
		serverConfig:       cfg,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes.
func (s *Server) setupRoutes() {
	s.mux.Handle("GET /api/v1/health", s.healthHandler)
	s.mux.Handle("GET /api/v1/ready", ReadyHandler(s.serverConfig.TemplatesDir, s.serverConfig.OutputDir))
	s.mux.Handle("GET /api/v1/capabilities", CapabilitiesHandler(s.serverConfig))
	s.mux.Handle("GET /api/v1/templates", s.templateService.ListTemplatesHandler())
	s.mux.Handle("GET /api/v1/templates/{name}", s.templateService.GetTemplateDetailsHandler())
	s.mux.Handle("GET /api/v1/slide-types", SlideTypesHandler())
	s.mux.Handle("GET /api/v1/semantic/schema", SemanticSchemaHandler())
	s.mux.Handle("GET /api/v1/semantic/kinds", SemanticKindsHandler(s.serverConfig.SemanticKinds, s.convertService.convertSem))
	s.mux.Handle("GET /api/v1/thumbnails/{filename}", ThumbnailsHandler(s.serverConfig.OutputDir, s.convertService.convertSem))
	s.mux.Handle("POST /api/v1/semantic/validate", semanticValidateHandler(s.serverConfig.SemanticValidator, s.convertService.convertSem, s.semanticFindings))
	s.mux.Handle("POST /api/v1/semantic/compile", SemanticCompileHandler(s.semanticFindings))
	s.mux.Handle("POST /api/v1/semantic/render", s.semanticRender)
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
	if !requestIDPattern.MatchString(reqID) {
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
			apierrors.WriteInternalError(w, apierrors.CodePanicRecovered, "Internal server error", nil)
		}
	}()
	if !s.allowCORS(w, r) || !s.authorize(w, r) {
		return
	}
	s.mux.ServeHTTP(w, r)
}
