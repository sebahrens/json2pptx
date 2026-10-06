package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// dual-org-ladder pattern — two parallel org columns (joint-venture team slides)
// ---------------------------------------------------------------------------
//
// Two columns of paired roles under an org-name heading. Unlike a
// hierarchical org chart, the rows carry no parent/child semantics — they line
// up matching roles between two organisations (e.g. client sponsor paired with
// consulting partner).
//
// The default look (go-slide-creator-kyvk5) owns each column with a bold
// heading on one accent rule and draws every pair as ONE pale band across both
// columns, so the pairing is read from the band; a short double-headed block
// arrow in the centre gutter says "counterparts". Names and roles are
// left-aligned to one edge per column. The highlighted pair is the only
// accent: its band takes the accent tint and its arrow the solid accent.
// overrides.style "lines" keeps the earlier open look (two header tiles,
// centred entries, a hairline pairing line) and "tiles" the role cards.

func init() {
	Default().Register(&dualOrgLadder{})
}

type dualOrgLadder struct{}

func (d *dualOrgLadder) Name() string { return "dual-org-ladder" }
func (d *dualOrgLadder) Description() string {
	return "Two parallel org columns: an org heading on a rule over 2–4 paired roles, each pair one band with a centre arrow (engagement-team / joint-venture slides)"
}
func (d *dualOrgLadder) UseWhen() string {
	return "Engagement-team, joint-venture, or paired-organisation slide showing 2–4 matched roles across two orgs side by side; prefer team-bios for a single org's team page, comparison-2col when the columns are pros/cons rather than role pairs, and an svggen org_chart diagram when the structure is hierarchical (parent/child)"
}
func (d *dualOrgLadder) NotWhen() string {
	return "Single-org team page (use team-bios), hierarchical org chart with reporting lines (use svggen org_chart), pros-and-cons or option comparison (use comparison-2col), or more than 4 paired rows (split across slides)"
}
func (d *dualOrgLadder) Version() int      { return 1 }
func (d *dualOrgLadder) CellsHint() string { return "2-4 rows" }
func (d *dualOrgLadder) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"team-bios", "swimlane", "scqa-summary"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}

