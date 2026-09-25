package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// contact-directory pattern — "Key contacts" page: 1-4 groups (regions,
// practices, offices), each a bold accent heading over a rule, then the
// group's people in rows of N. Each person is a small circular headshot (or
// an initials disc) beside a bold name and a muted title.
// ---------------------------------------------------------------------------
//
// Layout (columns = 3):
//
//   North America
//   ───────────────────────────────────────────────────────────────────
//   (JS)  Jane Smith          (AP)  Arun Patel         (LR)  Lila Romero
//         Managing Director         Partner                  Principal
//   Europe
//   ───────────────────────────────────────────────────────────────────
//   (TB)  Tom Becker          …
//
// Every person is TWO top-level grid columns — the headshot and the text —
// rather than a nested grid, so the readability preflight still sees each
// name / title cell (go-slide-creator-hdpq). Rows are content-sized in
// points; the pattern's vertical_align default centres the block. The rule
// under a heading is the heading cell's bottom accent bar, drawn in the row
// gap. A sparse directory (one or two rows of people) instead stacks each
// headshot above a centred name — one column per person — so a single row
// does not leave a thin strip in an empty slide.

func init() {
	Default().Register(&contactDirectory{})
}

type contactDirectory struct{}

// Pattern budgets.
const (
	cdMinGroups      = 1
	cdMaxGroups      = 4
	cdMaxPeople      = 24
	cdGroupNameMax   = 40
	cdNameMax        = 40
	cdTitleMax       = 60
	cdPhotoAltMax    = 200
	cdDefaultColumns = 4
	cdMinColumns     = 3
	cdMaxColumns     = 5

	cdColGapPt   = 8.0
	cdRowGapPt   = 6.0
	cdRulePt     = 1.0
	cdGroupGapPt = 6.0 // extra air above every group heading after the first
	// cdTextPadPt is the vertical allowance the shape-grid row estimator adds
	// to every text cell (the OOXML default 3.6pt top + bottom). The pattern
	// sets only left / right insets, so the renderer draws with less than
	// this and a row sized to it never reports an overflow.
	cdTextPadPt    = 7.2
	cdTextInsetLPt = 4.0
	cdTextInsetRPt = 2.0
	cdPhotoMinPt   = 30.0
	// Stacked (sparse) layout: headshot above a centred name.
	cdStackPhotoMaxPt = 128.0
	cdStackPhotoMinPt = 64.0
	cdStackColGapPt   = 16.0
	cdStackPhotoPadPt = 6.0 // air between the rule and the headshots
	cdPhotoStepPt     = 4.0
	// cdNameMaxLines is how many lines a name may wrap to before the fit
	// report calls it too long for the column.
	cdNameMaxLines = 2
	// cdSafetyPt is the slack left under a person's text so rounding never
	// pushes it into autofit.
	cdSafetyPt = 2.0
	// cdMeasureWidthFrac is the share of a text column's width the
	// measurement trusts.
	cdMeasureWidthFrac = 0.88
)

func (c *contactDirectory) Name() string { return "contact-directory" }
func (c *contactDirectory) Description() string {
	return "Key-contacts directory: 1-4 groups (regions / practices), each an accent heading over a rule, then people in rows of 3-5 — circular headshot (or initials disc) + bold name + muted title; up to 24 people"
}
func (c *contactDirectory) UseWhen() string {
	return "Key contacts / 'who to call' / directory slide listing up to 24 people by name and title, optionally grouped under 1–4 headings (regions, offices, practices), each with a small circular headshot or initials; prefer team-bios when each person needs a short bio (≤8 people, no groups), dual-org-ladder for paired client / consultant roles"
}
func (c *contactDirectory) NotWhen() string {
	return "People need a biography or credentials sentence (use team-bios), roles are paired across two organisations (use dual-org-ladder), the slide is a reporting hierarchy (use an org_chart diagram), or there are more than 24 contacts (split across slides)"
}
func (c *contactDirectory) Version() int      { return 1 }
func (c *contactDirectory) CellsHint() string { return "1-4 groups × 1-24 people" }
func (c *contactDirectory) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "conclude"},
		PairsWith:     []string{"team-bios", "agenda", "text-sidebar"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}

