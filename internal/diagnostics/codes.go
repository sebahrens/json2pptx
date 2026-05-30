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
	CodeMissingParameter  Code = "MISSING_PARAMETER"
	CodeInvalidParameter  Code = "INVALID_PARAMETER"
	CodeInvalidJSON       Code = "INVALID_JSON"
	CodeInvalidKey        Code = "INVALID_KEY"
	CodeInvalidGrid       Code = "INVALID_GRID"
	CodeInvalidSlide      Code = "INVALID_SLIDE"
	CodeInvalidSlideIndex Code = "INVALID_SLIDE_INDEX"
	CodeInvalidPath       Code = "INVALID_PATH"
	CodeAmbiguousInput    Code = "AMBIGUOUS_INPUT"
	CodeUnsupported       Code = "UNSUPPORTED"

	// CodeIdempotencyConflict is emitted when a caller reuses an idempotency_key
	// for a request whose normalized fingerprint differs from the cached one —
	// replaying would return a deck for the wrong content.
	CodeIdempotencyConflict Code = "IDEMPOTENCY_CONFLICT"
)

// Template family — template lookup, parsing, and metadata-validation failures.
const (
	CodeTemplateNotFound Code = "TEMPLATE_NOT_FOUND"
	CodeTemplateError    Code = "TEMPLATE_ERROR"
	CodeTemplatesDir     Code = "TEMPLATES_DIR"

	// Template-metadata validation findings emitted by validate-template.
	CodeTemplateMetadataParse       Code = "TEMPLATE_METADATA_PARSE"
	CodeTemplateMetadataVersion     Code = "TEMPLATE_METADATA_VERSION"
	CodeTemplateAspectRatioInvalid  Code = "TEMPLATE_ASPECT_RATIO_INVALID"
	CodeTemplateLayoutHintInvalid   Code = "TEMPLATE_LAYOUT_HINT_INVALID"
	CodeTemplateSectionNumberNaming Code = "TEMPLATE_SECTION_NUMBER_NAMING"
)

// Resource family — file and asset lookup failures.
const (
	CodeFileNotFound           Code = "FILE_NOT_FOUND"
	CodeStyleNotFound          Code = "STYLE_NOT_FOUND"
	CodeUnknownTableStyleID    Code = "UNKNOWN_TABLE_STYLE_ID"
	CodeUnknownThemeColor      Code = "UNKNOWN_THEME_COLOR"
	CodeUnknownPattern         Code = "UNKNOWN_PATTERN"
	CodeUnknownEnum            Code = "UNKNOWN_ENUM"
	CodeIconPath               Code = "ICON_PATH"
	CodeIconNotFound           Code = "ICON_NOT_FOUND"
	CodeIconPathExtInvalid     Code = "ICON_PATH_EXT_INVALID"
	CodeIconPathTraversal      Code = "ICON_PATH_TRAVERSAL"
	CodeIconPathSymlinkEscape  Code = "ICON_PATH_SYMLINK_ESCAPE"
	CodeIconAmbiguous          Code = "ICON_AMBIGUOUS"
	CodeIconMissing            Code = "ICON_MISSING"
	CodeIconList               Code = "ICON_LIST"
	CodeIconBundledNameUnknown Code = "ICON_BUNDLED_NAME_UNKNOWN"
	CodeIconFillIgnoredInline  Code = "ICON_FILL_IGNORED_ON_INLINE"
	CodeImagePath              Code = "IMAGE_PATH"
	CodeBackgroundImagePath    Code = "BACKGROUND_IMAGE_PATH"
	CodeAssetPathEnvUnset      Code = "ASSET_PATH_ENV_UNSET"
	CodeAssetTooLarge          Code = "ASSET_TOO_LARGE"
	CodeURLFetchFailed         Code = "URL_FETCH_FAILED"
	CodeURLResolverInit        Code = "URL_RESOLVER_INIT"
	CodeSVGInvalidRoot         Code = "SVG_INVALID_ROOT"
	CodeSVGUnsafeXML           Code = "SVG_UNSAFE_XML"
	CodeSVGParseError          Code = "SVG_PARSE_ERROR"
)

// Render family — generation and rendering failures.
const (
	CodeGenerationFailed       Code = "GENERATION_FAILED"
	CodeReadFailed             Code = "READ_FAILED"
	CodeRenderFailed           Code = "RENDER_FAILED"
	CodeLibreOfficeUnavailable Code = "LIBREOFFICE_UNAVAILABLE"
	CodeImageMagickUnavailable Code = "IMAGEMAGICK_UNAVAILABLE"
	CodeLibreOfficeTimeout     Code = "LIBREOFFICE_TIMEOUT"
	CodeImageMagickTimeout     Code = "IMAGEMAGICK_TIMEOUT"
	CodeOutputDir              Code = "OUTPUT_DIR"
	CodePatternError           Code = "PATTERN_ERROR"
	CodeStrictFit              Code = "STRICT_FIT"
	CodeValidationFailed       Code = "VALIDATION_FAILED"
	CodeOutputValidationError  Code = "OUTPUT_VALIDATION_ERROR"
	CodeOverlayFailed          Code = "OVERLAY_FAILED"
)