func (d *dualOrgLadder) ExemplarValues() any {
	return &DualOrgLadderValues{
		OrgA: "Client Organisation",
		OrgB: "Consulting Firm",
		Rows: []DualOrgLadderRow{
			{ANameField: "Bob Jones", ATitle: "Executive Sponsor", BNameField: "Betty Smith", BTitle: "Engagement Partner"},
			{ANameField: "Alex Chen", ATitle: "Steering Committee", BNameField: "Maria Lopez", BTitle: "Account Director"},
			{ANameField: "Sara Patel", ATitle: "Programme Lead", BNameField: "Tom Becker", BTitle: "Delivery Lead"},
			{ANameField: "Jordan Park", ATitle: "Workstream Owner", BNameField: "Lila Romero", BTitle: "Senior Consultant"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// DualOrgLadderRow is a single horizontal pair: one role in org A, one in org B.
type DualOrgLadderRow struct {
	ANameField string `json:"a_name"`
	ATitle     string `json:"a_title"`
	BNameField string `json:"b_name"`
	BTitle     string `json:"b_title"`
	// Highlight marks this pair (at most one): its band takes the accent tint
	// and its arrow the solid accent.
	Highlight bool `json:"highlight,omitempty"`
}

// DualOrgLadderValues holds the two org names and the paired role rows.
//
// ShowConnectors is *bool so the JSON-omitted state is distinguishable from an
// explicit false; nil means "use the default (true)".
type DualOrgLadderValues struct {
	OrgA           string             `json:"org_a"`
	OrgB           string             `json:"org_b"`
	Rows           []DualOrgLadderRow `json:"rows"`
	ShowConnectors *bool              `json:"show_connectors,omitempty"`
}

// DualOrgLadderOverrides controls accent colors and per-zone font sizes.
//
// AccentA is the pattern's accent: the rule under org A's heading and the
// highlighted pair (in the lines / tiles styles the left header fill and the
// pairing line). AccentB colours org B's rule (right header fill); it defaults
// to AccentA (a darker tone of it for the header tiles).
type DualOrgLadderOverrides struct {
	AccentA   string  `json:"accent_a,omitempty"`
	AccentB   string  `json:"accent_b,omitempty"`
	OrgSize   float64 `json:"org_size,omitempty"`
	NameSize  float64 `json:"name_size,omitempty"`
	TitleSize float64 `json:"title_size,omitempty"`
	// Style is "open" (default: headings on a rule, one pale band per pair
	// with a centre arrow), "lines" (two header tiles over centred entries
	// joined by a hairline, the default before go-slide-creator-kyvk5) or
	// "tiles" (every role a filled card with a stub connector, the look
	// before go-slide-creator-rpz53).
	Style string `json:"style,omitempty"`
}

// dualOrgStyles are the accepted overrides.style values.
var dualOrgStyles = []string{"open", "lines", "tiles"}

// dualOrgStyle is the style the ladder renders in ("open" when unset).
func dualOrgStyle(ovr *DualOrgLadderOverrides) string {
	if ovr == nil || ovr.Style == "" {
		return "open"
	}
	return ovr.Style
}

// Band ladder geometry (the default style).
const (
	// dualOrgBandGutterPt is the centre gutter the pairing arrow stands in.
	dualOrgBandGutterPt = 56.0
	// dualOrgRulePt is the thickness of the rule under each org heading.
	dualOrgRulePt = 2.0
	// dualOrgBandMaxPt caps a pair's band at four pairs, and
	// dualOrgBandMaxStepPt is what each pair fewer adds, so two or three
	// pairs keep a row pitch instead of stretching into slabs.
	dualOrgBandMaxPt     = 78.0
	dualOrgBandMaxStepPt = 10.0
	// dualOrgBandPadPt is the air a band keeps above and below its text
	// beyond the written fit, when the content area has it to give.
	dualOrgBandPadPt = 10.0
	// dualOrgHeadingFitFrac is the share of its estimated column a heading
	// is measured in.
	dualOrgHeadingFitFrac = 0.85
	// dualOrgHighlightBarPt is the accent bar on the highlighted band's
	// left edge.
	dualOrgHighlightBarPt = 4.0
	// dualOrgHeadingGapPt is the heading's distance from its rule.
	dualOrgHeadingGapPt = 3.0
	// The pairing arrow's frame inside its gutter cell (fractions).
	dualOrgArrowW = 0.64
	dualOrgArrowH = 0.26
)

// Four-pair budgets of the band ladder: a title over dualOrgBandTitleChars
// beside a name over dualOrgBandNameChars no longer fits at the written size.
const (
	dualOrgBandNameChars  = 51
	dualOrgBandTitleChars = 60
)

// dualOrgBandPadStepsPt are the top / bottom text margins a band tries, widest
// first: the uniform shape margin, then two tighter ones for wrapped text.
var dualOrgBandPadStepsPt = []float64{sizingInsetTBPt, 9, 5}

// Lines ladder geometry.
const (
	// dualOrgLinkPt is the width of the gutter between the two org columns
	// in the lines style, and so the length of the pairing line.
	dualOrgLinkPt = 40.0
	// dualOrgLinkLinePt is the pairing line's thickness.
	dualOrgLinkLinePt = 1.0
	// dualOrgOpenColGapPt is the lines / band grids' column gap: the pairing
	// line reaches both role entries and a band shows no seam.
	dualOrgOpenColGapPt = 0.01
)

// DualOrgLadderCellOverride is the shared per-cell override, indexed by row
// (the header row is index 0; body rows are indices 1..N).
type DualOrgLadderCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	dualOrgLadderMinRows       = 2
	dualOrgLadderMaxRows       = 4
	dualOrgLadderOrgMaxChars   = 60
	dualOrgLadderNameMaxChars  = 60
	dualOrgLadderTitleMaxChars = 80
)

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (d *dualOrgLadder) NewValues() any       { return &DualOrgLadderValues{} }
func (d *dualOrgLadder) NewOverrides() any    { return &DualOrgLadderOverrides{} }
func (d *dualOrgLadder) NewCellOverride() any { return &DualOrgLadderCellOverride{} }

func (d *dualOrgLadder) PostExpandWarnings(_ ExpandContext, values, overrides any) []string {
	v, ok := values.(*DualOrgLadderValues)
	if !ok || v == nil || len(v.Rows) < dualOrgLadderMaxRows {
		return nil
	}
	ovr, _ := overrides.(*DualOrgLadderOverrides)
	var warnings []string
	if dualOrgStyle(ovr) == "open" {
		// Measured on every shipped template (go-slide-creator-kyvk5): four
		// bands hold any one name or title at its schema maximum; only a
		// name over 51 characters beside a title over 60 shrinks.
		for i, row := range v.Rows {
			for _, field := range []struct{ name, text, person string }{{"a_title", row.ATitle, row.ANameField}, {"b_title", row.BTitle, row.BNameField}} {
				if runeLen(field.person) > dualOrgBandNameChars && runeLen(field.text) > dualOrgBandTitleChars {
					warnings = append(warnings, fmt.Sprintf("%s: dual-org-ladder rows[%d].%s has %d characters beside a %d-character name; four pairs hold about %d title characters beside a name over %d — shorten the name or title, or split the team across slides", ErrCodeBodyTooLong, i, field.name, runeLen(field.text), runeLen(field.person), dualOrgBandTitleChars, dualOrgBandNameChars))
				}
			}
		}
		return warnings
	}
	for i, row := range v.Rows {
		// Measured against the written size on every shipped template
		// (go-slide-creator-n1muf): about 62 title characters beside a short
		// name, or 47 each when name and title are both long (re-measured for the open
		// entries, whose pairing gutter is wider than the tiles' gap).
		for _, field := range []struct{ name, text, person string }{{"a_title", row.ATitle, row.ANameField}, {"b_title", row.BTitle, row.BNameField}} {
			if runeLen(field.text) > 62 {
				warnings = append(warnings, fmt.Sprintf("%s: dual-org-ladder rows[%d].%s has %d characters; four role rows hold about 62 title characters per card — shorten the title or split the team across slides", ErrCodeBodyTooLong, i, field.name, runeLen(field.text)))
			} else if runeLen(field.person) > 47 && runeLen(field.text) > 47 {
				warnings = append(warnings, fmt.Sprintf("%s: dual-org-ladder rows[%d].%s has %d characters beside a %d-character name; four role rows hold about 47 characters each when both are long — shorten the name or title, or split the team across slides", ErrCodeBodyTooLong, i, field.name, runeLen(field.text), runeLen(field.person)))
			}
		}
	}
	return warnings
}

func (d *dualOrgLadder) Schema() *Schema {
	rowSchema := ObjectSchema(
		map[string]*Schema{
			"a_name":    StringSchema(dualOrgLadderNameMaxChars).WithDescription("Name of the org A member on this row (rendered bold)"),
			"a_title":   StringSchema(dualOrgLadderTitleMaxChars).WithDescription("Role / title of the org A member; with four pairs keep it to about 60 characters when the name is over 51 (lines / tiles styles: about 62, or 47 when the name is also long)"),
			"b_name":    StringSchema(dualOrgLadderNameMaxChars).WithDescription("Name of the org B member on this row (rendered bold)"),
			"b_title":   StringSchema(dualOrgLadderTitleMaxChars).WithDescription("Role / title of the org B member; with four pairs keep it to about 60 characters when the name is over 51 (lines / tiles styles: about 62, or 47 when the name is also long)"),
			"highlight": BooleanSchema().WithDescription("Mark this pair (at most one): its band takes the accent tint and its arrow the solid accent"),
		},
		[]string{"a_name", "a_title", "b_name", "b_title"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"org_a":           StringSchema(dualOrgLadderOrgMaxChars).WithDescription("Name of organisation A (the left column's heading)"),
			"org_b":           StringSchema(dualOrgLadderOrgMaxChars).WithDescription("Name of organisation B (the right column's heading)"),
			"rows":            ArraySchema(rowSchema, dualOrgLadderMinRows, dualOrgLadderMaxRows).WithDescription("2–4 paired role rows; each row aligns one org A member with one org B member"),
			"show_connectors": BooleanSchema().WithDescription("When true (default), draw the pairing arrow (lines / tiles styles: the pairing line) between the two roles on every body row").WithDefault(true),
		},
		[]string{"org_a", "org_b", "rows"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent_a":   StringSchema(0).WithDescription("Scheme color for org A's heading rule and the highlighted pair; in the lines / tiles styles the left header fill and the pairing line (default accent1)").WithDefault("accent1"),
			"accent_b":   StringSchema(0).WithDescription("Scheme color for org B's heading rule (lines / tiles: the right header fill); defaults to accent_a (lines / tiles: a darker tone of it) so the two orgs read as peers rather than two brands").WithDefault(""),
			"org_size":   NumberSchema(6, 40).WithDescription("Font size for the org headings in points (default 16; lines / tiles 14)"),
			"name_size":  NumberSchema(6, 40).WithDescription("Font size for member names in points (default 14; lines / tiles 12)"),
			"title_size": NumberSchema(6, 40).WithDescription("Font size for member titles in points (default 12; lines / tiles 10)"),
			"style":      EnumSchema(dualOrgStyles...).WithDescription("open (default: org headings on a rule, each pair one pale band with a centre arrow, names left-aligned), lines (two header tiles over centred entries joined by a hairline) or tiles (every role a filled card with a stub connector)").WithDefault("open"),
		},
		nil,
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      overridesSchema,
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Two parallel org columns: an org heading on a rule above 2–4 paired roles, each pair one band with a centre arrow. Use for engagement-team / joint-venture slides where matched roles align horizontally without reporting hierarchy.")
}

func (d *dualOrgLadder) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*DualOrgLadderValues)
	if !ok || v == nil {
		return fmt.Errorf("dual-org-ladder: values must be *DualOrgLadderValues, got %T", values)
	}

	const name = "dual-org-ladder"
	var errs []error

	if strings.TrimSpace(v.OrgA) == "" {
		errs = append(errs, errRequired(name, "org_a"))
	} else if runeLen(v.OrgA) > dualOrgLadderOrgMaxChars {
		errs = append(errs, errMaxLength(name, "org_a", dualOrgLadderOrgMaxChars, runeLen(v.OrgA)))
	}
	if strings.TrimSpace(v.OrgB) == "" {
		errs = append(errs, errRequired(name, "org_b"))
	} else if runeLen(v.OrgB) > dualOrgLadderOrgMaxChars {
		errs = append(errs, errMaxLength(name, "org_b", dualOrgLadderOrgMaxChars, runeLen(v.OrgB)))
	}

	if len(v.Rows) < dualOrgLadderMinRows {
		errs = append(errs, errMinItems(name, "rows", dualOrgLadderMinRows, len(v.Rows), "(hint: use team-bios for a single-org team page, comparison-2col for non-role pairs)"))
	}
	if len(v.Rows) > dualOrgLadderMaxRows {
		errs = append(errs, errMaxItems(name, "rows", dualOrgLadderMaxRows, len(v.Rows), "(hint: split the team across two slides — each slide supports up to 4 paired rows)"))
	}

	if ovr, ok := overrides.(*DualOrgLadderOverrides); ok && ovr != nil && ovr.Style != "" && !slices.Contains(dualOrgStyles, ovr.Style) {
		errs = append(errs, errInvalidEnum(name, "overrides.style", ovr.Style, dualOrgStyles))
	}
	errs = append(errs, singleHighlightErrors(name, "pair", "pairs", len(v.Rows),
		func(i int) bool { return v.Rows[i].Highlight },
		func(i int) string { return fmt.Sprintf("rows[%d].highlight", i) })...)

	for i, row := range v.Rows {
		if strings.TrimSpace(row.ANameField) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].a_name", i)))
		} else if runeLen(row.ANameField) > dualOrgLadderNameMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].a_name", i), dualOrgLadderNameMaxChars, runeLen(row.ANameField)))
		}
		if strings.TrimSpace(row.ATitle) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].a_title", i)))
		} else if runeLen(row.ATitle) > dualOrgLadderTitleMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].a_title", i), dualOrgLadderTitleMaxChars, runeLen(row.ATitle)))
		}
		if strings.TrimSpace(row.BNameField) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].b_name", i)))
		} else if runeLen(row.BNameField) > dualOrgLadderNameMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].b_name", i), dualOrgLadderNameMaxChars, runeLen(row.BNameField)))
		}
		if strings.TrimSpace(row.BTitle) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].b_title", i)))
		} else if runeLen(row.BTitle) > dualOrgLadderTitleMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].b_title", i), dualOrgLadderTitleMaxChars, runeLen(row.BTitle)))
		}
	}

	// Cell overrides are keyed by emitted grid row (0 = header, 1..N = body rows).
	totalGridRows := len(v.Rows) + 1
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalGridRows, "(keys: 0 = header row, 1..N = body rows)"); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (d *dualOrgLadder) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*DualOrgLadderValues)
	if !ok {
		return nil, fmt.Errorf("dual-org-ladder: values must be *DualOrgLadderValues, got %T", values)
	}
	ovr := &DualOrgLadderOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*DualOrgLadderOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("dual-org-ladder: overrides must be *DualOrgLadderOverrides, got %T", overrides)
		}
	}

	accentA := ovr.AccentA
	if accentA == "" {
		accentA = ctx.DefaultAccent()
	}
	// accent_b defaults to a darker tone of accent_a, not accent2: the two orgs
	// are peers and a second brand hue says otherwise (go-slide-creator-at7ij).
	accentB := ovr.AccentB
	usePeerToneForB := accentB == ""
	showConnectors := true
	if v.ShowConnectors != nil {
		showConnectors = *v.ShowConnectors
	}
	if dualOrgStyle(ovr) == "open" {
		return expandDualOrgBands(ctx, v, ovr, cellOverrides, accentA, showConnectors), nil
	}

	orgSize := ResolveSize(ovr.OrgSize, scaleSubheadPt)
	nameSize := ResolveSize(ovr.NameSize, scaleBodyPt)
	titleSize := ResolveSize(ovr.TitleSize, scaleCaptionPt)

	open := dualOrgStyle(ovr) == "lines"
	headerRow := jsonschema.GridRowInput{
		Height: 18, // header is shorter than body rows
		Cells: []*jsonschema.GridCellInput{
			buildDualOrgHeaderCell(v.OrgA, accentFillJSON(accentA), orgSize),
			buildDualOrgHeaderCell(v.OrgB, headerBFill(accentA, accentB, usePeerToneForB), orgSize),
		},
	}
	if co, ok := cellOverrides[0]; ok {
		if cellOvr, ok2 := co.(*DualOrgLadderCellOverride); ok2 {
			// Index 0 is the header row: the text keys restyle both org
			// headers; the accent bar marks the left one.
			applyCellTextOverride(headerRow.Cells[0], cellOvr)
			applyCellTextOverride(headerRow.Cells[1], cellOvr)
			if cellOvr.AccentBar {
				headerRow.Cells[0].AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accentA, Width: 3}
			}
		}
	}

	// Each body row holds its role cards at their written size: with many rows
	// on a short content area the fixed 18% header squeezed them until the
	// cards were stored at 96% autofit, below the floor. A body-row minimum
	// (by the writer's own measure) makes the header give way instead
	// (go-slide-creator-n1muf).
	contentW, _ := contentAreaPt(ctx)
	cardW := (contentW - ctx.Gap(dualOrgColGapPt)) / 2
	if open {
		cardW = (contentW - dualOrgLinkPt) / 2
	}
	bodyMinPt := 0.0
	for _, row := range v.Rows {
		for _, c := range []*jsonschema.GridCellInput{
			buildDualOrgRoleCell(row.ANameField, row.ATitle, nameSize, titleSize),
			buildDualOrgRoleCell(row.BNameField, row.BTitle, nameSize, titleSize),
		} {
			bodyMinPt = math.Max(bodyMinPt, writtenFitHeightPt(ctx.themeFonts(), c.Shape.Text, cardW, 0))
		}
	}

	bodyRows := make([]jsonschema.GridRowInput, len(v.Rows))
	for i, row := range v.Rows {
		cells := []*jsonschema.GridCellInput{
			buildDualOrgRoleCell(row.ANameField, row.ATitle, nameSize, titleSize),
			buildDualOrgRoleCell(row.BNameField, row.BTitle, nameSize, titleSize),
		}
		gridRow := jsonschema.GridRowInput{Cells: cells, MinHeight: bodyMinPt}
		if open {
			gridRow.Cells = dualOrgOpenPair(ctx, cells, row.Highlight, showConnectors, accentA)
		}
		if showConnectors && !open {
			gridRow.Connector = &jsonschema.ConnectorSpecInput{
				Style: "line",
				Color: accentA,
				Width: 1,
			}
		}
		// Cell override key i+1 (header is index 0).
		if co, ok := cellOverrides[i+1]; ok {
			if cellOvr, ok2 := co.(*DualOrgLadderCellOverride); ok2 {
				applyCellTextOverride(cells[0], cellOvr)
				applyCellTextOverride(cells[1], cellOvr)
				if cellOvr.AccentBar {
					cells[0].AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accentA, Width: 3}
					cells[1].AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: barBColor(accentA, accentB, usePeerToneForB), Width: 3}
				}
			}
		}
		bodyRows[i] = gridRow
	}

	rows := make([]jsonschema.GridRowInput, 0, len(bodyRows)+1)
	rows = append(rows, headerRow)
	rows = append(rows, bodyRows...)

	colsJSON, _ := json.Marshal(2)
	grid := &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  ctx.Gap(dualOrgColGapPt), // visible gap between the two columns
		RowGap:  ctx.Gap(6),
		Rows:    rows,
	}
	if open {
		dualOrgOpenColumns(ctx, grid)
	}
	return grid, nil
}

