// Package diagnostics — describe-finding registry.
//
// describe-finding is the single read surface an agent uses to resolve the
// meaning of any finding code emitted anywhere in the pipeline, in one extra
// tool call, without scanning docs. This file backs it from the shared
// diagnostics taxonomy so the metadata covers the dotted-namespace codes
// (TPL/FIT/GRID/RENDER/POLICY/INPUT) emitted by MCP, CLI, HTTP generation,
// validation, repair, render, inspect, palette audit, and output validation —
// not just the patterns-only fit findings.
//
// Lookup is unified through Describe: the patterns.FindingMeta registry owns the
// lowercase fit/chart/pattern codes (and the few SCREAMING_SNAKE codes it
// already documents, e.g. the TEMPLATE_METADATA_* warnings and UNKNOWN_ENUM),
// and codeMetaRegistry below owns the remaining diagnostics.AllCodes() codes.
// TestDescribeCoversAllDiagnosticCodes asserts that every declared code resolves,
// so adding a code to codes.go without a describe entry fails CI.
package diagnostics

import (
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Severity ranks reused from the describe_finding output schema. Diagnostics
// codes are MCP error / warning results, so they map onto "refuse" (the engine
// refused to proceed) or "review" (advisory; the run continued).
const (
	describeSeverityRefuse = "refuse"
	describeSeverityReview = "review"
)

// Describe returns the agent-facing metadata for a finding code, or
// (nil, false) when no entry exists. The code may be a bare legacy code
// ("MISSING_PARAMETER", "placeholder_overflow", "chart.zero_sum_pie") or a
// dotted namespaced code from a finding envelope ("INPUT.MISSING_PARAMETER",
// "FIT.placeholder_overflow") — the namespace prefix is stripped before lookup
// so the describe_command examples emitted on the wire are runnable verbatim.
func Describe(code string) (*patterns.FindingMeta, bool) {
	legacy := stripNamespacePrefix(code)
	if m, ok := patterns.GetFindingMeta(legacy); ok {
		return m, true
	}
	if m, ok := codeMetaRegistry[legacy]; ok {
		out := m
		return &out, true
	}
	return nil, false
}

// AllDescribableCodes returns the sorted union of every code that Describe can
// resolve: the patterns fit/chart/pattern codes plus the diagnostics codes
// declared here. describe_finding advertises this list on the unknown-code
// error path.
func AllDescribableCodes() []string {
	set := make(map[string]struct{}, len(codeMetaRegistry)+64)
	for _, c := range patterns.AllFindingMetaCodes() {
		set[c] = struct{}{}
	}
	for c := range codeMetaRegistry {
		set[c] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// stripNamespacePrefix removes a leading "<NS>." when NS is one of the declared
// finding namespaces. Non-namespace dotted prefixes (e.g. "chart.") are left
// intact so they reach the patterns registry verbatim.
func stripNamespacePrefix(code string) string {
	for _, ns := range AllNamespaces() {
		if strings.HasPrefix(code, ns+".") {
			return code[len(ns)+1:]
		}
	}
	return code
}

// codeMetaRegistry documents every diagnostics.AllCodes() code that the
// patterns registry does not already own. Keep it in sync with codes.go:
// TestDescribeCoversAllDiagnosticCodes fails the build when a declared code has
// no entry here or in the patterns registry.
var codeMetaRegistry = map[string]patterns.FindingMeta{
	// ---- Input family — request / JSON-payload problems ----

	CodeMissingParameter: {
		Code:        CodeMissingParameter,
		Summary:     "A required tool or CLI argument was not supplied.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Argument validation finds a required parameter absent or empty.",
		RemediationSteps: []string{
			"Supply the parameter named in evidence.path.",
			"Replay the call in next_tool_call with the missing field filled in; example_value shows a valid value.",
		},
		ExampleBefore: `repair_slide({"slide": {...}})  // "fixes" omitted`,
		ExampleAfter:  `repair_slide({"slide": {...}, "fixes": [{"kind": "reduce_text"}]})`,
		RelatedCodes:  []string{CodeInvalidParameter},
	},
	CodeInvalidParameter: {
		Code:        CodeInvalidParameter,
		Summary:     "A supplied argument has the wrong type or an illegal value.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Argument validation finds a parameter present but malformed (wrong JSON type or out-of-range value).",
		RemediationSteps: []string{
			"Correct the value at evidence.path to match evidence.expected_type.",
			"Consult get_input_schema for the parameter's accepted shape.",
		},
		RelatedCodes: []string{CodeMissingParameter, CodeInvalidJSON, CodeUnknownEnum},
	},
	CodeInvalidGrid: {
		Code:        CodeInvalidGrid,
		Summary:     "A shape_grid is structurally invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Dry-run / preflight finds a shape_grid whose rows, columns, or cell spans do not form a valid grid.",
		RemediationSteps: []string{
			"Fix the grid's rows/cols and cell row_span/col_span so cells tile the grid without gaps or overlaps.",
			"Start from a known-good skeleton via expand_pattern, then edit values.",
		},
		RelatedCodes: []string{CodePatternError, patterns.ErrCodeInvalidShape},
	},
	CodeInvalidJSON: {
		Code:        CodeInvalidJSON,
		Summary:     "The JSON payload could not be parsed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The deck JSON or request body is not syntactically valid JSON.",
		RemediationSteps: []string{
			"Fix the syntax error at the reported offset (unbalanced braces, trailing commas, unquoted keys).",
			"Validate the document with a JSON linter before resubmitting.",
		},
		RelatedCodes: []string{CodeInvalidKey, CodeInvalidParameter},
	},
	CodeInvalidKey: {
		Code:        CodeInvalidKey,
		Summary:     "An object contains a key the schema does not allow.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Strict key checking finds an unknown or misspelled field on a slide, content item, or value object.",
		RemediationSteps: []string{
			"Remove or rename the offending key at evidence.path.",
			"Check get_input_schema for the allowed field names at that level.",
		},
		RelatedCodes: []string{CodeInvalidParameter, patterns.ErrCodeUnknownKey},
	},
	CodeInvalidSlide: {
		Code:        CodeInvalidSlide,
		Summary:     "A slide object is structurally invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A slide cannot be interpreted — e.g. it has no resolvable layout_id/slide_type or malformed content.",
		RemediationSteps: []string{
			"Give the slide a valid layout_id or slide_type and well-formed content[].",
			"Compare the slide against get_input_schema.",
		},
		RelatedCodes: []string{CodeInvalidSlideIndex, CodeValidationFailed},
	},
	CodeInvalidSlideIndex: {
		Code:        CodeInvalidSlideIndex,
		Summary:     "A slide index argument is out of range.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A tool references a slide index that does not exist in the deck.",
		RemediationSteps: []string{
			"Use a 0-based index inside [0, slide_count).",
			"Read the deck length first if you are unsure how many slides exist.",
		},
		RelatedCodes: []string{CodeInvalidPath, CodeInvalidSlide},
	},
	CodeInvalidPath: {
		Code:        CodeInvalidPath,
		Summary:     "A JSON path or file path argument is malformed or unresolvable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A path argument cannot be parsed or does not resolve to a node/file.",
		RemediationSteps: []string{
			"Supply a valid path; for files prefer an absolute path.",
			"For JSON pointers, confirm each segment exists in the document.",
		},
		RelatedCodes: []string{CodeFileNotFound, CodeInvalidSlideIndex},
	},
	CodeAmbiguousInput: {
		Code:        CodeAmbiguousInput,
		Summary:     "The request could be interpreted more than one way.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Two mutually exclusive inputs were supplied (e.g. both inline JSON and a file path for the same argument).",
		RemediationSteps: []string{
			"Supply exactly one of the conflicting inputs.",
			"Check the message for which inputs collided.",
		},
		RelatedCodes: []string{CodeInvalidParameter},
	},
	CodeUnsupported: {
		Code:        CodeUnsupported,
		Summary:     "The requested operation or option is not supported.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A tool receives an operation or feature it does not implement (e.g. an unknown deck-patch op kind).",
		RemediationSteps: []string{
			"Use one of the supported values listed in fix.params.allowed.",
			"Check get_capabilities for the supported operation set.",
		},
		RelatedCodes: []string{CodeInvalidParameter, CodeUnknownEnum},
	},

	// ---- Template family — template lookup / parsing failures ----

	CodeTemplateNotFound: {
		Code:        CodeTemplateNotFound,
		Summary:     "The named template could not be found in the templates directory.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Template resolution cannot find a .pptx matching the requested template name.",
		RemediationSteps: []string{
			"Call list_templates and pick a listed name.",
			"Confirm --templates-dir points at the directory holding the .pptx files.",
		},
		RelatedCodes: []string{CodeTemplatesDir, CodeTemplateError},
	},
	CodeTemplateError: {
		Code:        CodeTemplateError,
		Summary:     "The template could not be parsed or is structurally invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Template parsing fails — corrupt archive, missing theme, or unreadable slide layouts.",
		RemediationSteps: []string{
			"Run validate-template to see the structural problem.",
			"Re-export the template, or fall back to a bundled template.",
		},
		RelatedCodes: []string{CodeTemplateNotFound, CodeTemplateMetadataParse},
	},
	CodeTemplatesDir: {
		Code:        CodeTemplatesDir,
		Summary:     "The templates directory is missing or unreadable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The configured templates directory does not exist or cannot be listed.",
		RemediationSteps: []string{
			"Pass a valid --templates-dir (MCP: templates_dir).",
			"Confirm the directory exists and contains .pptx files.",
		},
		RelatedCodes: []string{CodeTemplateNotFound},
	},

	// ---- Resource family — file and asset lookup failures ----

	CodeFileNotFound: {
		Code:        CodeFileNotFound,
		Summary:     "A referenced file does not exist.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A file path argument (deck JSON or an asset) cannot be opened.",
		RemediationSteps: []string{
			"Check the path; prefer an absolute path.",
			"Confirm the file exists and is readable by the process.",
		},
		RelatedCodes: []string{CodeInvalidPath, CodeReadFailed},
	},
	CodeStyleNotFound: {
		Code:        CodeStyleNotFound,
		Summary:     "A referenced named style is not defined.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A table/cell style_id or named style cannot be resolved against the template.",
		RemediationSteps: []string{
			"List the template's styles via list_templates and pick one.",
			"Or drop the style reference to fall back to the template default.",
		},
		RelatedCodes: []string{CodeUnknownTableStyleID},
	},
	CodeUnknownTableStyleID: {
		Code:        CodeUnknownTableStyleID,
		Summary:     "A table references a style_id the template does not define.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Table generation cannot find the requested style_id in the template's table_styles registry.",
		RemediationSteps: []string{
			"Pick a style_id from list_templates → table_styles.",
			"Or omit style_id to use the template default.",
		},
		RelatedCodes: []string{CodeStyleNotFound, patterns.ErrCodeUnknownTableStyleID},
	},
	CodeUnknownThemeColor: {
		Code:        CodeUnknownThemeColor,
		Summary:     "A semantic color name is not part of the template theme.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Color resolution receives a scheme name outside the accent1..accent6 / lt1 / dk1 / lt2 / dk2 set.",
		RemediationSteps: []string{
			"Use a valid scheme name; run resolve-theme to inspect the template's palette.",
			"Avoid raw hex unless the template's brand allowlist permits it.",
		},
		RelatedCodes: []string{CodeUnknownEnum, patterns.ErrCodeHexFillNonBrand},
	},
	CodeUnknownPattern: {
		Code:        CodeUnknownPattern,
		Summary:     "A pattern name is not registered.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Pattern expansion receives a name not present in the pattern registry.",
		RemediationSteps: []string{
			"Call list_patterns and pick a registered name.",
			"Use recommend_pattern to find a pattern that fits the content.",
		},
		RelatedCodes: []string{CodePatternError},
	},
	CodeIconPath: {
		Code:        CodeIconPath,
		Summary:     "An icon path argument is invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon path cannot be used (a general path failure not covered by a more specific ICON_PATH_* code).",
		RemediationSteps: []string{
			"Supply a valid icon path, or use a bundled icon name instead.",
			"List bundled icons via the icons command.",
		},
		RelatedCodes: []string{CodeIconNotFound, CodeIconPathExtInvalid},
	},
	CodeIconNotFound: {
		Code:        CodeIconNotFound,
		Summary:     "The requested icon could not be found.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon name or path does not resolve to a bundled icon or a readable file.",
		RemediationSteps: []string{
			"List icons and use a known name; check evidence.suggestions for near matches.",
			"For a file icon, confirm the path exists.",
		},
		RelatedCodes: []string{CodeIconAmbiguous, CodeIconBundledNameUnknown},
	},
	CodeIconPathExtInvalid: {
		Code:        CodeIconPathExtInvalid,
		Summary:     "An icon path has an unsupported file extension.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon path's extension is not one of the allowed image/SVG types.",
		RemediationSteps: []string{
			"Use a .svg or .png asset.",
			"Convert the source asset to a supported format.",
		},
		RelatedCodes: []string{CodeIconPath},
	},
	CodeIconPathTraversal: {
		Code:        CodeIconPathTraversal,
		Summary:     "An icon path attempts directory traversal outside the allowed root.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon path contains \"..\" segments that escape the asset root.",
		RemediationSteps: []string{
			"Place the asset under the allowed asset root.",
			"Reference it with a relative path that does not contain \"..\".",
		},
		RelatedCodes: []string{CodeIconPathSymlinkEscape, CodeIconPath},
	},
	CodeIconPathSymlinkEscape: {
		Code:        CodeIconPathSymlinkEscape,
		Summary:     "An icon path resolves through a symlink that escapes the allowed root.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Symlink resolution of an icon path lands outside the asset root.",
		RemediationSteps: []string{
			"Store the asset directly under the asset root rather than behind a symlink.",
			"Reference the real file location.",
		},
		RelatedCodes: []string{CodeIconPathTraversal, CodeIconPath},
	},
	CodeIconAmbiguous: {
		Code:        CodeIconAmbiguous,
		Summary:     "An icon name matches more than one bundled icon.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon name resolves to multiple bundled icons across icon sets.",
		RemediationSteps: []string{
			"Use the fully-qualified icon name from fix.params / evidence.suggestions.",
			"Disambiguate by prefixing the icon set.",
		},
		RelatedCodes: []string{CodeIconNotFound},
	},
	CodeIconMissing: {
		Code:        CodeIconMissing,
		Summary:     "An icon reference is empty.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon entry supplies none of name, path, or svg_data.",
		RemediationSteps: []string{
			"Supply exactly one of name, path, or svg_data on the icon entry.",
		},
		RelatedCodes: []string{CodeIconNotFound},
	},
	CodeIconList: {
		Code:        CodeIconList,
		Summary:     "Listing the available icons failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The bundled icon catalog cannot be enumerated.",
		RemediationSteps: []string{
			"Retry the request.",
			"If it persists, report the failure with the input_sha256.",
		},
		RelatedCodes: []string{CodeIconNotFound},
	},
	CodeIconBundledNameUnknown: {
		Code:        CodeIconBundledNameUnknown,
		Summary:     "A bundled icon name is not in the catalog.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon name does not match any bundled icon.",
		RemediationSteps: []string{
			"List icons and pick a known name.",
			"Check evidence.suggestions for the closest valid names.",
		},
		RelatedCodes: []string{CodeIconNotFound, CodeIconAmbiguous},
	},
	CodeIconFillIgnoredInline: {
		Code:        CodeIconFillIgnoredInline,
		Summary:     "An icon `fill` was ignored because inline `svg_data` is set.",
		Severity:    describeSeverityReview,
		WhenEmitted: "An icon supplies both inline svg_data and a fill color; the engine cannot recolor pre-rendered SVG markup, so fill is dropped.",
		RemediationSteps: []string{
			"Pre-color the inline svg_data markup directly.",
			"Or remove svg_data and use name/path with fill instead.",
		},
		ExampleBefore: `{"icon": {"svg_data": "<svg>...</svg>", "fill": "accent1"}}`,
		ExampleAfter:  `{"icon": {"name": "rocket", "fill": "accent1"}}`,
	},
	CodeImagePath: {
		Code:        CodeImagePath,
		Summary:     "An image path is invalid or unresolvable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An image_value path cannot be opened or is an unsupported format.",
		RemediationSteps: []string{
			"Check the path and use a supported image format (PNG/JPG).",
			"Confirm the file is readable by the process.",
		},
		RelatedCodes: []string{CodeFileNotFound, CodeBackgroundImagePath},
	},
	CodeBackgroundImagePath: {
		Code:        CodeBackgroundImagePath,
		Summary:     "A slide background image path is invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A slide's background image path cannot be opened or is an unsupported format.",
		RemediationSteps: []string{
			"Check the background image path and format.",
			"Confirm the file is readable.",
		},
		RelatedCodes: []string{CodeImagePath},
	},
	CodeAssetPathEnvUnset: {
		Code:        CodeAssetPathEnvUnset,
		Summary:     "An asset path requires an environment variable that is not set.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An asset is referenced via an env-rooted path but the named environment variable is unset.",
		RemediationSteps: []string{
			"Set the environment variable named in the message.",
			"Or reference the asset with a direct path instead of an env-rooted one.",
		},
		RelatedCodes: []string{CodeImagePath, CodeIconPath},
	},
	CodeAssetTooLarge: {
		Code:        CodeAssetTooLarge,
		Summary:     "An asset exceeds the maximum allowed size.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An icon or image asset's byte size exceeds the configured cap (see get_capabilities for the limit).",
		RemediationSteps: []string{
			"Recompress or downscale the asset to fit under the limit.",
			"Use a vector (SVG) icon instead of a large raster image.",
		},
		RelatedCodes: []string{CodeImagePath},
	},
	CodeURLFetchFailed: {
		Code:        CodeURLFetchFailed,
		Summary:     "A remote asset URL could not be fetched.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An http(s) asset fetch fails — network error, non-200 status, or timeout.",
		RemediationSteps: []string{
			"Check the URL and network reachability.",
			"Download the asset and reference it by local path instead.",
		},
		RelatedCodes: []string{CodeURLResolverInit},
	},
	CodeURLResolverInit: {
		Code:        CodeURLResolverInit,
		Summary:     "The URL asset resolver failed to initialize.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The remote-asset resolver cannot start (e.g. cache directory or configuration problem).",
		RemediationSteps: []string{
			"Check the resolver's cache directory and permissions.",
			"Retry; reference assets locally if remote fetch is unavailable.",
		},
		RelatedCodes: []string{CodeURLFetchFailed},
	},
	CodeSVGInvalidRoot: {
		Code:        CodeSVGInvalidRoot,
		Summary:     "Inline SVG data has an invalid root element.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "svg_data does not begin with a valid <svg> root element.",
		RemediationSteps: []string{
			"Ensure the markup is a well-formed SVG document with an <svg> root.",
		},
		RelatedCodes: []string{CodeSVGParseError, CodeSVGUnsafeXML},
	},
	CodeSVGUnsafeXML: {
		Code:        CodeSVGUnsafeXML,
		Summary:     "Inline SVG contains unsafe XML (DTD, entities, or external references).",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The SVG safety scan finds disallowed XML constructs that could trigger entity expansion or external fetches.",
		RemediationSteps: []string{
			"Remove DOCTYPE/entity declarations and external references from the SVG.",
			"Sanitize the SVG with a trusted tool before embedding.",
		},
		RelatedCodes: []string{CodeSVGParseError, CodeSVGInvalidRoot},
	},
	CodeSVGParseError: {
		Code:        CodeSVGParseError,
		Summary:     "Inline SVG data could not be parsed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "SVG XML parsing fails on the supplied svg_data.",
		RemediationSteps: []string{
			"Fix the SVG markup; validate it with an SVG tool.",
		},
		RelatedCodes: []string{CodeSVGInvalidRoot, CodeSVGUnsafeXML},
	},

	// ---- Render family — generation and rendering failures ----

	CodeGenerationFailed: {
		Code:        CodeGenerationFailed,
		Summary:     "Deck generation failed before output was written.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The generation pipeline returns an error prior to producing the .pptx.",
		RemediationSteps: []string{
			"Read the message for the failing stage.",
			"Run validate / preflight on the deck to isolate the cause, then retry.",
		},
		RelatedCodes: []string{CodeRenderFailed, CodeValidationFailed},
	},
	CodeReadFailed: {
		Code:        CodeReadFailed,
		Summary:     "The input deck or a referenced file could not be read.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Reading the deck JSON or a referenced file fails (I/O error or permissions).",
		RemediationSteps: []string{
			"Check the path and file permissions.",
			"Confirm the deck JSON is present and well-formed.",
		},
		RelatedCodes: []string{CodeFileNotFound, CodeInvalidJSON},
	},
	CodeRenderFailed: {
		Code:        CodeRenderFailed,
		Summary:     "Rendering the deck to PPTX or images failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A render step (OOXML assembly or image conversion) returns an error.",
		RemediationSteps: []string{
			"Read the message for the failing render stage.",
			"For image output, confirm LibreOffice and ImageMagick are available.",
		},
		RelatedCodes: []string{CodeGenerationFailed, CodeLibreOfficeUnavailable},
	},
	CodeLibreOfficeUnavailable: {
		Code:        CodeLibreOfficeUnavailable,
		Summary:     "LibreOffice is required for this operation but is not available.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A PPTX→image step needs LibreOffice and it is not installed or not on PATH.",
		RemediationSteps: []string{
			"Install LibreOffice and ensure the soffice binary is on PATH.",
			"Skip image conversion if only the .pptx is needed.",
		},
		RelatedCodes: []string{CodeImageMagickUnavailable, CodeRenderFailed},
	},
	CodeImageMagickUnavailable: {
		Code:        CodeImageMagickUnavailable,
		Summary:     "ImageMagick is required for this operation but is not available.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An image conversion step needs ImageMagick and it is not installed or not on PATH.",
		RemediationSteps: []string{
			"Install ImageMagick and ensure convert/magick is on PATH.",
			"Skip image conversion if only the .pptx is needed.",
		},
		RelatedCodes: []string{CodeLibreOfficeUnavailable, CodeRenderFailed},
	},
	CodeLibreOfficeTimeout: {
		Code:        CodeLibreOfficeTimeout,
		Summary:     "LibreOffice exceeded its render deadline and was killed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A PPTX→PDF conversion subprocess ran past its bounded deadline; its process group was terminated so it could not hold the render lock indefinitely.",
		RemediationSteps: []string{
			"Retry the render (pass force=true) — a single wedged conversion is often transient.",
			"If it recurs, the LibreOffice environment is likely stuck: restart it, or skip image rendering and ship the .pptx without thumbnails.",
		},
		RelatedCodes: []string{CodeImageMagickTimeout, CodeLibreOfficeUnavailable, CodeRenderFailed},
	},
	CodeImageMagickTimeout: {
		Code:        CodeImageMagickTimeout,
		Summary:     "ImageMagick exceeded its render deadline and was killed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A PDF→PNG conversion subprocess ran past its bounded deadline and its process group was terminated.",
		RemediationSteps: []string{
			"Retry the render (pass force=true).",
			"If it recurs, lower the render density or skip image rendering and ship the .pptx without thumbnails.",
		},
		RelatedCodes: []string{CodeLibreOfficeTimeout, CodeImageMagickUnavailable, CodeRenderFailed},
	},
	CodeOutputDir: {
		Code:        CodeOutputDir,
		Summary:     "The output directory is missing or not writable.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "The engine cannot create or write to the requested output directory.",
		RemediationSteps: []string{
			"Pass a writable --output directory.",
			"Check directory permissions and available disk space.",
		},
		RelatedCodes: []string{CodeRenderFailed},
	},
	CodePatternError: {
		Code:        CodePatternError,
		Summary:     "A pattern failed to expand.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Pattern expansion returns an error from bad values or an internal failure.",
		RemediationSteps: []string{
			"Validate the pattern values with validate_pattern.",
			"Inspect the contract via show_pattern and correct the values.",
		},
		RelatedCodes: []string{CodeUnknownPattern, CodeInvalidGrid},
	},
	CodeStrictFit: {
		Code:        CodeStrictFit,
		Summary:     "Strict-fit mode refused the deck because content overflows.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "generate runs with strict_fit enabled and a fit finding carries a refuse-level action.",
		RemediationSteps: []string{
			"Fix the underlying fit finding (split or shorten the offending content).",
			"Or lower strict-fit to warn to allow the engine's shrink/truncate fallback.",
		},
		RelatedCodes: []string{patterns.ErrCodeFitOverflow, patterns.ErrCodePlaceholderOverflow, patterns.ErrCodeDensityExceeded},
	},
	CodeValidationFailed: {
		Code:        CodeValidationFailed,
		Summary:     "Input validation failed with at least one error-severity finding.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "validate_input (or the validate gate before generation) reports an error-severity problem.",
		RemediationSteps: []string{
			"Read each finding in the envelope and fix it.",
			"Re-run validate until ok is true, then generate.",
		},
		RelatedCodes: []string{CodeGenerationFailed, CodeInvalidSlide},
	},
	CodeOutputValidationError: {
		Code:        CodeOutputValidationError,
		Summary:     "The generated PPTX failed structural output validation.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Post-generation validate-output finds the produced .pptx structurally invalid.",
		RemediationSteps: []string{
			"Report the failure with the offending deck for diagnosis.",
			"Re-run with a simpler deck to isolate the slide that triggers it.",
		},
		RelatedCodes: []string{CodeRenderFailed},
	},
	CodeOverlayFailed: {
		Code:        CodeOverlayFailed,
		Summary:     "Rendering an annotated overlay failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A preview/examine overlay (the annotated SVG/PNG) could not be produced.",
		RemediationSteps: []string{
			"Retry; the underlying report is still produced without the overlay.",
			"Check that the overlay inputs (layout geometry) resolved.",
		},
		RelatedCodes: []string{CodeRenderFailed},
	},

	// ---- Settings family — template settings operations ----

	CodeSettingsError: {
		Code:        CodeSettingsError,
		Summary:     "A template-settings operation failed.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "Reading or writing template settings returns an error.",
		RemediationSteps: []string{
			"Check the settings store path and permissions.",
			"Retry the operation.",
		},
		RelatedCodes: []string{CodeSettingsWriteDisabled},
	},
	CodeSettingsWriteDisabled: {
		Code:        CodeSettingsWriteDisabled,
		Summary:     "Writing template settings is disabled.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A write to template settings is attempted while settings writes are disabled by configuration.",
		RemediationSteps: []string{
			"Enable settings writes in the server configuration.",
			"Run in an environment that permits settings writes.",
		},
		RelatedCodes: []string{CodeSettingsError},
	},

	// ---- Inspect family — visual QA failures ----

	CodeInvalidImage: {
		Code:        CodeInvalidImage,
		Summary:     "An image supplied for inspection is invalid.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "inspect_slide_images receives an unreadable or malformed image.",
		RemediationSteps: []string{
			"Supply a valid PNG/JPG image.",
			"Re-render the slide to images before inspecting.",
		},
		RelatedCodes: []string{CodeInspectDisabled},
	},
	CodeInspectDisabled: {
		Code:        CodeInspectDisabled,
		Summary:     "Visual inspection is disabled.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "inspect is invoked while the visual-QA backend is disabled or unconfigured.",
		RemediationSteps: []string{
			"Enable and configure the inspect backend.",
			"Skip inspection if the visual-QA agent is unavailable.",
		},
		RelatedCodes: []string{CodeInvalidImage},
	},
	CodeVisionTimeout: {
		Code:        CodeVisionTimeout,
		Summary:     "A vision inspection API call exceeded its deadline.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "A Claude vision request during inspect_slide_images (or the visual_qa loop) ran past its per-request or total inspection deadline.",
		RemediationSteps: []string{
			"Retry the inspection — a single stalled API call is usually transient.",
			"If it recurs, reduce parallelism or fall back to the heuristic inspector (unset ANTHROPIC_API_KEY) to degrade gracefully.",
		},
		RelatedCodes: []string{CodeInspectDisabled, CodeInvalidImage},
	},

	// ---- Internal family — unexpected server-side failures ----

	CodeInternal: {
		Code:        CodeInternal,
		Summary:     "An unexpected internal error occurred.",
		Severity:    describeSeverityRefuse,
		WhenEmitted: "An unhandled server-side failure surfaces; the operation could not complete.",
		RemediationSteps: []string{
			"Retry the request.",
			"If it persists, report the failure with the input_sha256 for correlation.",
		},
	},
}
