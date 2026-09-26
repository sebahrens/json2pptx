package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
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

// Measured with TestTeamBiosBudgetProbe across all four bundled templates at
// default sizes. A fifth member adds a second card row and halves text height.
func teamBiosReadableBioBudget(members int) int {
	if members > 4 {
		return 141
	}
	return teamBiosBioMaxChars
}

func (t *teamBios) Schema() *Schema {
	memberSchema := ObjectSchema(
		map[string]*Schema{
			"name":        StringSchema(teamBiosNameMaxChars).WithDescription("Person's full name (rendered bold)"),
			"role":        StringSchema(teamBiosRoleMaxChars).WithDescription("Role or title (rendered in accent color)"),
			"bio":         StringSchema(teamBiosBioMaxChars).WithDescription("Short bio. Approximate readable limit: 220 characters with 1-4 members, 141 with 5-8; longer bios emit BODY_TOO_LONG."),
			"photo":       PhotoSchema("Headshot, cover-cropped to a centered square photo frame; omit it to draw a square initials tile (alt defaults to the member's name and role)", teamBiosPhotoAltMaxChars),
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
func (t *teamBios) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*TeamBiosValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	budget := teamBiosReadableBioBudget(len(v.Members))
	for i, m := range v.Members {
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
	nameSize := ResolveSize(ovr.NameSize, 14.0)
	roleSize := ResolveSize(ovr.RoleSize, 11.0)
	bioSize := ResolveSize(ovr.BioSize, 10.0)
	photoLabelSize := ResolveSize(ovr.PhotoLabelSize, 14.0)

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
				photoCells[col] = buildTeamBiosPhotoCell(m, accent, photoLabelSize)
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
		Gap:     10,
		RowGap:  0,
		Rows:    rows,
	}
	return grid, nil
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

func buildTeamBiosPhotoCell(m TeamBiosMember, accent string, photoLabelSize float64) *jsonschema.GridCellInput {
	// A real headshot renders as a shape_grid image cell, which the generator
	// cover-crops to the frame; the initials tile is the fallback, not the only
	// option it used to be (go-slide-creator-hdpq).
	if cell := patternPhotoCell(m.Photo, teamBiosPhotoAlt(m)); cell != nil {
		// Images normally cover-crop the full grid cell. A headshot needs a
		// square frame inside its wide card column so faces are not cut into
		// letterboxes; explicit contain also bypasses default-fill expansion.
		cell.Fit = "contain"
		return cell
	}
	label := strings.TrimSpace(m.PhotoLabel)
	if label == "" {
		label = deriveInitials(m.Name)
	}
	return &jsonschema.GridCellInput{
		Fit: "contain",
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(`"lt2"`),
			Text:     buildTeamBiosCenteredText(label, photoLabelSize, true, accent),
		},
	}
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
		{Content: m.Name, Size: nameSize, Bold: true, Color: "dk1", Align: "l"},
		{Content: m.Role, Size: roleSize, Color: accent, Align: "l"},
	}
	if strings.TrimSpace(m.Bio) != "" {
		paras = append(paras, teamBiosParagraph{
			Content: m.Bio, Size: bioSize, Color: "dk2", Align: "l",
		})
	}
	obj := teamBiosTextObj{
		Paragraphs:    paras,
		Align:         "l",
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
