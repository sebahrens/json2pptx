package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// softDarkOrangePalette stands in for the private template that exposed
// go-slide-creator-pr5bx: soft charcoal darks rather than black, and an orange
// primary with paler accents behind it. On #FD5108 neither the charcoal
// (3.7:1) nor white (3.3:1) reads at body size, so a pattern asking for the
// best theme ink on the plain accent gets one that fails.
var softDarkOrangePalette = map[string]string{
	"dk1": "#2E353A", "dk2": "#2E353A", "lt1": "#FFFFFF", "lt2": "#FFE8D4",
	"accent1": "#FD5108", "accent2": "#FE8A4F", "accent3": "#FFB38A",
	"accent4": "#A9AFB8", "accent5": "#BCC2C9", "accent6": "#D0D5DA",
}

// inkAuditVariant is one way of expanding a pattern for the ink audit.
type inkAuditVariant struct {
	label     string
	values    json.RawMessage
	overrides map[string]string
	// strict variants are written by hand and must expand; a variant made by
	// trying an enum value may be one the content does not allow.
	strict bool
}

func (v inkAuditVariant) overridesJSON() json.RawMessage {
	if len(v.overrides) == 0 {
		return nil
	}
	raw, _ := json.Marshal(v.overrides)
	return raw
}

// inkAuditExtraValues are value sets the exemplars do not exercise: an odd
// number of timeline stops (the middle link of the gradient is the plain
// accent), the chevron and arrow steps of a flow, highlighted steps, and the
// chevron strip.
var inkAuditExtraValues = map[string][]inkAuditVariant{
	"timeline-horizontal": {
		{label: "seven chevrons", strict: true, overrides: map[string]string{"style": "chevron"}, values: json.RawMessage(`[{"label":"Discovery","date":"Jan","body":"Scoping"},{"label":"Design","date":"Feb","body":"Blueprint"},{"label":"Build","date":"Mar","body":"Waves 1-2"},{"label":"Test","date":"Apr","body":"Rehearsal"},{"label":"Pilot","date":"May","body":"One site"},{"label":"Cutover","date":"Jun","body":"The weekend"},{"label":"Close","date":"Jul","body":"Handover"}]`)},
		{label: "seven gantt bars", strict: true, overrides: map[string]string{"style": "gantt"}, values: json.RawMessage(`[{"label":"Discovery","date":"2025-01","end_date":"2025-06"},{"label":"Design","date":"2025-02","end_date":"2025-07"},{"label":"Build","date":"2025-03","end_date":"2025-08"},{"label":"Test","date":"2025-04","end_date":"2025-09"},{"label":"Pilot","date":"2025-05","end_date":"2025-10"},{"label":"Cutover","date":"2025-06","end_date":"2025-11"},{"label":"Close","date":"2025-07","end_date":"2025-12"}]`)},
	},
	"process-flow": {
		{label: "chevron steps", strict: true, values: json.RawMessage(`{"steps":[{"label":"Collect intake","type":"chevron"},{"label":"Score the case","type":"chevron"},{"label":"Approve funding","type":"chevron","highlight":true},{"label":"Ship the pilot","type":"chevron"}]}`)},
		{label: "arrow steps", strict: true, values: json.RawMessage(`{"steps":[{"label":"Collect intake","type":"arrow"},{"label":"Score the case","type":"arrow"},{"label":"Approve funding","type":"arrow","highlight":true},{"label":"Ship the pilot","type":"arrow"}]}`)},
		{label: "decision and highlight", strict: true, values: json.RawMessage(`{"steps":[{"label":"Collect intake"},{"label":"Worth funding?","type":"decision"},{"label":"Approve funding","highlight":true},{"label":"Ship the pilot"}]}`)},
	},
	"value-chain": {
		{label: "highlighted step", strict: true, values: json.RawMessage(`{"steps":[{"label":"Source","description":"Qualified suppliers"},{"label":"Make","description":"Two plants","highlight":true},{"label":"Move","description":"Regional hubs"},{"label":"Sell","description":"Direct and partner"},{"label":"Serve","description":"Field teams"}]}`)},
	},
	"numbered-step-strip": {
		{label: "chevron strip", strict: true, values: json.RawMessage(`{"style":"chevron","steps":[{"label":"Diagnose","body":"Baseline the spend"},{"label":"Design","body":"Agree the target"},{"label":"Deliver","body":"Run three waves"},{"label":"Sustain","body":"Hand over the plan"}]}`)},
	},
}

// stringEnums lists a schema object's string properties that carry an enum.
func stringEnums(schema *patterns.Schema) map[string][]string {
	out := map[string][]string{}
	if schema == nil {
		return out
	}
	for name, prop := range schema.Properties() {
		if values := prop.EnumValues(); len(values) > 0 {
			out[name] = values
		}
	}
	return out
}

