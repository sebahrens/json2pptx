package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/types"
)

// DefaultConvertTimeout is the default timeout for convert operations (2 minutes).
const DefaultConvertTimeout = 2 * time.Minute

// ConvertService handles presentation conversion.
type ConvertService struct {
	templatesDir     string
	outputDir        string
	templateAnalyzer TemplateAnalyzer
	pipeline         pipeline.Pipeline
}

// NewConvertService creates a new convert service.
func NewConvertService(templatesDir, outputDir string, templateAnalyzer TemplateAnalyzer, p pipeline.Pipeline) *ConvertService {
	return &ConvertService{
		templatesDir:     templatesDir,
		outputDir:        outputDir,
		templateAnalyzer: templateAnalyzer,
		pipeline:         p,
	}
}

// MaxRequestBodySize is the maximum allowed size for request bodies (10MB).
const MaxRequestBodySize = 10 * 1024 * 1024

// SVG scale limits for parameter validation.
const (
	MinSVGScale = 0.5
	MaxSVGScale = 10.0
)

// ConvertHandler handles POST /api/v1/convert requests.
func (cs *ConvertService) ConvertHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		ctx, cancel := context.WithTimeout(r.Context(), DefaultConvertTimeout)
		defer cancel()

		// Validate Content-Type
		contentType := r.Header.Get("Content-Type")
		if contentType != "" {
			mediaType := strings.TrimSpace(strings.Split(contentType, ";")[0])
			if mediaType != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, apierrors.CodeInvalidContentType,
					"Content-Type must be application/json", nil)
				return
			}
		}

		// Parse and validate request
		req, templatePath, err := cs.parseAndValidateRequest(w, r)
		if err != nil {
			return
		}

		// Get template analysis
		templateAnalysis, err := cs.templateAnalyzer.GetOrAnalyzeTemplate(templatePath)
		if err != nil {
			slog.Error("failed to analyze template", "path", templatePath, "error", err)
			writeError(w, http.StatusInternalServerError, apierrors.CodeTemplateError,
				"Failed to analyze template", nil)
			return
		}

		// Generate unique output filename
		outputFilename, err := generateUniqueFilename()
		if err != nil {
			slog.Error("failed to generate secure filename", "error", err)
			writeError(w, http.StatusInternalServerError, apierrors.CodeGenerationError,
				"Failed to generate output filename", nil)
			return
		}
		outputFilename += ".pptx"
		outputPath := filepath.Join(cs.outputDir, outputFilename)

		// Convert API slides to PresentationDefinition
		presentation := apiSlidesToPresentation(req.Slides)

		// Build pipeline request
		pipelineReq := pipeline.ConvertRequest{
			Presentation:          presentation,
			TemplateAnalysis:      templateAnalysis,
			OutputPath:            outputPath,
			TemplatePath:          templatePath,
			SVGScale:              req.Options.SVGScale,
			ExcludeTemplateSlides: req.Options.ExcludeTemplateSlides,
		}

		// Execute conversion
		pipelineResult, err := cs.pipeline.Convert(ctx, pipelineReq)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				writeError(w, http.StatusGatewayTimeout, apierrors.CodeRequestTimeout,
					"Request processing timed out", nil)
				return
			}
			cs.handlePipelineError(w, err)
			return
		}

		genResult := &generationResult{
			OutputPath:     pipelineResult.OutputPath,
			OutputFilename: outputFilename,
			SlideCount:     pipelineResult.SlideCount,
			Warnings:       pipelineResult.Warnings,
		}

		cs.sendResponse(w, req, templateAnalysis, genResult, startTime)
	}
}

