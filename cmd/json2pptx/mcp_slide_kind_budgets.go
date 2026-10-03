package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/templates"
)

// list_slide_kinds budgets (go-slide-creator-iubjb).
//
// The kind catalogue said "past its text budgets it degrades" without stating
// them, and the budgets that depend on the template — how long a title or a
// takeaway can be — were learned one refusal at a time: a 40-character closing
// title the catalogue allowed wrapped on a template whose closing title holds
// 20. A budgets request returns, per kind, every authored field's budget: the
// title, subtitle and takeaway measured on the named template (or the
// tightest of the shipped templates when none is named), and the fixed
// budgets the compiler enforces on every template.

// slideKindBudget is one authored field's text budget.
type slideKindBudget struct {
	// Field is the payload path under the slide; "[]" marks a list entry and
	// "+" joins fields that share one budget.
	Field string `json:"field"`
	// MaxChars is the most characters the field holds.
	MaxChars int `json:"max_chars,omitempty"`
	// MaxCharsPerLine is a title's conservative single-line width.
	MaxCharsPerLine int `json:"max_chars_per_line,omitempty"`
	// MaxLines is the line count MaxChars assumes.
	MaxLines int `json:"max_lines,omitempty"`
	// MinItems / MaxItems bound a list field.
	MinItems int `json:"min_items,omitempty"`
	MaxItems int `json:"max_items,omitempty"`
	// Basis is "measured" (from the template's own placeholders and chrome
	// band) or "fixed" (the same on every template).
	Basis string `json:"basis"`
	Note  string `json:"note,omitempty"`
}

// slideKindBudgetBasis says which template(s) the measured budgets describe.
type slideKindBudgetBasis struct {
	// Template is the template the budgets were measured on.
	Template string `json:"template,omitempty"`
	// Templates lists the shipped templates whose tightest budget is reported
	// when no template was named.
	Templates []string `json:"templates,omitempty"`
	Note      string   `json:"note"`
}

// measuredTextBudget is one measured field: total characters, characters per
// line (titles) and the line count the total assumes.
type measuredTextBudget struct {
	MaxChars, PerLine, Lines int
}

func (b measuredTextBudget) tighter(o measuredTextBudget) measuredTextBudget {
	if b.MaxChars == 0 {
		return o
	}
	if o.MaxChars == 0 {
		return b
	}
	out := b
	out.MaxChars = min(b.MaxChars, o.MaxChars)
	if o.PerLine > 0 && (b.PerLine == 0 || o.PerLine < b.PerLine) {
		out.PerLine = o.PerLine
	}
	if o.Lines > 0 && (b.Lines == 0 || o.Lines < b.Lines) {
		out.Lines = o.Lines
	}
	return out
}

// Layout roles a kind's title is measured on.
const (
	budgetRoleCover   = "cover"
	budgetRoleSection = "section"
	budgetRoleClosing = "closing"
	budgetRoleContent = "content"
)

// templateTextBudgets are one template's measured budgets.
type templateTextBudgets struct {
	Title    map[string]measuredTextBudget // by layout role
	Subtitle map[string]measuredTextBudget // cover and closing
	Takeaway measuredTextBudget
}

func (t templateTextBudgets) tighter(o templateTextBudgets) templateTextBudgets {
	out := templateTextBudgets{Title: map[string]measuredTextBudget{}, Subtitle: map[string]measuredTextBudget{}}
	for _, role := range []string{budgetRoleCover, budgetRoleSection, budgetRoleClosing, budgetRoleContent} {
		out.Title[role] = t.Title[role].tighter(o.Title[role])
		out.Subtitle[role] = t.Subtitle[role].tighter(o.Subtitle[role])
	}
	out.Takeaway = t.Takeaway.tighter(o.Takeaway)
	return out
}

// budgetLayoutRoles maps a layout's canonical type to the title role it
// carries. Content kinds render on the template's One Content layout or, for a
// pattern, its Blank + Title layout; the tighter title of the two is reported.
var budgetLayoutRoles = map[types.CanonicalLayoutType]string{
	types.CanonicalLayoutTitleSlide:     budgetRoleCover,
	types.CanonicalLayoutSectionDivider: budgetRoleSection,
	types.CanonicalLayoutClosing:        budgetRoleClosing,
	types.CanonicalLayoutOneContent:     budgetRoleContent,
	types.CanonicalLayoutBlankTitle:     budgetRoleContent,
}