// ---------------------------------------------------------------------------
// Band ladder (default style)
// ---------------------------------------------------------------------------

// expandDualOrgBands builds the default ladder on [org A | gutter | org B]:
// a heading row, a rule row and one band row per pair.
func expandDualOrgBands(ctx ExpandContext, v *DualOrgLadderValues, ovr *DualOrgLadderOverrides, cellOverrides map[int]any, accentA string, showConnectors bool) *jsonschema.ShapeGridInput {
	accentB := ovr.AccentB
	if accentB == "" {
		accentB = accentA
	}
	nameSize := ResolveSize(ovr.NameSize, scaleSubheadPt)
	titleSize := ResolveSize(ovr.TitleSize, scaleBodyPt)

	areaW, areaH := sizingAreaPt(ctx)
	colW := (areaW - dualOrgBandGutterPt) / 2
	fonts := ctx.themeFonts()

	// Both headings share one size: the lead step when each org name holds
	// one line of its column (with a margin for a narrower real zone), else
	// the subhead step, where a long name wraps.
	orgSize := ovr.OrgSize
	if orgSize <= 0 {
		orgSize = scaleLeadPt
		lineW := (colW - 2*sizingInsetLRPt) * dualOrgHeadingFitFrac
		for _, org := range []string{v.OrgA, v.OrgB} {
			if measuredLines(org, fonts.Minor, true, scaleLeadPt, lineW) > 1 {
				orgSize = scaleSubheadPt
			}
		}
	}

	headingA, headingB := buildDualOrgHeadingCell(v.OrgA, orgSize), buildDualOrgHeadingCell(v.OrgB, orgSize)
	if cellOvr, ok := cellOverrides[0].(*DualOrgLadderCellOverride); ok {
		applyCellTextOverride(headingA, cellOvr)
		applyCellTextOverride(headingB, cellOvr)
		if cellOvr.AccentBar {
			headingA.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accentA, Width: 3}
		}
	}
	// Measured in a narrower column than the estimate, so a heading that
	// wraps in the real zone already has its second line.
	headingPt := math.Max(
		writtenFitHeightPt(fonts, headingA.Shape.Text, colW*dualOrgHeadingFitFrac, 0),
		writtenFitHeightPt(fonts, headingB.Shape.Text, colW*dualOrgHeadingFitFrac, 0))
	rule := func(accent string) *jsonschema.GridCellInput {
		return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: accentFillJSON(accent), Line: noLine}}
	}

	band, _ := parseFillTone(surfaceFillJSON(ctx, "subtle", NeutralTint4))
	marker := structuralDarkTone(ctx)

	// A band stands at a comfortable pitch: its written fit plus some air,
	// as far as the content area allows, and never a slab. Long names and
	// roles on four pairs give up the band's top and bottom margin before
	// any text shrinks.
	rowGap := ctx.Gap(6)
	n := float64(len(v.Rows))
	bandsPt := areaH - headingPt - dualOrgRulePt - (n+1)*rowGap
	var pairs [][2]*jsonschema.GridCellInput
	fitPt := 0.0
	for _, padPt := range dualOrgBandPadStepsPt {
		pairs, fitPt = make([][2]*jsonschema.GridCellInput, len(v.Rows)), 0
		for i, row := range v.Rows {
			cellOvr, _ := cellOverrides[i+1].(*DualOrgLadderCellOverride)
			pairs[i] = dualOrgBandPair(ctx, row, band, [2]string{accentA, accentB}, [2]float64{nameSize, titleSize}, padPt, colW, cellOvr)
			for _, c := range pairs[i] {
				fitPt = math.Max(fitPt, writtenFitHeightPt(fonts, c.Shape.Text, colW, 0))
			}
		}
		if n*fitPt <= bandsPt {
			break
		}
	}
	bandMin := fitPt + math.Floor(clampPt(bandsPt/n-fitPt, 0, 2*dualOrgBandPadPt))
	bandMax := math.Max(bandMin, dualOrgBandMaxPt+float64(dualOrgLadderMaxRows-len(v.Rows))*dualOrgBandMaxStepPt)

	rows := make([]jsonschema.GridRowInput, 0, len(v.Rows)+2)
	rows = append(rows,
		jsonschema.GridRowInput{MinHeight: headingPt, MaxHeight: headingPt, Cells: []*jsonschema.GridCellInput{headingA, {}, headingB}},
		jsonschema.GridRowInput{MinHeight: dualOrgRulePt, MaxHeight: dualOrgRulePt, Cells: []*jsonschema.GridCellInput{rule(accentA), {}, rule(accentB)}})
	for i, row := range v.Rows {
		tone := band
		arrow := marker
		if row.Highlight {
			tone = inactiveTintTone(accentA)
			arrow = fillTone{Color: accentA}
		}
		gutter := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: tone.fillJSON(), Line: metricListBandLine(tone)}}
		if showConnectors {
			gutter.Layers = []jsonschema.LayerInput{{
				Name:  "pairing",
				Frame: jsonschema.LayerFrameInput{X: (1 - dualOrgArrowW) / 2, Y: (1 - dualOrgArrowH) / 2, W: dualOrgArrowW, H: dualOrgArrowH},
				Shape: &jsonschema.ShapeSpecInput{Geometry: "leftRightArrow", Fill: arrow.fillJSON(), Line: noLine},
			}}
		}
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: bandMin, MaxHeight: bandMax,
			Cells: []*jsonschema.GridCellInput{pairs[i][0], gutter, pairs[i][1]},
		})
	}

	gutterPct := dualOrgBandGutterPt / areaW * 100
	cols, _ := json.Marshal([]float64{(100 - gutterPct) / 2, gutterPct, (100 - gutterPct) / 2})
	return &jsonschema.ShapeGridInput{Columns: cols, ColGap: dualOrgOpenColGapPt, RowGap: rowGap, Rows: rows}
}

