package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// dryRunOutput is the top-level JSON printed to stdout in dry-run mode and
// returned by the validate_input MCP tool on the success path. Every warning,
// error, validation finding, and fit finding is folded into the single Findings
// envelope by buildFindingsEnvelope; the legacy parallel arrays
// (warnings/errors/validation_warnings/diagnostics/fit_findings) have been
// removed from the wire. The structural fields (counts, slides) are preserved.
// See docs/AGENT_DIAGNOSTICS.md for the envelope contract.
type dryRunOutput struct {
	Valid        bool          `json:"valid"`
	SlideCount   int           `json:"slide_count"`
	ChartCount   int           `json:"chart_count"`
	DiagramCount int           `json:"diagram_count"`
	TableCount   int           `json:"table_count"`
	ShapeCount   int           `json:"shape_count"`
	Slides       []dryRunSlide `json:"slides"`

	// Findings is the single agent-facing diagnostic surface: a FindingEnvelope
	// carrying every warning, error, validation finding, and fit finding for the
	// deck. It is populated by buildFindingsEnvelope just before serialization,
	// so a freshly built dryRunOutput (e.g. a unit test that calls
	// validateSlidesAgainstTemplate directly) carries an empty envelope and
	// populates the Diagnostics/FitFindings accumulators instead.
	Findings diagnostics.FindingEnvelope `json:"findings"`

	// ResponseFingerprint is a sha256 hex digest of the canonical JSON of this
	// response with the field zeroed. Agents may use it as a cache key.
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`

	// Diagnostics and FitFindings are internal accumulators that
	// buildFindingsEnvelope folds into Findings. They are never serialized — the
	// wire carries only the Findings envelope. Diagnostics is the complete
	// superset of every warning, error, and validation finding produced during
	// validation.
	Diagnostics []diagnostics.Diagnostic `json:"-"`
	FitFindings []patterns.FitFinding    `json:"-"`

	// Envelope metadata captured during validation and stamped onto Findings by
	// buildFindingsEnvelope.
	subcommand  string
	template    string
	inputSHA256 string
}

// buildFindingsEnvelope folds the accumulated Diagnostics and FitFindings into
// the single Findings envelope. Call it exactly once, just before serialization;
// the error-severity findings determine Findings.OK.
//
// The combined diagnostics are stably sorted by descending severity so
// findings[0] is the highest-severity issue — the "most important fix first"
// invariant documented in the skill FINDINGS.md. Within a severity the original
// order is preserved (validation diagnostics in document order, then fit
// findings in their canonical order), keeping output deterministic.
func (o *dryRunOutput) buildFindingsEnvelope() {
	ds := make([]diagnostics.Diagnostic, 0, len(o.Diagnostics)+len(o.FitFindings))
	ds = append(ds, o.Diagnostics...)
	ds = append(ds, diagnostics.FromFitFindings(o.FitFindings)...)
	ds = dedupeDiagnostics(ds)
	sort.SliceStable(ds, func(i, j int) bool {
		return severityRank(ds[i].Severity) < severityRank(ds[j].Severity)
	})
	o.Findings = diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  o.subcommand,
		Template:    o.template,
		InputSHA256: o.inputSHA256,
	}, ds)
}

// dedupeDiagnostics collapses findings that say the same thing about the same
// place. Two detectors can legitimately notice one fact — a table's row count is
// checked by the validator AND by the fit report — and the deck then carried it
// twice, once as a warning and once as an info, so an agent filtering by
// severity saw the same table in two different work lists
// (go-slide-creator-7xyy).
//
// Identity is (code, path, message): same words about the same field. The
// survivor is the richer record — a remediation, then a next_tool_call, then the
// higher severity — so nothing actionable is lost to the collapse. Order is
// otherwise preserved.
func dedupeDiagnostics(ds []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	type key struct{ code, path, message string }
	seen := make(map[key]int, len(ds))
	out := make([]diagnostics.Diagnostic, 0, len(ds))
	for _, d := range ds {
		k := key{d.Code, d.Path, d.Message}
		if idx, ok := seen[k]; ok {
			if richerDiagnostic(d, out[idx]) {
				out[idx] = d
			}
			continue
		}
		seen[k] = len(out)
		out = append(out, d)
	}
	return out
}

// richerDiagnostic reports whether candidate carries more for an agent to act on
// than incumbent.
func richerDiagnostic(candidate, incumbent diagnostics.Diagnostic) bool {
	if (candidate.Fix != nil) != (incumbent.Fix != nil) {
		return candidate.Fix != nil
	}
	if (candidate.NextToolCall != nil) != (incumbent.NextToolCall != nil) {
		return candidate.NextToolCall != nil
	}
	return severityRank(candidate.Severity) < severityRank(incumbent.Severity)
}

// severityRank orders diagnostics for the findings envelope: error before
// warning before info.
func severityRank(s diagnostics.Severity) int {
	switch s {
	case diagnostics.SeverityError:
		return 0
	case diagnostics.SeverityWarning:
		return 1
	default:
		return 2
	}
}

// dryRunSlide describes one slide in the dry-run report.
type dryRunSlide struct {
	SlideNumber  int                 `json:"slide_number"`
	Title        string              `json:"title,omitempty"`
	LayoutID     string              `json:"layout_id"`
	LayoutName   string              `json:"layout_name,omitempty"`
	Placeholders []dryRunPlaceholder `json:"placeholders"`
	ShapeCount   int                 `json:"shape_count,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

// dryRunPlaceholder describes a placeholder mapping in the dry-run report.
type dryRunPlaceholder struct {
	PlaceholderID string `json:"placeholder_id"`
	ContentType   string `json:"content_type"`
	MaxChars      int    `json:"max_chars,omitempty"`
	Truncated     bool   `json:"truncated,omitempty"`
	TruncateAt    int    `json:"truncate_at,omitempty"`
}

// runJSONDryRun validates JSON input against the template without generating
// a PPTX file. It checks layout_id references, placeholder_id references,
// and content types. A non-empty designModeOverride replaces input.DesignMode
// after parsing so the CLI flag wins over the JSON field. When strictUnknownKeys
// is true, unknown JSON keys are reported as errors (matching MCP
// strict_unknown_keys=true semantics) instead of warnings.
func runJSONDryRun(jsonPath, templatesDir, configPath, designModeOverride string, strictUnknownKeys bool) error {
	output := dryRunOutput{
		Valid:      true,
		Slides:     []dryRunSlide{},
		subcommand: "generate -dry-run",
	}

	// Read JSON input
	var inputData []byte
	var err error
	if jsonPath == "-" {
		inputData, err = io.ReadAll(os.Stdin)
	} else {
		inputData, err = os.ReadFile(jsonPath)
	}
	if err != nil {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "READ_FAILED", Message: fmt.Sprintf("failed to read JSON input: %v", err),
			Severity: diagnostics.SeverityError,
		})
		return writeDryRunOutput(output)
	}
	output.inputSHA256 = diagnostics.ComputeInputSHA256(inputData)

	// Parse JSON as PresentationInput (superset of legacy JSONInput)
	var input PresentationInput
	var patchInput PresentationPatchInput
	if err := json.Unmarshal(inputData, &patchInput); err == nil && len(patchInput.Operations) > 0 {
		patched, patchErr := applyPresentationPatch(patchInput)
		if patchErr != nil {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
				Code: "PATCH_ERROR", Message: fmt.Sprintf("failed to apply patch: %v", patchErr),
				Severity: diagnostics.SeverityError,
			})
			return writeDryRunOutput(output)
		}
		input = *patched
	} else {
		if err := json.Unmarshal(inputData, &input); err != nil {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
				Code: "INVALID_JSON", Message: fmt.Sprintf("failed to parse JSON: %v", err),
				Severity: diagnostics.SeverityError,
			})
			return writeDryRunOutput(output)
		}
	}

	// CLI --design-mode override wins over the JSON field.
	if designModeOverride != "" {
		input.DesignMode = designModeOverride
	}

	applyDefaults(&input)
	output.template = input.Template

	// Load configuration (config file + env overrides) before resolving
	// template-dependent settings, so named-style resolution and template
	// lookup share one effective templates dir. config.Load is always called
	// (even with an empty path) so environment overrides apply without --config.
	// An empty templatesDir means --templates-dir was not explicitly set, so the
	// config-file/env/default value is preserved.
	cfg, err := config.Load(configPath)
	if err != nil {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "SETTINGS_ERROR", Message: fmt.Sprintf("failed to load config: %v", err),
			Severity: diagnostics.SeverityError,
		})
		return writeDryRunOutput(output)
	}
	if templatesDir != "" {
		cfg.Templates.Dir = templatesDir
	}

	// Resolve named style references from template settings (shared with MCP).
	resolveInputNamedSettingsForDir(cfg.Templates.Dir, &input)

	// Check for unknown keys (additionalProperties:false). Warnings by default;
	// when --strict-unknown-keys is set, unknown keys become errors (mirroring
	// MCP validate_input strict_unknown_keys=true semantics).
	for _, ve := range checkInputUnknownKeys(inputData) {
		if strictUnknownKeys {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationError(ve))
		} else {
			output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarning(ve))
		}
	}

	// Enum validation — unknown values for transition, transition_speed, build, background.fit.
	for _, ve := range checkInputEnumValues(&input) {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationError(ve))
	}

	// Validate required fields
	if input.Template == "" {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "template", Message: "template is required in JSON input",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "template"}},
		})
	}
	if len(input.Slides) == 0 {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "REQUIRED", Path: "slides", Message: "at least one slide is required",
			Severity: diagnostics.SeverityError,
			Fix:      &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "slides"}},
		})
	}
	if !output.Valid {
		return writeDryRunOutput(output)
	}

	// Resolve template for validation
	templatePath, templateCleanup, err := resolveTemplatePath(input.Template, cfg.Templates.Dir)
	if err != nil {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "TEMPLATE_NOT_FOUND", Path: "template",
			Message:  templateNotFoundError(input.Template, cfg.Templates.Dir),
			Severity: diagnostics.SeverityError,
		})
		return writeDryRunOutput(output)
	}
	defer templateCleanup()

	cache := template.NewMemoryCache(24 * time.Hour)
	templateAnalysis, err := getOrAnalyzeTemplate(templatePath, cache)
	if err != nil {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
			Code: "TEMPLATE_ERROR", Path: "template",
			Message:  fmt.Sprintf("template analysis failed: %v", err),
			Severity: diagnostics.SeverityError,
		})
		return writeDryRunOutput(output)
	}

	// Resolve canonical layout names before validation so agents can use
	// stable aliases like "title", "content", "blank" in dry-run mode.
	resolveCanonicalLayoutIDs(input.Slides, templateAnalysis.Layouts)

	// Validate slides against template
	validateSlidesAgainstTemplate(&output, input.Slides, templateAnalysis)
	appendGridConverterPreflight(&output, &input, generator.NewSVGConverter().IsPNGAvailable())

	return writeDryRunOutput(output)
}

