package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/layoutpreview"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// examineConformance is the merged validate-template + template-check evidence
// written to conformance.json. Both checks examine the same template from
// different angles, so the examination bundles them under one file.
type examineConformance struct {
	Template         string                      `json:"template"`
	ValidateTemplate examineValidateSummary      `json:"validate_template"`
	TemplateCheck    *template.ConformanceReport `json:"template_check"`
}

// examineValidateSummary is the validate-template slice of conformance.json.
type examineValidateSummary struct {
	Valid        bool                        `json:"valid"`
	Capabilities validateTemplateCapabilites `json:"capabilities"`
	Findings     diagnostics.FindingEnvelope `json:"findings"`
}

// canonicalRolesReport is the per-layout canonical-group + per-placeholder-role
// classification written to canonical_roles.json.
type canonicalRolesReport struct {
	Template string                `json:"template"`
	Layouts  []canonicalRolesEntry `json:"layouts"`
}

type canonicalRolesEntry struct {
	ID              string                      `json:"id"`
	Name            string                      `json:"name"`
	CanonicalType   string                      `json:"canonical_type"`
	CanonicalFamily string                      `json:"canonical_family"`
	Placeholders    []canonicalRolesPlaceholder `json:"placeholders"`
}

type canonicalRolesPlaceholder struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	Role           string  `json:"role"`
	RoleConfidence float64 `json:"role_confidence"`
}