// dualOrgBandPair is the two role cells of one pair on its band: the neutral
// band, or the accent tint with an accent edge for the highlighted pair.
// accents are [accent_a, accent_b], sizes [name, title]; padPt is the text's
// top / bottom margin.
func dualOrgBandPair(ctx ExpandContext, row DualOrgLadderRow, band fillTone, accents [2]string, sizes [2]float64, padPt, colW float64, cellOvr *DualOrgLadderCellOverride) [2]*jsonschema.GridCellInput {
	tone := band
	if row.Highlight {
		tone = inactiveTintTone(accents[0])
	}
	nameInk := readableTextOn(ctx, tone, "dk1")
	titleInk := nameInk
	if nameInk != "lt1" {
		titleInk = readableInkOn(ctx, tone, "dk2", 4.5)
	}
	pair := [2]*jsonschema.GridCellInput{
		buildDualOrgBandCell(row.ANameField, row.ATitle, sizes[0], sizes[1], nameInk, titleInk, tone, padPt),
		buildDualOrgBandCell(row.BNameField, row.BTitle, sizes[0], sizes[1], nameInk, titleInk, tone, padPt),
	}
	if cellOvr != nil {
		for i, c := range pair {
			applyCellTextOverride(c, cellOvr)
			if cellOvr.AccentBar {
				c.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accents[i], Width: 3}
			}
		}
	}
	if row.Highlight {
		// The tint of a muted accent sits close to the neutral band, so the
		// highlighted pair is also marked on its edge.
		pair[0].Layers = []jsonschema.LayerInput{{
			Name:  "highlight-edge",
			Frame: jsonschema.LayerFrameInput{W: dualOrgHighlightBarPt / colW, H: 1},
			Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: accentFillJSON(accents[0]), Line: noLine},
		}}
	}
	return pair
}