// CLI generate --dry-run does not run the broader fit-report collector. Keep
// its existing scope while surfacing the grid raster dependency that would
// otherwise make the subsequent generation fail.
func appendGridConverterPreflight(output *dryRunOutput, input *PresentationInput, converterAvailable bool) {
	for _, finding := range collectChartDryRenderFindingsWithConverter(input, nil, "", "warn", converterAvailable) {
		if finding.Message != generator.GridDiagramConverterMissingMessage {
			continue
		}
		output.FitFindings = append(output.FitFindings, finding)
		output.Valid = false
	}
}

// validateJSONContentValue checks that a JSON content item's value can be
// parsed according to its declared type. Returns error/warning messages.
func validateJSONContentValue(item JSONContentItem, slideNum, contentNum int) string {
	switch item.Type {
	case "text":
		var text string
		if err := json.Unmarshal(item.Value, &text); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid text value: %v", slideNum, contentNum, err)
		}
	case "bullets":
		var bullets []string
		if err := json.Unmarshal(item.Value, &bullets); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid bullets value (expected array of strings): %v", slideNum, contentNum, err)
		}
	case "image":
		var img struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(item.Value, &img); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid image value: %v", slideNum, contentNum, err)
		}
		if img.Path == "" {
			return fmt.Sprintf("slide %d, content %d: image path is required", slideNum, contentNum)
		}
	case "chart":
		var chart types.ChartSpec                                  //nolint:staticcheck // backward compatibility
		if err := json.Unmarshal(item.Value, &chart); err != nil { //nolint:staticcheck // backward compatibility
			return fmt.Sprintf("slide %d, content %d: invalid chart value: %v", slideNum, contentNum, err)
		}
		if chart.Type == "" {
			return fmt.Sprintf("slide %d, content %d: chart type is required", slideNum, contentNum)
		}
		// Validate chart data structure via svggen
		spec := chart.ToDiagramSpec()
		if w := validateDiagramSpec(spec, slideNum, contentNum); w != "" {
			return w
		}
	case "body_and_bullets":
		var bab struct {
			Body    string   `json:"body"`
			Bullets []string `json:"bullets"`
		}
		if err := json.Unmarshal(item.Value, &bab); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid body_and_bullets value: %v", slideNum, contentNum, err)
		}
	case "bullet_groups":
		var bg struct {
			Groups []struct {
				Bullets []string `json:"bullets"`
			} `json:"groups"`
		}
		if err := json.Unmarshal(item.Value, &bg); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid bullet_groups value: %v", slideNum, contentNum, err)
		}
		if len(bg.Groups) == 0 {
			return fmt.Sprintf("slide %d, content %d: bullet_groups must have at least one group", slideNum, contentNum)
		}
	case "table":
		var table struct {
			Headers []string          `json:"headers"`
			Rows    []json.RawMessage `json:"rows"`
		}
		if err := json.Unmarshal(item.Value, &table); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid table value: %v", slideNum, contentNum, err)
		}
	case "diagram":
		var diagram types.DiagramSpec
		if err := json.Unmarshal(item.Value, &diagram); err != nil {
			return fmt.Sprintf("slide %d, content %d: invalid diagram value: %v", slideNum, contentNum, err)
		}
		if diagram.Type == "" {
			return fmt.Sprintf("slide %d, content %d: diagram type is required", slideNum, contentNum)
		}
		// Validate diagram data structure via svggen
		if w := validateDiagramSpec(&diagram, slideNum, contentNum); w != "" {
			return w
		}
	}
	return ""
}

