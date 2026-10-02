package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// team-bios pattern — photo placeholder + name/role/bio cards (4-up or 8-up)
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&teamBios{})
}

type teamBios struct{}

func (t *teamBios) Name() string { return "team-bios" }
func (t *teamBios) Description() string {
	return "Team / 'Our People' grid — headshot (or initials placeholder) above name + role + 2-line bio per member (1–8 members, up to 4 per row)"
}
func (t *teamBios) UseWhen() string {
	return "Team or 'Our People' slide with 1–8 members, each with a headshot (members[].photo) or an initials placeholder above name + role + short bio; prefer card-grid when items are generic features rather than people, agenda-with-images for narrative agenda rows"
}
func (t *teamBios) NotWhen() string {
	return "Items are generic feature cards without a person photo (use card-grid), a key-contacts / directory page of names and titles without bios, grouped by region or with more than 8 people (use contact-directory), more than 8 members with bios (split across slides), or items are numbered agenda sections (use agenda-with-images)"
}
func (t *teamBios) Version() int      { return 2 }
func (t *teamBios) CellsHint() string { return "1-8" }
func (t *teamBios) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"agenda", "stat-hero", "card-grid"},
		DensityClass:  "medium",
		AccentWeight:  "subtle",
	}
}

func (t *teamBios) ExemplarValues() any {
	return &TeamBiosValues{
		Members: []TeamBiosMember{
			{Name: "Jane Smith", Role: "Project Lead", Bio: "10 years in supply-chain strategy. Previously at McKinsey."},
			{Name: "Arun Patel", Role: "Lead Engineer", Bio: "Built data platforms at two scale-ups. MIT alumnus."},
			{Name: "Lila Romero", Role: "Design Lead", Bio: "Service-design specialist. Ex-IDEO, ex-Frog."},
			{Name: "Tom Becker", Role: "Client Partner", Bio: "Account director for top-10 retailers. Frankfurt office."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// TeamBiosMember is a single team member card.
type TeamBiosMember struct {
	Name       string                     `json:"name"`                  // Person's full name (rendered bold)
	Role       string                     `json:"role"`                  // Role / title (rendered in accent color)
	Bio        string                     `json:"bio,omitempty"`         // Optional short bio (~2 lines).
	Photo      *jsonschema.GridImageInput `json:"photo,omitempty"`       // Optional headshot {path | url, alt}; cover-cropped to the frame. Without it the initials placeholder is drawn.
	PhotoLabel string                     `json:"photo_label,omitempty"` // Optional label shown in the photo placeholder; defaults to the person's initials. Ignored when photo is set.
}

// TeamBiosValues holds the team member cards (1–8 members).
type TeamBiosValues struct {
	Members []TeamBiosMember `json:"members"`
}

// TeamBiosOverrides controls accent color and per-zone font sizes.
type TeamBiosOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	NameSize       float64 `json:"name_size,omitempty"`        // Default 14
	RoleSize       float64 `json:"role_size,omitempty"`        // Default 11
	BioSize        float64 `json:"bio_size,omitempty"`         // Default 10
	PhotoLabelSize float64 `json:"photo_label_size,omitempty"` // Default 14 (initials prominent)
}

// TeamBiosCellOverride is the shared per-cell override, indexed by member.
type TeamBiosCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	teamBiosMaxMembers    = 8
	teamBiosMaxPerRow     = 4
	teamBiosMinPerRow     = 1
	teamBiosNameMaxChars  = 60
	teamBiosRoleMaxChars  = 80
	teamBiosBioMaxChars   = 220
	teamBiosPhotoMaxChars = 8
	// teamBiosPhotoAltMaxChars matches image-text-split's alt budget.
	teamBiosPhotoAltMaxChars = 200
)

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

// ImageAssets exposes each member's photo so hosts resolve its path / url the
// way they do for shape_grid image cells (go-slide-creator-hdpq).
func (t *teamBios) ImageAssets(values any) []ImageAssetRef {
	v, ok := values.(*TeamBiosValues)
	if !ok || v == nil {
		return nil
	}
	var refs []ImageAssetRef
	for i := range v.Members {
		if v.Members[i].Photo != nil {
			refs = append(refs, ImageAssetRef{
				Field: fmt.Sprintf("members/%d/photo", i),
				Image: v.Members[i].Photo,
			})
		}
	}
	return refs
}

func (t *teamBios) NewValues() any       { return &TeamBiosValues{} }
func (t *teamBios) NewOverrides() any    { return &TeamBiosOverrides{} }
func (t *teamBios) NewCellOverride() any { return &TeamBiosCellOverride{} }

// Measured with TestTeamBiosBudgetProbe against the written size on every shipped template at
// default sizes, every shape keeping the uniform 0.5 cm text margin. A fifth
// member adds a second card row and halves text height, which also tightens
// the name and role lines.
func teamBiosReadableBioBudget(members int) int {
	if members > 4 {
		return 40
	}
	return teamBiosBioMaxChars
}

// teamBiosTwoRowNameBudget / teamBiosTwoRowRoleBudget are the readable name
// and role lengths with five to eight members.
const (
	teamBiosTwoRowNameBudget = 52
	teamBiosTwoRowRoleBudget = 40
)

func (t *teamBios) Schema() *Schema {
	memberSchema := ObjectSchema(
		map[string]*Schema{
			"name":        StringSchema(teamBiosNameMaxChars).WithDescription("Person's full name (rendered bold); about 52 readable characters with 5-8 members"),
			"role":        StringSchema(teamBiosRoleMaxChars).WithDescription("Role or title (rendered in accent color); about 40 readable characters with 5-8 members"),
			"bio":         StringSchema(teamBiosBioMaxChars).WithDescription("Short bio. Approximate readable limit: 220 characters with 1-4 members, 40 with 5-8; longer bios emit BODY_TOO_LONG."),
			"photo":       PhotoSchema("Headshot, cover-cropped into a centred circle; omit it to draw a circular initials disc (alt defaults to the member's name and role)", teamBiosPhotoAltMaxChars),
			"photo_label": StringSchema(teamBiosPhotoMaxChars).WithDescription("Label centred in the initials placeholder when no photo is given; defaults to initials derived from name"),
		},
		[]string{"name", "role"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"members": ArraySchema(memberSchema, 1, teamBiosMaxMembers).WithDescription("Team members (1–8). 1–4 render as a single row; 5–8 render as two rows of up to 4."),
		},
		[]string{"members"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color for the role text and photo frame (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"name_size":        NumberSchema(6, 40).WithDescription("Font size for member name in points (default 14)"),
			"role_size":        NumberSchema(6, 40).WithDescription("Font size for role/title in points (default 11)"),
			"bio_size":         NumberSchema(6, 40).WithDescription("Font size for bio paragraph in points (default 10)"),
			"photo_label_size": NumberSchema(6, 60).WithDescription("Font size for the photo-placeholder initials in points (default 14)"),
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
	}).WithDescription("Team / 'Our People' card grid: a headshot (members[].photo) or an initials placeholder above name + role + short bio. 1–4 members render in a single row; 5–8 members render as two rows of up to four.")
}

func (t *teamBios) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*TeamBiosValues)
	if !ok || v == nil {
		return fmt.Errorf("team-bios: values must be *TeamBiosValues, got %T", values)
	}

	const name = "team-bios"
	var errs []error

	if len(v.Members) < 1 {
		errs = append(errs, errMinItems(name, "members", 1, len(v.Members), ""))
	}
	if len(v.Members) > teamBiosMaxMembers {
		errs = append(errs, errMaxItems(name, "members", teamBiosMaxMembers, len(v.Members), "(hint: split the team across two slides — each slide supports up to 8 members)"))
	}

	for i, m := range v.Members {
		namePath := fmt.Sprintf("members[%d].name", i)
		if strings.TrimSpace(m.Name) == "" {
			errs = append(errs, errRequired(name, namePath))
		} else if runeLen(m.Name) > teamBiosNameMaxChars {
			errs = append(errs, errMaxLength(name, namePath, teamBiosNameMaxChars, runeLen(m.Name)))
		}
		rolePath := fmt.Sprintf("members[%d].role", i)
		if strings.TrimSpace(m.Role) == "" {
			errs = append(errs, errRequired(name, rolePath))
		} else if runeLen(m.Role) > teamBiosRoleMaxChars {
			errs = append(errs, errMaxLength(name, rolePath, teamBiosRoleMaxChars, runeLen(m.Role)))
		}
		if runeLen(m.Bio) > teamBiosBioMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("members[%d].bio", i), teamBiosBioMaxChars, runeLen(m.Bio)))
		}
		if runeLen(m.PhotoLabel) > teamBiosPhotoMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("members[%d].photo_label", i), teamBiosPhotoMaxChars, runeLen(m.PhotoLabel)))
		}
		errs = append(errs, validatePatternPhoto(name, fmt.Sprintf("members[%d].photo", i), m.Photo, teamBiosPhotoAltMaxChars)...)
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, len(v.Members), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// PostExpandWarnings reports bios past the measured budget for the card count.
func (t *teamBios) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*TeamBiosValues)
	if !ok || v == nil {
		return nil
	}
	warnings := teamBiosBudgetWarnings(v)
	if len(warnings) > 0 || len(v.Members) == 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return warnings
	}
	// The character budgets assume a typical content area; with the
	// template's own area the cards are measured against it
	// (go-slide-creator-n1muf).
	ovr, _ := overrides.(*TeamBiosOverrides)
	if ovr == nil {
		ovr = &TeamBiosOverrides{}
	}
	if fit := teamBiosFit(ctx, v, ovr); !fit.fits {
		warnings = append(warnings, fmt.Sprintf("%s: team-bios cards need %.0fpt at readable sizes with the smallest headshot but the content area holds about %.0fpt — shorten the bios, names or roles, or split the team across slides", ErrCodeBodyTooLong, fit.totalPt, fit.areaH))
	}
	return warnings
}