// buildDualOrgHeadingCell is one org heading: bold text standing on the rule
// under it, on the same left edge as the names of its column.
func buildDualOrgHeadingCell(orgName string, size float64) *jsonschema.GridCellInput {
	text, _ := json.Marshal(dualOrgTextObj{
		Paragraphs:    []dualOrgParagraph{{Content: orgName, Size: size, Bold: true, Color: "dk1", Align: "l"}},
		Align:         "l",
		VerticalAlign: "b",
	})
	// An explicit left inset keeps both headings on their names' edge: the
	// first column's would otherwise move out to the slide title's text line.
	text = withTextInsetSides(withInsetLeft(text, sizingInsetLRPt), dualOrgHeadingGapPt, "inset_bottom")
	text = withTextInsetSides(text, 0, "inset_top")
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: noLine, Text: text}}
}

// buildDualOrgBandCell is one role in a pair's band: bold name over the role,
// left-aligned, on the band's fill (outlined in its own colour so the band
// shows no seams).
func buildDualOrgBandCell(memberName, title string, nameSize, titleSize float64, nameInk, titleInk string, tone fillTone, padPt float64) *jsonschema.GridCellInput {
	text, _ := json.Marshal(dualOrgTextObj{
		Paragraphs: []dualOrgParagraph{
			{Content: memberName, Size: nameSize, Bold: true, Color: nameInk, Align: "l"},
			{Content: title, Size: titleSize, Color: titleInk, Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "ctr",
	})
	if padPt != sizingInsetTBPt {
		text = withTextInsetSides(text, padPt, "inset_top", "inset_bottom")
	}
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: tone.fillJSON(), Line: metricListBandLine(tone), Text: text}}
}

