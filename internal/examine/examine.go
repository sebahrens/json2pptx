// Package examine implements the reusable template-examination service behind
// the `json2pptx examine-template` CLI subcommand. It takes a user-provided
// PPTX template and produces a single, machine- and human-readable report
// describing what the template supports: its theme, slide dimensions, every
// slide layout with canonical layout/placeholder roles, font-aware character
// budgets, exact placeholder bounds, z-order, derived content zones, the
// four-family canonical coverage, derivable higher-level layouts, and a
// FindingEnvelope folding every diagnostic (including TPL.LAYOUT.MISSING_ROLE
// for an absent canonical family).
//
// The service is read-only: it reports facts, preview geometry, and
// remediation suggestions only — it never mutates the template. It is consumed
// by the CLI (which materialises the report into a directory of artifacts) and
// is structured so the report-building core (BuildReport) is testable from
// synthetic layouts without a PPTX on disk.
package examine

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// emuPerInch is the OOXML English Metric Units per inch (914400 EMU = 1 inch).
const emuPerInch = 914400.0

// Default 16:9 slide dimensions in EMU, used when presentation.xml omits sldSz.
const (
	defaultSlideWidthEMU  = 12192000
	defaultSlideHeightEMU = 6858000
)

// CodeLayoutMissingRole is the legacy finding code emitted when a template lacks
// one of the four content-bearing canonical layout families. The shared
// envelope adapter namespaces it to "TPL.LAYOUT.MISSING_ROLE" via
// diagnostics.ClassifyCode (the "layout." prefix routes it to the TPL
// namespace).
const CodeLayoutMissingRole = "LAYOUT.MISSING_ROLE"

// Subcommand is the surface name stamped onto the report's FindingEnvelope.
const Subcommand = "examine-template"

// canonicalFamilies is the ordered set of content-bearing canonical layout
// families every usable template should provide. A template missing any of
// these gets a TPL.LAYOUT.MISSING_ROLE finding and present=false in
// canonical_coverage. Utility families (blank, blank+title) are intentionally
// excluded — they are not required for a deck to be authorable.
var canonicalFamilies = []types.CanonicalLayoutFamily{
	types.LayoutFamilyTitleSlide,
	types.LayoutFamilySectionDivider,
	types.LayoutFamilyOneContent,
	types.LayoutFamilyQAClosing,
}

// Options configures an examination run.
type Options struct {
	// TemplatePath is the on-disk path of the template, used for the display
	// name. The SHA-256 is read from the reader.
	TemplatePath string
	// Strict fails metadata validation on warnings, not just errors.
	Strict bool
}

// Report is the complete examination result. It carries the structural facts
// (theme, slide dimensions, masters, layouts) plus the agent-facing diagnostic
// surface (Findings). The CLI materialises this into report.json and the
// per-layout artifact tree.
//
// Findings is a nested FindingEnvelope (under the "findings" key), matching the
// established validate-template shape: report.json itself carries extra
// structural fields the envelope schema forbids at its top level, so the
// envelope lives one level down where it validates against
// docs/api/finding-envelope.schema.json.
type Report struct {
	Template          string                       `json:"template"`
	SHA256            string                       `json:"sha256,omitempty"`
	AspectRatio       string                       `json:"aspect_ratio"`
	Slide             SlideDimensions              `json:"slide"`
	Theme             ThemeReport                  `json:"theme"`
	Masters           []MasterReport               `json:"masters"`
	CanonicalCoverage map[string]CanonicalCoverage `json:"canonical_coverage"`
	DerivableLayouts  []DerivableLayoutReport      `json:"derivable_layouts"`
	Layouts           []LayoutReport               `json:"layouts"`
	Findings          diagnostics.FindingEnvelope  `json:"findings"`
}

// SlideDimensions reports the slide canvas size in both EMU and inches.
type SlideDimensions struct {
	WidthEMU  int64   `json:"width_emu"`
	HeightEMU int64   `json:"height_emu"`
	WidthIn   float64 `json:"width_in"`
	HeightIn  float64 `json:"height_in"`
}