// resolveCanonicalLayoutIDs resolves canonical layout names (e.g. "title",
// "content", "blank") to concrete slideLayoutN IDs using tag-based matching
// against the available template layouts. This MUST be called before any
// validation or generation that compares layout_id values against the template.
func resolveCanonicalLayoutIDs(slides []SlideInput, layouts []types.LayoutMetadata) {
	if len(layouts) == 0 {
		return
	}
	for i := range slides {
		if slides[i].LayoutID != "" {
			requested := slides[i].LayoutID
			percent := layout.DerivedColumnLeftPercent(requested)
			if percent != 0 {
				slides[i].ColumnLeftPercent = percent
			} else if layout.IsCanonicalName(requested) ||
				(slides[i].ColumnBaseLayoutID != "" && requested != slides[i].ColumnBaseLayoutID) {
				slides[i].ColumnLeftPercent = 0
				slides[i].ColumnBaseLayoutID = ""
			}
			if resolved, ok := layout.ResolveCanonicalLayoutID(slides[i].LayoutID, layouts); ok {
				slides[i].LayoutID = resolved
				if percent != 0 {
					slides[i].ColumnBaseLayoutID = resolved
				}
			}
		}
	}
}

// validateSlidesAgainstTemplate validates the slides in a PresentationInput
// against a resolved TemplateAnalysis, populating the dryRunOutput with
// per-slide details and accumulating diagnostics in output.Diagnostics (folded
// into the Findings envelope at serialization). This is the shared validation
// core used by both the CLI dry-run and the MCP validate_input handler.
func validateSlidesAgainstTemplate(output *dryRunOutput, slides []SlideInput, analysis *types.TemplateAnalysis) { //nolint:gocognit,gocyclo
	// Build layout and placeholder lookup maps from template analysis
	layoutByID := make(map[string]types.LayoutMetadata, len(analysis.Layouts))
	for _, l := range analysis.Layouts {
		layoutByID[l.ID] = l
	}

	type phKey struct{ layoutID, phID string }
	phByKey := make(map[phKey]types.PlaceholderInfo)
	for _, l := range analysis.Layouts {
		for _, ph := range l.Placeholders {
			phByKey[phKey{l.ID, ph.ID}] = ph
		}
	}

	// Build table style lookup for style_id validation.
	tableStyleByID := make(map[string]string, len(analysis.TableStyles))
	availableStyleIDs := make([]string, 0, len(analysis.TableStyles))
	for _, ts := range analysis.TableStyles {
		tableStyleByID[ts.ID] = ts.Name
		availableStyleIDs = append(availableStyleIDs, ts.ID)
	}

	output.SlideCount = len(slides)

	// The layout each slide will actually land on, so a title without an
	// explicit layout_id is still measured against a real box instead of
	// falling through to the character estimate (go-slide-creator-t64e).
	// Only the title measurement uses it: placeholder-existence validation
	// stays on the author's own layout_id, because the generator auto-maps
	// placeholder IDs and a predicted layout would produce false
	// placeholder_not_found errors.
	predictedLayouts := predictSlideLayouts(&PresentationInput{Slides: slides}, analysis.Layouts)

	// Validate each slide
	for i, slideInput := range slides {
		slide := dryRunSlide{
			SlideNumber:  i + 1,
			LayoutID:     slideInput.LayoutID,
			Placeholders: []dryRunPlaceholder{},
		}

		// Check layout_id reference. layout_id and slide_type are alternatives:
		// the generator auto-selects a layout when only slide_type is provided,
		// so the validator only errors when BOTH are missing.
		lm, layoutFound := layoutByID[slideInput.LayoutID]
		if slideInput.LayoutID == "" {
			if slideInput.SlideType == "" {
				output.Valid = false
				msg := fmt.Sprintf("slide %d: layout_id or slide_type is required", i+1)
				output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
					Code:     patterns.ErrCodeRequired,
					Path:     slidepath.SlideField(i, "layout_id"),
					Message:  msg,
					Severity: diagnostics.SeverityError,
					Fix: &diagnostics.Fix{
						Kind:   "provide_value",
						Params: map[string]any{"field": "layout_id_or_slide_type"},
					},
				})
			}
		} else if !layoutFound {
			output.Valid = false
			path := slidepath.SlideField(i, "layout_id")
			var msg string
			fix := &patterns.FixSuggestion{Kind: "use_one_of", Params: map[string]any{"path": "layout_id"}}
			if layout.IsCanonicalName(slideInput.LayoutID) {
				availableMap := layout.ResolveAllCanonicalLayouts(analysis.Layouts)
				available := make([]string, 0, len(availableMap))
				for name := range availableMap {
					available = append(available, name)
				}
				sort.Strings(available)
				msg = fmt.Sprintf("slide %d: %s", i+1, generator.CanonicalLayoutNotFoundError(slideInput.LayoutID, available))
				fix.Params["available"] = available
				if suggestion := generator.SuggestedCanonicalLayout(slideInput.LayoutID, available); suggestion != "" {
					fix.Params["did_you_mean"] = suggestion
					fix.Params["value"] = suggestion
				} else {
					// No safe concrete target: the message still lists options, but
					// repair_slide.use_one_of would reject a missing value.
					fix = nil
				}
			} else {
				available := make([]string, 0, len(layoutByID))
				for id := range layoutByID {
					available = append(available, id)
				}
				msg = fmt.Sprintf("slide %d: %s", i+1, generator.LayoutNotFoundError(slideInput.LayoutID, available))
				fix.Params["available"] = generator.FormatAvailableIDs(available)
				if match, _ := generator.ClosestMatch(slideInput.LayoutID, available, 3); match != "" {
					fix.Params["did_you_mean"] = match
					fix.Params["value"] = match
				} else {
					fix = nil
				}
			}
			ve := &patterns.ValidationError{
				Path:    path,
				Code:    patterns.ErrCodeUnknownLayoutID,
				Message: msg,
				Fix:     fix,
			}
			output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationError(ve))
		} else {
			slide.LayoutName = lm.Name
		}

		// Validate content items using ContentInput.ResolveValue() for typed + legacy support
		for j, item := range slideInput.Content {
			ph := dryRunPlaceholder{
				PlaceholderID: item.PlaceholderID,
				ContentType:   item.Type,
			}

			if item.PlaceholderID == "" {
				output.Valid = false
				msg := fmt.Sprintf("slide %d, content %d: placeholder_id is required", i+1, j+1)
				output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
					Code:     patterns.ErrCodeRequired,
					Path:     slidepath.ContentField(i, j, "placeholder_id"),
					Message:  msg,
					Severity: diagnostics.SeverityError,
					Fix: &diagnostics.Fix{
						Kind:   "provide_value",
						Params: map[string]any{"field": "placeholder_id"},
					},
				})
			} else if layoutFound {
				// Check placeholder_id reference against layout
				phInfo, phFound := phByKey[phKey{slideInput.LayoutID, item.PlaceholderID}]
				if !phFound {
					output.Valid = false
					available := make([]string, 0, len(lm.Placeholders))
					for _, ph := range lm.Placeholders {
						available = append(available, ph.ID)
					}
					path := slidepath.ContentField(i, j, "placeholder_id")
					msg := fmt.Sprintf("slide %d: %s", i+1, generator.PlaceholderNotFoundError(item.PlaceholderID, slideInput.LayoutID, available))
					fix := &patterns.FixSuggestion{
						Kind:   "use_one_of",
						Params: map[string]any{"available": generator.FormatAvailableIDs(available)},
					}
					if match, _ := generator.ClosestMatch(item.PlaceholderID, available, 3); match != "" {
						fix.Params["did_you_mean"] = match
					}
					ve := &patterns.ValidationError{
						Path:    path,
						Code:    patterns.ErrCodePlaceholderNotFound,
						Message: msg,
						Fix:     fix,
					}
					output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationError(ve))
				} else {
					ph.MaxChars = generator.ReportedMaxChars(&phInfo)

					// Titles: measured fit against the resolved title box and
					// inherited title style replaces the character estimate
					// (go-slide-creator-vjwn). The capacity reported for the
					// placeholder is measured too: estimateMaxChars claimed 29
					// chars for a Blank+Title box that renders 96 comfortably,
					// so agents shortened titles to a number the renderer never
					// used (go-slide-creator-jcph).
					titleMeasured := false
					if item.Type == "text" && phInfo.Type == types.PlaceholderTitle {
						resolved, _ := item.ResolveValue()
						if text, ok := resolved.(string); ok {
							m := measureTitleInPlaceholder(text, &phInfo)
							if m.OK {
								titleMeasured = true
								if d := m.diagnostic(i, j); d != nil {
									output.Diagnostics = append(output.Diagnostics, *d)
								}
							}
						}
					}

					// Check character limits for text content
					if !titleMeasured && item.Type == "text" && phInfo.MaxChars > 0 { //nolint:nestif // mirrors the measured-title branch above
						resolved, _ := item.ResolveValue()
						if text, ok := resolved.(string); ok && len(text) > phInfo.MaxChars {
							ph.Truncated = true
							ph.TruncateAt = phInfo.MaxChars
							msg := fmt.Sprintf("slide %d, content %d: text (%d chars) exceeds placeholder %q limit (%d chars)",
								i+1, j+1, len(text), item.PlaceholderID, phInfo.MaxChars)
							output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
								Code:     patterns.ErrCodeMaxLength,
								Path:     slidepath.ContentField(i, j, "text"),
								Message:  msg,
								Severity: diagnostics.SeverityWarning,
								Fix: &diagnostics.Fix{
									Kind: "shrink_text",
									Params: map[string]any{
										"max_chars":     phInfo.MaxChars,
										"current_chars": len(text),
									},
								},
							})
						}
					}
				}
			}

			// No layout_id: measure the title against the layout the generator
			// will pick. Nothing else about the item is validated here — see the
			// note on predictedLayouts (go-slide-creator-t64e).
			if !layoutFound && slideInput.LayoutID == "" && item.Type == "text" && item.PlaceholderID != "" {
				if phInfo := titlePlaceholderIn(predictedLayouts[i], item.PlaceholderID); phInfo != nil {
					resolved, _ := item.ResolveValue()
					if text, ok := resolved.(string); ok {
						m := measureTitleInPlaceholder(text, phInfo)
						if capacity := generator.TitlePlaceholderCapacityChars(phInfo); capacity > 0 {
							ph.MaxChars = capacity
						}
						if d := m.diagnostic(i, j); d != nil {
							output.Diagnostics = append(output.Diagnostics, *d)
						}
					}
				}
			}

			// Validate content type
			if item.Type == "" {
				output.Valid = false
				msg := fmt.Sprintf("slide %d, content %d: type is required", i+1, j+1)
				output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
					Code:     patterns.ErrCodeRequired,
					Path:     slidepath.ContentField(i, j, "type"),
					Message:  msg,
					Severity: diagnostics.SeverityError,
					Fix: &diagnostics.Fix{
						Kind:   "provide_value",
						Params: map[string]any{"field": "type"},
					},
				})
			} else {
				validTypes := []string{"text", "bullets", "body_and_bullets", "bullet_groups", "table", "image", "chart", "diagram"}
				switch item.Type {
				case "text", "bullets", "body_and_bullets", "bullet_groups", "table", "image", "chart", "diagram":
					// valid types
				default:
					output.Valid = false
					msg := fmt.Sprintf("slide %d, content %d: unknown type %q (must be text, bullets, body_and_bullets, bullet_groups, table, image, chart, or diagram)",
						i+1, j+1, item.Type)
					fix := &diagnostics.Fix{
						Kind:   "use_one_of",
						Params: map[string]any{"available": validTypes},
					}
					if match, _ := generator.ClosestMatch(item.Type, validTypes, 3); match != "" {
						fix.Params["did_you_mean"] = match
					}
					output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
						Code:     diagnostics.CodeUnknownEnum,
						Path:     slidepath.ContentField(i, j, "type"),
						Message:  msg,
						Severity: diagnostics.SeverityError,
						Fix:      fix,
					})
				}
				// Count content types
				switch item.Type {
				case "chart":
					output.ChartCount++
				case "diagram":
					output.DiagramCount++
				case "table":
					output.TableCount++
					// Density check for content-level table.
					table := resolveTableFromContent(&item)
					if table != nil {
						tablePath := slidepath.ContentIndex(i, j)
						densityWarnings := pipeline.DetectTableDensity(table, tablePath)
						output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarnings(densityWarnings)...)

						// Warn when both header_background and style_id are explicitly authored.
						if table.Style != nil {
							if w := generator.WarnStyleCollision(i,
								table.Style.HeaderBackground != nil,
								table.Style.StyleID != "",
							); w != "" {
								collisionVE := &patterns.ValidationError{
									Path:    tablePath,
									Code:    "style_collision",
									Message: w,
								}
								output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarning(collisionVE))
							}
						}
						// Reject style_id values that the renderer would drop because
						// they are not GUID-shaped (typos or XML-metacharacter
						// injection attempts). Error, since they can never render.
						if d := invalidTableStyleIDDiagnostic(table, tablePath, i); d != nil {
							output.Valid = false
							output.Diagnostics = append(output.Diagnostics, *d)
						}
						// Validate style_id against template's declared table styles.
						// Advisory only — an unknown (but well-formed) style_id does
						// not invalidate the deck (Valid is left unchanged).
						if vw := validateTableStyleID(table, tablePath, i, tableStyleByID, availableStyleIDs); vw != nil {
							output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarning(vw))
						}
						// Reject conditional-format fills that are neither a scheme
						// color nor a 6-digit hex value. resolveConditionalFill drops
						// such values to prevent malformed/injected OOXML, so surface
						// a clear error instead of silently losing the fill.
						for _, d := range invalidConditionalFillDiagnostics(table, tablePath, i) {
							output.Valid = false
							output.Diagnostics = append(output.Diagnostics, d)
						}
						// A rule outside the vocabulary, or a threshold the rule
						// cannot use, applies no fill at all. Say which, with the
						// allowed list, instead of leaving the cell quietly plain
						// (go-slide-creator-6hlu).
						for _, d := range conditionalRuleDiagnostics(table, tablePath, i) {
							output.Valid = false
							output.Diagnostics = append(output.Diagnostics, d)
						}
					}
				}
			}

			// Validate content value is parseable using ResolveValue
			if item.Type != "" {
				_, valueErr := item.ResolveValue()
				// ResolveValue deliberately leaves legacy chart/diagram "value"
				// decoding to its callers. Dry-run must still reject malformed
				// values before optional svggen warnings are collected.
				if valueErr == nil && len(item.Value) > 0 {
					switch item.Type {
					case "chart":
						if item.ChartValue == nil {
							var chart types.ChartSpec //nolint:staticcheck // legacy input
							valueErr = json.Unmarshal(item.Value, &chart)
						}
					case "diagram":
						if item.DiagramValue == nil {
							var diagram types.DiagramSpec
							valueErr = json.Unmarshal(item.Value, &diagram)
						}
					}
				}
				if valueErr != nil {
					output.Valid = false
					msg := fmt.Sprintf("slide %d, content %d: %v", i+1, j+1, valueErr)
					output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
						Code:     diagnostics.CodeInvalidParameter,
						Path:     slidepath.ContentField(i, j, item.Type+"_value"),
						Message:  msg,
						Severity: diagnostics.SeverityError,
						Details:  map[string]any{"cause": valueErr.Error()},
					})
				}
			}

			// Detect legacy authoring form: "value" field instead of typed fields
			if item.UsesLegacyValue() {
				typedField := item.Type + "_value"
				path := slidepath.ContentIndex(i, j)
				legacyVE := &patterns.ValidationError{
					Path:    path,
					Code:    "legacy_authoring_form",
					Message: fmt.Sprintf("slide %d, content %d: uses legacy \"value\" field; prefer \"%s\" for new decks", i+1, j+1, typedField),
					Fix: &patterns.FixSuggestion{
						Kind:   "rewrite_field",
						Params: map[string]any{"from": "value", "to": typedField},
					},
				}
				output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarning(legacyVE))
			}

			// Validate chart/diagram data structure via svggen
			if item.Type == "chart" || item.Type == "diagram" {
				chartWarnings := validateContentDiagramData(item, i+1, j+1)
				for _, w := range chartWarnings {
					output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
						Code:     patterns.ErrCodeChartValueCoerced,
						Path:     slidepath.ContentIndex(i, j),
						Message:  w,
						Severity: diagnostics.SeverityWarning,
					})
				}
			}

			slide.Placeholders = append(slide.Placeholders, ph)
		}

		// Validate shape_grid if present
		if slideInput.ShapeGrid != nil {
			gridCounts, gridWarnings, gridErrors, gridValWarnings := validateShapeGrid(slideInput.ShapeGrid, i+1)
			slide.ShapeCount = gridCounts.Shapes
			output.ShapeCount += gridCounts.Shapes
			output.TableCount += gridCounts.Tables
			output.DiagramCount += gridCounts.Diagrams
			output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarnings(gridValWarnings)...)
			for _, w := range gridWarnings {
				output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
					Code:     diagnostics.CodeInvalidGrid,
					Path:     slidepath.ShapeGrid(i),
					Message:  w,
					Severity: diagnostics.SeverityWarning,
				})
			}
			if len(gridErrors) > 0 {
				output.Valid = false
				for _, e := range gridErrors {
					output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
						Code:     diagnostics.CodeInvalidGrid,
						Path:     slidepath.ShapeGrid(i),
						Message:  e,
						Severity: diagnostics.SeverityError,
					})
				}
			}

			// Validate style_id for tables inside shape_grid cells.
			for rowIdx, row := range slideInput.ShapeGrid.Rows {
				for cellIdx, cell := range row.Cells {
					if cell != nil && cell.Table != nil {
						tablePath := slidepath.GridCellField(i, rowIdx, cellIdx, "table")
						if d := invalidTableStyleIDDiagnostic(cell.Table, tablePath, i); d != nil {
							output.Valid = false
							output.Diagnostics = append(output.Diagnostics, *d)
						}
						if vw := validateTableStyleID(cell.Table, tablePath, i, tableStyleByID, availableStyleIDs); vw != nil {
							output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarning(vw))
						}
					}
				}
			}
		}

		// Pattern values: generate validates every named pattern against the
		// pattern's own value schema and refuses the deck on failure, so
		// validate must run the same check rather than silently accepting a
		// deck generate will reject (go-slide-creator-xvek).
		patternCtx := patterns.ExpandContext{
			Theme:       analysis.Theme,
			Metadata:    analysis.Metadata,
			SlideWidth:  analysis.SlideWidth,
			SlideHeight: analysis.SlideHeight,
			SlideIndex:  i,
		}
		if pd := slidePatternDiagnostics(&slideInput, i, patternCtx, patterns.Default()); len(pd) > 0 {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, pd...)
		}

		// Grid diagram data: generate aborts on svggen validation failures in
		// shape_grid diagrams, including those produced by pattern expansion
		// (e.g. chart-insights-split), so report them as errors here.
		gridDiags := gridDiagramValidationDiagnostics(slideInput.ShapeGrid, i, "shape_grid", slidepath.ShapeGrid(i))
		if pg := expandSlidePatternGrid(&slideInput, i, analysis.SlideWidth, analysis.SlideHeight, &analysis.Theme); pg != nil {
			gridDiags = append(gridDiags, gridDiagramValidationDiagnostics(pg, i, "pattern "+slideInput.Pattern.Name, slidepath.SlideField(i, "pattern"))...)
		}
		if len(gridDiags) > 0 {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, gridDiags...)
		}

		// Takeaway warning: a slide that argues from data and says nothing
		// about what the data means loses most of its narrative value — the
		// audience cannot tell what the chart is supposed to argue. Warn (do
		// not error) when missing, and stay quiet when the title already
		// carries the argument.
		if strings.TrimSpace(slideInput.Takeaway) == "" && slideRequiresTakeaway(slideInput) && !slideTitleStatesTakeaway(slideInput) {
			msg := fmt.Sprintf("slide %d: this slide argues from data — set a takeaway headline (or make the title a full sentence) so the audience knows the 'so what'", i+1)
			ve := &patterns.ValidationError{
				Path:    slidepath.SlideField(i, "takeaway"),
				Code:    patterns.ErrCodeTakeawayMissing,
				Message: msg,
				Fix: &patterns.FixSuggestion{
					Kind:   "provide_value",
					Params: map[string]any{"field": "takeaway"},
				},
			}
			// Advisory only — a missing takeaway does not invalidate the deck
			// (Valid is left unchanged), so it is a warning.
			output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarning(ve))
		}

		output.Slides = append(output.Slides, slide)
	}
}