// ---------------------------------------------------------------------------
// Lines ladder (overrides.style "lines")
// ---------------------------------------------------------------------------

// dualOrgOpenPair is one open pair: the two role entries with no card, joined
// by the pairing line in the gutter. A highlighted pair is one tinted band
// instead.
func dualOrgOpenPair(ctx ExpandContext, cells []*jsonschema.GridCellInput, highlight, showConnectors bool, accent string) []*jsonschema.GridCellInput {
	link := &jsonschema.GridCellInput{}
	fill, line := json.RawMessage(`"none"`), noLine
	switch {
	case highlight:
		tone := inactiveTintTone(accent)
		fill = tone.fillJSON()
		// Outlined in its own colour so the band shows no seams.
		line = metricListBandLine(tone)
		ink := readableTextOn(ctx, tone, "dk1")
		for _, c := range cells {
			c.Shape.Text = recolorTextInk(recolorTextInk(c.Shape.Text, "dk1", ink), "dk2", ink)
		}
		link = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: fill, Line: line}}
	case showConnectors:
		link = &jsonschema.GridCellInput{
			MaxHeight: dualOrgLinkLinePt,
			Shape:     &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: accentFillJSON(accent), Line: noLine},
		}
	}
	for _, c := range cells {
		c.Shape.Fill, c.Shape.Line = fill, line
	}
	return []*jsonschema.GridCellInput{cells[0], link, cells[1]}
}