// teamBiosBudgetWarnings reports names, roles and bios past the measured
// character budgets for the member count.
func teamBiosBudgetWarnings(v *TeamBiosValues) []string {
	var warnings []string
	budget := teamBiosReadableBioBudget(len(v.Members))
	for i, m := range v.Members {
		if len(v.Members) > 4 {
			if n := runeLen(m.Name); n > teamBiosTwoRowNameBudget {
				warnings = append(warnings, fmt.Sprintf("%s: team-bios members[%d].name is %d characters; %d members hold about %d name characters per card — shorten the name or split the team across slides", ErrCodeBodyTooLong, i, n, len(v.Members), teamBiosTwoRowNameBudget))
			}
			if n := runeLen(m.Role); n > teamBiosTwoRowRoleBudget {
				warnings = append(warnings, fmt.Sprintf("%s: team-bios members[%d].role is %d characters; %d members hold about %d role characters per card — shorten the role or split the team across slides", ErrCodeBodyTooLong, i, n, len(v.Members), teamBiosTwoRowRoleBudget))
			}
		}
		if n := runeLen(m.Bio); n > budget {
			warnings = append(warnings, fmt.Sprintf(
				"%s: team-bios members[%d].bio is %d characters; %d members hold about %d bio characters per card before text shrinks below the readable minimum — shorten the bio or split the team across slides",
				ErrCodeBodyTooLong, i, n, len(v.Members), budget))
		}
	}
	return warnings
}

