package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// dual-org-ladder pattern — two parallel org columns (joint-venture team slides)
// ---------------------------------------------------------------------------
//
// Two columns of paired role cards with an org-name header above each column.
// Unlike a hierarchical org chart, the rows carry no parent/child semantics —
// they line up matching roles between two organisations (e.g. client sponsor
// paired with consulting partner). An optional thin accent connector renders
// between the paired cards on each body row.

func init() {
	Default().Register(&dualOrgLadder{})
}

type dualOrgLadder struct{}

func (d *dualOrgLadder) Name() string { return "dual-org-ladder" }
func (d *dualOrgLadder) Description() string {
	return "Two parallel org columns with org-name headers and 2–6 paired role cards (engagement-team / joint-venture slides)"
}
func (d *dualOrgLadder) UseWhen() string {
	return "Engagement-team, joint-venture, or paired-organisation slide showing 2–6 matched roles across two orgs side by side; prefer team-bios for a single org's team page, comparison-2col when the columns are pros/cons rather than role pairs, and an svggen org_chart diagram when the structure is hierarchical (parent/child)"
}
func (d *dualOrgLadder) NotWhen() string {
	return "Single-org team page (use team-bios), hierarchical org chart with reporting lines (use svggen org_chart), pros-and-cons or option comparison (use comparison-2col), or more than 6 paired rows (split across slides)"
}
func (d *dualOrgLadder) Version() int      { return 1 }
func (d *dualOrgLadder) CellsHint() string { return "2-6 rows" }
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
// AccentA defaults to accent1 (left header fill); AccentB defaults to accent2
// (right header fill). The shared connector line uses AccentA when drawn.
type DualOrgLadderOverrides struct {
	AccentA    string  `json:"accent_a,omitempty"`
	AccentB    string  `json:"accent_b,omitempty"`
	OrgSize    float64 `json:"org_size,omitempty"`
	NameSize   float64 `json:"name_size,omitempty"`
	TitleSize  float64 `json:"title_size,omitempty"`
}

// DualOrgLadderCellOverride is the shared per-cell override, indexed by row
// (the header row is index 0; body rows are indices 1..N).
type DualOrgLadderCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	dualOrgLadderMinRows       = 2
	dualOrgLadderMaxRows       = 6
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