// dualOrgOpenColumns lays the open ladder on [org A | link | org B]: the link
// column is the gutter the pairing line runs through, touching both role
// entries.
func dualOrgOpenColumns(ctx ExpandContext, grid *jsonschema.ShapeGridInput) {
	areaW, _ := sizingAreaPt(ctx)
	linkPct := dualOrgLinkPt / areaW * 100
	grid.Columns, _ = json.Marshal([]float64{(100 - linkPct) / 2, linkPct, (100 - linkPct) / 2})
	grid.ColGap = dualOrgOpenColGapPt
	header := grid.Rows[0].Cells
	grid.Rows[0].Cells = []*jsonschema.GridCellInput{header[0], {}, header[1]}
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

func buildDualOrgHeaderCell(orgName string, fill json.RawMessage, size float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     fill,
			Text:     buildDualOrgHeaderText(orgName, size),
		},
	}
}

// headerBFill is the right org's header fill: a darker tone of accent_a by
// default, or the authored accent_b verbatim when one was given.
func headerBFill(accentA, accentB string, usePeerTone bool) json.RawMessage {
	if usePeerTone {
		return peerFillJSON(accentA)
	}
	return accentFillJSON(accentB)
}

// dualOrgColGapPt is the visible gap between the two org columns.
const dualOrgColGapPt = 24.0

