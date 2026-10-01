package examine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
)

// Markdown renders a human-readable summary of the report: a header, the
// canonical-coverage pass/fail matrix, the derivable-layout matrix, a per-layout
// table, and the remediation list pulled from the findings envelope.
func (r *Report) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Template examination: %s\n\n", r.Template)
	if r.SHA256 != "" {
		fmt.Fprintf(&b, "- SHA-256: `%s`\n", r.SHA256)
	}
	fmt.Fprintf(&b, "- Aspect ratio: %s\n", r.AspectRatio)
	fmt.Fprintf(&b, "- Slide: %.3f in × %.3f in\n", r.Slide.WidthIn, r.Slide.HeightIn)
	fmt.Fprintf(&b, "- Theme: %s (titles %s, body %s)\n", emptyDash(r.Theme.Name), emptyDash(r.Theme.TitleFont), emptyDash(r.Theme.BodyFont))
	status := "PASS"
	if !r.Findings.OK {
		status = "ATTENTION"
	}
	fmt.Fprintf(&b, "- Findings: %s — %s\n\n", status, r.Findings.Summary)

	// Canonical coverage matrix.
	b.WriteString("## Canonical coverage\n\n")
	b.WriteString("| Family | Present | Layouts |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, fam := range canonicalFamilies {
		c := r.CanonicalCoverage[string(fam)]
		mark := "FAIL"
		if c.Present {
			mark = "OK"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", string(fam), mark, emptyDash(strings.Join(c.Layouts, ", ")))
	}
	b.WriteString("\n")

	writeGridMarkdown(&b, r.Grid)

	// Derivable capabilities matrix.
	b.WriteString("## Derivable capabilities\n\n")
	b.WriteString("| Capability | Ready | Layout ID | Request via | Missing |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, d := range r.DerivableLayouts {
		mark := "no"
		if d.Ready {
			mark = "yes"
		}
		address := "—"
		if d.AddressableAs != nil {
			address = *d.AddressableAs
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", d.Name, mark, address, d.RequestVia, emptyDash(strings.Join(d.Missing, "; ")))
	}
	b.WriteString("\n")

	// Layout inventory.
	b.WriteString("## Layouts\n\n")
	b.WriteString("| # | Name | Canonical | Family | Conf | Placeholders |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for i := range r.Layouts {
		l := &r.Layouts[i]
		ct := l.CanonicalType
		if ct == "" {
			ct = "unknown"
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %.2f | %d |\n",
			l.Index, emptyDash(l.Name), ct, emptyDash(l.CanonicalFamily), l.CanonicalConfidence, len(l.Placeholders))
	}
	b.WriteString("\n")

	// Remediation list from findings.
	b.WriteString("## Remediation\n\n")
	if len(r.Findings.Findings) == 0 {
		b.WriteString("No findings — template is ready to use.\n")
		return b.String()
	}
	findings := make([]diagnosticLine, 0, len(r.Findings.Findings))
	for _, f := range r.Findings.Findings {
		findings = append(findings, diagnosticLine{code: f.Code, sev: string(f.Severity), msg: f.Message})
	}
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].code < findings[j].code })
	for _, f := range findings {
		fmt.Fprintf(&b, "- **[%s] %s** — %s\n", strings.ToUpper(f.sev), f.code, f.msg)
	}
	return b.String()
}

// diagnosticLine is a flattened finding for the remediation list.
type diagnosticLine struct {
	code, sev, msg string
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// writeGridMarkdown renders the template grid section (go-slide-creator-5ms8c).
func writeGridMarkdown(b *strings.Builder, g template.ResolvedGrid) {
	b.WriteString("## Grid\n\n")
	declared := "none (engine defaults)"
	if len(g.Declared) > 0 {
		declared = strings.Join(g.Declared, ", ")
	}
	fmt.Fprintf(b, "- Declared in metadata: %s\n", declared)
	fmt.Fprintf(b, "- margin_pct %.1f, columns %d, gutter_pt %g, title_gap_pt %g\n", g.MarginPct, g.Columns, g.GutterPt, g.TitleGapPt)
	if f := g.Frame; f != nil {
		fmt.Fprintf(b, "- Content frame: title bottom %.1fpt, content top %.1fpt, left %.1fpt, right %.1fpt\n", f.TitleBottomPt, f.ContentTopPt, f.LeftPt, f.RightPt)
	}
	b.WriteString("\n")
}