func (t *teamBios) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*TeamBiosValues)
	if !ok {
		return nil, fmt.Errorf("team-bios: values must be *TeamBiosValues, got %T", values)
	}
	ovr := &TeamBiosOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*TeamBiosOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("team-bios: overrides must be *TeamBiosOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	// The name steps 14 -> 12pt only when that lets every card be written
	// unshrunk in the content area (go-slide-creator-n1muf).
	fit := teamBiosFit(ctx, v, ovr)
	nameSize := fit.nameSize
	roleSize := ResolveSize(ovr.RoleSize, scaleDenseBodyPt)
	bioSize := ResolveSize(ovr.BioSize, scaleCaptionPt)
	photoLabelSize := ResolveSize(ovr.PhotoLabelSize, scaleSubheadPt)

	// Layout: members are arranged left-to-right, up to teamBiosMaxPerRow per
	// card-row. Each card-row produces TWO grid rows — a photo row (40% of card
	// height) and a text row (60% of card height). The grid column count is the
	// number of cards in the widest row. For 1–4 members this is len(Members);
	// for 5–8 it is 4.
	columns := len(v.Members)
	if columns > teamBiosMaxPerRow {
		columns = teamBiosMaxPerRow
	}
	if columns < teamBiosMinPerRow {
		columns = teamBiosMinPerRow
	}

	// Photo zone target height (in template-relative units); text zone is the
	// remainder. We express row heights as ratios — the resolver normalises them
	// against the total available height.
	const (
		photoRowHeight = 4.0 // ~40% of a card-row
		textRowHeight  = 6.0 // ~60% of a card-row
	)

	// The headshot placeholder is the people primitive contact-directory
	// draws too — an accent-tint disc with bold accent initials — instead of
	// a large lt2 square with small initials (go-slide-creator-q4fut).
	disc, discInk := headshotDisc(ctx, accent)
	initialsSize := teamBiosInitialsSize(ctx, v.Members, fit, columns, ovr.PhotoLabelSize, photoLabelSize)

	var rows []jsonschema.GridRowInput

	for start := 0; start < len(v.Members); start += teamBiosMaxPerRow {
		end := start + teamBiosMaxPerRow
		if end > len(v.Members) {
			end = len(v.Members)
		}
		members := v.Members[start:end]

		// Photo row.
		photoCells := make([]*jsonschema.GridCellInput, columns)
		textCells := make([]*jsonschema.GridCellInput, columns)
		for col := 0; col < columns; col++ {
			if col < len(members) {
				memberIdx := start + col
				m := members[col]
				photoCells[col] = buildTeamBiosPhotoCell(m, disc, discInk, initialsSize)
				textCells[col] = buildTeamBiosTextCell(m, accent, nameSize, roleSize, bioSize)

				// Cell overrides apply to the text cell (where name/role/bio live).
				if co, ok := cellOverrides[memberIdx]; ok {
					if cellOvr, ok2 := co.(*TeamBiosCellOverride); ok2 {
						applyCellTextOverride(textCells[col], cellOvr)
						if cellOvr.AccentBar {
							textCells[col].AccentBar = &jsonschema.AccentBarInput{
								Position: "left",
								Color:    accent,
								Width:    3,
							}
						}
					}
				}
			} else {
				// Empty filler cell so the grid stays uniform when the row is short.
				photoCells[col] = buildTeamBiosEmptyCell()
				textCells[col] = buildTeamBiosEmptyCell()
			}
		}

		rows = append(rows,
			jsonschema.GridRowInput{Height: photoRowHeight, Cells: photoCells},
			jsonschema.GridRowInput{Height: textRowHeight, Cells: textCells},
		)
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, columns)),
		Gap:     ctx.Gap(teamBiosGapPt),
		RowGap:  0,
		Rows:    rows,
	}
	capTeamBiosTextRows(ctx, grid, columns)
	floorTeamBiosTextRows(grid, fit)
	return grid, nil
}