func (c *contactDirectory) ExemplarValues() any {
	return &ContactDirectoryValues{
		Groups: []ContactDirectoryGroup{
			{Name: "North America", People: []ContactDirectoryPerson{
				{Name: "Jane Smith", Title: "Managing Director & Partner, New York"},
				{Name: "Arun Patel", Title: "Partner, Toronto"},
				{Name: "Lila Romero", Title: "Principal, Chicago"},
				{Name: "Tom Becker", Title: "Principal, San Francisco"},
			}},
			{Name: "Europe", People: []ContactDirectoryPerson{
				{Name: "Sophie Laurent", Title: "Managing Director & Partner, Paris"},
				{Name: "Henrik Nilsson", Title: "Partner, Stockholm"},
				{Name: "Marta Kowalska", Title: "Principal, Warsaw"},
			}},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ContactDirectoryPerson is one contact: name, optional title and headshot.
type ContactDirectoryPerson struct {
	Name  string                     `json:"name"`
	Title string                     `json:"title,omitempty"`
	Photo *jsonschema.GridImageInput `json:"photo,omitempty"` // {path | url, alt}; circular, cover-cropped. Without it an initials disc is drawn.
}

// ContactDirectoryGroup is one heading (region, office, practice) and its people.
type ContactDirectoryGroup struct {
	Name   string                   `json:"name"`
	People []ContactDirectoryPerson `json:"people"`
}

// ContactDirectoryValues holds 1-4 groups with up to 24 people in total.
type ContactDirectoryValues struct {
	Groups []ContactDirectoryGroup `json:"groups"`
}

// ContactDirectoryOverrides are the pattern-level overrides.
type ContactDirectoryOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	Columns        int     `json:"columns,omitempty"`    // people per row, 3-5 (default 4)
	NameSize       float64 `json:"name_size,omitempty"`  // default 14 (12-13 when dense)
	TitleSize      float64 `json:"title_size,omitempty"` // default 12
}

func (c *contactDirectory) NewValues() any       { return &ContactDirectoryValues{} }
func (c *contactDirectory) NewOverrides() any    { return &ContactDirectoryOverrides{} }
func (c *contactDirectory) NewCellOverride() any { return nil }

// ImageAssets exposes every headshot so hosts resolve its path / url like a
// shape_grid image cell.
func (c *contactDirectory) ImageAssets(values any) []ImageAssetRef {
	v, ok := values.(*ContactDirectoryValues)
	if !ok || v == nil {
		return nil
	}
	var refs []ImageAssetRef
	for g := range v.Groups {
		for p := range v.Groups[g].People {
			if img := v.Groups[g].People[p].Photo; img != nil {
				refs = append(refs, ImageAssetRef{Field: fmt.Sprintf("groups/%d/people/%d/photo", g, p), Image: img})
			}
		}
	}
	return refs
}

func (c *contactDirectory) Schema() *Schema {
	person := ObjectSchema(map[string]*Schema{
		"name":  StringSchema(cdNameMax).WithDescription("Full name (bold; may wrap to two lines)"),
		"title": StringSchema(cdTitleMax).WithDescription("Role / title / office in a smaller muted line"),
		"photo": PhotoSchema("Headshot, cover-cropped into a small circle; omit it to draw an initials disc (alt defaults to the name and title)", cdPhotoAltMax),
	}, []string{"name"}).WithAdditionalProperties(false)

	group := ObjectSchema(map[string]*Schema{
		"name":   StringSchema(cdGroupNameMax).WithDescription("Group heading, e.g. a region, office or practice (accent bold, rule underneath)"),
		"people": ArraySchema(person, 1, cdMaxPeople).WithDescription("People in this group, laid out in rows of `columns`"),
	}, []string{"name", "people"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"groups": ArraySchema(group, cdMinGroups, cdMaxGroups).WithDescription("1-4 groups; at most 24 people across all groups. A single ungrouped list is one group"),
	}, []string{"groups"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":          StringSchema(0).WithDescription("Accent scheme color for the group headings, rules and initials discs (default accent1)").WithDefault("accent1"),
		"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"columns":         IntegerSchema(cdMinColumns, cdMaxColumns).WithDescription("People per row (default 4)").WithDefault(cdDefaultColumns),
		"name_size":       NumberSchema(12, 28).WithDescription("Name font size in points (default 14, stepped down to 12 when the directory is dense)"),
		"title_size":      NumberSchema(12, 20).WithDescription("Title font size in points (default 12)"),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":    valuesSchema,
		"overrides": overridesSchema,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Key-contacts directory: grouped rows of circular headshots with name and title")
}

func (c *contactDirectory) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*ContactDirectoryValues)
	if !ok || v == nil {
		return fmt.Errorf("contact-directory: values must be *ContactDirectoryValues, got %T", values)
	}
	const name = "contact-directory"
	var errs []error

	if overrides != nil {
		ovr, ok := overrides.(*ContactDirectoryOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("contact-directory: overrides must be *ContactDirectoryOverrides, got %T", overrides))
		} else if ovr.Columns != 0 && (ovr.Columns < cdMinColumns || ovr.Columns > cdMaxColumns) {
			errs = append(errs, errOutOfRange(name, "overrides.columns", cdMinColumns, cdMaxColumns, ovr.Columns))
		}
	}

	if len(v.Groups) < cdMinGroups {
		errs = append(errs, errMinItems(name, "groups", cdMinGroups, len(v.Groups), "(hint: put an ungrouped list in one group)"))
	}
	if len(v.Groups) > cdMaxGroups {
		errs = append(errs, errMaxItems(name, "groups", cdMaxGroups, len(v.Groups), "(hint: merge groups or split the directory across slides)"))
	}
	total := 0
	for g, grp := range v.Groups {
		gp := fmt.Sprintf("groups[%d]", g)
		if strings.TrimSpace(grp.Name) == "" {
			errs = append(errs, errRequired(name, gp+".name"))
		} else if runeLen(grp.Name) > cdGroupNameMax {
			errs = append(errs, errMaxLength(name, gp+".name", cdGroupNameMax, runeLen(grp.Name)))
		}
		if len(grp.People) == 0 {
			errs = append(errs, errMinItems(name, gp+".people", 1, 0, "(hint: drop an empty group)"))
		}
		total += len(grp.People)
		for p, person := range grp.People {
			pp := fmt.Sprintf("%s.people[%d]", gp, p)
			if strings.TrimSpace(person.Name) == "" {
				errs = append(errs, errRequired(name, pp+".name"))
			} else if runeLen(person.Name) > cdNameMax {
				errs = append(errs, errMaxLength(name, pp+".name", cdNameMax, runeLen(person.Name)))
			}
			if runeLen(person.Title) > cdTitleMax {
				errs = append(errs, errMaxLength(name, pp+".title", cdTitleMax, runeLen(person.Title)))
			}
			errs = append(errs, validatePatternPhoto(name, pp+".photo", person.Photo, cdPhotoAltMax)...)
		}
	}
	if total > cdMaxPeople {
		errs = append(errs, errMaxItems(name, "groups[].people", cdMaxPeople, total, "(hint: split the directory across two slides — each holds up to 24 people)"))
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"contact-directory: cell_overrides are not supported (use overrides)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

// cdLayout is the measured geometry of one expansion.
type cdLayout struct {
	columns             int
	nameSize, titleSize float64
	headSize            float64
	photoPt             float64
	personW             float64   // photo + text column share, in points (gaps excluded)
	textW               float64   // text column width in points
	headPt              []float64 // heading row height per group (group gap included)
	rowPt               [][]float64
	// stacked puts the headshot ABOVE a centred name / title (a sparse
	// directory); otherwise the headshot sits left of the text. In stacked
	// mode rowPt is the photo row and textRowPt the text row under it.
	stacked   bool
	textRowPt [][]float64
	naturalPt float64
	fits      bool
}

// cdTypeStep is one name / title size pair of the type scale search.
type cdTypeStep struct{ name, title float64 }

func cdColumns(ovr *ContactDirectoryOverrides) int {
	if ovr != nil && ovr.Columns >= cdMinColumns && ovr.Columns <= cdMaxColumns {
		return ovr.Columns
	}
	return cdDefaultColumns
}

// cdPersonRows splits people into rows of n.
func cdPersonRows(people []ContactDirectoryPerson, n int) [][]ContactDirectoryPerson {
	var rows [][]ContactDirectoryPerson
	for start := 0; start < len(people); start += n {
		end := min(start+n, len(people))
		rows = append(rows, people[start:end])
	}
	return rows
}

// cdTextHeightPt is the measured height of one person's name + title at the
// text column width, with the pattern's own tight insets.
func cdTextHeightPt(ctx ExpandContext, p ContactDirectoryPerson, nameSize, titleSize, textW float64) float64 {
	// paragraphLines removes the default LR insets; measure against ~88% of
	// the real width, because the renderer's own autofit measures with its
	// font, not the theme's, and a name that just fits there may not here.
	w := (textW-cdTextInsetLPt-cdTextInsetRPt)*cdMeasureWidthFrac + 2*sizingInsetLRPt
	h := float64(paragraphLines(ctx, sizedPara{text: p.Name, sizePt: nameSize, bold: true}, w)) * nameSize * sizingLineSpacing
	if strings.TrimSpace(p.Title) != "" {
		h += float64(paragraphLines(ctx, sizedPara{text: p.Title, sizePt: titleSize}, w)) * titleSize * sizingLineSpacing
	}
	return h + cdTextPadPt + cdSafetyPt
}

func cdMeasure(ctx ExpandContext, v *ContactDirectoryValues, ovr *ContactDirectoryOverrides) cdLayout {
	areaW, areaH := sizingAreaPt(ctx)
	n := cdColumns(ovr)
	personRows := 0
	for _, g := range v.Groups {
		personRows += (len(g.People) + n - 1) / n
	}
	// A sparse directory earns larger headshots and type rather than a small
	// block floating in an empty slide; a dense one starts smaller.
	steps := []cdTypeStep{{14, 12}, {13, 12}, {12, 12}}
	photoMax := 60.0
	switch {
	case personRows <= 2:
		steps = append([]cdTypeStep{{20, 15}, {18, 14}, {16, 13}}, steps...)
		photoMax = 120
	case personRows <= 4:
		steps = append([]cdTypeStep{{16, 13}}, steps...)
		photoMax = 72
	}
	if ovr != nil && (ovr.NameSize > 0 || ovr.TitleSize > 0) {
		steps = []cdTypeStep{{ResolveSize(ovr.NameSize, 14), ResolveSize(ovr.TitleSize, 12)}}
	}
	// A sparse directory (one or two rows of people) stacks each headshot
	// above a centred name: side-by-side, a single row is a thin strip in an
	// empty slide.
	var stacked cdLayout
	if personRows <= 2 {
		stackW := (areaW - cdStackColGapPt*float64(n-1)) / float64(n)
		stackMax := math.Min(cdStackPhotoMaxPt, math.Floor(stackW*0.6))
	stackSearch:
		for _, st := range steps {
			for photo := stackMax; photo >= cdStackPhotoMinPt; photo -= cdPhotoStepPt {
				if lay := cdMeasureStacked(ctx, v, n, st.name, st.title, photo, stackW, areaH); lay.fits {
					stacked = lay
					break stackSearch
				}
			}
		}
	}
	side := cdMeasureSide(ctx, v, n, steps, photoMax, areaW, areaH)
	// Stacked wins unless it had to set the names smaller than the
	// side-by-side layout manages: large type is what reads across a room.
	if stacked.fits && (!side.fits || stacked.nameSize >= side.nameSize) {
		return stacked
	}
	return side
}

// cdMeasureSide measures the side-by-side layout (headshot left of the text).
func cdMeasureSide(ctx ExpandContext, v *ContactDirectoryValues, n int, steps []cdTypeStep, photoMax, areaW, areaH float64) cdLayout {
	usable := areaW - cdColGapPt*float64(2*n-1)
	personW := usable / float64(n)
	photoMax = math.Min(photoMax, math.Floor(personW*0.42))

	// Give up type size before the headshot shrinks far: first try every
	// type step with a headshot of at least 70% of the maximum, and only
	// then let the headshot fall to its floor.
	var lay cdLayout
	for photo := photoMax; photo >= math.Max(cdPhotoMinPt, photoMax*0.7); photo -= cdPhotoStepPt {
		for _, st := range steps {
			lay = cdMeasureAt(ctx, v, n, st.name, st.title, photo, personW, areaH)
			if lay.fits {
				return lay
			}
		}
	}
	for _, st := range steps {
		for photo := photoMax; photo >= cdPhotoMinPt; photo -= cdPhotoStepPt {
			lay = cdMeasureAt(ctx, v, n, st.name, st.title, photo, personW, areaH)
			if lay.fits {
				return lay
			}
		}
	}
	return lay
}

func cdMeasureAt(ctx ExpandContext, v *ContactDirectoryValues, n int, nameSize, titleSize, photo, personW, areaH float64) cdLayout {
	lay := cdLayout{
		columns:   n,
		nameSize:  nameSize,
		titleSize: titleSize,
		headSize:  nameSize + 2,
		photoPt:   photo,
		personW:   personW,
		textW:     personW - photo,
	}
	rows := 0
	for g, grp := range v.Groups {
		head := math.Ceil(lay.headSize*sizingLineSpacing + cdTextPadPt + cdSafetyPt)
		if g > 0 {
			head += cdGroupGapPt
		}
		lay.headPt = append(lay.headPt, head)
		lay.naturalPt += head
		rows++
		var heights []float64
		for _, r := range cdPersonRows(grp.People, n) {
			h := photo
			for _, p := range r {
				h = math.Max(h, cdTextHeightPt(ctx, p, nameSize, titleSize, lay.textW))
			}
			h = math.Ceil(h)
			heights = append(heights, h)
			lay.naturalPt += h
			rows++
		}
		lay.rowPt = append(lay.rowPt, heights)
	}
	lay.naturalPt += cdRowGapPt * float64(rows-1)
	lay.fits = lay.naturalPt <= areaH && lay.namesFit(ctx, v)
	return lay
}

// cdMeasureStacked measures the sparse layout: per person row, a photo row
// and a text row, each person one column of stackW points.
func cdMeasureStacked(ctx ExpandContext, v *ContactDirectoryValues, n int, nameSize, titleSize, photo, stackW, areaH float64) cdLayout {
	lay := cdLayout{
		columns:   n,
		nameSize:  nameSize,
		titleSize: titleSize,
		headSize:  nameSize + 2,
		photoPt:   photo,
		personW:   stackW,
		textW:     stackW,
		stacked:   true,
	}
	rows := 0
	for g, grp := range v.Groups {
		head := math.Ceil(lay.headSize*sizingLineSpacing + cdTextPadPt + cdSafetyPt)
		if g > 0 {
			head += cdGroupGapPt
		}
		lay.headPt = append(lay.headPt, head)
		lay.naturalPt += head
		rows++
		var photos, texts []float64
		for _, r := range cdPersonRows(grp.People, n) {
			text := 0.0
			for _, p := range r {
				text = math.Max(text, cdTextHeightPt(ctx, p, nameSize, titleSize, stackW))
			}
			text = math.Ceil(text)
			// The photo row carries the air between the rule and the photo.
			photos = append(photos, photo+cdStackPhotoPadPt)
			texts = append(texts, text)
			lay.naturalPt += photo + cdStackPhotoPadPt + text
			rows += 2
		}
		lay.rowPt = append(lay.rowPt, photos)
		lay.textRowPt = append(lay.textRowPt, texts)
	}
	lay.naturalPt += cdRowGapPt * float64(rows-1)
	lay.fits = lay.naturalPt <= areaH && lay.namesFit(ctx, v)
	return lay
}

// namesFit reports whether every word of every name fits on one line of the
// text column: a long surname must wrap between words, never mid-word.
func (lay cdLayout) namesFit(ctx ExpandContext, v *ContactDirectoryValues) bool {
	font := cdFont(ctx)
	w := lay.textW - cdTextInsetLPt - cdTextInsetRPt
	for _, g := range v.Groups {
		for _, p := range g.People {
			for _, word := range strings.Fields(p.Name) {
				if measuredLines(word, font, true, lay.nameSize, w*0.95) > 1 {
					return false
				}
			}
		}
	}
	return true
}

func cdFont(ctx ExpandContext) string {
	if f := strings.TrimSpace(ctx.Theme.BodyFont); f != "" {
		return f
	}
	return "Arial"
}

// PostExpandWarnings reports names that wrap past two lines and directories
// that do not fit the content area even at the smallest type and headshot.
func (c *contactDirectory) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*ContactDirectoryValues)
	if !ok || v == nil || len(v.Groups) == 0 {
		return nil
	}
	ovr, _ := overrides.(*ContactDirectoryOverrides)
	lay := cdMeasure(ctx, v, ovr)
	var warnings []string
	w := lay.textW - cdTextInsetLPt - cdTextInsetRPt + 2*sizingInsetLRPt
	for g, grp := range v.Groups {
		for p, person := range grp.People {
			if lines := paragraphLines(ctx, sizedPara{text: person.Name, sizePt: lay.nameSize, bold: true}, w); lines > cdNameMaxLines {
				warnings = append(warnings, fmt.Sprintf(
					"%s: contact-directory groups[%d].people[%d].name wraps to %d lines at %d people per row; a name holds about %d lines — shorten the name or lower overrides.columns",
					ErrCodeBodyTooLong, g, p, lines, lay.columns, cdNameMaxLines))
			}
		}
	}
	if !lay.fits {
		_, areaH := sizingAreaPt(ctx)
		warnings = append(warnings, fmt.Sprintf(
			"%s: contact-directory groups need %.0fpt at the smallest type and headshot but the content area holds about %.0fpt — raise overrides.columns, merge groups, shorten titles, or split the directory across slides",
			ErrCodeBodyTooLong, lay.naturalPt, areaH))
	}
	return warnings
}

func (c *contactDirectory) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*ContactDirectoryValues)
	if !ok {
		return nil, fmt.Errorf("contact-directory: values must be *ContactDirectoryValues, got %T", values)
	}
	ovr := &ContactDirectoryOverrides{}
	if overrides != nil {
		if ovr, ok = overrides.(*ContactDirectoryOverrides); !ok {
			return nil, fmt.Errorf("contact-directory: overrides must be *ContactDirectoryOverrides, got %T", overrides)
		}
	}
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	lay := cdMeasure(ctx, v, ovr)
	n := lay.columns
	areaW, _ := sizingAreaPt(ctx)
	usable := areaW - cdColGapPt*float64(2*n-1)

	// Columns: photo, text, photo, text, … as percentages of the width left
	// after the gaps. A stacked (sparse) directory has one column per person.
	span, colGap := 2*n, cdColGapPt
	var cols []float64
	if lay.stacked {
		span, colGap = n, cdStackColGapPt
		for i := 0; i < n; i++ {
			cols = append(cols, math.Round(1000/float64(n))/10)
		}
	} else {
		photoPct := math.Round(lay.photoPt/usable*1000) / 10
		textPct := math.Round((100/float64(n)-photoPct)*10) / 10
		for i := 0; i < n; i++ {
			cols = append(cols, photoPct, textPct)
		}
	}

	headInk := inkOnLight(ctx, accent, 3.0)
	nameInk := inkOnLight(ctx, "dk1", 4.5)
	titleInk := inkOnLight(ctx, "dk2", 4.5)
	discTone := inactiveTintTone(accent)
	// Initials are large bold text: the accent reads on its own pale tint on
	// most palettes (3:1 is the large-text bar); a light accent falls back to
	// the measured theme ink. Without a theme the accent is kept.
	discInk := accent
	if ratio, ok := fillContrast(ctx, fillTone{Color: accent}, discTone); ok && ratio < 4.5 {
		discInk = readableTextOn(ctx, discTone, accent)
	}

	var rows []jsonschema.GridRowInput
	for g, grp := range v.Groups {
		headJSON, _ := json.Marshal(chartInsightsText{
			Paragraphs:    []chartInsightsParagraph{{Content: grp.Name, Size: lay.headSize, Bold: true, Color: headInk, Align: "l"}},
			Align:         "l",
			VerticalAlign: "b",
		})
		// The rule under the heading is the heading cell's bottom accent bar,
		// drawn in the row gap, so it costs no row of its own.
		rows = append(rows, jsonschema.GridRowInput{
			MinHeight: lay.headPt[g], MaxHeight: lay.headPt[g],
			Cells: []*jsonschema.GridCellInput{{
				ColSpan:   span,
				Shape:     &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: cdInsetText(headJSON, 0)},
				AccentBar: &jsonschema.AccentBarInput{Position: "bottom", Color: accent, Width: cdRulePt},
			}},
		})
		for r, people := range cdPersonRows(grp.People, n) {
			if lay.stacked {
				rows = append(rows, cdStackedRows(people, n, lay, lay.rowPt[g][r], lay.textRowPt[g][r], discTone, discInk, nameInk, titleInk)...)
				continue
			}
			var cells []*jsonschema.GridCellInput
			for _, p := range people {
				cells = append(cells,
					cdPhotoCell(p, discTone, discInk, lay.photoPt),
					cdTextCell(p, lay, nameInk, titleInk),
				)
			}
			if short := n - len(people); short > 0 {
				cells = append(cells, &jsonschema.GridCellInput{ColSpan: 2 * short, Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}})
			}
			h := lay.rowPt[g][r]
			rows = append(rows, jsonschema.GridRowInput{MinHeight: h, MaxHeight: h, Cells: cells})
		}
	}

	colsJSON, _ := json.Marshal(cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  colGap,
		RowGap:  cdRowGapPt,
		Rows:    rows,
	}
	return grid, nil
}

