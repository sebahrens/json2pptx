package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// textFitAllowlistPath is the JSON allow-list of templates exempt from the
// text-fit corpus gate. Failures on allow-listed templates are reported via
// t.Logf so they remain visible in CI output but do not fail the test.
// Repairing a template changes its sha256, which drops the allow-list entry
// and re-enables enforcement.
const textFitAllowlistPath = "testdata/text_fit_levels/allowlist.json"

// TestTextFitCorpus stresses the text-fit engine against every body
// placeholder on every layout of every bundled template, at six progressively
// denser content levels (P0–P5).
//
// For each (template, layout, body placeholder, P-level) cell, the test calls
// textfit.Calculate against the placeholder's resolved bounds + font size and
// classifies the resulting FitResult into one of six overflow severities.
// It then asserts the actual severity is no worse than the fixture's nominal
// level — a P2 fixture must never trigger P5 overflow, though a P3 fixture
// that happens to fit at full size on a very large body placeholder is
// allowed (better-than-expected is never an error).
//
// Designer templates known to have undersized or broken body placeholders are
// allow-listed by sha256; their failures are logged via t.Logf rather than
// failing the gate. Acceptance criterion #4 ("All templates green after vqad
// fixes land") is reified by the allow-list: as each template is repaired its
// sha256 will no longer match, automatically restoring the gate.
//
// Acceptance: bd go-slide-creator-f5v6.
func TestTextFitCorpus(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	tmplDir := filepath.Join(projectRoot, "templates")

	files, err := filepath.Glob(filepath.Join(tmplDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no templates found under %s", tmplDir)
	}
	sort.Strings(files)

	allowlist := loadTextFitAllowlist(t)
	allowed := make(map[string]textFitAllowlistEntry, len(allowlist.Exceptions))
	for _, e := range allowlist.Exceptions {
		if e.SHA256 == "" {
			t.Fatalf("text-fit allow-list entry for %s is missing sha256", e.Template)
		}
		if e.TrackingIssue == "" {
			t.Fatalf("text-fit allow-list entry for %s is missing tracking_issue", e.Template)
		}
		allowed[strings.ToLower(e.SHA256)] = e
	}

	levels := loadTextFitLevels(t, filepath.Join("testdata", "text_fit_levels"))

	for _, file := range files {
		file := file
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate(file)
			if err != nil {
				t.Fatalf("OpenTemplate(%s): %v", name, err)
			}
			defer func() { _ = reader.Close() }()

			layouts, err := template.ParseLayouts(reader)
			if err != nil {
				t.Fatalf("ParseLayouts(%s): %v", name, err)
			}
			if _, err := template.NormalizeLayoutFiles(reader, layouts); err != nil {
				t.Fatalf("NormalizeLayoutFiles(%s): %v", name, err)
			}

			failures, tested := collectTextFitFailures(layouts, name, levels)

			sha := strings.ToLower(reader.Hash())
			if entry, ok := allowed[sha]; ok {
				if entry.Template != name {
					t.Errorf(
						"text-fit allow-list entry %s names template %q but matched %q via sha256 %s — fix the allow-list entry",
						entry.TrackingIssue, entry.Template, name, sha,
					)
				}
				if len(failures) == 0 {
					t.Errorf(
						"%s is on the text-fit allow-list (tracking %s) but every body placeholder now stays within its expected overflow level — remove the allow-list entry",
						name, entry.TrackingIssue,
					)
				}
				for _, f := range failures {
					t.Logf("%s allow-listed under %s: %s", name, entry.TrackingIssue, f)
				}
				return
			}

			if tested == 0 {
				// Not necessarily an error — a template with zero body
				// placeholders has nothing to measure — but it almost always
				// indicates a broken/empty template, so make it visible.
				t.Logf("template %s has no measurable body placeholders", name)
			}

			if len(failures) > 0 {
				t.Errorf(
					"template %s is not on the text-fit allow-list and has %d failure(s) (sha256=%s).\n"+
						"  Either repair the template, or add an exception to %s with sha256=%s and a tracking issue.\n  %s",
					name, len(failures), sha,
					textFitAllowlistPath, sha,
					strings.Join(failures, "\n  "),
				)
			}
		})
	}
}