// slideRequiresTakeaway reports whether a slide argues from data, and so wants
// a takeaway / "so what" headline. True when any content item is a chart, when
// diagram_value is a chart-shaped diagram type, or when the slide's pattern is
// marked DataVisual in its own taxonomy.
//
// The pattern half used to be the name prefix "matrix-", which meant the nudge
// landed on exactly one pattern and never on chart-insights-split — the pattern
// that literally embeds a chart — nor on waterfall-bridge or
// horizontal-bar-with-callouts, which are charts drawn as shape grids
// (go-slide-creator-g2cy).
func slideRequiresTakeaway(s SlideInput) bool {
	for _, item := range s.Content {
		switch item.Type {
		case "chart":
			return true
		case "diagram":
			if item.DiagramValue != nil && isChartishDiagramType(item.DiagramValue.Type) {
				return true
			}
		}
		if item.ChartValue != nil {
			return true
		}
	}
	if s.Pattern != nil {
		if p, ok := patterns.Default().Get(s.Pattern.Name); ok && p.Taxonomy().DataVisual {
			return true
		}
	}
	return false
}

// isChartishDiagramType reports whether a diagram type renders as a data
// chart (bar, line, area, scatter, etc.) rather than a structural diagram.
// Chart-shaped diagrams benefit from a takeaway in the same way native
// chart placeholders do.
func isChartishDiagramType(t string) bool {
	switch t {
	case "bar", "bar_chart", "line", "line_chart", "area", "area_chart",
		"scatter", "bubble", "pie", "donut", "stacked_bar", "grouped_bar",
		"waterfall", "funnel", "radar", "gauge", "treemap":
		return true
	}
	return false
}