// teamBiosGapPt is the gutter between member columns.
const teamBiosGapPt = 10.0

// teamBiosFitResult is the name size the cards are written at, each card
// row's written text height, and the headshot height that leaves room for
// them in the content area.
type teamBiosFitResult struct {
	nameSize         float64
	textPt           []float64
	photoPt, totalPt float64
	nominalPt, areaH float64
	gapsPt           float64
	fits             bool
}

// teamBiosFit measures every member's name + role + bio with the writer's
// own fit at the real column width. The headshot row keeps its 40% share of
// a card row while the text holds in the rest; otherwise it gives way, down
// to the smallest square that still writes its initials unshrunk, and only
// then does the default 14pt name step to the 12pt floor.
func teamBiosFit(ctx ExpandContext, v *TeamBiosValues, ovr *TeamBiosOverrides) teamBiosFitResult {
	areaW, areaH := sizingAreaPt(ctx)
	columns := min(max(len(v.Members), teamBiosMinPerRow), teamBiosMaxPerRow)
	colW := equalColumnWidthPt(areaW, columns, ctx.Gap(teamBiosGapPt))
	roleSize, bioSize := ResolveSize(ovr.RoleSize, scaleDenseBodyPt), ResolveSize(ovr.BioSize, scaleCaptionPt)
	photoLabelSize := ResolveSize(ovr.PhotoLabelSize, scaleSubheadPt)
	sizes := []float64{ResolveSize(ovr.NameSize, scaleSubheadPt)}
	if ovr.NameSize == 0 {
		sizes = append(sizes, teamBiosMinNamePt)
	}
	// The smallest headshot square that still writes its initials unshrunk.
	photoMin := teamBiosPhotoMinPt
	for _, m := range v.Members {
		if m.Photo != nil {
			continue
		}
		label := strings.TrimSpace(m.PhotoLabel)
		if label == "" {
			label = deriveInitials(m.Name)
		}
		text := buildTeamBiosCenteredText(label, photoLabelSize, true, "accent1")
		for photoMin < math.Min(colW, areaH) && writtenNeedOrOverflowPt(ctx.Theme.BodyFont, text, photoMin) > photoMin {
			photoMin += 4
		}
	}
	measure := func(nameSize float64) teamBiosFitResult {
		cardRows := (len(v.Members) + teamBiosMaxPerRow - 1) / teamBiosMaxPerRow
		k := float64(max(cardRows, 1))
		// The grid gap also separates each headshot row from its text row.
		gaps := (2*k - 1) * ctx.Gap(teamBiosGapPt)
		r := teamBiosFitResult{nameSize: nameSize, areaH: areaH, gapsPt: gaps, nominalPt: (areaH - gaps) / k}
		text := 0.0
		for start := 0; start < len(v.Members); start += teamBiosMaxPerRow {
			need := 0.0
			for _, m := range v.Members[start:min(start+teamBiosMaxPerRow, len(v.Members))] {
				need = math.Max(need, writtenNeedOrOverflowPt(ctx.Theme.BodyFont, buildTeamBiosTextContent(m, nameSize, roleSize, bioSize, "accent1"), colW))
			}
			r.textPt = append(r.textPt, need)
			text += need
		}
		r.photoPt = math.Floor(math.Min(0.4*r.nominalPt, (areaH-gaps-text)/k))
		r.fits = r.photoPt >= photoMin
		r.totalPt = text + gaps + k*math.Max(r.photoPt, photoMin)
		return r
	}
	for _, size := range sizes {
		if r := measure(size); r.fits {
			return r
		}
	}
	// Nothing fits: keep the first size, so an overflowing payload is not
	// also set smaller before it is shrunk.
	return measure(sizes[0])
}