// TestTextFitCorpus_AllowlistEntriesPointAtRealTemplates makes sure every
// text-fit allow-list entry matches a current templates/*.pptx file by
// sha256. A stale entry (template repaired or removed) means the allow-list
// is lying about what is currently broken.
func TestTextFitCorpus_AllowlistEntriesPointAtRealTemplates(t *testing.T) {
	al := loadTextFitAllowlist(t)
	if len(al.Exceptions) == 0 {
		t.Skip("text-fit allow-list is empty — nothing to validate")
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	tmplDir := filepath.Join(projectRoot, "templates")
	files, err := filepath.Glob(filepath.Join(tmplDir, "*.pptx"))
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}

	hashes := make(map[string]string, len(files))
	for _, f := range files {
		r, err := template.OpenTemplate(f)
		if err != nil {
			t.Fatalf("OpenTemplate(%s): %v", f, err)
		}
		hashes[strings.ToLower(r.Hash())] = filepath.Base(f)
		_ = r.Close()
	}

	for _, entry := range al.Exceptions {
		key := strings.ToLower(entry.SHA256)
		got, ok := hashes[key]
		if !ok {
			t.Errorf(
				"stale text-fit allow-list entry: %s (sha256 %s) does not match any current template; remove it from %s",
				entry.Template, entry.SHA256, textFitAllowlistPath,
			)
			continue
		}
		if got != entry.Template {
			t.Errorf(
				"text-fit allow-list entry names %s but sha256 %s now belongs to %s; update %s",
				entry.Template, entry.SHA256, got, textFitAllowlistPath,
			)
		}
	}
}

// collectTextFitFailures walks every body placeholder on every layout in the
// template, runs textfit.Calculate for each P-level fixture, and returns
// human-readable failure strings for every placeholder × level whose
// classified severity exceeds the fixture's nominal level. Returns the
// failure list and the count of placeholders actually measured.
func collectTextFitFailures(layouts []types.LayoutMetadata, templateName string, levels []textFitLevel) ([]string, int) {
	var failures []string
	tested := 0

	for li := range layouts {
		layout := &layouts[li]
		for pi := range layout.Placeholders {
			ph := &layout.Placeholders[pi]
			if !isMeasurableBody(ph) {
				continue
			}
			tested++
			for _, lvl := range levels {
				params := textfit.Params{
					WidthEMU:    ph.Bounds.Width,
					HeightEMU:   ph.Bounds.Height,
					FontSizeHPt: pickFontSizeHPt(ph.FontSize),
					FontName:    pickFontName(ph.FontFamily),
					Paragraphs:  lvl.Paragraphs,
				}
				res, err := textfit.Calculate(params)
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"template=%s layout=%q placeholder=%q level=P%d: textfit.Calculate error: %v",
						templateName, layout.Name, ph.ID, lvl.Level, err,
					))
					continue
				}
				got := classifyFitResult(res)
				if got > lvl.Level {
					failures = append(failures, fmt.Sprintf(
						"template=%s layout=%q placeholder=%q level=P%d: got severity P%d (FontScale=%d LnSpcReduction=%d Overflow=%t), expected ≤ P%d",
						templateName, layout.Name, ph.ID, lvl.Level,
						got, res.FontScale, res.LnSpcReduction, res.Overflow,
						lvl.Level,
					))
				}
			}
		}
	}
	return failures, tested
}

// textFitLevel pairs a P-level (0–5) with its fixture text split into
// paragraphs.
type textFitLevel struct {
	Level      int
	Paragraphs []string
}