// takeawayTitleMinWords is the length at which a title stops reading as a label
// ("Options matrix") and starts reading as a claim. Six words is the shortest
// full sentence that carries a subject, a verb and an object with anything to
// say ("Enterprise carried the year, SMB did not").
const takeawayTitleMinWords = 6

// slideTitleStatesTakeaway reports whether the slide's title already makes the
// argument, in which case asking for a separate takeaway is noise: the one
// place the old lint fired was a matrix-2x2 titled "Prioritise the four
// initiatives in the top-right quadrant", which is the takeaway
// (go-slide-creator-g2cy).
//
// A title states the takeaway when it is long enough to be a sentence AND
// carries a verb. The test is deliberately biased toward false negatives — a
// title with a verb this does not know still gets the nudge, which is the
// status quo, while a wrong suppression silently removes the signal.
func slideTitleStatesTakeaway(s SlideInput) bool {
	_, title := extractTitleText(s)
	words := strings.Fields(title)
	if len(words) < takeawayTitleMinWords {
		return false
	}
	for i, w := range words {
		word := strings.ToLower(strings.Trim(w, `.,;:!?"'()[]`))
		if takeawayTitleVerbs[word] {
			return true
		}
		// An imperative is a verb only at the head of the title: "Fund the SMB
		// pod" argues, "Fund performance by vintage" labels.
		if i == 0 && takeawayTitleImperatives[word] {
			return true
		}
	}
	return false
}