// measureTemplateBudgets measures a template's title, subtitle and takeaway
// budgets: the numbers examine_template reports for the same placeholders, and
// the takeaway band the takeaway fit finding measures.
func measureTemplateBudgets(analysis *types.TemplateAnalysis) templateTextBudgets {
	out := templateTextBudgets{Title: map[string]measuredTextBudget{}, Subtitle: map[string]measuredTextBudget{}}
	for i := range analysis.Layouts {
		layout := &analysis.Layouts[i]
		role, ok := budgetLayoutRoles[template.EffectiveCanonicalType(layout)]
		if !ok {
			continue
		}
		for j := range layout.Placeholders {
			ph := &layout.Placeholders[j]
			switch ph.Type {
			case types.PlaceholderTitle:
				perLine := generator.ReportedMaxCharsPerLine(ph)
				m := measuredTextBudget{MaxChars: generator.ReportedMaxChars(ph), PerLine: perLine}
				if perLine > 0 {
					m.Lines = max(1, m.MaxChars/perLine)
				}
				out.Title[role] = out.Title[role].tighter(m)
			case types.PlaceholderSubtitle:
				out.Subtitle[role] = out.Subtitle[role].tighter(measuredTextBudget{MaxChars: generator.ReportedMaxChars(ph)})
			}
		}
		if role == budgetRoleContent {
			_, _, budget := takeawayBandBudget(SlideInput{LayoutID: layout.ID, Takeaway: "x"}, analysis.Layouts, analysis.SlideWidth, analysis.SlideHeight)
			out.Takeaway = out.Takeaway.tighter(measuredTextBudget{MaxChars: budget.MaxChars, Lines: budget.MaxLines})
		}
	}
	return out
}

// budgetRoleForKind is the layout role a kind's title is measured on, or ""
// for the raw escape hatch, which authors its own slide.
func budgetRoleForKind(k semantic.SlideKind) string {
	switch k {
	case semantic.KindTitle:
		return budgetRoleCover
	case semantic.KindSection:
		return budgetRoleSection
	case semantic.KindClosing:
		return budgetRoleClosing
	case semantic.KindRawJSON2pptx:
		return ""
	}
	return budgetRoleContent
}

// slideKindBudgets assembles a kind's budgets: the measured title, subtitle
// and takeaway for the fields the kind reads, then its fixed budgets.
func slideKindBudgets(k semantic.SlideKind, measured templateTextBudgets) []slideKindBudget {
	role := budgetRoleForKind(k)
	if role == "" {
		return nil
	}
	reads := map[string]bool{}
	for _, name := range semantic.PayloadFieldNames(k) {
		reads[name] = true
	}
	var out []slideKindBudget
	if m := measured.Title[role]; reads["title"] && m.MaxChars > 0 {
		out = append(out, slideKindBudget{Field: "title", MaxChars: m.MaxChars, MaxCharsPerLine: m.PerLine, MaxLines: m.Lines, Basis: "measured"})
	}
	if m := measured.Subtitle[role]; reads["subtitle"] && k != semantic.KindStat && m.MaxChars > 0 {
		out = append(out, slideKindBudget{Field: "subtitle", MaxChars: m.MaxChars, Basis: "measured"})
	}
	if m := measured.Takeaway; reads["takeaway"] && m.MaxChars > 0 {
		out = append(out, slideKindBudget{Field: "takeaway", MaxChars: m.MaxChars, MaxLines: m.Lines, Basis: "measured"})
	}
	for _, b := range semantic.KindFieldBudgets(k) {
		out = append(out, slideKindBudget{Field: b.Field, MaxChars: b.MaxChars, MinItems: b.MinItems, MaxItems: b.MaxItems, Basis: "fixed", Note: b.Note})
	}
	return out
}

// embeddedTemplateNames lists the templates embedded in the binary.
func embeddedTemplateNames() []string {
	entries, err := fs.ReadDir(templates.Embedded, ".")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".pptx") {
			names = append(names, strings.TrimSuffix(e.Name(), ".pptx"))
		}
	}
	sort.Strings(names)
	return names
}

// newBudgetTemplateCache is the template cache budgets are measured through
// when no server config supplies one (the CLI's `semantic kinds`).
func newBudgetTemplateCache() *template.MemoryCache { return template.NewMemoryCache(time.Hour) }

// analyzeTemplateBytes analyses an embedded template: the parser needs a file,
// so the bytes go to a temporary one named after the template.
func analyzeTemplateBytes(name string, data []byte, cache types.TemplateCache) (*types.TemplateAnalysis, error) {
	dir, err := os.MkdirTemp("", "json2pptx-budgets-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := filepath.Join(dir, name+".pptx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	return getOrAnalyzeTemplate(path, cache)
}

// shippedBudgets caches the tightest measured budgets across the shipped
// templates: they are embedded, so the answer cannot change while the process
// runs.
var shippedBudgets struct {
	once   sync.Once
	value  templateTextBudgets
	names  []string
	failed error
}

// tightestShippedBudgets measures every shipped template once and keeps, per
// field, the tightest budget: copy written to it fits all of them.
func tightestShippedBudgets() (templateTextBudgets, []string, error) {
	s := &shippedBudgets
	s.once.Do(func() {
		cache := newBudgetTemplateCache()
		for _, name := range embeddedTemplateNames() {
			data, err := fs.ReadFile(templates.Embedded, name+".pptx")
			if err != nil {
				s.failed = fmt.Errorf("read shipped template %s: %w", name, err)
				return
			}
			analysis, err := analyzeTemplateBytes(name, data, cache)
			if err != nil {
				s.failed = fmt.Errorf("analyze shipped template %s: %w", name, err)
				return
			}
			measured := measureTemplateBudgets(analysis)
			if len(s.names) == 0 {
				s.value = measured
			} else {
				s.value = s.value.tighter(measured)
			}
			s.names = append(s.names, name)
		}
	})
	return s.value, s.names, s.failed
}
