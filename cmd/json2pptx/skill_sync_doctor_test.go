package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/svggen/core"
)

// ---------------------------------------------------------------------------
// Allowlists and baselines
// ---------------------------------------------------------------------------

// validationOnlyAllowlist lists finding codes present in
// patterns.AllFitFindingCodes() but used only for input validation (emitted
// by pattern.Validate), not by the fit-report pipeline. They appear in
// validate_input error responses, not in the SKILL.md finding-code tables.
var validationOnlyAllowlist = map[string]string{
	"required":               "input validation, not a fit-report finding",
	"max_length":             "input validation, not a fit-report finding",
	"out_of_range":           "input validation, not a fit-report finding",
	"count_mismatch":         "input validation, not a fit-report finding",
	"unknown_key":            "input validation, not a fit-report finding",
	"min_items":              "input validation, not a fit-report finding",
	"max_items":              "input validation, not a fit-report finding",
	"empty_value":            "input validation, not a fit-report finding",
	"unknown_layout_id":      "input validation, not a fit-report finding",
	"callout_unsupported":    "input validation, not a fit-report finding",
	"UNKNOWN_ENUM":           "input validation, not a fit-report finding",
	"placeholder_not_found":  "input validation, not a fit-report finding",
	"unknown_table_style_id": "input validation, not a fit-report finding",
	"wrong_pattern":          "input validation, not a fit-report finding",
	"invalid_shape":          "input validation, not a fit-report finding",
}

// stringLiteralCodes lists finding codes emitted as string literals rather
// than via patterns.ErrCode* or svggen.Finding* constants. The value is the
// source file where the literal lives.
var stringLiteralCodes = map[string]string{
	"findings_truncated": "fit_findings_collect.go",
	"contrast_autofixed": "fit_findings_collect.go",
}

// knownSkillDrift lists codes emitted in Go but not yet documented in SKILL.md.
// Entries are warnings, not failures, giving time to update the docs. When a
// code IS added to SKILL.md, remove it here — the staleness subtest catches
// entries that are no longer needed.
var knownSkillDrift = map[string]string{
	"chart_data_empty":     "pre-flight chart diagnostic, SKILL.md lists chart.* variant",
	"chart_shape_inferred": "pre-flight chart diagnostic, SKILL.md lists chart.* variant",
	"chart_value_coerced":  "pre-flight chart diagnostic, SKILL.md lists chart.* variant",
	"pattern_overcrowded":  "documented in FIT_FINDINGS.md but not yet in SKILL.md table",
	"pattern_underfilled":  "documented in FIT_FINDINGS.md but not yet in SKILL.md table",
	"sparse_layout":        "documented in FIT_FINDINGS.md but not yet in SKILL.md table",
}

// knownFitDocDrift lists codes emitted in Go but not yet documented in
// docs/FIT_FINDINGS.md. Same warning-only semantics as knownSkillDrift.
var knownFitDocDrift = map[string]string{
	// Chart codes — documented in SKILL.md's chart-codes table but not
	// yet in FIT_FINDINGS.md.
	"chart.all_zero_series":         "chart code, documented in SKILL.md only",
	"chart.auto_log_scale_applied":  "chart code, documented in SKILL.md only",
	"chart.capacity_exceeded":       "chart code, documented in SKILL.md only",
	"chart.invalid_numeric":         "chart code, documented in SKILL.md only",
	"chart.invalid_time_format":     "chart code, documented in SKILL.md only",
	"chart.label_clipped":           "chart code, documented in SKILL.md only",
	"chart.label_ellipsized":        "chart code, documented in SKILL.md only",
	"chart.label_truncated":         "chart code, documented in SKILL.md only",
	"chart.legend_overflow_dropped": "chart code, documented in SKILL.md only",
	"chart.negative_on_log":         "chart code, documented in SKILL.md only",
	"chart.overflow_suppressed":     "chart code, documented in SKILL.md only",
	"chart.scatter_label_skipped":   "chart code, documented in SKILL.md only",
	"chart.tick_thinned":            "chart code, documented in SKILL.md only",
	"chart.zero_sum_pie":            "chart code, documented in SKILL.md only",
	// Render-time codes — documented in SKILL.md table but missing from
	// FIT_FINDINGS.md.
	"column_width_deficit":         "render-time code, SKILL.md only",
	"diagram_clamped":              "render-time code, SKILL.md only",
	"diagram_render_failed":        "render-time code, SKILL.md only",
	"divider_too_thin":             "pre-flight code, SKILL.md only",
	"hex_fill_non_brand":           "validation code also used in fit report",
	"mixed_fill_scheme":            "pre-flight code, SKILL.md only",
	"no_autofit_overflow":          "render-time code, SKILL.md only",
	"pagination_default_threshold": "render-time code, SKILL.md only",
	"placeholder_remapped":         "render-time code, SKILL.md only",
	"readability_trimmed":          "render-time code, SKILL.md only",
	"stacked_tables":               "pre-flight code, SKILL.md only",
	"table_font_scaled":            "render-time code, SKILL.md only",
	"table_rows_truncated":         "render-time code, SKILL.md only",
	"text_overflow":                "render-time code, SKILL.md only",
	"text_trimmed":                 "render-time code, SKILL.md only",
}