// runExamineTemplate implements the "examine-template" subcommand. It produces
// a directory of artifacts an agent or human can read to learn exactly what a
// user-provided template supports: report.json (+ report.md), per-layout raw
// XML / parsed JSON / annotated SVG / best-effort PNG, master XML+JSON,
// theme.json, conformance.json, and canonical_roles.json.
func runExamineTemplate() error {
	fs := flag.NewFlagSet("examine-template", flag.ContinueOnError)
	outDir := fs.String("out", "./examination", "Output directory for the examination artifacts")
	jsonPath := fs.String("json", "", "Also write report.json to this path (in addition to <out>/report.json)")
	strict := fs.Bool("strict", false, "Fail metadata validation on warnings, not just errors")
	dpi := fs.Int("dpi", 110, "DPI for the per-layout PNG render (when LibreOffice + ImageMagick are available)")
	noPNG := fs.Bool("no-png", false, "Skip the LibreOffice PNG render pass (SVG overlays are always produced)")
	gate := fs.Bool("gate", false, "Apply the template CI gate: write gate.json and exit non-zero on any violation")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx examine-template <template.pptx> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Examine a PPTX template and emit a full capability report (visual + XML + canonical roles).\n\n")
		fmt.Fprintf(os.Stderr, "Outputs (under --out):\n")
		fmt.Fprintf(os.Stderr, "  report.json report.md theme.json conformance.json canonical_roles.json\n")
		fmt.Fprintf(os.Stderr, "  layouts/slideLayoutN__<canonical>.{json,xml,svg,png}\n")
		fmt.Fprintf(os.Stderr, "  master/slideMasterN.{xml,json}\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx examine-template templates/midnight-blue.pptx --out ./examination\n")
		fmt.Fprintf(os.Stderr, "  json2pptx examine-template new.pptx --out ./out --json ./out/report.json\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	// Accept the template path either before the flags (the documented
	// `examine-template <path> [options]` order) or after them.
	rest := os.Args[1:]
	var templatePath string
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		templatePath = rest[0]
		rest = rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if templatePath == "" && fs.NArg() > 0 {
		templatePath = fs.Arg(0)
	} else if templatePath != "" && fs.NArg() > 0 {
		fs.Usage()
		return fmt.Errorf("examine-template: unexpected extra arguments: %v", fs.Args())
	}
	if templatePath == "" {
		fs.Usage()
		return fmt.Errorf("examine-template: exactly one <template.pptx> argument is required")
	}

	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()

	report, err := examine.Examine(reader, examine.Options{TemplatePath: templatePath, Strict: *strict})
	if err != nil {
		return fmt.Errorf("examine-template: %w", err)
	}

	if err := writeExamination(reader, report, templatePath, *outDir, *strict); err != nil {
		return err
	}

	// Best-effort PNG render of each layout (graceful no-op when LibreOffice /
	// ImageMagick are absent).
	pngCount := 0
	if !*noPNG {
		pngCount = renderLayoutPNGs(reader, report, templatePath, *outDir, *dpi)
	}

	if *jsonPath != "" {
		if err := writeJSONFile(*jsonPath, report); err != nil {
			return fmt.Errorf("examine-template: write %s: %w", *jsonPath, err)
		}
	}

	printExamineSummary(report, *outDir, pngCount, *noPNG)

	if *gate {
		return runExamineGate(report, *outDir)
	}
	return nil
}

// examineGateResult is the gate verdict written to gate.json: a per-template
// pass/fail roll-up an agent or CI step can branch on, plus the precise
// violations behind a failure.
type examineGateResult struct {
	Template   string                  `json:"template"`
	Passed     bool                    `json:"passed"`
	Violations []examine.GateViolation `json:"violations"`
}

// runExamineGate applies the template CI gate to a built report, writes the
// verdict to <out>/gate.json (so a failing run still leaves an inspectable
// artifact), prints the result, and returns a non-nil error when any gate
// check fails — which propagates to a non-zero process exit via main.dispatch.
func runExamineGate(report *examine.Report, outDir string) error {
	violations := examine.Gate(report)
	if violations == nil {
		violations = []examine.GateViolation{}
	}
	result := examineGateResult{
		Template:   report.Template,
		Passed:     len(violations) == 0,
		Violations: violations,
	}
	if err := writeJSONFile(filepath.Join(outDir, "gate.json"), result); err != nil {
		return fmt.Errorf("examine-template: write gate.json: %w", err)
	}

	if len(violations) == 0 {
		fmt.Println("Gate:     PASS (no violations)")
		return nil
	}

	fmt.Printf("Gate:     FAIL (%d violation(s))\n", len(violations))
	for _, v := range violations {
		fmt.Printf("  [%s] %s\n", v.Code, v.Message)
	}
	return fmt.Errorf("examine-template gate failed: %d violation(s) in %s", len(violations), report.Template)
}

// writeExamination materialises the report into the output directory tree.
func writeExamination(reader *template.Reader, report *examine.Report, templatePath, outDir string, strict bool) error {
	layoutsDir := filepath.Join(outDir, "layouts")
	masterDir := filepath.Join(outDir, "master")
	for _, d := range []string{outDir, layoutsDir, masterDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}

	if err := writeJSONFile(filepath.Join(outDir, "report.json"), report); err != nil {
		return fmt.Errorf("write report.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "report.md"), []byte(report.Markdown()), 0o644); err != nil { //nolint:gosec // diagnostic output
		return fmt.Errorf("write report.md: %w", err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "theme.json"), report.Theme); err != nil {
		return fmt.Errorf("write theme.json: %w", err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "canonical_roles.json"), buildCanonicalRoles(report)); err != nil {
		return fmt.Errorf("write canonical_roles.json: %w", err)
	}
	if err := writeJSONFile(filepath.Join(outDir, "conformance.json"), buildExamineConformance(reader, report, templatePath, strict)); err != nil {
		return fmt.Errorf("write conformance.json: %w", err)
	}

	// Per-layout artifacts: parsed JSON, pretty raw XML, annotated SVG.
	for i := range report.Layouts {
		l := &report.Layouts[i]
		base := filepath.Join(layoutsDir, l.AssetBase)
		if err := writeJSONFile(base+".json", l); err != nil {
			return fmt.Errorf("write %s.json: %w", l.AssetBase, err)
		}
		if raw, rerr := reader.ReadFile(l.XMLPath); rerr == nil {
			if err := os.WriteFile(base+".xml", prettyXML(raw), 0o644); err != nil { //nolint:gosec // diagnostic output
				return fmt.Errorf("write %s.xml: %w", l.AssetBase, err)
			}
		}
		svg := examine.RenderLayoutSVG(*l, report.Slide, report.Theme)
		if err := os.WriteFile(base+".svg", []byte(svg), 0o644); err != nil { //nolint:gosec // diagnostic output
			return fmt.Errorf("write %s.svg: %w", l.AssetBase, err)
		}
	}

	// Master artifacts: pretty raw XML + minimal parsed JSON.
	for _, m := range report.Masters {
		base := filepath.Join(masterDir, m.Name)
		if raw, rerr := reader.ReadFile(m.XMLPath); rerr == nil {
			if err := os.WriteFile(base+".xml", prettyXML(raw), 0o644); err != nil { //nolint:gosec // diagnostic output
				return fmt.Errorf("write %s.xml: %w", m.Name, err)
			}
		}
		if err := writeJSONFile(base+".json", m); err != nil {
			return fmt.Errorf("write %s.json: %w", m.Name, err)
		}
	}
	return nil
}

// renderLayoutPNGs renders one slide per layout and copies the resulting PNGs
// into layouts/<assetBase>.png. Returns the number of PNGs written; zero when
// LibreOffice / ImageMagick are unavailable.
func renderLayoutPNGs(reader *template.Reader, report *examine.Report, templatePath, outDir string, dpi int) int {
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return 0
	}
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Hash:         reader.Hash(),
		AspectRatio:  report.AspectRatio,
		Layouts:      layouts,
		Theme:        template.ParseTheme(reader),
	}
	res, err := layoutpreview.Generate(templatePath, analysis, &layoutpreview.Options{DPI: dpi})
	if err != nil || res == nil {
		return 0
	}
	layoutsDir := filepath.Join(outDir, "layouts")
	count := 0
	for i := range report.Layouts {
		l := &report.Layouts[i]
		src, ok := res.Paths[l.ID]
		if !ok {
			continue
		}
		data, rerr := os.ReadFile(src) //nolint:gosec // path from layoutpreview cache
		if rerr != nil {
			continue
		}
		if werr := os.WriteFile(filepath.Join(layoutsDir, l.AssetBase+".png"), data, 0o644); werr == nil { //nolint:gosec // diagnostic output
			count++
		}
	}
	return count
}

