// Package diagnostics — error code taxonomy.
//
// Every code emitted via MCPSimpleError / MCPDiagnosticsError MUST be declared
// here as a typed constant. Inline string literals are prohibited in new code.
// Codes are SCREAMING_SNAKE_CASE, grouped by family.
package diagnostics

// Code is a machine-readable error identifier. All codes MUST be
// SCREAMING_SNAKE_CASE and declared in this file.
type Code = string

// Input family — problems with request parameters or JSON payloads.
const (
	CodeMissingParameter Code = "MISSING_PARAMETER"
	CodeInvalidParameter Code = "INVALID_PARAMETER"
	CodeInvalidJSON      Code = "INVALID_JSON"
	CodeInvalidKey       Code = "INVALID_KEY"
	CodeInvalidGrid       Code = "INVALID_GRID"
	CodeInvalidSlide      Code = "INVALID_SLIDE"
	CodeInvalidSlideIndex Code = "INVALID_SLIDE_INDEX"
	CodeInvalidPath      Code = "INVALID_PATH"
	CodeAmbiguousInput    Code = "AMBIGUOUS_INPUT"
	CodeUnsupported      Code = "UNSUPPORTED"
)

// Template family — template lookup and parsing failures.
const (
	CodeTemplateNotFound Code = "TEMPLATE_NOT_FOUND"
	CodeTemplateError    Code = "TEMPLATE_ERROR"
	CodeTemplatesDir     Code = "TEMPLATES_DIR"
)

// Resource family — file and asset lookup failures.
const (
	CodeFileNotFound Code = "FILE_NOT_FOUND"
	CodeStyleNotFound Code = "STYLE_NOT_FOUND"
	CodeUnknownTableStyleID Code = "UNKNOWN_TABLE_STYLE_ID"
	CodeUnknownThemeColor   Code = "UNKNOWN_THEME_COLOR"
	CodeUnknownPattern      Code = "UNKNOWN_PATTERN"
	CodeUnknownEnum         Code = "UNKNOWN_ENUM"
	CodeIconPath            Code = "ICON_PATH"
	CodeIconList            Code = "ICON_LIST"
)

// Render family — generation and rendering failures.
const (
	CodeGenerationFailed       Code = "GENERATION_FAILED"
	CodeReadFailed             Code = "READ_FAILED"
	CodeRenderFailed           Code = "RENDER_FAILED"
	CodeLibreOfficeUnavailable Code = "LIBREOFFICE_UNAVAILABLE"
	CodeImageMagickUnavailable Code = "IMAGEMAGICK_UNAVAILABLE"
	CodeOutputDir              Code = "OUTPUT_DIR"
	CodePatternError           Code = "PATTERN_ERROR"
	CodeStrictFit              Code = "STRICT_FIT"
	CodeValidationFailed       Code = "VALIDATION_FAILED"
	CodeOutputValidationError  Code = "OUTPUT_VALIDATION_ERROR"
)

// Settings family — template settings operations.
const (
	CodeSettingsError        Code = "SETTINGS_ERROR"
	CodeSettingsWriteDisabled Code = "SETTINGS_WRITE_DISABLED"
)

// Internal family — unexpected server-side failures.
const (
	CodeInternal Code = "INTERNAL"
)

// AllCodes returns the complete list of declared error codes. Used by tests
// to verify no undeclared codes are emitted at runtime.
func AllCodes() []Code {
	return []Code{
		// Input
		CodeMissingParameter,
		CodeInvalidParameter,
		CodeInvalidGrid,
		CodeInvalidJSON,
		CodeInvalidKey,
		CodeInvalidSlide,
		CodeInvalidSlideIndex,
		CodeInvalidPath,
		CodeAmbiguousInput,
		CodeUnsupported,
		// Template
		CodeTemplateNotFound,
		CodeTemplateError,
		CodeTemplatesDir,
		// Resource
		CodeFileNotFound,
		CodeStyleNotFound,
		CodeUnknownTableStyleID,
		CodeUnknownThemeColor,
		CodeUnknownPattern,
		CodeUnknownEnum,
		CodeIconPath,
		CodeIconList,
		// Render
		CodeGenerationFailed,
		CodeReadFailed,
		CodeRenderFailed,
		CodeLibreOfficeUnavailable,
		CodeImageMagickUnavailable,
		CodeOutputDir,
		CodePatternError,
		CodeStrictFit,
		CodeValidationFailed,
		CodeOutputValidationError,
		// Settings
		CodeSettingsError,
		CodeSettingsWriteDisabled,
		// Internal
		CodeInternal,
	}
}