func buildDualOrgRoleCell(memberName, title string, nameSize, titleSize float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			// A neutral card, not an outlined empty box (go-slide-creator-pgdkp).
			Fill: neutralFillJSON(NeutralTint4),
			Line: noLine,
			Text: buildDualOrgRoleText(memberName, title, nameSize, titleSize),
		},
	}
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type dualOrgParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type dualOrgTextObj struct {
	Paragraphs    []dualOrgParagraph `json:"paragraphs"`
	Align         string             `json:"align"`
	VerticalAlign string             `json:"vertical_align"`
}

func buildDualOrgHeaderText(orgName string, size float64) json.RawMessage {
	obj := dualOrgTextObj{
		Paragraphs: []dualOrgParagraph{
			{Content: orgName, Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

func buildDualOrgRoleText(memberName, title string, nameSize, titleSize float64) json.RawMessage {
	obj := dualOrgTextObj{
		Paragraphs: []dualOrgParagraph{
			{Content: memberName, Size: nameSize, Bold: true, Color: "dk1", Align: "ctr"},
			{Content: title, Size: titleSize, Color: "dk2", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

// barBColor is the right column's accent-bar colour. An accent bar takes a bare
// scheme name with no tint, so the peer default reuses accent_a rather than
// reaching for accent2 — the bar marks the same org the header does.
func barBColor(accentA, accentB string, usePeerTone bool) string {
	if usePeerTone {
		return accentA
	}
	return accentB
}