// inkAuditVariants expands one pattern every way that changes what it paints
// text on: its exemplar, each value of every enum override (style,
// cell_accent_mode, label_style, emphasis, ...), each value of every
// top-level values enum (values.style, loop_style) and the extra value sets.
func inkAuditVariants(t *testing.T, pat patterns.Pattern) []inkAuditVariant {
	t.Helper()
	ex, ok := pat.(patterns.Exemplar)
	if !ok {
		return nil
	}
	exemplar, err := json.Marshal(ex.ExemplarValues())
	if err != nil {
		t.Fatalf("%s: %v", pat.Name(), err)
	}
	bases := append([]inkAuditVariant{{label: "exemplar", values: exemplar}}, inkAuditExtraValues[pat.Name()]...)
	variants := append([]inkAuditVariant(nil), bases...)
	schema := pat.Schema()
	overrideEnums := stringEnums(schema.Property("overrides"))
	for _, base := range bases {
		for _, key := range inkAuditSortedKeys(overrideEnums) {
			for _, value := range overrideEnums[key] {
				if _, pinned := base.overrides[key]; pinned {
					continue
				}
				overrides := map[string]string{key: value}
				for k, v := range base.overrides {
					overrides[k] = v
				}
				variants = append(variants, inkAuditVariant{label: base.label + ", overrides." + key + "=" + value, values: base.values, overrides: overrides})
			}
		}
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(exemplar, &object) == nil {
		valueEnums := stringEnums(schema.Property("values"))
		for _, key := range inkAuditSortedKeys(valueEnums) {
			for _, value := range valueEnums[key] {
				patched := make(map[string]json.RawMessage, len(object)+1)
				for k, v := range object {
					patched[k] = v
				}
				patched[key], _ = json.Marshal(value)
				values, _ := json.Marshal(patched)
				variants = append(variants, inkAuditVariant{label: "exemplar, values." + key + "=" + value, values: values})
			}
		}
	}
	return variants
}

func inkAuditSortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// inkAuditColor resolves a scheme name or hex colour against the theme.
func inkAuditColor(theme types.ThemeInfo, name string) (svggen.Color, bool) {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "#") {
		c, err := svggen.ParseColor(name)
		return c, err == nil
	}
	switch name {
	case "tx1":
		name = "dk1"
	case "tx2":
		name = "dk2"
	case "bg1":
		name = "lt1"
	case "bg2":
		name = "lt2"
	}
	for _, c := range theme.Colors {
		if c.Name == name {
			parsed, err := svggen.ParseColor(c.RGB)
			return parsed, err == nil
		}
	}
	return svggen.Color{}, false
}

// inkAuditFill returns the opaque colour a shape's fill shows, or false for
// no fill and for a fill too translucent to be what the text sits on.
func inkAuditFill(theme types.ThemeInfo, raw json.RawMessage) (svggen.Color, bool) {
	if len(raw) == 0 {
		return svggen.Color{}, false
	}
	var fill struct {
		Color  string   `json:"color"`
		Alpha  *float64 `json:"alpha"`
		LumMod int      `json:"lumMod"`
		LumOff int      `json:"lumOff"`
		Tint   int      `json:"tint"`
		Shade  int      `json:"shade"`
	}
	if err := json.Unmarshal(raw, &fill); err != nil {
		if err := json.Unmarshal(raw, &fill.Color); err != nil {
			return svggen.Color{}, false
		}
	}
	if fill.Color == "" || strings.EqualFold(fill.Color, "none") || (fill.Alpha != nil && *fill.Alpha < 50) {
		return svggen.Color{}, false
	}
	base, ok := inkAuditColor(theme, fill.Color)
	canvas, cok := inkAuditColor(theme, "lt1")
	if !ok || !cok {
		return svggen.Color{}, false
	}
	mods := patterns.ColorMods{LumMod: fill.LumMod, LumOff: fill.LumOff, Tint: fill.Tint, Shade: fill.Shade}
	if fill.Alpha != nil && *fill.Alpha < 100 {
		mods.Alpha, mods.HasAlpha = *fill.Alpha/100, true
	}
	return patterns.EffectiveColorMods(base, mods, canvas), true
}

// inkAuditThemeInk reports whether a text colour is one of the neutral theme
// inks patterns choose by measurement (readableTextOn and its callers).
func inkAuditThemeInk(c string) bool {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "lt1", "bg1", "dk1", "tx1", "dk2", "tx2":
		return true
	}
	return false
}

