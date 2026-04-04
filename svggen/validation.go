// ValidationError, ValidationErrors, and error code constants have been moved
// to core/validation.go. They are re-exported via core_aliases.go.
//
// This file contains the Decoder and related parsing functions which depend on
// internal/safeyaml and therefore stay in the root package.
package svggen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/internal/safeyaml"
)

// DecodeOptions configures the parsing and validation behavior.
type DecodeOptions struct {
	// Strict enables strict mode:
	// - Disallows unknown fields
	// - Validates all constraints
	// - Requires explicit types
	Strict bool

	// Format specifies the input format: "json", "yaml", or "auto" (default).
	// Auto-detection uses the first non-whitespace character.
	Format string

	// Defaults applies default values for missing optional fields.
	Defaults bool
}

// DefaultDecodeOptions returns the default decode options.
func DefaultDecodeOptions() DecodeOptions {
	return DecodeOptions{
		Strict:   false,
		Format:   "auto",
		Defaults: true,
	}
}

// Decoder handles parsing and validation of request envelopes.
type Decoder struct {
	opts DecodeOptions
}

// NewDecoder creates a decoder with the specified options.
func NewDecoder(opts DecodeOptions) *Decoder {
	return &Decoder{opts: opts}
}

// Decode parses and validates input data into a RequestEnvelope.
func (d *Decoder) Decode(data []byte) (*RequestEnvelope, error) {
	format := d.detectFormat(data)

	var req RequestEnvelope
	var parseErr error

	switch format {
	case "json":
		parseErr = d.decodeJSON(data, &req)
	case "yaml":
		parseErr = d.decodeYAML(data, &req)
	default:
		return nil, &ValidationError{
			Code:    ErrCodeParseFailed,
			Message: fmt.Sprintf("unsupported format: %s", format),
		}
	}

	if parseErr != nil {
		return nil, parseErr
	}

	// Apply defaults if enabled
	if d.opts.Defaults {
		d.applyDefaults(&req)
	}

	// Validate the request
	if err := d.validate(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// DecodeReader reads from an io.Reader and decodes.
func (d *Decoder) DecodeReader(r io.Reader) (*RequestEnvelope, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, &ValidationError{
			Code:    ErrCodeParseFailed,
			Message: fmt.Sprintf("failed to read input: %v", err),
		}
	}
	return d.Decode(data)
}

// detectFormat determines the input format from the data.
func (d *Decoder) detectFormat(data []byte) string {
	if d.opts.Format != "" && d.opts.Format != "auto" {
		return d.opts.Format
	}

	// Skip leading whitespace
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "json" // Default to JSON for empty input
	}

	// JSON starts with { or [
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return "json"
	}

	// YAML otherwise
	return "yaml"
}

// decodeJSON parses JSON with optional strict mode.
func (d *Decoder) decodeJSON(data []byte, req *RequestEnvelope) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	if d.opts.Strict {
		dec.DisallowUnknownFields()
	}

	if err := dec.Decode(req); err != nil {
		return d.wrapParseError(err, "json")
	}

	return nil
}

// decodeYAML parses YAML with optional strict mode.
// Uses safeyaml to protect against YAML bombs (size/depth/alias limits).
func (d *Decoder) decodeYAML(data []byte, req *RequestEnvelope) error {
	var err error
	if d.opts.Strict {
		err = safeyaml.UnmarshalStrict(data, req)
	} else {
		err = safeyaml.Unmarshal(data, req)
	}

	if err != nil {
		return d.wrapParseError(err, "yaml")
	}

	return nil
}

// wrapParseError converts parse errors to ValidationError.
func (d *Decoder) wrapParseError(err error, format string) error {
	if err == nil {
		return nil
	}

	// Handle JSON-specific errors
	var jsonSyntaxErr *json.SyntaxError
	if errors.As(err, &jsonSyntaxErr) {
		return &ValidationError{
			Code:    ErrCodeParseFailed,
			Message: fmt.Sprintf("invalid JSON at offset %d: %v", jsonSyntaxErr.Offset, jsonSyntaxErr),
		}
	}

	var jsonTypeErr *json.UnmarshalTypeError
	if errors.As(err, &jsonTypeErr) {
		return &ValidationError{
			Field:   jsonTypeErr.Field,
			Code:    ErrCodeInvalidType,
			Message: fmt.Sprintf("expected %s, got %s", jsonTypeErr.Type, jsonTypeErr.Value),
		}
	}

	// Handle unknown field errors (from strict mode)
	errStr := err.Error()
	if strings.Contains(errStr, "unknown field") {
		// Extract field name from error message
		field := extractFieldFromError(errStr)
		return &ValidationError{
			Field:   field,
			Code:    ErrCodeUnknownField,
			Message: "unknown field in request",
		}
	}

	// Generic parse error
	return &ValidationError{
		Code:    ErrCodeParseFailed,
		Message: fmt.Sprintf("failed to parse %s: %v", format, err),
	}
}