// teamBiosMinNamePt is the floor the default 14pt name steps down to;
// teamBiosPhotoMinPt the smallest headshot square the search starts from.
const (
	teamBiosMinNamePt  = 12.0
	teamBiosPhotoMinPt = 44.0
)

// floorTeamBiosTextRows keeps every text row at or above the written fit of
// its tallest member: sized by the theme-font model and a 60% share alone,
// two-row teams were written at 68-96% autofit on the shorter content areas
// (go-slide-creator-n1muf). When the text needs more than its share, the
// headshot rows give way in points. A payload that does not fit even then
// keeps the proportional rows and is reported by PostExpandWarnings.
func floorTeamBiosTextRows(grid *jsonschema.ShapeGridInput, fit teamBiosFitResult) {
	if !fit.fits || len(grid.Rows) != 2*len(fit.textPt) {
		return
	}
	// The text rows share 60% of the area: capped rows take their caps,
	// proportional rows 60% of a card row each.
	textShare := 0.6*fit.areaH - fit.gapsPt
	capped := grid.VerticalAlign != ""
	holds, total := true, 0.0
	for r := 1; r < len(grid.Rows); r += 2 {
		need := fit.textPt[r/2]
		if capped {
			total += math.Max(grid.Rows[r].MaxHeight, need)
		} else {
			holds = holds && need <= 0.6*fit.nominalPt
		}
	}
	if holds && total <= textShare {
		// Lift any cap the writer needs more than the model gave it.
		for r := 1; capped && r < len(grid.Rows); r += 2 {
			need := fit.textPt[r/2]
			grid.Rows[r].MinHeight = need
			grid.Rows[r].MaxHeight = math.Max(grid.Rows[r].MaxHeight, need)
		}
		return
	}
	for r := 0; r+1 < len(grid.Rows); r += 2 {
		need := fit.textPt[r/2]
		grid.Rows[r].Height, grid.Rows[r].Flex = 0, 0
		grid.Rows[r].MinHeight, grid.Rows[r].MaxHeight = fit.photoPt, fit.photoPt
		grid.Rows[r+1].Height, grid.Rows[r+1].Flex = 0, 1
		grid.Rows[r+1].MinHeight, grid.Rows[r+1].MaxHeight = need, need
	}
	grid.VerticalAlign = GridVerticalAlignDefault
}