// takeawayTitleVerbs are the finite verb forms that turn a title into a claim
// wherever they appear. Auxiliaries and copulas carry most sentences; the rest
// are movement, causation and consequence verbs in forms that are not also
// common nouns — "grew", not "growth"; "climbs", not "climb". Noun-ambiguous
// forms ("costs", "drives", "needs", "wins") are left out on purpose: they
// head far more labels than claims.
var takeawayTitleVerbs = map[string]bool{
	// copulas and auxiliaries
	"is": true, "are": true, "was": true, "were": true, "be": true,
	"been": true, "being": true, "am": true,
	"has": true, "have": true, "had": true,
	"does": true, "do": true, "did": true,
	"will": true, "would": true, "can": true, "cannot": true, "could": true,
	"must": true, "should": true, "may": true, "might": true, "shall": true,
	// movement and magnitude
	"grew": true, "grows": true, "growing": true,
	"rose": true, "rises": true, "rising": true,
	"fell": true, "falling": true,
	"doubled": true, "doubles": true, "halved": true, "tripled": true,
	"outpaced": true, "outpaces": true, "lagged": true, "lags": true,
	"led": true, "trails": true, "beat": true, "beats": true,
	"held": true, "holds": true, "slipped": true, "slips": true,
	"added": true, "adds": true, "lost": true, "loses": true,
	"gained": true, "climbed": true, "climbs": true,
	"dropped": true, "improved": true, "improves": true,
	"worsened": true, "stalled": true, "widened": true, "narrowed": true,
	// causation and consequence
	"drove": true, "driving": true, "explains": true, "explained": true,
	"created": true, "creates": true, "delivered": true, "delivers": true,
	"protected": true, "protects": true, "threatens": true, "threatened": true,
	"requires": true, "required": true, "depends": true, "hinges": true,
	"makes": true, "made": true, "won": true, "bought": true, "sold": true,
	"funded": true, "chose": true, "shifted": true, "consolidated": true,
	"recommends": true, "recommended": true,
	"carried": true, "gave": true, "took": true, "kept": true,
	"brought": true, "turned": true, "cleared": true, "missed": true,
	"hit": true, "reached": true, "landed": true, "closed": true,
}

// takeawayTitleImperatives are verbs only at the head of a title. "Fund the
// SMB success pod in Q3" argues; "Fund performance by vintage" labels.
var takeawayTitleImperatives = map[string]bool{
	"prioritise": true, "prioritize": true, "focus": true,
	"choose": true, "pick": true, "fund": true, "invest": true,
	"stop": true, "start": true, "shift": true, "move": true, "keep": true,
	"cut": true, "accept": true, "reject": true, "approve": true,
	"recommend": true, "consolidate": true, "expand": true, "exit": true,
	"buy": true, "sell": true, "build": true, "double": true, "treat": true,
	"protect": true, "deliver": true, "act": true, "hold": true, "back": true,
}

// hexColorRe matches #RGB or #RRGGBB hex color strings.
var hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// brandHexAllowlist contains lowercase hex values that are deliberately used as
// raw colors (black, white) and should NOT trigger the hex_fill_non_brand
// warning. Lookups must lowercase the input first.
var brandHexAllowlist = map[string]bool{
	"#000000": true, "#000": true,
	"#ffffff": true, "#fff": true,
}

// isAllowlistedHex reports whether a hex color string is in the brand allowlist.
func isAllowlistedHex(s string) bool {
	return brandHexAllowlist[strings.ToLower(s)]
}

// schemeColorNames aliases the canonical set from the pptx package, plus "none".
var schemeColorNames = func() map[string]bool {
	m := make(map[string]bool, len(pptx.SchemeColorNames)+1)
	for k, v := range pptx.SchemeColorNames {
		m[k] = v
	}
	m["none"] = true
	return m
}()

// isValidFillColor checks whether a color string is a valid hex color or scheme color name.
func isValidFillColor(s string) bool {
	return hexColorRe.MatchString(s) || schemeColorNames[s]
}

// gridContentCounts holds counts of content types found inside shape_grid cells.
type gridContentCounts struct {
	Shapes   int
	Tables   int
	Diagrams int
}