// extractFieldFromError attempts to extract a field name from an error message.
func extractFieldFromError(errMsg string) string {
	// JSON: "unknown field \"foo\""
	// YAML: "field foo not found"
	if idx := strings.Index(errMsg, "\""); idx >= 0 {
		end := strings.Index(errMsg[idx+1:], "\"")
		if end >= 0 {
			return errMsg[idx+1 : idx+1+end]
		}
	}
	return ""
}

// applyDefaults sets default values for missing optional fields.
func (d *Decoder) applyDefaults(req *RequestEnvelope) {
	// Output defaults
	if req.Output.Format == "" {
		req.Output.Format = "svg"
	}
	if req.Output.Width == 0 {
		req.Output.Width = 800
	}
	if req.Output.Height == 0 {
		req.Output.Height = 600
	}
	if req.Output.Scale == 0 {
		req.Output.Scale = 2.0
	}

	// Style defaults
	if req.Style.FontFamily == "" {
		req.Style.FontFamily = "Arial"
	}
	if req.Style.Background == "" {
		if req.Output.Format == "svg" {
			req.Style.Background = "transparent"
		} else {
			req.Style.Background = "white"
		}
	}
}

// validate performs semantic validation on the request.
func (d *Decoder) validate(req *RequestEnvelope) error {
	errs := &ValidationErrors{}

	// Required field: type
	if req.Type == "" {
		errs.Add(ValidationError{
			Field:   "type",
			Code:    ErrCodeRequired,
			Message: "diagram type is required. Set \"type\" to a diagram name, e.g. \"bar_chart\", \"pie_chart\", \"timeline\", \"org_chart\"",
		})
	}

	// Required field: data
	if req.Data == nil {
		errs.Add(ValidationError{
			Field:   "data",
			Code:    ErrCodeRequired,
			Message: "data payload is required. Provide a \"data\" object with the diagram's input fields",
		})
	}

	// Validate output format if specified
	if req.Output.Format != "" {
		validFormats := map[string]bool{"svg": true, "png": true, "pdf": true}
		if !validFormats[req.Output.Format] {
			errs.Add(ValidationError{
				Field:   "output.format",
				Code:    ErrCodeInvalidValue,
				Message: "format must be 'svg', 'png', or 'pdf'",
				Value:   req.Output.Format,
			})
		}
	}

	// Validate dimensions if specified
	if req.Output.Width < 0 {
		errs.Add(ValidationError{
			Field:   "output.width",
			Code:    ErrCodeConstraint,
			Message: "width must be non-negative",
			Value:   req.Output.Width,
		})
	}
	if req.Output.Height < 0 {
		errs.Add(ValidationError{
			Field:   "output.height",
			Code:    ErrCodeConstraint,
			Message: "height must be non-negative",
			Value:   req.Output.Height,
		})
	}
	if req.Output.Scale != 0 && (req.Output.Scale < MinSVGScale || req.Output.Scale > MaxSVGScale) {
		errs.Add(ValidationError{
			Field:   "output.scale",
			Code:    ErrCodeConstraint,
			Message: fmt.Sprintf("scale must be between %.1f and %.1f", MinSVGScale, MaxSVGScale),
			Value:   req.Output.Scale,
		})
	}

	return errs.AsError()
}

// DecodeStrict is a convenience function for strict parsing.
func DecodeStrict(data []byte) (*RequestEnvelope, error) {
	opts := DefaultDecodeOptions()
	opts.Strict = true
	return NewDecoder(opts).Decode(data)
}

// DecodeJSON parses JSON input with default options.
func DecodeJSON(data []byte) (*RequestEnvelope, error) {
	opts := DefaultDecodeOptions()
	opts.Format = "json"
	return NewDecoder(opts).Decode(data)
}

// DecodeYAML parses YAML input with default options.
func DecodeYAML(data []byte) (*RequestEnvelope, error) {
	opts := DefaultDecodeOptions()
	opts.Format = "yaml"
	return NewDecoder(opts).Decode(data)
}