// buildCanonicalRoles projects the report into the canonical_roles.json shape.
func buildCanonicalRoles(report *examine.Report) canonicalRolesReport {
	out := canonicalRolesReport{Template: report.Template}
	for i := range report.Layouts {
		l := &report.Layouts[i]
		entry := canonicalRolesEntry{
			ID:              l.ID,
			Name:            l.Name,
			CanonicalType:   l.CanonicalType,
			CanonicalFamily: l.CanonicalFamily,
		}
		for j := range l.Placeholders {
			ph := &l.Placeholders[j]
			entry.Placeholders = append(entry.Placeholders, canonicalRolesPlaceholder{
				ID:             ph.ID,
				Type:           ph.Type,
				Role:           ph.Role,
				RoleConfidence: ph.RoleConfidence,
			})
		}
		out.Layouts = append(out.Layouts, entry)
	}
	return out
}

// buildExamineConformance merges the validate-template and template-check
// evidence into one structure for conformance.json.
func buildExamineConformance(reader *template.Reader, report *examine.Report, templatePath string, strict bool) examineConformance {
	merged := examineConformance{Template: report.Template}

	layouts, err := template.ParseLayouts(reader)
	if err == nil {
		theme := template.ParseTheme(reader)
		vr := template.ValidateTemplateMetadata(reader, strict)
		template.ApplyMetadataHints(layouts, vr.Metadata)

		synthLayouts := make([]types.LayoutMetadata, len(layouts))
		copy(synthLayouts, layouts)
		synthAnalysis := &types.TemplateAnalysis{
			TemplatePath: templatePath,
			Hash:         reader.Hash(),
			AspectRatio:  report.AspectRatio,
			Layouts:      synthLayouts,
			Theme:        theme,
			Metadata:     vr.Metadata,
		}
		_ = template.SynthesizeIfNeeded(reader, synthAnalysis)

		sectionDiags := checkSectionNumberNaming(layouts)
		merged.ValidateTemplate = examineValidateSummary{
			Valid:        vr.Valid,
			Capabilities: detectCapabilities(synthAnalysis.Layouts, synthAnalysis.Synthesis),
			Findings:     buildTemplateFindings(report.Template, vr.Diagnostics, sectionDiags),
		}
	}

	if cr, cerr := template.CheckConformance(templatePath); cerr == nil {
		merged.TemplateCheck = cr
	}
	return merged
}