// handlePipelineError translates pipeline errors to HTTP responses.
func (cs *ConvertService) handlePipelineError(w http.ResponseWriter, err error) {
	var parseErr *pipeline.ParseError
	var validationErr *pipeline.ValidationError
	var layoutErr *pipeline.LayoutError
	var genErr *pipeline.GenerationError

	switch {
	case errors.As(err, &parseErr):
		slog.Warn("failed to parse input", "error", parseErr)
		errorCode := apierrors.CodeInvalidInput
		details := map[string]interface{}{}
		if parseErr.Line > 0 {
			details["line"] = parseErr.Line
			details["field"] = parseErr.Field
		}
		if parseErr.Field == "type" && isInvalidSlideTypeError(parseErr.Message) {
			errorCode = apierrors.CodeInvalidSlideType
			details["supported_types"] = getSupportedTypeNames()
		}
		writeError(w, http.StatusBadRequest, errorCode,
			fmt.Sprintf("Input parsing failed: %s", parseErr.Message), details)
	case errors.As(err, &validationErr):
		slog.Warn("presentation validation failed", "error", validationErr)
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidInput,
			validationErr.Message, map[string]interface{}{
				"line":  validationErr.Line,
				"field": validationErr.Field,
			})
	case errors.As(err, &layoutErr):
		slog.Error("failed to select layout", "error", layoutErr)
		writeError(w, http.StatusInternalServerError, apierrors.CodeGenerationError,
			"Failed to prepare slides", nil)
	case errors.As(err, &genErr):
		slog.Error("failed to generate presentation", "error", genErr)
		writeError(w, http.StatusInternalServerError, apierrors.CodeGenerationError,
			"Failed to generate presentation", nil)
	default:
		slog.Error("unexpected pipeline error", "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.CodeGenerationError,
			"An unexpected error occurred", nil)
	}
}

// parseAndValidateRequest parses and validates the incoming request.
func (cs *ConvertService) parseAndValidateRequest(w http.ResponseWriter, r *http.Request) (*ConvertRequest, string, error) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeError(w, http.StatusRequestEntityTooLarge, apierrors.CodeRequestTooLarge,
				fmt.Sprintf("Request body exceeds maximum size of %d bytes", MaxRequestBodySize), nil)
			return nil, "", err
		}
		slog.Warn("failed to parse request", "error", err)
		writeJSONParseError(w, err)
		return nil, "", err
	}

	// Validate required fields
	if len(req.Slides) == 0 {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
			"At least one slide is required", map[string]interface{}{"field": "slides"})
		return nil, "", fmt.Errorf("validation failed")
	}
	if req.Template == "" {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
			"Template name is required", map[string]interface{}{"field": "template"})
		return nil, "", fmt.Errorf("validation failed")
	}

	// Apply defaults
	if req.Options == nil {
		req.Options = &ConvertOptions{}
	}
	if req.Options.OutputFormat == "" {
		req.Options.OutputFormat = "file"
	}
	// Template example slides should never appear in generated output;
	// always exclude them regardless of what the client sends.
	req.Options.ExcludeTemplateSlides = true

	// Validate output format
	if req.Options.OutputFormat != "file" && req.Options.OutputFormat != "base64" {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
			fmt.Sprintf("Invalid output format: %s (must be 'file' or 'base64')", req.Options.OutputFormat),
			map[string]interface{}{"field": "options.output_format"})
		return nil, "", fmt.Errorf("validation failed")
	}

	// Validate SVG scale
	if req.Options.SVGScale != 0 && (req.Options.SVGScale < MinSVGScale || req.Options.SVGScale > MaxSVGScale) {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidRequest,
			fmt.Sprintf("Invalid svg_scale: %.2f (must be between %.1f and %.1f)", req.Options.SVGScale, MinSVGScale, MaxSVGScale),
			map[string]interface{}{"field": "options.svg_scale"})
		return nil, "", fmt.Errorf("validation failed")
	}

	// Validate template name (prevent path traversal)
	if err := ValidateTemplateName(req.Template); err != nil {
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidTemplate,
			err.Error(), map[string]interface{}{"field": "template"})
		return nil, "", fmt.Errorf("validation failed")
	}

	// Check template exists
	templatePath := filepath.Join(cs.templatesDir, req.Template+".pptx")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, apierrors.CodeTemplateNotFound,
			fmt.Sprintf("Template '%s' not found", req.Template),
			map[string]interface{}{"field": "template"})
		return nil, "", fmt.Errorf("validation failed")
	}

	return &req, templatePath, nil
}