// inkAuditText walks a shape's text object and reports each theme-ink colour
// that misses the WCAG AA bar of its own size on fill. Size and weight are
// inherited from the enclosing node, as the writer inherits them.
func inkAuditText(theme types.ThemeInfo, node any, size float64, bold bool, fill svggen.Color, report func(string)) {
	switch v := node.(type) {
	case map[string]any:
		if s, ok := v["size"].(float64); ok && s > 0 {
			size = s
		}
		if b, ok := v["bold"].(bool); ok {
			bold = b
		}
		if c, ok := v["color"].(string); ok && inkAuditThemeInk(c) {
			content, _ := v["content"].(string)
			if ink, resolved := inkAuditColor(theme, c); resolved && (content != "" || v["paragraphs"] == nil) {
				if ratio, bar := ink.ContrastWith(fill), patterns.TextContrastThreshold(size, bold); ratio < bar {
					report(fmt.Sprintf("%s text %q (%.0fpt) on %s: contrast %.2f < %.1f", c, content, size, fill.Hex(), ratio, bar))
				}
			}
		}
		for key, child := range v {
			if key == "paragraphs" || key == "runs" {
				inkAuditText(theme, child, size, bold, fill, report)
			}
		}
	case []any:
		for _, child := range v {
			inkAuditText(theme, child, size, bold, fill, report)
		}
	}
}

// inkAuditGrid checks every filled, text-bearing shape of an expanded grid:
// cell shapes, layer shapes and nested grids.
func inkAuditGrid(theme types.ThemeInfo, grid *jsonschema.ShapeGridInput, report func(string)) (checked int) {
	if grid == nil {
		return 0
	}
	shape := func(s *jsonschema.ShapeSpecInput) {
		if s == nil || len(s.Text) == 0 {
			return
		}
		fill, ok := inkAuditFill(theme, s.Fill)
		if !ok {
			return
		}
		var text any
		if json.Unmarshal(s.Text, &text) != nil {
			return
		}
		checked++
		inkAuditText(theme, text, 0, false, fill, report)
	}
	for ri := range grid.Rows {
		for _, cell := range grid.Rows[ri].Cells {
			if cell == nil {
				continue
			}
			shape(cell.Shape)
			for _, layer := range cell.Layers {
				shape(layer.Shape)
			}
			checked += inkAuditGrid(theme, cell.Grid, report)
		}
	}
	return checked
}

// softDarkOrangeContext is ctx (a shipped template's geometry and fonts)
// with its theme colours replaced by softDarkOrangePalette.
func softDarkOrangeContext(ctx patterns.ExpandContext) patterns.ExpandContext {
	colors := make([]types.ThemeColor, len(ctx.Theme.Colors))
	copy(colors, ctx.Theme.Colors)
	for i := range colors {
		if rgb, ok := softDarkOrangePalette[colors[i].Name]; ok {
			colors[i].RGB = rgb
		}
	}
	ctx.Theme.Colors = colors
	return ctx
}

// TestPatternInkContrastOnLocalTemplateCorpus expands every pattern, every
// way that changes its fills, through the template-aware path generation
// uses, and asserts each theme-ink text colour it paints on a filled shape
// clears the WCAG AA bar for its size. readableTextOn answers the best theme
// ink even when none reads, so a chevron, arrow or accent block on a template
// with soft darks used to carry 3.77:1 body text (go-slide-creator-pr5bx).
//
// It runs on every local template (the gitignored p-style included when
// present) and on a committed stand-in for the template that exposed the
// defect: midnight-blue's geometry with charcoal darks and an orange primary.
func TestPatternInkContrastOnLocalTemplateCorpus(t *testing.T) {
	type themed struct {
		name string
		ctx  patterns.ExpandContext
	}
	var themes []themed
	for _, path := range testutil.TestTemplatePaths() {
		name := strings.TrimSuffix(filepath.Base(path), ".pptx")
		ctx, _, err := resolveExpandContext(name, testutil.TemplatesDir())
		if err != nil {
			t.Fatal(err)
		}
		// Under -short (the sharded race step) the audit keeps the one
		// template whose geometry the stand-in borrows, and the stand-in;
		// the corpus job runs every template (go-slide-creator-efhg2).
		if testing.Short() && name != "midnight-blue" {
			continue
		}
		themes = append(themes, themed{name, ctx})
		if name == "midnight-blue" {
			themes = append(themes, themed{"soft-darks-orange", softDarkOrangeContext(ctx)})
		}
	}
	for _, theme := range themes {
		t.Run(theme.name, func(t *testing.T) {
			checked := 0
			for _, pat := range patterns.Default().List() {
				for _, variant := range inkAuditVariants(t, pat) {
					grid, _, err := expandPattern(&PatternInput{Name: pat.Name(), Values: variant.values, Overrides: variant.overridesJSON()}, theme.ctx, patterns.Default())
					if err != nil && variant.strict {
						t.Fatalf("%s [%s]: %v", pat.Name(), variant.label, err)
					}
					if err != nil {
						// An enum value the exemplar's content does not allow
						// (a style needing more steps) is not this test's.
						continue
					}
					checked += inkAuditGrid(theme.ctx.Theme, grid, func(msg string) {
						t.Errorf("%s [%s]: %s", pat.Name(), variant.label, msg)
					})
				}
			}
			if checked < 500 {
				t.Errorf("audited only %d filled text shapes; the walk has lost its coverage", checked)
			}
		})
	}
}