// validateShapeGrid validates a ShapeGridInput and returns content counts,
// warnings, errors, and structured validation warnings.
func validateShapeGrid(grid *ShapeGridInput, slideNum int) (counts gridContentCounts, warnings []string, errors []string, valWarnings []*patterns.ValidationError) {
	if len(grid.Rows) == 0 {
		errors = append(errors, fmt.Sprintf("slide %d: shape_grid has no rows", slideNum))
		return
	}

	if _, ok := shapegrid.ParseVerticalAlign(grid.VerticalAlign); !ok {
		errors = append(errors, fmt.Sprintf("slide %d: shape_grid vertical_align must be one of \"stretch\", \"top\", \"center\", \"bottom\", got %q", slideNum, grid.VerticalAlign))
	}

	// Validate columns
	if len(grid.Columns) > 0 {
		var n float64
		if err := json.Unmarshal(grid.Columns, &n); err != nil {
			var arr []float64
			if err := json.Unmarshal(grid.Columns, &arr); err != nil {
				errors = append(errors, fmt.Sprintf("slide %d: shape_grid columns must be a number or array of numbers", slideNum))
			}
		}
	}

	for rowIdx, row := range grid.Rows {
		for cellIdx, cell := range row.Cells {
			if cell == nil {
				continue
			}
			if cell.Shape != nil {
				counts.Shapes++
				// Validate geometry name
				if cell.Shape.Geometry == "" {
					errors = append(errors, fmt.Sprintf("slide %d: shape_grid row %d cell %d: geometry is required", slideNum, rowIdx+1, cellIdx+1))
				} else if !pptx.IsKnownGeometry(cell.Shape.Geometry) {
					errors = append(errors, fmt.Sprintf("slide %d: shape_grid row %d cell %d: unknown geometry %q", slideNum, rowIdx+1, cellIdx+1, cell.Shape.Geometry))
				}
				// Validate fill color
				errors = appendShapeFillInputError(errors, cell.Shape.Fill, slideNum, rowIdx+1, cellIdx+1)
				vw := validateShapeFillColor(cell.Shape.Fill, slideNum, rowIdx+1, cellIdx+1, &warnings)
				valWarnings = append(valWarnings, vw...)
			}
			if cell.Table != nil {
				counts.Shapes++
				counts.Tables++
				// Density check for embedded table.
				tablePath := slidepath.GridCellField(slideNum-1, rowIdx, cellIdx, "table")
				valWarnings = append(valWarnings, pipeline.DetectTableDensity(cell.Table, tablePath)...)
			}
			if cell.Diagram != nil {
				counts.Shapes++
				counts.Diagrams++
				if cell.Diagram.Type == "" {
					errors = append(errors, fmt.Sprintf("slide %d: shape_grid row %d cell %d: diagram type is required", slideNum, rowIdx+1, cellIdx+1))
				}
			}
		}
	}

	return
}

func appendShapeFillInputError(errors []string, raw json.RawMessage, slideNum, row, cell int) []string {
	if len(raw) == 0 {
		return errors
	}
	if _, err := shapegrid.ResolveFillInput(raw); err != nil {
		return append(errors, fmt.Sprintf("slide %d: shape_grid row %d cell %d: %v", slideNum, row, cell, err))
	}
	return errors
}

// validateShapeFillColor checks that a shape fill value has a valid color format.
// It also returns structured warnings for hex colors not in the brand allowlist.
func validateShapeFillColor(raw json.RawMessage, slideNum, row, cell int, warnings *[]string) []*patterns.ValidationError {
	if len(raw) == 0 {
		return nil
	}
	var valWarnings []*patterns.ValidationError
	checkHex := func(color string) {
		if hexColorRe.MatchString(color) && !isAllowlistedHex(color) {
			path := slidepath.GridCellField(slideNum-1, row-1, cell-1, "shape/fill")
			valWarnings = append(valWarnings, &patterns.ValidationError{
				Pattern: "shape_grid",
				Path:    path,
				Code:    patterns.ErrCodeHexFillNonBrand,
				Message: fmt.Sprintf("slide %d: shape_grid row %d cell %d: fill color %q is a raw hex value; prefer a scheme color for template portability", slideNum, row, cell, color),
				Fix:     &patterns.FixSuggestion{Kind: "use_semantic_color", Params: map[string]any{"message": "use accent1/accent2/lt2/dk1 instead"}},
			})
		}
	}
	// Try string form
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s != "" && !isValidFillColor(s) {
			*warnings = append(*warnings, fmt.Sprintf("slide %d: shape_grid row %d cell %d: fill color %q should be #RGB, #RRGGBB, or a scheme color name (e.g. accent1, dk1)", slideNum, row, cell, s))
		}
		checkHex(s)
		return valWarnings
	}
	// Try object form
	var obj ShapeFillInput
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.Color != "" && !isValidFillColor(obj.Color) {
			*warnings = append(*warnings, fmt.Sprintf("slide %d: shape_grid row %d cell %d: fill color %q should be #RGB, #RRGGBB, or a scheme color name (e.g. accent1, dk1)", slideNum, row, cell, obj.Color))
		}
		checkHex(obj.Color)
	}
	return valWarnings
}

// validateTableStyleID checks whether an authored style_id is declared in the
// template's tableStyles.xml. Returns a ValidationError if the GUID is unknown,
// or nil if no style_id is set, if it's the @template-default sentinel, or if
// the GUID is found in the template.
func validateTableStyleID(table *TableInput, tablePath string, slideIdx int, styleByID map[string]string, availableIDs []string) *patterns.ValidationError {
	if table.Style == nil || table.Style.StyleID == "" {
		return nil
	}
	styleID := table.Style.StyleID

	// @template-default is always valid — it resolves at generation time.
	if styleID == template.TemplateDefaultSentinel {
		return nil
	}

	// If no table styles are declared in the template, skip validation — the
	// template's tableStyles.xml may be absent and we can't enumerate.
	if len(styleByID) == 0 {
		return nil
	}

	if _, ok := styleByID[styleID]; ok {
		return nil
	}

	path := tablePath + ".style.style_id"
	msg := fmt.Sprintf("slide %d: style_id %q not found in template table styles; use list_templates to see available table_styles",
		slideIdx+1, styleID)

	fix := &patterns.FixSuggestion{
		Kind:   "use_one_of",
		Params: map[string]any{"available": generator.FormatAvailableIDs(availableIDs)},
	}
	if match, _ := generator.ClosestMatch(styleID, availableIDs, 10); match != "" {
		fix.Params["did_you_mean"] = match
	}

	return &patterns.ValidationError{
		Path:    path,
		Code:    patterns.ErrCodeUnknownTableStyleID,
		Message: msg,
		Fix:     fix,
	}
}

// invalidTableStyleIDDiagnostic returns an error diagnostic when an authored
// style_id is neither empty, the @template-default sentinel, nor a well-formed
// OOXML table style GUID. The renderer only emits GUID-shaped style IDs (see
// generator.IsValidTableStyleID / types.IsValidTableStyleID) and drops anything
// else to prevent malformed or injected OOXML, so surface a clear error —
// rather than silently losing the style_id — when an XML-metacharacter or typo
// value is authored.
//
// This is distinct from validateTableStyleID, which only warns when a
// well-formed GUID is not declared by the template (advisory). Here the value
// can never render at all, so the deck is reported invalid.
func invalidTableStyleIDDiagnostic(table *TableInput, tablePath string, slideIdx int) *diagnostics.Diagnostic {
	if table == nil || table.Style == nil {
		return nil
	}
	styleID := table.Style.StyleID
	if styleID == "" || styleID == template.TemplateDefaultSentinel || types.IsValidTableStyleID(styleID) {
		return nil
	}
	return &diagnostics.Diagnostic{
		Code: diagnostics.CodeInvalidParameter,
		Path: tablePath + ".style.style_id",
		Message: fmt.Sprintf("slide %d: table style_id %q is invalid; use %q or a table style GUID such as %q",
			slideIdx+1, styleID, template.TemplateDefaultSentinel, types.DefaultTableStyleID),
		Severity: diagnostics.SeverityError,
	}
}