// generationResult holds the result of presentation generation.
type generationResult struct {
	OutputPath     string
	OutputFilename string
	SlideCount     int
	Warnings       []string
}

// sendResponse builds and sends the appropriate response based on output format.
func (cs *ConvertService) sendResponse(
	w http.ResponseWriter,
	req *ConvertRequest,
	templateAnalysis *types.TemplateAnalysis,
	genResult *generationResult,
	startTime time.Time,
) {
	processingTimeMs := time.Since(startTime).Milliseconds()

	if req.Options.OutputFormat == "base64" {
		filename := "presentation.pptx"
		if templateAnalysis != nil && templateAnalysis.Metadata != nil && templateAnalysis.Metadata.Name != "" {
			filename = templateAnalysis.Metadata.Name + ".pptx"
		}
		cs.sendBase64Response(w, filename, genResult, processingTimeMs)
		return
	}

	cs.sendFileResponse(w, genResult, processingTimeMs)
}

// sendBase64Response sends a base64-encoded response.
func (cs *ConvertService) sendBase64Response(
	w http.ResponseWriter,
	filename string,
	genResult *generationResult,
	processingTimeMs int64,
) {
	data, err := os.ReadFile(genResult.OutputPath)
	if err != nil {
		slog.Error("failed to read generated file", "path", genResult.OutputPath, "error", err)
		writeError(w, http.StatusInternalServerError, apierrors.CodeGenerationError,
			"Failed to read generated file", nil)
		return
	}

	_ = os.Remove(genResult.OutputPath)

	encoded := base64.StdEncoding.EncodeToString(data)

	resp := ConvertResponseBase64{
		Success:  true,
		Data:     encoded,
		Filename: filename,
		Stats: ConvertStats{
			SlideCount:       genResult.SlideCount,
			ProcessingTimeMs: processingTimeMs,
			Warnings:         genResult.Warnings,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// sendFileResponse sends a file download response.
func (cs *ConvertService) sendFileResponse(
	w http.ResponseWriter,
	genResult *generationResult,
	processingTimeMs int64,
) {
	expiresAt := time.Now().Add(1 * time.Hour)

	resp := ConvertResponseFile{
		Success:   true,
		FileURL:   "/api/v1/download/" + genResult.OutputFilename,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		Stats: ConvertStats{
			SlideCount:       genResult.SlideCount,
			ProcessingTimeMs: processingTimeMs,
			Warnings:         genResult.Warnings,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// generateUniqueFilename creates a unique filename using cryptographic random bytes.
func generateUniqueFilename() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate secure filename: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// writeJSONParseError translates a json.Decoder error into a structured HTTP
// error response. Syntax errors get INVALID_JSON with offset; type errors get
// INVALID_JSON with field and expected type; other errors get INVALID_REQUEST.
func writeJSONParseError(w http.ResponseWriter, err error) {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidJSON,
			fmt.Sprintf("JSON syntax error: %s", syntaxErr.Error()),
			map[string]any{"offset": syntaxErr.Offset})
	case errors.As(err, &typeErr):
		details := map[string]any{
			"field":         typeErr.Field,
			"expected_type": typeErr.Type.String(),
			"got_value":     typeErr.Value,
		}
		if typeErr.Offset > 0 {
			details["offset"] = typeErr.Offset
		}
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidJSON,
			fmt.Sprintf("JSON type error at field %q: expected %s, got %s",
				typeErr.Field, typeErr.Type, typeErr.Value), details)
	default:
		writeError(w, http.StatusBadRequest, apierrors.CodeInvalidJSON,
			fmt.Sprintf("Failed to parse request body: %s", err.Error()), nil)
	}
}

// Request and response types

// ConvertRequest is the request for POST /api/v1/convert.
// Accepts JSON slide definitions instead of markdown.
type ConvertRequest struct {
	Template string          `json:"template"`
	Slides   []APISlide      `json:"slides"`
	Options  *ConvertOptions `json:"options,omitempty"`
}

// APISlide describes a single slide in the JSON input.
type APISlide struct {
	Type            string     `json:"type"`                        // Slide type: content, title, section, two-column, image, chart, diagram, comparison, blank
	Title           string     `json:"title,omitempty"`             // Slide title
	Content         APIContent `json:"content,omitempty"`           // Slide content
	SpeakerNotes    string     `json:"speaker_notes,omitempty"`     // Speaker notes
	Source          string     `json:"source,omitempty"`            // Source attribution
	Transition      string     `json:"transition,omitempty"`        // Slide transition type
	TransitionSpeed string     `json:"transition_speed,omitempty"`  // Transition speed
	Build           string     `json:"build,omitempty"`             // Build animation
}

// APIContent holds the content fields for a slide.
type APIContent struct {
	Body    string   `json:"body,omitempty"`    // Plain text body
	Bullets []string `json:"bullets,omitempty"` // Bullet points
}

// ConvertOptions contains optional settings for conversion.
type ConvertOptions struct {
	OutputFormat          string  `json:"output_format"`
	SVGScale              float64 `json:"svg_scale,omitempty"`
	ExcludeTemplateSlides bool    `json:"exclude_template_slides,omitempty"`
}

// ConvertResponseFile is the response for file output format.
type ConvertResponseFile struct {
	Success   bool         `json:"success"`
	FileURL   string       `json:"file_url"`
	ExpiresAt string       `json:"expires_at"`
	Stats     ConvertStats `json:"stats"`
}

// ConvertResponseBase64 is the response for base64 output format.
type ConvertResponseBase64 struct {
	Success  bool         `json:"success"`
	Data     string       `json:"data"`
	Filename string       `json:"filename"`
	Stats    ConvertStats `json:"stats"`
}

// ConvertStats contains conversion statistics.
type ConvertStats struct {
	SlideCount       int      `json:"slide_count"`
	ProcessingTimeMs int64    `json:"processing_time_ms"`
	Warnings         []string `json:"warnings"`
}

// apiSlidesToPresentation converts API slide definitions to a PresentationDefinition.
func apiSlidesToPresentation(slides []APISlide) *types.PresentationDefinition {
	pres := &types.PresentationDefinition{
		Metadata: types.Metadata{
			Title: "Presentation",
		},
		Slides: make([]types.SlideDefinition, len(slides)),
	}
	for i, s := range slides {
		slideType := types.SlideType(s.Type)
		if slideType == "" {
			slideType = types.SlideTypeContent
		}
		pres.Slides[i] = types.SlideDefinition{
			Index:           i,
			Title:           s.Title,
			Type:            slideType,
			SpeakerNotes:    s.SpeakerNotes,
			Source:          s.Source,
			Transition:      s.Transition,
			TransitionSpeed: s.TransitionSpeed,
			Build:           s.Build,
			Content: types.SlideContent{
				Body:    s.Content.Body,
				Bullets: s.Content.Bullets,
			},
		}
	}
	return pres
}

// isInvalidSlideTypeError checks if an error message indicates an invalid slide type.
func isInvalidSlideTypeError(message string) bool {
	return len(message) > 19 && message[:19] == "unknown slide type:"
}

// getSupportedTypeNames returns a list of supported slide type names.
func getSupportedTypeNames() []string {
	supported := types.SupportedSlideTypes()
	result := make([]string, len(supported))
	for i, st := range supported {
		result[i] = string(st.Type)
	}
	return result
}