// Settings family — template settings operations.
const (
	CodeSettingsError         Code = "SETTINGS_ERROR"
	CodeSettingsWriteDisabled Code = "SETTINGS_WRITE_DISABLED"
)

// Inspect family — visual QA failures.
const (
	CodeInvalidImage    Code = "INVALID_IMAGE"
	CodeInspectDisabled Code = "INSPECT_DISABLED"
	CodeVisionTimeout   Code = "VISION_TIMEOUT"

	// CodeVisionInspectionFailed marks a per-slide vision inspection that did not
	// produce findings because the Claude vision backend failed: an API error
	// (non-200 status), a transport/decode error, or malformed model output. It
	// is distinct from a clean inspection with zero defects, so an agent never
	// mistakes a backend failure for a passing slide.
	CodeVisionInspectionFailed Code = "VISION_INSPECTION_FAILED"

	// CodeHeuristicInspectionFailed marks a per-slide heuristic inspection that
	// could not run (e.g. the image bytes failed to decode). It stays clearly
	// labeled heuristic/degraded so it is never mixed with vision-backed defects.
	CodeHeuristicInspectionFailed Code = "HEURISTIC_INSPECTION_FAILED"
)

// Semantic family — compact semantic deck-spec validation gates.
//
// These are emitted by internal/semantic.Validate when a semantic DeckSpec
// violates an authoring rule (missing required field, unknown kind/archetype,
// out-of-range density, missing takeaway, placeholder/weak content). They live
// in the INPUT namespace (see classifyMap in envelope.go).
const (
	CodeSemanticRequired         Code = "SEMANTIC_REQUIRED"
	CodeSemanticUnknownKind      Code = "SEMANTIC_UNKNOWN_KIND"
	CodeSemanticUnknownArchetype Code = "SEMANTIC_UNKNOWN_ARCHETYPE"
	CodeSemanticTakeawayRequired Code = "SEMANTIC_TAKEAWAY_REQUIRED"
	CodeSemanticDensity          Code = "SEMANTIC_DENSITY"
	CodeSemanticWeakContent      Code = "SEMANTIC_WEAK_CONTENT"
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
		CodeIdempotencyConflict,
		// Template
		CodeTemplateNotFound,
		CodeTemplateError,
		CodeTemplatesDir,
		CodeTemplateMetadataParse,
		CodeTemplateMetadataVersion,
		CodeTemplateAspectRatioInvalid,
		CodeTemplateLayoutHintInvalid,
		CodeTemplateSectionNumberNaming,
		// Resource
		CodeFileNotFound,
		CodeStyleNotFound,
		CodeUnknownTableStyleID,
		CodeUnknownThemeColor,
		CodeUnknownPattern,
		CodeUnknownEnum,
		CodeIconPath,
		CodeIconNotFound,
		CodeIconPathExtInvalid,
		CodeIconPathTraversal,
		CodeIconPathSymlinkEscape,
		CodeIconAmbiguous,
		CodeIconMissing,
		CodeIconList,
		CodeIconBundledNameUnknown,
		CodeIconFillIgnoredInline,
		CodeImagePath,
		CodeBackgroundImagePath,
		CodeAssetPathEnvUnset,
		CodeAssetTooLarge,
		CodeURLFetchFailed,
		CodeURLResolverInit,
		CodeSVGInvalidRoot,
		CodeSVGUnsafeXML,
		CodeSVGParseError,
		// Render
		CodeGenerationFailed,
		CodeReadFailed,
		CodeRenderFailed,
		CodeLibreOfficeUnavailable,
		CodeImageMagickUnavailable,
		CodeLibreOfficeTimeout,
		CodeImageMagickTimeout,
		CodeOutputDir,
		CodePatternError,
		CodeStrictFit,
		CodeValidationFailed,
		CodeOutputValidationError,
		CodeOverlayFailed,
		// Settings
		CodeSettingsError,
		CodeSettingsWriteDisabled,
		// Inspect
		CodeInvalidImage,
		CodeInspectDisabled,
		CodeVisionTimeout,
		CodeVisionInspectionFailed,
		CodeHeuristicInspectionFailed,
		// Semantic
		CodeSemanticRequired,
		CodeSemanticUnknownKind,
		CodeSemanticUnknownArchetype,
		CodeSemanticTakeawayRequired,
		CodeSemanticDensity,
		CodeSemanticWeakContent,
		// Internal
		CodeInternal,
	}
}