// ThemeReport is the theme section of the report.
type ThemeReport struct {
	Name      string            `json:"name"`
	TitleFont string            `json:"title_font"`
	BodyFont  string            `json:"body_font"`
	Colors    map[string]string `json:"colors"`
}

// MasterReport names a slide master and where its raw XML lives in the package.
type MasterReport struct {
	Name    string `json:"name"`
	XMLPath string `json:"xml_path"`
}

// CanonicalCoverage records whether a canonical layout family is present and
// which layouts provide it.
type CanonicalCoverage struct {
	Family  string   `json:"family"`
	Present bool     `json:"present"`
	Layouts []string `json:"layouts"`
}

// DerivableLayoutReport mirrors template.DerivableLayout for serialization.
type DerivableLayoutReport struct {
	Name    string   `json:"name"`
	Ready   bool     `json:"ready"`
	Missing []string `json:"missing,omitempty"`
}

// LayoutReport describes a single layout: its canonical classification, the
// asset base name shared by its rendered/annotated/raw artifacts, its derived
// content zone, and every placeholder with role, font-aware budget, exact
// bounds, and z-order.
type LayoutReport struct {
	Index               int                 `json:"index"`
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Tags                []string            `json:"tags"`
	CanonicalType       string              `json:"canonical_type"`
	CanonicalFamily     string              `json:"canonical_family"`
	CanonicalConfidence float64             `json:"canonical_confidence"`
	AssetBase           string              `json:"asset_base"`
	XMLPath             string              `json:"xml_path"`
	ContentZone         ZoneReport          `json:"content_zone"`
	Placeholders        []PlaceholderReport `json:"placeholders"`
}

// ZoneReport is the derived safe content area (title-bottom, footer-top,
// side-margins) in EMU.
type ZoneReport struct {
	LeftEMU   int64 `json:"left_emu"`
	TopEMU    int64 `json:"top_emu"`
	RightEMU  int64 `json:"right_emu"`
	BottomEMU int64 `json:"bottom_emu"`
}

// PlaceholderReport describes one placeholder. ZIndex is the document order of
// the placeholder in the layout shape tree (later = drawn on top). FontPt and
// MaxChars are the font-aware budget the engine uses for fit decisions.
type PlaceholderReport struct {
	ID             string       `json:"id"`
	Type           string       `json:"type"`
	Role           string       `json:"role"`
	RoleConfidence float64      `json:"role_confidence"`
	Index          int          `json:"index"`
	ZIndex         int          `json:"z_index"`
	FontPt         float64      `json:"font_pt"`
	MaxChars       int          `json:"max_chars"`
	Bounds         BoundsReport `json:"bounds"`
}

// BoundsReport carries a placeholder's rectangle in both EMU and inches.
type BoundsReport struct {
	XEMU int64   `json:"x_emu"`
	YEMU int64   `json:"y_emu"`
	WEMU int64   `json:"w_emu"`
	HEMU int64   `json:"h_emu"`
	XIn  float64 `json:"x_in"`
	YIn  float64 `json:"y_in"`
	WIn  float64 `json:"w_in"`
	HIn  float64 `json:"h_in"`
}

// Inputs is the decoupled input to BuildReport, so the report-building logic is
// testable from synthetic layouts without a PPTX reader.
type Inputs struct {
	Template       string
	SHA256         string
	AspectRatio    string
	SlideWidthEMU  int64
	SlideHeightEMU int64
	Theme          types.ThemeInfo
	Layouts        []types.LayoutMetadata
	Masters        []MasterReport
	// MetadataDiagnostics are diagnostics produced by metadata validation,
	// folded into the report's findings alongside canonical-coverage findings.
	MetadataDiagnostics []diagnostics.Diagnostic
}