// printExamineSummary prints a short human-readable summary to stdout.
func printExamineSummary(report *examine.Report, outDir string, pngCount int, noPNG bool) {
	fmt.Printf("Examined: %s\n", report.Template)
	fmt.Printf("Output:   %s\n", outDir)
	fmt.Printf("Layouts:  %d   Findings: %s (%s)\n", len(report.Layouts), boolPassAttention(report.Findings.OK), report.Findings.Summary)

	fmt.Println("Canonical coverage:")
	for _, fam := range []string{"title-slide", "section-divider", "one-content", "qa-closing"} {
		c := report.CanonicalCoverage[fam]
		mark := "MISSING"
		if c.Present {
			mark = "ok"
		}
		fmt.Printf("  %-16s %s\n", fam, mark)
	}

	switch {
	case noPNG:
		fmt.Println("PNG render: skipped (--no-png)")
	case pngCount == 0:
		fmt.Println("PNG render: skipped (LibreOffice / ImageMagick not found); SVG overlays written")
	default:
		fmt.Printf("PNG render: %d layout image(s) written\n", pngCount)
	}
}

func boolPassAttention(ok bool) string {
	if ok {
		return "PASS"
	}
	return "ATTENTION"
}

// writeJSONFile marshals v as indented JSON to path.
func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644) //nolint:gosec // diagnostic output to user-specified path
}

// prettyXML re-indents XML by inserting newlines and indentation between tags
// without re-parsing, so namespaces, prefixes, and significant text content are
// preserved byte-for-byte while inter-tag formatting whitespace is normalised.
func prettyXML(raw []byte) []byte {
	const indent = "  "
	s := strings.TrimSpace(string(raw))
	var out strings.Builder
	depth := 0
	for i := 0; i < len(s); {
		if s[i] != '<' {
			i = writeStrayXMLText(&out, s, i)
			continue
		}
		end := strings.IndexByte(s[i:], '>')
		if end < 0 {
			out.WriteString(s[i:])
			break
		}
		tag := s[i : i+end+1]
		i += end + 1
		if isXMLCloseTag(tag) && depth > 0 {
			depth--
		}
		writeIndentedXMLTag(&out, tag, indent, depth)
		if isXMLOpenTag(tag) {
			depth++
			i, depth = writeInlineXMLText(&out, s, i, depth)
		}
	}
	out.WriteByte('\n')
	return []byte(out.String())
}

func isXMLCloseTag(tag string) bool { return strings.HasPrefix(tag, "</") }

func isXMLOpenTag(tag string) bool {
	selfClosing := strings.HasSuffix(tag, "/>") || strings.HasPrefix(tag, "<?") || strings.HasPrefix(tag, "<!")
	return !isXMLCloseTag(tag) && !selfClosing
}

// writeStrayXMLText copies non-whitespace text up to the next tag, dropping
// formatting whitespace between top-level tags. Returns the new scan position.
func writeStrayXMLText(out *strings.Builder, s string, i int) int {
	k := strings.IndexByte(s[i:], '<')
	if k < 0 {
		if strings.TrimSpace(s[i:]) != "" {
			out.WriteString(s[i:])
		}
		return len(s)
	}
	if strings.TrimSpace(s[i:i+k]) != "" {
		out.WriteString(s[i : i+k])
	}
	return i + k
}

// writeIndentedXMLTag writes a tag on its own indented line.
func writeIndentedXMLTag(out *strings.Builder, tag, indent string, depth int) {
	if out.Len() > 0 {
		out.WriteByte('\n')
	}
	out.WriteString(strings.Repeat(indent, depth))
	out.WriteString(tag)
}

// writeInlineXMLText keeps simple text content and its matching close tag on the
// same line as the opening tag (so <a:t>hello</a:t> stays intact). Returns the
// new scan position and depth.
func writeInlineXMLText(out *strings.Builder, s string, i, depth int) (int, int) {
	k := strings.IndexByte(s[i:], '<')
	if k < 0 {
		k = len(s) - i
	}
	if strings.TrimSpace(s[i:i+k]) == "" {
		return i, depth
	}
	out.WriteString(s[i : i+k])
	i += k
	if i < len(s) && strings.HasPrefix(s[i:], "</") {
		if j := strings.IndexByte(s[i:], '>'); j >= 0 {
			out.WriteString(s[i : i+j+1])
			if depth > 0 {
				depth--
			}
			i += j + 1
		}
	}
	return i, depth
}