// capTeamBiosTextRows sizes each card-row's text row to its tallest member
// text instead of 60% of the card height, and centres the block. Short bios
// used to sit at the top of a tall empty text band, so the slide read
// top-heavy with ~2in of blank space below the cards (VERTICAL_IMBALANCE,
// go-slide-creator-u9xfy). The photo row keeps its nominal share.
func capTeamBiosTextRows(ctx ExpandContext, grid *jsonschema.ShapeGridInput, columns int) {
	const (
		padPt      = 6.0
		photoShare = 40.0 // percent of a card-row
	)
	w, h := contentAreaPt(ctx)
	cardRows := len(grid.Rows) / 2
	if cardRows == 0 || columns == 0 {
		return
	}
	textW := equalColumnWidthPt(w, columns, grid.Gap) - 2*defaultShapeInsetLRPt
	nominalCard := h / float64(cardRows)
	capped := false
	for r := 1; r < len(grid.Rows); r += 2 {
		tallest := 0.0
		for _, c := range grid.Rows[r].Cells {
			if c != nil && c.Shape != nil && len(c.Shape.Text) > 0 {
				tallest = math.Max(tallest, shapeTextHeightPt(ctx.Theme.BodyFont, c.Shape.Text, textW))
			}
		}
		// The row must hold the text plus the shape's own 0.5 cm top and
		// bottom insets; sizing it to the text alone left the frame too
		// short and autofit shrank the name below the readable floor.
		need := math.Ceil(tallest + 2*defaultShapeInsetTBPt + 2*padPt)
		if tallest <= 0 || need >= nominalCard*0.6 {
			continue
		}
		grid.Rows[r].MaxHeight = need
		capped = true
	}
	if !capped {
		return
	}
	// Row Height values are ratios that only a stretching grid normalises; a
	// centred block reads them as percentages. Pin each photo row to its 40%
	// share of a card-row and let each text row flex up to its cap.
	for r := 0; r+1 < len(grid.Rows); r += 2 {
		grid.Rows[r].Height = photoShare / float64(cardRows)
		grid.Rows[r+1].Height = 0
		grid.Rows[r+1].Flex = 1
		if grid.Rows[r+1].MaxHeight == 0 {
			grid.Rows[r+1].MaxHeight = math.Floor(nominalCard * 0.6)
		}
	}
	grid.VerticalAlign = GridVerticalAlignDefault
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

// buildTeamBiosPhotoCell is the shared headshot primitive: a real headshot
// cover-cropped into a circle (go-slide-creator-hdpq), else the initials disc.
// Both are square ("contain") so faces are not cut into letterboxes inside a
// wide card column.
func buildTeamBiosPhotoCell(m TeamBiosMember, disc fillTone, ink string, labelSize float64) *jsonschema.GridCellInput {
	label := strings.TrimSpace(m.PhotoLabel)
	if label == "" {
		label = deriveInitials(m.Name)
	}
	return headshotCell(m.Photo, teamBiosPhotoAlt(m), label, disc, ink, labelSize)
}

// teamBiosInitialsSize scales the initials with the disc the way
// contact-directory does (30% of its diameter) so a large disc does not
// carry small initials; an explicit photo_label_size wins, the 14pt default
// is the floor, and a longer custom photo_label (over 3 characters) keeps
// the default so it is not set larger than the disc holds.
func teamBiosInitialsSize(ctx ExpandContext, members []TeamBiosMember, fit teamBiosFitResult, columns int, override, base float64) float64 {
	if override != 0 {
		return base
	}
	for _, m := range members {
		if m.Photo == nil && runeLen(strings.TrimSpace(m.PhotoLabel)) > 3 {
			return base
		}
	}
	areaW, _ := sizingAreaPt(ctx)
	diameter := fit.photoPt
	if colW := equalColumnWidthPt(areaW, columns, ctx.Gap(teamBiosGapPt)); colW > 0 && colW < diameter {
		diameter = colW
	}
	return math.Max(base, math.Round(diameter*0.3))
}

// teamBiosPhotoAlt describes a headshot for a reader who cannot see it: the
// person and what they do, which is the whole informational content of the
// picture on this slide.
func teamBiosPhotoAlt(m TeamBiosMember) string {
	name := strings.TrimSpace(m.Name)
	role := strings.TrimSpace(m.Role)
	switch {
	case name != "" && role != "":
		return name + ", " + role
	case name != "":
		return name
	case role != "":
		return role
	default:
		return "Team member photo"
	}
}

func buildTeamBiosTextCell(m TeamBiosMember, accent string, nameSize, roleSize, bioSize float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     buildTeamBiosTextContent(m, nameSize, roleSize, bioSize, accent),
		},
	}
}