// Examine parses a template via the reader and builds the full report.
func Examine(reader *template.Reader, opts Options) (*Report, error) {
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("parse layouts: %w", err)
	}
	theme := template.ParseTheme(reader)
	vr := template.ValidateTemplateMetadata(reader, opts.Strict)
	template.ApplyMetadataHints(layouts, vr.Metadata)

	w, h := template.ParseSlideDimensions(reader)
	if w == 0 || h == 0 {
		w, h = defaultSlideWidthEMU, defaultSlideHeightEMU
	}

	aspect := "16:9"
	if vr.Metadata != nil && vr.Metadata.AspectRatio != "" {
		aspect = vr.Metadata.AspectRatio
	}

	masters := collectMasters(reader)

	return BuildReport(Inputs{
		Template:            displayName(opts.TemplatePath),
		SHA256:              reader.Hash(),
		AspectRatio:         aspect,
		SlideWidthEMU:       w,
		SlideHeightEMU:      h,
		Theme:               theme,
		Layouts:             layouts,
		Masters:             masters,
		MetadataDiagnostics: vr.Diagnostics,
	}), nil
}

// BuildReport assembles a Report from already-parsed inputs. It is the pure
// core shared by Examine and the unit tests.
func BuildReport(in Inputs) *Report {
	w, h := in.SlideWidthEMU, in.SlideHeightEMU
	if w == 0 || h == 0 {
		w, h = defaultSlideWidthEMU, defaultSlideHeightEMU
	}

	layoutReports := make([]LayoutReport, len(in.Layouts))
	for i := range in.Layouts {
		layoutReports[i] = buildLayoutReport(&in.Layouts[i], w, h)
	}

	coverage, coverageDiags := buildCanonicalCoverage(in.Layouts)

	diags := make([]diagnostics.Diagnostic, 0, len(in.MetadataDiagnostics)+len(coverageDiags))
	diags = append(diags, in.MetadataDiagnostics...)
	diags = append(diags, coverageDiags...)
	sort.SliceStable(diags, func(i, j int) bool {
		return severityRank(diags[i].Severity) < severityRank(diags[j].Severity)
	})

	env := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  Subcommand,
		Template:    in.Template,
		InputSHA256: in.SHA256,
	}, diags)

	return &Report{
		Template:    in.Template,
		SHA256:      in.SHA256,
		AspectRatio: in.AspectRatio,
		Slide: SlideDimensions{
			WidthEMU:  w,
			HeightEMU: h,
			WidthIn:   inches(w),
			HeightIn:  inches(h),
		},
		Theme:             buildThemeReport(in.Theme),
		Masters:           in.Masters,
		CanonicalCoverage: coverage,
		DerivableLayouts:  buildDerivable(in.Layouts),
		Layouts:           layoutReports,
		Findings:          env,
	}
}

// buildCanonicalCoverage groups layouts by canonical family and emits a
// TPL.LAYOUT.MISSING_ROLE finding for each absent content-bearing family.
func buildCanonicalCoverage(layouts []types.LayoutMetadata) (map[string]CanonicalCoverage, []diagnostics.Diagnostic) {
	byFamily := make(map[types.CanonicalLayoutFamily][]string)
	for i := range layouts {
		fam := template.EffectiveCanonicalType(&layouts[i]).Family()
		byFamily[fam] = append(byFamily[fam], layouts[i].Name)
	}

	coverage := make(map[string]CanonicalCoverage, len(canonicalFamilies))
	var diags []diagnostics.Diagnostic
	for _, fam := range canonicalFamilies {
		names := byFamily[fam]
		present := len(names) > 0
		coverage[string(fam)] = CanonicalCoverage{
			Family:  string(fam),
			Present: present,
			Layouts: names,
		}
		if !present {
			diags = append(diags, diagnostics.Diagnostic{
				Code:     CodeLayoutMissingRole,
				Severity: diagnostics.SeverityWarning,
				Message: fmt.Sprintf(
					"Template has no %s layout. Decks that need a %s slide cannot resolve a native layout; "+
						"add a layout in this family or rely on synthesis where supported.",
					familyLabel(fam), familyLabel(fam),
				),
				Details: map[string]any{
					"family":               string(fam),
					"expected_layout_type": string(canonicalTypeForFamily(fam)),
				},
			})
		}
	}
	return coverage, diags
}