// ---------------------------------------------------------------------------
// TestSkillSyncDoctor_FindingCodes — bidirectional finding-code reconciliation
// ---------------------------------------------------------------------------

func TestSkillSyncDoctor_FindingCodes(t *testing.T) {
	allCodes := collectAllFindingCodes(t)

	fitCodes := make([]string, 0, len(allCodes))
	for _, code := range allCodes {
		if _, ok := validationOnlyAllowlist[code]; ok {
			continue
		}
		fitCodes = append(fitCodes, code)
	}

	skillText := readFile(t, "../../skills/generate-deck/SKILL.md")
	fitDocText := readFile(t, "../../docs/FIT_FINDINGS.md")

	// Direction 1a: code → SKILL.md
	t.Run("code_in_skill", func(t *testing.T) {
		for _, code := range fitCodes {
			if !strings.Contains(skillText, code) {
				if reason, ok := knownSkillDrift[code]; ok {
					t.Logf("KNOWN DRIFT: %q not in SKILL.md (%s)", code, reason)
					continue
				}
				t.Errorf("finding code %q emitted in Go code but not documented in SKILL.md\n"+
					"  Fix: add %q to the finding codes table in skills/generate-deck/SKILL.md\n"+
					"  Or: add to knownSkillDrift with justification", code, code)
			}
		}
	})

	// Direction 1b: code → FIT_FINDINGS.md
	t.Run("code_in_fit_findings", func(t *testing.T) {
		for _, code := range fitCodes {
			if !strings.Contains(fitDocText, code) {
				if reason, ok := knownFitDocDrift[code]; ok {
					t.Logf("KNOWN DRIFT: %q not in FIT_FINDINGS.md (%s)", code, reason)
					continue
				}
				t.Errorf("finding code %q emitted in Go code but not documented in docs/FIT_FINDINGS.md\n"+
					"  Fix: add a ### `%s` section to docs/FIT_FINDINGS.md\n"+
					"  Or: add to knownFitDocDrift with justification", code, code)
			}
		}
	})

	// Direction 2a: SKILL.md → code
	skillDocCodes := extractDocumentedCodes(skillText, allCodes)
	t.Run("skill_in_code", func(t *testing.T) {
		codeSet := toSet(allCodes)
		for _, code := range skillDocCodes {
			if _, ok := stringLiteralCodes[code]; ok {
				verifyStringLiteral(t, code, stringLiteralCodes[code])
				continue
			}
			if !codeSet[code] {
				t.Errorf("finding code %q documented in SKILL.md but never emitted in Go code\n"+
					"  Fix: add emission in the engine, or remove %q from SKILL.md", code, code)
			}
		}
	})

	// Direction 2b: FIT_FINDINGS.md → code
	fitDocDocCodes := extractDocumentedCodes(fitDocText, allCodes)
	t.Run("fit_findings_in_code", func(t *testing.T) {
		codeSet := toSet(allCodes)
		for _, code := range fitDocDocCodes {
			if _, ok := stringLiteralCodes[code]; ok {
				verifyStringLiteral(t, code, stringLiteralCodes[code])
				continue
			}
			if !codeSet[code] {
				t.Errorf("finding code %q documented in docs/FIT_FINDINGS.md but never emitted in Go code\n"+
					"  Fix: add emission in the engine, or remove %q from docs/FIT_FINDINGS.md", code, code)
			}
		}
	})

	// Verify string-literal codes still exist in their source files.
	t.Run("string_literal_codes_exist", func(t *testing.T) {
		for code, srcFile := range stringLiteralCodes {
			verifyStringLiteral(t, code, srcFile)
		}
	})

	// Staleness check: flag knownSkillDrift entries that have been fixed.
	t.Run("skill_drift_staleness", func(t *testing.T) {
		for code, reason := range knownSkillDrift {
			if strings.Contains(skillText, code) {
				t.Errorf("knownSkillDrift entry %q is stale — SKILL.md now documents it (was: %s)\n"+
					"  Fix: remove %q from knownSkillDrift", code, reason, code)
			}
		}
	})

	// Staleness check: flag knownFitDocDrift entries that have been fixed.
	t.Run("fit_doc_drift_staleness", func(t *testing.T) {
		for code, reason := range knownFitDocDrift {
			if strings.Contains(fitDocText, code) {
				t.Errorf("knownFitDocDrift entry %q is stale — FIT_FINDINGS.md now documents it (was: %s)\n"+
					"  Fix: remove %q from knownFitDocDrift", code, reason, code)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestSkillSyncDoctor_OverridesFields — pattern override schema well-formedness
// ---------------------------------------------------------------------------

func TestSkillSyncDoctor_OverridesFields(t *testing.T) {
	for _, p := range patterns.Default().List() {
		schema := p.Schema()
		if schema == nil {
			continue
		}
		schemaBytes, err := json.Marshal(schema)
		if err != nil {
			t.Errorf("pattern %q: failed to marshal schema: %v", p.Name(), err)
			continue
		}
		overrideFields := extractOverrideFields(schemaBytes)
		if len(overrideFields) == 0 {
			continue
		}
		t.Run(p.Name()+"_overrides_nonempty", func(t *testing.T) {
			for _, f := range overrideFields {
				if f == "" {
					t.Errorf("pattern %q: override schema contains empty field name", p.Name())
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestSkillSyncDoctor_ChartPromotionTable — chart code documentation and
// promotion-ladder validity
// ---------------------------------------------------------------------------

func TestSkillSyncDoctor_ChartPromotionTable(t *testing.T) {
	skillText := readFile(t, "../../skills/generate-deck/SKILL.md")

	for _, code := range allChartFindingCodes() {
		t.Run(code, func(t *testing.T) {
			if !strings.Contains(skillText, code) {
				t.Errorf("chart code %q not documented in SKILL.md", code)
			}

			base := core.Finding{Code: code, Severity: core.SeverityWarning}
			for _, level := range []string{"off", "warn", "strict"} {
				result := core.PromoteFindings([]core.Finding{base}, level)
				sev := result[0].Severity
				switch sev {
				case core.SeverityInfo, core.SeverityWarning,
					core.SeverityShrinkOrSplit, core.SeverityRefuse:
					// valid
				default:
					t.Errorf("level=%q: unexpected severity %q", level, sev)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func allChartFindingCodes() []string {
	return []string{
		core.FindingInvalidNumeric,
		core.FindingZeroSumPie,
		core.FindingNegativeOnLog,
		core.FindingAllZeroSeries,
		core.FindingCapacityExceeded,
		core.FindingInvalidTimeFormat,
		core.FindingAutoLogScaleApplied,
		core.FindingTickThinned,
		core.FindingScatterLabelSkipped,
		core.FindingLabelTruncated,
		core.FindingLabelEllipsized,
		core.FindingLabelClipped,
		core.FindingLegendOverflowDropped,
		core.FindingOverflowSuppressed,
	}
}

func collectAllFindingCodes(t *testing.T) []string {
	t.Helper()
	codes := patterns.AllFitFindingCodes()
	codes = append(codes, allChartFindingCodes()...)
	for code := range stringLiteralCodes {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// extractDocumentedCodes returns the subset of knownCodes (plus
// stringLiteralCodes) that appear as backtick-quoted identifiers in docText.
func extractDocumentedCodes(docText string, knownCodes []string) []string {
	var found []string
	seen := make(map[string]bool)
	for _, code := range knownCodes {
		if strings.Contains(docText, "`"+code+"`") {
			found = append(found, code)
			seen[code] = true
		}
	}
	for code := range stringLiteralCodes {
		if !seen[code] && strings.Contains(docText, "`"+code+"`") {
			found = append(found, code)
		}
	}
	sort.Strings(found)
	return found
}

func verifyStringLiteral(t *testing.T, code, srcFile string) {
	t.Helper()
	data, err := os.ReadFile(srcFile)
	if err != nil {
		t.Errorf("cannot read %s to verify %q: %v", srcFile, code, err)
		return
	}
	if !strings.Contains(string(data), `"`+code+`"`) {
		t.Errorf("finding code %q expected as string literal in %s but not found\n"+
			"  Fix: add the literal, or update stringLiteralCodes", code, srcFile)
	}
}

func extractOverrideFields(schemaBytes []byte) []string {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	overridesRaw, ok := schema.Properties["overrides"]
	if !ok {
		return nil
	}
	var overridesSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(overridesRaw, &overridesSchema); err != nil {
		return nil
	}
	fields := make([]string, 0, len(overridesSchema.Properties))
	for name := range overridesSchema.Properties {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(data)
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}