func buildTeamBiosEmptyCell() *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
		},
	}
}

// ---------------------------------------------------------------------------
// Text content builders
// ---------------------------------------------------------------------------

type teamBiosParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type teamBiosTextObj struct {
	Paragraphs    []teamBiosParagraph `json:"paragraphs"`
	Align         string              `json:"align"`
	VerticalAlign string              `json:"vertical_align"`
}

func buildTeamBiosCenteredText(label string, size float64, bold bool, color string) json.RawMessage {
	obj := teamBiosTextObj{
		Paragraphs: []teamBiosParagraph{
			{Content: label, Size: size, Bold: bold, Color: color, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(obj)
	return data
}

func buildTeamBiosTextContent(m TeamBiosMember, nameSize, roleSize, bioSize float64, accent string) json.RawMessage {
	paras := []teamBiosParagraph{
		{Content: m.Name, Size: nameSize, Bold: true, Color: "dk1", Align: "ctr"},
		{Content: m.Role, Size: roleSize, Color: accent, Align: "ctr"},
	}
	if strings.TrimSpace(m.Bio) != "" {
		paras = append(paras, teamBiosParagraph{
			Content: m.Bio, Size: bioSize, Color: "dk2", Align: "ctr",
		})
	}
	// Name, role and bio centre under the centred headshot so each column
	// has one axis, not a centred disc over a left-aligned text block
	// (go-slide-creator-q4fut, go-slide-creator-wd6p6).
	obj := teamBiosTextObj{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: "t",
	}
	data, _ := json.Marshal(obj)
	return data
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// deriveInitials returns up to two uppercase initials from name. Falls back to
// "?" when name has no letters. Examples: "Jane Smith" → "JS",
// "María García-López" → "MG", "Madonna" → "M".
func deriveInitials(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-'
	})
	var initials []rune
	for _, p := range parts {
		for _, r := range p {
			if unicode.IsLetter(r) {
				initials = append(initials, unicode.ToUpper(r))
				break
			}
		}
		if len(initials) >= 2 {
			break
		}
	}
	if len(initials) == 0 {
		return "?"
	}
	return string(initials)
}