// buildLayoutReport projects a parsed layout into a LayoutReport, including its
// canonical classification, asset base, derived content zone, and placeholders
// in z-order.
func buildLayoutReport(l *types.LayoutMetadata, slideW, slideH int64) LayoutReport {
	ct := template.EffectiveCanonicalType(l)
	phs := make([]PlaceholderReport, len(l.Placeholders))
	for i := range l.Placeholders {
		phs[i] = buildPlaceholderReport(&l.Placeholders[i], i)
	}

	tags := l.Tags
	if tags == nil {
		tags = []string{}
	}

	return LayoutReport{
		Index:               l.Index,
		ID:                  l.ID,
		Name:                l.Name,
		Tags:                tags,
		CanonicalType:       string(ct),
		CanonicalFamily:     string(ct.Family()),
		CanonicalConfidence: l.CanonicalConfidence,
		AssetBase:           assetBase(l.ID, ct),
		XMLPath:             "ppt/slideLayouts/" + l.ID + ".xml",
		ContentZone:         computeZone(phs, slideW, slideH),
		Placeholders:        phs,
	}
}

// buildPlaceholderReport projects a placeholder, carrying its document order as
// z-index and converting EMU bounds to inches.
func buildPlaceholderReport(ph *types.PlaceholderInfo, zIndex int) PlaceholderReport {
	return PlaceholderReport{
		ID:             ph.ID,
		Type:           string(ph.Type),
		Role:           string(ph.Role),
		RoleConfidence: ph.RoleConfidence,
		Index:          ph.Index,
		ZIndex:         zIndex,
		FontPt:         round2(float64(ph.FontSize) / 100.0),
		MaxChars:       ph.MaxChars,
		Bounds: BoundsReport{
			XEMU: ph.Bounds.X,
			YEMU: ph.Bounds.Y,
			WEMU: ph.Bounds.Width,
			HEMU: ph.Bounds.Height,
			XIn:  inches(ph.Bounds.X),
			YIn:  inches(ph.Bounds.Y),
			WIn:  inches(ph.Bounds.Width),
			HIn:  inches(ph.Bounds.Height),
		},
	}
}

// computeZone derives the safe content area from a layout's placeholders: the
// top edge sits below the header band (title/eyebrow/subtitle), the bottom edge
// above any footer chrome, and the sides bracket the content placeholders.
// Falls back to symmetric 5% margins where a signal is absent.
func computeZone(phs []PlaceholderReport, slideW, slideH int64) ZoneReport {
	left := int64(float64(slideW) * 0.05)
	right := int64(float64(slideW) * 0.95)
	top := int64(float64(slideH) * 0.05)
	bottom := int64(float64(slideH) * 0.95)

	haveSide := false
	var sideLeft, sideRight int64
	for i := range phs {
		ph := &phs[i]
		b := ph.Bounds
		switch ph.Role {
		case string(types.PlaceholderRoleTitle),
			string(types.PlaceholderRoleEyebrow),
			string(types.PlaceholderRoleSubtitle):
			// Header band: push the content top below it, but only when the
			// header lives in the upper half (a section_number can be huge and
			// centered, so it is excluded above).
			if bot := b.YEMU + b.HEMU; bot > top && b.YEMU < slideH/2 {
				top = bot
			}
		case string(types.PlaceholderRoleFooter),
			string(types.PlaceholderRoleDate),
			string(types.PlaceholderRolePageNumber):
			if b.YEMU < bottom && b.YEMU > slideH/2 {
				bottom = b.YEMU
			}
		case string(types.PlaceholderRoleBody),
			string(types.PlaceholderRoleImage),
			string(types.PlaceholderRoleChart):
			if b.WEMU <= 0 {
				continue
			}
			if !haveSide || b.XEMU < sideLeft {
				sideLeft = b.XEMU
			}
			if r := b.XEMU + b.WEMU; !haveSide || r > sideRight {
				sideRight = r
			}
			haveSide = true
		}
	}
	if haveSide && sideLeft < sideRight {
		left, right = sideLeft, sideRight
	}
	if top >= bottom {
		top = int64(float64(slideH) * 0.05)
		bottom = int64(float64(slideH) * 0.95)
	}
	return ZoneReport{LeftEMU: left, TopEMU: top, RightEMU: right, BottomEMU: bottom}
}