// condFillRe matches the strict allowlist for table conditional-format fills:
// an optional leading '#' followed by exactly six hexadecimal digits.
var condFillRe = regexp.MustCompile(`^#?[0-9a-fA-F]{6}$`)

// isValidConditionalFill reports whether a table conditional-format fill value
// is allowed by generator.resolveConditionalFill: a scheme color name, or a
// 6-digit hex color (with or without a leading '#'). Any other value — a typo
// or an attribute/element injection attempt — is rejected.
func isValidConditionalFill(fill string) bool {
	if condFillRe.MatchString(fill) {
		return true
	}
	// A '#'-prefixed value that is not exactly six hex digits (e.g. "#FFF") is
	// not something resolveConditionalFill can emit, so reject it here too.
	if strings.HasPrefix(fill, "#") {
		return false
	}
	// The only remaining valid values are scheme color names.
	return isValidFillColor(fill)
}

// invalidConditionalFillDiagnostics returns an error diagnostic for every cell
// whose conditional-format fill is neither a scheme color nor a 6-digit hex
// value. The renderer drops such values (see generator.resolveConditionalFill)
// to avoid malformed or injected OOXML, so the deck is reported invalid with a
// clear, actionable message instead of silently losing the fill.
func invalidConditionalFillDiagnostics(table *TableInput, tablePath string, slideIdx int) []diagnostics.Diagnostic {
	if table == nil {
		return nil
	}
	var out []diagnostics.Diagnostic
	for ri, row := range table.Rows {
		for ci, cell := range row {
			if cell.Conditional == nil || cell.Conditional.Fill == "" {
				continue
			}
			if isValidConditionalFill(cell.Conditional.Fill) {
				continue
			}
			out = append(out, diagnostics.Diagnostic{
				Code: diagnostics.CodeInvalidParameter,
				Path: fmt.Sprintf("%s.rows[%d][%d].conditional.fill", tablePath, ri, ci),
				Message: fmt.Sprintf("slide %d: table conditional fill %q is invalid; use a scheme color (e.g. accent2) or a 6-digit hex value (e.g. #CC0000)",
					slideIdx+1, cell.Conditional.Fill),
				Severity: diagnostics.SeverityError,
			})
		}
	}
	return out
}

// conditionalRuleDiagnostics reports table conditional-format rules the renderer
// cannot act on: a rule outside the closed vocabulary, and a threshold whose
// shape the rule cannot use (a string where a number is compared, a missing
// operand, a "between" without two bounds). Each one means the cell renders
// with no conditional fill, which is invisible in the JSON.
func conditionalRuleDiagnostics(table *TableInput, tablePath string, slideIdx int) []diagnostics.Diagnostic {
	if table == nil {
		return nil
	}
	var out []diagnostics.Diagnostic
	for ri, row := range table.Rows {
		for ci, cell := range row {
			cond := cell.Conditional
			if cond == nil {
				continue
			}
			base := fmt.Sprintf("%s.rows[%d][%d].conditional", tablePath, ri, ci)
			if !types.ConditionalRuleKnown(cond.Rule) {
				params := map[string]any{
					"path":    base + ".rule",
					"allowed": types.ConditionalRules,
				}
				message := fmt.Sprintf("slide %d: table conditional rule %q is not recognised, so the cell gets no fill; use one of %s",
					slideIdx+1, cond.Rule, strings.Join(types.ConditionalRules, ", "))
				// A typo is the common case, so name the rule the author meant
				// rather than leaving them to scan the list.
				if match, _ := generator.ClosestMatch(cond.Rule, types.ConditionalRules, 3); match != "" {
					params["did_you_mean"] = match
					message = fmt.Sprintf("slide %d: table conditional rule %q is not recognised, so the cell gets no fill; did you mean %q? (allowed: %s)",
						slideIdx+1, cond.Rule, match, strings.Join(types.ConditionalRules, ", "))
				}
				out = append(out, diagnostics.Diagnostic{
					Code:     diagnostics.CodeInvalidParameter,
					Path:     base + ".rule",
					Message:  message,
					Severity: diagnostics.SeverityError,
					Fix:      &diagnostics.Fix{Kind: "use_one_of", Params: params},
				})
				continue
			}
			if msg := conditionalThresholdProblem(cond); msg != "" {
				out = append(out, diagnostics.Diagnostic{
					Code:     diagnostics.CodeInvalidParameter,
					Path:     base + ".threshold",
					Message:  fmt.Sprintf("slide %d: table conditional rule %q %s, so the cell gets no fill", slideIdx+1, conditionalRuleName(cond.Rule), msg),
					Severity: diagnostics.SeverityError,
					Fix: &diagnostics.Fix{
						Kind:   "provide_value",
						Params: map[string]any{"path": base + ".threshold"},
					},
				})
			}
		}
	}
	return out
}

// conditionalRuleName spells the empty rule the way the message reads.
func conditionalRuleName(rule string) string {
	if rule == "" {
		return types.ConditionalRuleAlways
	}
	return rule
}

// conditionalThresholdProblem returns a clause describing why a rule cannot use
// its threshold, or "" when the pair is usable.
func conditionalThresholdProblem(cond *jsonschema.ConditionalFormatInput) string {
	value, ok := cond.ThresholdValue()
	if !ok {
		return "needs a number, a string or a two-number range as its threshold"
	}
	switch cond.Rule {
	case types.ConditionalRuleThreshold, types.ConditionalRuleGTE, types.ConditionalRuleLTE:
		switch v := value.(type) {
		case float64:
			return ""
		case string:
			if _, numeric := parseThresholdNumber(v); numeric {
				return ""
			}
			return fmt.Sprintf("compares numbers, but its threshold %q is not one", v)
		default:
			return "needs a number as its threshold"
		}
	case types.ConditionalRuleBetween:
		if pair, isPair := value.([]float64); isPair && len(pair) == 2 {
			return ""
		}
		return "needs a two-number range as its threshold, e.g. [0, 10]"
	case types.ConditionalRuleEquals, types.ConditionalRuleContains:
		if value == nil {
			return "needs a threshold to compare the cell against"
		}
		if cond.Rule == types.ConditionalRuleContains {
			if text, isText := value.(string); !isText || strings.TrimSpace(text) == "" {
				return "needs a non-empty string as its threshold"
			}
		}
		return ""
	default:
		// always / positive / negative take no threshold; one is harmless.
		return ""
	}
}

// parseThresholdNumber reads a numeric threshold written as a string, the way
// the renderer does.
func parseThresholdNumber(s string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// writeDryRunOutput writes the dry-run result as JSON to stdout. The accumulated
// diagnostics and fit findings are folded into the single Findings envelope just
// before encoding. Returns nil on valid output, or an error to signal exit code 1
// for invalid.
func writeDryRunOutput(output dryRunOutput) error {
	output.buildFindingsEnvelope()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("failed to encode dry-run output: %w", err)
	}

	if !output.Valid {
		return fmt.Errorf("dry-run validation failed")
	}
	return nil
}