func (d *dualOrgLadder) Schema() *Schema {
	rowSchema := ObjectSchema(
		map[string]*Schema{
			"a_name":  StringSchema(dualOrgLadderNameMaxChars).WithDescription("Name of the org A member on this row (rendered bold)"),
			"a_title": StringSchema(dualOrgLadderTitleMaxChars).WithDescription("Role / title of the org A member"),
			"b_name":  StringSchema(dualOrgLadderNameMaxChars).WithDescription("Name of the org B member on this row (rendered bold)"),
			"b_title": StringSchema(dualOrgLadderTitleMaxChars).WithDescription("Role / title of the org B member"),
		},
		[]string{"a_name", "a_title", "b_name", "b_title"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"org_a":           StringSchema(dualOrgLadderOrgMaxChars).WithDescription("Name of organisation A (rendered in the left header)"),
			"org_b":           StringSchema(dualOrgLadderOrgMaxChars).WithDescription("Name of organisation B (rendered in the right header)"),
			"rows":            ArraySchema(rowSchema, dualOrgLadderMinRows, dualOrgLadderMaxRows).WithDescription("2–6 paired role rows; each row aligns one org A member with one org B member"),
			"show_connectors": BooleanSchema().WithDescription("When true (default), draw a thin accent connector line between the paired cards on every body row").WithDefault(true),
		},
		[]string{"org_a", "org_b", "rows"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent_a":   StringSchema(0).WithDescription("Scheme color for the left org header and the shared connector (default accent1)").WithDefault("accent1"),
			"accent_b":   StringSchema(0).WithDescription("Scheme color for the right org header (default accent2)").WithDefault("accent2"),
			"org_size":   NumberSchema(6, 40).WithDescription("Font size for the org-name headers in points (default 14)"),
			"name_size":  NumberSchema(6, 40).WithDescription("Font size for member names in points (default 12)"),
			"title_size": NumberSchema(6, 40).WithDescription("Font size for member titles in points (default 10)"),
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
	}).WithDescription("Two parallel org columns: org-name headers above 2–6 paired role rows. Use for engagement-team / joint-venture slides where matched roles align horizontally without reporting hierarchy.")
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
	} else if len(v.OrgA) > dualOrgLadderOrgMaxChars {
		errs = append(errs, errMaxLength(name, "org_a", dualOrgLadderOrgMaxChars, len(v.OrgA)))
	}
	if strings.TrimSpace(v.OrgB) == "" {
		errs = append(errs, errRequired(name, "org_b"))
	} else if len(v.OrgB) > dualOrgLadderOrgMaxChars {
		errs = append(errs, errMaxLength(name, "org_b", dualOrgLadderOrgMaxChars, len(v.OrgB)))
	}

	if len(v.Rows) < dualOrgLadderMinRows {
		errs = append(errs, errMinItems(name, "rows", dualOrgLadderMinRows, len(v.Rows), "(hint: use team-bios for a single-org team page, comparison-2col for non-role pairs)"))
	}
	if len(v.Rows) > dualOrgLadderMaxRows {
		errs = append(errs, errMaxItems(name, "rows", dualOrgLadderMaxRows, len(v.Rows), "(hint: split the team across two slides — each slide supports up to 6 paired rows)"))
	}

	for i, row := range v.Rows {
		if strings.TrimSpace(row.ANameField) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].a_name", i)))
		} else if len(row.ANameField) > dualOrgLadderNameMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].a_name", i), dualOrgLadderNameMaxChars, len(row.ANameField)))
		}
		if strings.TrimSpace(row.ATitle) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].a_title", i)))
		} else if len(row.ATitle) > dualOrgLadderTitleMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].a_title", i), dualOrgLadderTitleMaxChars, len(row.ATitle)))
		}
		if strings.TrimSpace(row.BNameField) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].b_name", i)))
		} else if len(row.BNameField) > dualOrgLadderNameMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].b_name", i), dualOrgLadderNameMaxChars, len(row.BNameField)))
		}
		if strings.TrimSpace(row.BTitle) == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("rows[%d].b_title", i)))
		} else if len(row.BTitle) > dualOrgLadderTitleMaxChars {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("rows[%d].b_title", i), dualOrgLadderTitleMaxChars, len(row.BTitle)))
		}
	}

	// Cell overrides are keyed by emitted grid row (0 = header, 1..N = body rows).
	totalGridRows := len(v.Rows) + 1
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalGridRows, "(keys: 0 = header row, 1..N = body rows)"); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (d *dualOrgLadder) Expand(_ ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
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
		accentA = "accent1"
	}
	accentB := ovr.AccentB
	if accentB == "" {
		accentB = "accent2"
	}
	orgSize := ResolveSize(ovr.OrgSize, 14.0)
	nameSize := ResolveSize(ovr.NameSize, 12.0)
	titleSize := ResolveSize(ovr.TitleSize, 10.0)

	showConnectors := true
	if v.ShowConnectors != nil {
		showConnectors = *v.ShowConnectors
	}

	headerRow := jsonschema.GridRowInput{
		Height: 18, // header is shorter than body rows
		Cells: []*jsonschema.GridCellInput{
			buildDualOrgHeaderCell(v.OrgA, accentA, orgSize),
			buildDualOrgHeaderCell(v.OrgB, accentB, orgSize),
		},
	}
	if co, ok := cellOverrides[0]; ok {
		if cellOvr, ok2 := co.(*DualOrgLadderCellOverride); ok2 && cellOvr.AccentBar {
			headerRow.Cells[0].AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accentA, Width: 3}
		}
	}

	bodyRows := make([]jsonschema.GridRowInput, len(v.Rows))
	for i, row := range v.Rows {
		cells := []*jsonschema.GridCellInput{
			buildDualOrgRoleCell(row.ANameField, row.ATitle, nameSize, titleSize),
			buildDualOrgRoleCell(row.BNameField, row.BTitle, nameSize, titleSize),
		}
		gridRow := jsonschema.GridRowInput{Cells: cells}
		if showConnectors {
			gridRow.Connector = &jsonschema.ConnectorSpecInput{
				Style: "line",
				Color: accentA,
				Width: 1,
			}
		}
		// Cell override key i+1 (header is index 0).
		if co, ok := cellOverrides[i+1]; ok {
			if cellOvr, ok2 := co.(*DualOrgLadderCellOverride); ok2 && cellOvr.AccentBar {
				gridRow.Cells[0].AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accentA, Width: 3}
				gridRow.Cells[1].AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accentB, Width: 3}
			}
		}
		bodyRows[i] = gridRow
	}

	rows := make([]jsonschema.GridRowInput, 0, len(bodyRows)+1)
	rows = append(rows, headerRow)
	rows = append(rows, bodyRows...)

	colsJSON, _ := json.Marshal(2)
	return &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  24, // visible gap between the two columns
		RowGap:  6,
		Rows:    rows,
	}, nil
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

func buildDualOrgHeaderCell(orgName, accent string, size float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     buildDualOrgHeaderText(orgName, size),
		},
	}
}

func buildDualOrgRoleCell(memberName, title string, nameSize, titleSize float64) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     json.RawMessage(`{"color": "dk2", "width": 0.75}`),
			Text:     buildDualOrgRoleText(memberName, title, nameSize, titleSize),
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