// buildDerivable maps template.DerivableLayouts into the report shape.
func buildDerivable(layouts []types.LayoutMetadata) []DerivableLayoutReport {
	dls := template.DerivableLayouts(layouts)
	out := make([]DerivableLayoutReport, len(dls))
	for i, d := range dls {
		out[i] = DerivableLayoutReport{Name: d.Name, Ready: d.Ready, Missing: d.Missing}
	}
	return out
}

// buildThemeReport converts ThemeInfo into the serializable theme section.
func buildThemeReport(theme types.ThemeInfo) ThemeReport {
	colors := make(map[string]string, len(theme.Colors))
	for _, c := range theme.Colors {
		colors[c.Name] = c.RGB
	}
	return ThemeReport{
		Name:      theme.Name,
		TitleFont: theme.TitleFont,
		BodyFont:  theme.BodyFont,
		Colors:    colors,
	}
}

// collectMasters lists the slide masters in the template package.
func collectMasters(reader *template.Reader) []MasterReport {
	files, err := reader.ListFiles("ppt/slideMasters/slideMaster*.xml")
	if err != nil || len(files) == 0 {
		return []MasterReport{}
	}
	sort.Strings(files)
	out := make([]MasterReport, 0, len(files))
	for _, f := range files {
		name := trimExt(filepath.Base(f))
		out = append(out, MasterReport{Name: name, XMLPath: f})
	}
	return out
}

// assetBase returns the shared base name for a layout's artifacts, e.g.
// "slideLayout3__section-divider".
func assetBase(id string, ct types.CanonicalLayoutType) string {
	return id + "__" + canonicalSlug(ct)
}

// canonicalSlug maps a canonical layout type to a filesystem-friendly slug.
func canonicalSlug(ct types.CanonicalLayoutType) string {
	switch ct {
	case types.CanonicalLayoutTitleSlide:
		return "title-slide"
	case types.CanonicalLayoutOneContent:
		return "one-content"
	case types.CanonicalLayoutTwoContent:
		return "two-content"
	case types.CanonicalLayoutSectionDivider:
		return "section-divider"
	case types.CanonicalLayoutBlank:
		return "blank"
	case types.CanonicalLayoutBlankTitle:
		return "blank-title"
	case types.CanonicalLayoutClosing:
		return "closing"
	default:
		return "unknown"
	}
}

// canonicalTypeForFamily returns the representative canonical layout type for a
// family, used in finding evidence to name the expected layout.
func canonicalTypeForFamily(fam types.CanonicalLayoutFamily) types.CanonicalLayoutType {
	switch fam {
	case types.LayoutFamilyTitleSlide:
		return types.CanonicalLayoutTitleSlide
	case types.LayoutFamilySectionDivider:
		return types.CanonicalLayoutSectionDivider
	case types.LayoutFamilyOneContent:
		return types.CanonicalLayoutOneContent
	case types.LayoutFamilyQAClosing:
		return types.CanonicalLayoutClosing
	default:
		return types.CanonicalLayoutUnknown
	}
}

// familyLabel returns a human-readable label for a canonical family.
func familyLabel(fam types.CanonicalLayoutFamily) string {
	switch fam {
	case types.LayoutFamilyTitleSlide:
		return "title-slide (Title Slide)"
	case types.LayoutFamilySectionDivider:
		return "section-divider (Section Divider)"
	case types.LayoutFamilyOneContent:
		return "one-content (One/Two Content)"
	case types.LayoutFamilyQAClosing:
		return "qa-closing (Closing)"
	default:
		return string(fam)
	}
}

// severityRank orders diagnostics error-first for the findings array.
func severityRank(s diagnostics.Severity) int {
	switch s {
	case diagnostics.SeverityError:
		return 0
	case diagnostics.SeverityWarning:
		return 1
	default:
		return 2
	}
}

// displayName returns the base file name of a template path.
func displayName(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

// trimExt strips a file extension.
func trimExt(name string) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}

// inches converts EMU to inches, rounded to 3 decimals for stable wire output.
func inches(emu int64) float64 {
	return round3(float64(emu) / emuPerInch)
}

func round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