// loadTextFitLevels reads P0..P5.txt from the given directory and returns
// them as paragraphs (one paragraph per non-empty line, mirroring how the
// engine receives bulleted content from the slide pipeline).
func loadTextFitLevels(t *testing.T, dir string) []textFitLevel {
	t.Helper()
	out := make([]textFitLevel, 0, 6)
	for lvl := 0; lvl <= 5; lvl++ {
		path := filepath.Join(dir, fmt.Sprintf("P%d.txt", lvl))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture %s: %v", path, err)
		}
		paras := splitParagraphs(string(data))
		// P0 may collapse to a single short line; that is intentional.
		out = append(out, textFitLevel{Level: lvl, Paragraphs: paras})
	}
	return out
}

// splitParagraphs splits a fixture into one paragraph per non-empty line.
func splitParagraphs(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

// classifyFitResult maps a textfit.FitResult to a P-level severity in [0,5].
//
//	P0: fits at full size (no scaling)
//	P1: font scaled ≥90%
//	P2: font scaled ≥75%
//	P3: font scaled to minimum, no line spacing reduction
//	P4: font at minimum + line spacing reduction, no overflow
//	P5: still overflows at engine floor
func classifyFitResult(r textfit.FitResult) int {
	switch {
	case r.Overflow:
		return 5
	case r.LnSpcReduction > 0:
		return 4
	case r.FontScale == 0:
		return 0
	case r.FontScale >= 90_000:
		return 1
	case r.FontScale >= 75_000:
		return 2
	default:
		return 3
	}
}

// isMeasurableBody reports whether the placeholder is a body/content slot
// large enough to be a meaningful fit-stress target.
//
// Filters out:
//   - non-body types (title, subtitle, image, chart, table, utility)
//   - the "Section Number" placeholder (oversized display numeral, not body
//     text — feeding paragraphs into it is meaningless)
//   - placeholders with zero or implausibly small bounds (master inheritance
//     was incomplete, or the placeholder is decorative)
func isMeasurableBody(ph *types.PlaceholderInfo) bool {
	if ph == nil {
		return false
	}
	if ph.Type != types.PlaceholderBody && ph.Type != types.PlaceholderContent {
		return false
	}
	if strings.EqualFold(ph.ID, "Section Number") {
		return false
	}
	// Require at least 1.5" × 0.75" to qualify as a body slot. Smaller
	// placeholders (badges, captions, decorative slots) produce noisy fit
	// findings that aren't part of the body-text contract this test enforces.
	const minWidth = int64(1_371_600) // 1.5 inches
	const minHeight = int64(685_800)  // 0.75 inches
	if ph.Bounds.Width < minWidth || ph.Bounds.Height < minHeight {
		return false
	}
	return true
}

// pickFontSizeHPt returns the placeholder font size in hundredths of a point,
// falling back to the engine default when the template did not resolve one.
func pickFontSizeHPt(resolved int) int {
	if resolved > 0 {
		return resolved
	}
	return 2000 // 20pt — matches textfit's internal default (tokens.BodyDefaultHPt)
}

// pickFontName returns the placeholder font family, defaulting to Arial so
// the fontcache resolves to embedded Liberation Sans in headless environments.
func pickFontName(resolved string) string {
	if resolved != "" {
		return resolved
	}
	return "Arial"
}

// textFitAllowlist mirrors the on-disk JSON schema for the text-fit
// allow-list. Same shape as the canonical placeholder allow-list so the two
// can share tooling.
type textFitAllowlist struct {
	SchemaVersion int                     `json:"$schema_version"`
	Comment       string                  `json:"comment,omitempty"`
	Exceptions    []textFitAllowlistEntry `json:"exceptions"`
}

type textFitAllowlistEntry struct {
	Template      string `json:"template"`
	SHA256        string `json:"sha256"`
	TrackingIssue string `json:"tracking_issue"`
	Reason        string `json:"reason"`
}

func loadTextFitAllowlist(t *testing.T) textFitAllowlist {
	t.Helper()
	data, err := os.ReadFile(textFitAllowlistPath)
	if err != nil {
		t.Fatalf("read text-fit allow-list %s: %v", textFitAllowlistPath, err)
	}
	var al textFitAllowlist
	if err := json.Unmarshal(data, &al); err != nil {
		t.Fatalf("parse text-fit allow-list %s: %v", textFitAllowlistPath, err)
	}
	return al
}