// cdStackedRows emits one sparse person row: centred headshots (capped at
// the headshot size inside a slightly taller row, which keeps air under the
// rule) over centred names and titles.
func cdStackedRows(people []ContactDirectoryPerson, n int, lay cdLayout, photoRowPt, textRowPt float64, disc fillTone, discInk, nameInk, titleInk string) []jsonschema.GridRowInput {
	photos := make([]*jsonschema.GridCellInput, 0, n)
	texts := make([]*jsonschema.GridCellInput, 0, n)
	for _, p := range people {
		photo := cdPhotoCell(p, disc, discInk, lay.photoPt)
		photo.MaxHeight = lay.photoPt
		photos = append(photos, photo)
		texts = append(texts, cdTextCellAligned(p, lay, nameInk, titleInk, "ctr", "t"))
	}
	if short := n - len(people); short > 0 {
		empty := func() *jsonschema.GridCellInput {
			return &jsonschema.GridCellInput{ColSpan: short, Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`)}}
		}
		photos = append(photos, empty())
		texts = append(texts, empty())
	}
	return []jsonschema.GridRowInput{
		{MinHeight: photoRowPt, MaxHeight: photoRowPt, Cells: photos},
		{MinHeight: textRowPt, MaxHeight: textRowPt, Cells: texts},
	}
}

// cdInsetText adds the pattern's tight left / right insets to a
// marshalled text object.
func cdInsetText(textJSON json.RawMessage, left float64) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(textJSON, &obj); err != nil {
		return textJSON
	}
	obj["inset_left"] = left
	obj["inset_right"] = cdTextInsetRPt
	out, err := json.Marshal(obj)
	if err != nil {
		return textJSON
	}
	return out
}

// cdPhotoCell is the circular headshot, or an initials disc without one.
func cdPhotoCell(p ContactDirectoryPerson, disc fillTone, discInk string, photoPt float64) *jsonschema.GridCellInput {
	alt := strings.TrimSpace(p.Name)
	if t := strings.TrimSpace(p.Title); t != "" && alt != "" {
		alt += ", " + t
	}
	if cell := patternPhotoCell(p.Photo, firstNonEmpty(alt, "Contact photo")); cell != nil {
		cell.Fit = "contain"
		cell.Image.Geometry = "ellipse"
		return cell
	}
	// Two bold initials fill the ellipse's inscribed text box at 30% of the
	// disc, never under the 12pt floor.
	initialsSize := math.Max(shapegrid.MinTextSizePt, math.Round(photoPt*0.3))
	textJSON, _ := json.Marshal(chartInsightsText{
		Paragraphs:    []chartInsightsParagraph{{Content: deriveInitials(p.Name), Size: initialsSize, Bold: true, Color: discInk, Align: "ctr"}},
		Align:         "ctr",
		VerticalAlign: "ctr",
	})
	return &jsonschema.GridCellInput{
		Fit: "contain",
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "ellipse",
			Fill:     disc.fillJSON(),
			Text:     cdZeroInsetText(textJSON),
		},
	}
}

// cdZeroInsetText strips the insets from a small centred label so two
// initials fit a small disc.
func cdZeroInsetText(textJSON json.RawMessage) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(textJSON, &obj); err != nil {
		return textJSON
	}
	// An all-zero inset reads as "unset" (the 7.2pt default), which wraps
	// two initials inside a small disc; a hairline keeps the text box wide.
	for _, k := range []string{"inset_left", "inset_right", "inset_top", "inset_bottom"} {
		obj[k] = 0.5
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return textJSON
	}
	return out
}

// cdTextCell is the bold name over the muted title, centred on the headshot.
func cdTextCell(p ContactDirectoryPerson, lay cdLayout, nameInk, titleInk string) *jsonschema.GridCellInput {
	return cdTextCellAligned(p, lay, nameInk, titleInk, "l", "ctr")
}

func cdTextCellAligned(p ContactDirectoryPerson, lay cdLayout, nameInk, titleInk, align, vAlign string) *jsonschema.GridCellInput {
	paras := []chartInsightsParagraph{{Content: p.Name, Size: lay.nameSize, Bold: true, Color: nameInk, Align: align}}
	if t := strings.TrimSpace(p.Title); t != "" {
		paras = append(paras, chartInsightsParagraph{Content: t, Size: lay.titleSize, Color: titleInk, Align: align})
	}
	textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: align, VerticalAlign: vAlign})
	left := cdTextInsetLPt
	if align == "ctr" {
		left = cdTextInsetRPt
	}
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(`"none"`),
		Text:     cdInsetText(textJSON, left),
	}}
}
