package template

import (
	"encoding/xml"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	layoutpkg "github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// ConformanceStatus indicates the outcome of an individual conformance check.
type ConformanceStatus string

const (
	ConformanceStatusPass ConformanceStatus = "pass"
	ConformanceStatusFail ConformanceStatus = "fail"
	ConformanceStatusWarn ConformanceStatus = "warn"
)

// ConformanceCheck is a single conformance check result.
type ConformanceCheck struct {
	Category string            `json:"category"`
	Check    string            `json:"check"`
	Status   ConformanceStatus `json:"status"`
	Detail   string            `json:"detail,omitempty"`
}

// ConformanceReport is the aggregated result of running every conformance
// check against a template.
type ConformanceReport struct {
	Template string             `json:"template"`
	SHA256   string             `json:"sha256,omitempty"`
	Pass     bool               `json:"pass"`
	Checks   []ConformanceCheck `json:"checks"`
}

// FailCount returns the number of checks with status fail.
func (r *ConformanceReport) FailCount() int {
	n := 0
	for _, c := range r.Checks {
		if c.Status == ConformanceStatusFail {
			n++
		}
	}
	return n
}

// WarnCount returns the number of checks with status warn.
func (r *ConformanceReport) WarnCount() int {
	n := 0
	for _, c := range r.Checks {
		if c.Status == ConformanceStatusWarn {
			n++
		}
	}
	return n
}

// MandatoryLayout describes the requirements for one of the mandatory layout
// roles every template must provide.
type MandatoryLayout struct {
	Role         string   // canonical role name (matches ClassifyCanonicalRole)
	Names        []string // accepted layout names (case-insensitive)
	Tags         []string // alternative tag set; any single match is sufficient
	Placeholders []string // required placeholder types: "title", "subtitle", "body"
	MinBody      int      // minimum number of body-type placeholders
}

// MandatoryLayouts is the canonical list of mandatory layout roles enforced by
// template-check.
var MandatoryLayouts = []MandatoryLayout{
	{
		Role:         CanonicalRoleTitleSlide,
		Names:        []string{"title slide"},
		Tags:         []string{"title-slide"},
		Placeholders: []string{"title", "subtitle"},
	},
	{
		Role:         CanonicalRoleOneContent,
		Names:        []string{"one content", "content"},
		Tags:         []string{"content"},
		Placeholders: []string{"title", "body"},
	},
	{
		Role:         CanonicalRoleTwoContent,
		Names:        []string{"two content", "comparison"},
		Tags:         []string{"two-column"},
		Placeholders: []string{"title"},
		MinBody:      2,
	},
	{
		Role:         CanonicalRoleSectionDivider,
		Names:        []string{"section divider", "section header"},
		Tags:         []string{"section-header"},
		Placeholders: []string{"title"},
	},
	{
		Role:  CanonicalRoleBlank,
		Names: []string{"blank"},
		Tags:  []string{"blank"},
	},
	{
		Role:         CanonicalRoleBlankTitle,
		Names:        []string{"blank + title", "blank layout"},
		Tags:         []string{"blank-title"},
		Placeholders: []string{"title"},
	},
	{
		Role:         CanonicalRoleClosing,
		Names:        []string{"closing", "thank you", "end slide"},
		Tags:         []string{"closing"},
		Placeholders: []string{"title"},
	},
}

// CheckConformance opens the template at path, runs every conformance check,
// and returns a structured report. Callers that only need the binary
// pass/fail signal can inspect report.Pass; callers that need to drive
// CI gates or repair pipelines can iterate report.Checks.
func CheckConformance(path string) (*ConformanceReport, error) {
	reader, err := OpenTemplate(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()
	return CheckConformanceReader(reader, filepath.Base(path))
}

// CheckConformanceReader runs every conformance check against an already
// opened template. name is the display name stamped on the report.
func CheckConformanceReader(reader *Reader, name string) (*ConformanceReport, error) {
	layouts, err := ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layouts: %w", err)
	}

	slideW, slideH, sizeChecks := checkSlideSize(reader)

	var checks []ConformanceCheck
	checks = append(checks, sizeChecks...)
	checks = append(checks, checkMandatoryLayouts(layouts)...)
	checks = append(checks, checkLayoutNameMismatches(layouts)...)
	checks = append(checks, checkDuplicateLayoutSignatures(layouts)...)
	checks = append(checks, checkSectionNumber(layouts)...)
	checks = append(checks, checkPlaceholdersOnCanvas(layouts, slideW, slideH)...)
	checks = append(checks, checkFooterChromeCompleteness(layouts))
	checks = append(checks, checkConflictingLayoutTags(layouts)...)
	checks = append(checks, checkTextPlaceholderOverlap(layouts)...)
	checks = append(checks, checkImageTitleLayerOrder(layouts)...)
	checks = append(checks, checkContentTitleGeometry(layouts)...)
	checks = append(checks, checkContentTitleBodyHierarchy(layouts)...)
	staticText, err := checkLayoutStaticText(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to check layout text: %w", err)
	}
	checks = append(checks, staticText...)
	themeChecks, themeOK := checkThemeSource(reader)
	checks = append(checks, themeChecks...)
	if themeOK {
		checks = append(checks, checkTheme(ParseTheme(reader))...)
	}
	checks = append(checks, checkMetadataColors(reader)...)

	pass := true
	for _, c := range checks {
		if c.Status == ConformanceStatusFail {
			pass = false
			break
		}
	}

	return &ConformanceReport{
		Template: name,
		SHA256:   reader.Hash(),
		Pass:     pass,
		Checks:   checks,
	}, nil
}

// checkContentTitleBodyHierarchy flags content layouts where the first-level
// body type is at least as large as the title. Unknown font sizes are not
// evidence of an inversion, and utility/divider layouts are out of scope.
func checkContentTitleBodyHierarchy(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck
	for i := range layouts {
		layout := &layouts[i]
		role, _, _ := ClassifyCanonicalRole(layout)
		if role != CanonicalRoleOneContent && role != CanonicalRoleTwoContent && !isContentLayoutName(layout.Name) {
			continue
		}
		var titleSize, bodySize int
		for _, ph := range layout.Placeholders {
			if IsDisclosurePlaceholder(ph) || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
				continue
			}
			switch ph.Type {
			case types.PlaceholderTitle:
				if ph.FontSize > 0 && (titleSize == 0 || ph.FontSize < titleSize) {
					titleSize = ph.FontSize
				}
			case types.PlaceholderBody, types.PlaceholderContent:
				if ph.FontSize > bodySize {
					bodySize = ph.FontSize
				}
			}
		}
		if titleSize == 0 || bodySize == 0 || bodySize < titleSize {
			continue
		}
		checks = append(checks, ConformanceCheck{
			Category: "typography", Check: "Content title larger than body", Status: ConformanceStatusWarn,
			Detail: fmt.Sprintf("Layout %q (%s): title %.0fpt, body %.0fpt; increase title size or reduce first-level body size",
				layout.Name, layout.ID, float64(titleSize)/100, float64(bodySize)/100),
		})
	}
	return checks
}

// isContentLayoutName reports whether a layout is named as a mandatory One or
// Two Content layout. An oversized body font can stop the classifier from
// recognising the role, which is exactly when the hierarchy check matters
// (go-slide-creator-csclk.37).
func isContentLayoutName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, ml := range MandatoryLayouts {
		if ml.Role != CanonicalRoleOneContent && ml.Role != CanonicalRoleTwoContent {
			continue
		}
		for _, n := range ml.Names {
			if lower == n {
				return true
			}
		}
	}
	return false
}

// checkFooterChromeCompleteness catches templates whose partial utility
// placeholder set would otherwise make footer rendering depend on a fallback.
// A layout with no footer slots is intentional and is not flagged.
func checkFooterChromeCompleteness(layouts []types.LayoutMetadata) ConformanceCheck {
	var incomplete []string
	for _, layout := range layouts {
		present := make(map[string]bool, len(layout.FooterRegions))
		for _, region := range layout.FooterRegions {
			present[region.Type] = true
		}
		if len(present) == 0 {
			continue
		}
		var missing []string
		for _, typ := range footerChromeTypes {
			if !present[typ] {
				missing = append(missing, typ)
			}
		}
		if len(missing) > 0 {
			incomplete = append(incomplete, fmt.Sprintf("%s missing %s", layout.Name, strings.Join(missing, ", ")))
		}
	}
	if len(incomplete) > 0 {
		return ConformanceCheck{
			Category: "chrome", Check: "Footer placeholder set complete or absent", Status: ConformanceStatusWarn,
			Detail: strings.Join(incomplete, "; "),
		}
	}
	return ConformanceCheck{
		Category: "chrome", Check: "Footer placeholder set complete or absent", Status: ConformanceStatusPass,
		Detail: "Every layout either has all dt/ftr/sldNum footer slots or none",
	}
}

// generatorLayoutIDs maps the mandatory roles every deck depends on to the
// canonical layout_id generation resolves them through. A layout that only
// shares a tag with the role (Two Content carries "content", Closing carries
// "title-slide") is not what generation picks, so template-check holds these
// roles to the generator's own resolution (go-slide-creator-csclk.32).
var generatorLayoutIDs = map[string]string{
	CanonicalRoleTitleSlide: "title",
	CanonicalRoleOneContent: "content",
}

// checkMandatoryLayouts verifies all mandatory layouts are present with
// required placeholders.
func checkMandatoryLayouts(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck
	resolved := layoutpkg.ResolveAllCanonicalLayouts(layouts)

	for _, ml := range MandatoryLayouts {
		found, layout := findMandatoryLayout(layouts, ml)

		if layoutID, ok := generatorLayoutIDs[ml.Role]; ok {
			id, resolvable := resolved[layoutID]
			if !resolvable {
				checks = append(checks, ConformanceCheck{
					Category: "layout",
					Check:    fmt.Sprintf("Mandatory layout: %s", ml.Role),
					Status:   ConformanceStatusFail,
					Detail:   unresolvableRoleDetail(ml, layoutID, found, layout),
				})
				continue
			}
			if l := FindLayout(layouts, id); l != nil {
				found, layout = true, *l
			}
		}

		if !found {
			checks = append(checks, ConformanceCheck{
				Category: "layout",
				Check:    fmt.Sprintf("Mandatory layout: %s", ml.Role),
				Status:   ConformanceStatusFail,
				Detail:   fmt.Sprintf("No layout found matching names %v or tags %v", ml.Names, ml.Tags),
			})
			continue
		}

		checks = append(checks, ConformanceCheck{
			Category: "layout",
			Check:    fmt.Sprintf("Mandatory layout: %s", ml.Role),
			Status:   ConformanceStatusPass,
			Detail:   fmt.Sprintf("Found: %q (ID: %s)", layout.Name, layout.ID),
		})

		checks = append(checks, checkPlaceholders(ml, layout)...)
	}

	return checks
}

// unresolvableRoleDetail explains why generation cannot resolve layoutID for a
// mandatory role. When a layout was matched by name, the detail names the
// reason its classification tag is missing: an oversized first-level body font
// leaves too little text capacity to count as a content body
// (go-slide-creator-csclk.37).
func unresolvableRoleDetail(ml MandatoryLayout, layoutID string, found bool, layout types.LayoutMetadata) string {
	base := fmt.Sprintf("layout_id %q does not resolve to any layout, so %s slides cannot generate", layoutID, ml.Role)
	if !found {
		return base + fmt.Sprintf("; no layout matches names %v or tags %v", ml.Names, ml.Tags)
	}
	detail := fmt.Sprintf("%s; %q (ID: %s) must carry tag %v and none of %v (has %v)",
		base, layout.Name, layout.ID, ml.Tags, excludedTagsFor(layoutID), layout.Tags)
	if ml.Role != CanonicalRoleOneContent {
		return detail
	}
	var titleSize int
	for _, ph := range layout.Placeholders {
		if ph.Type == types.PlaceholderTitle && ph.FontSize > 0 {
			titleSize = ph.FontSize
		}
	}
	for _, ph := range layout.Placeholders {
		if IsDisclosurePlaceholder(ph) || (ph.Type != types.PlaceholderBody && ph.Type != types.PlaceholderContent) {
			continue
		}
		if ph.MaxChars > 0 && ph.MaxChars < minUsableBodyChars {
			detail += fmt.Sprintf("; body placeholder %q holds only ~%d chars (< %d) at %.0fpt",
				ph.ID, ph.MaxChars, minUsableBodyChars, float64(ph.FontSize)/100)
			if titleSize > 0 {
				detail += fmt.Sprintf(" (title %.0fpt)", float64(titleSize)/100)
			}
			detail += "; reduce the first-level body font size"
			break
		}
	}
	return detail
}

// excludedTagsFor lists the tags that disqualify a layout from layoutID in the
// generator's canonical resolution (internal/layout canonicalNames).
func excludedTagsFor(layoutID string) []string {
	switch layoutID {
	case "title":
		return []string{"blank-title", "closing"}
	case "content":
		return []string{"two-column", "section-header"}
	}
	return nil
}

// findMandatoryLayout searches for a mandatory layout by name, tag, or
// canonical-role classification. See the original template_check.go comment
// for the rationale behind the three-tier fallback.
func findMandatoryLayout(layouts []types.LayoutMetadata, ml MandatoryLayout) (bool, types.LayoutMetadata) {
	for _, l := range layouts {
		lowerName := strings.ToLower(l.Name)
		for _, name := range ml.Names {
			if lowerName == name {
				return true, l
			}
		}
	}

	for _, l := range layouts {
		for _, reqTag := range ml.Tags {
			for _, lt := range l.Tags {
				if lt == reqTag {
					return true, l
				}
			}
		}
	}

	if canonical := mandatoryRoleFor(ml); canonical != "" {
		for i := range layouts {
			l := &layouts[i]
			role, _, conf := ClassifyCanonicalRole(l)
			if role == canonical && conf >= CanonicalConfidenceThreshold {
				return true, *l
			}
		}
	}

	return false, types.LayoutMetadata{}
}

// mandatoryRoleFor maps a MandatoryLayout entry to the canonical role name
// used by ClassifyCanonicalRole.
func mandatoryRoleFor(ml MandatoryLayout) string {
	switch ml.Role {
	case CanonicalRoleTitleSlide,
		CanonicalRoleOneContent,
		CanonicalRoleTwoContent,
		CanonicalRoleSectionDivider,
		CanonicalRoleBlank,
		CanonicalRoleBlankTitle,
		CanonicalRoleClosing:
		return ml.Role
	}
	return ""
}

// checkPlaceholders verifies a layout has the required placeholders.
func checkPlaceholders(ml MandatoryLayout, layout types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck

	for _, reqType := range ml.Placeholders {
		found := false
		for _, ph := range layout.Placeholders {
			if matchesPlaceholderRequirement(ph, reqType) {
				found = true
				break
			}
		}

		status := ConformanceStatusPass
		detail := ""
		if !found {
			status = ConformanceStatusFail
			detail = fmt.Sprintf("Layout %q missing required %q placeholder", layout.Name, reqType)
		}

		checks = append(checks, ConformanceCheck{
			Category: "placeholder",
			Check:    fmt.Sprintf("%s: has %s placeholder", ml.Role, reqType),
			Status:   status,
			Detail:   detail,
		})
	}

	if ml.MinBody > 0 {
		bodyCount := 0
		for _, ph := range layout.Placeholders {
			if !IsDisclosurePlaceholder(ph) && (ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent) {
				bodyCount++
			}
		}

		status := ConformanceStatusPass
		detail := ""
		if bodyCount < ml.MinBody {
			status = ConformanceStatusFail
			detail = fmt.Sprintf("Layout %q has %d body placeholder(s), need %d", layout.Name, bodyCount, ml.MinBody)
		}

		checks = append(checks, ConformanceCheck{
			Category: "placeholder",
			Check:    fmt.Sprintf("%s: minimum %d body placeholders", ml.Role, ml.MinBody),
			Status:   status,
			Detail:   detail,
		})
	}

	return checks
}

// matchesPlaceholderRequirement checks if a placeholder matches a requirement
// string.
func matchesPlaceholderRequirement(ph types.PlaceholderInfo, req string) bool {
	if IsDisclosurePlaceholder(ph) {
		return false
	}
	switch req {
	case "title":
		return ph.Type == types.PlaceholderTitle
	case "subtitle":
		return ph.Type == types.PlaceholderSubtitle
	case "body":
		return ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent
	default:
		return false
	}
}

// checkLayoutNameMismatches walks every layout and emits a WARN when the
// layout is structurally a canonical role but is named something else.
func checkLayoutNameMismatches(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck

	for i := range layouts {
		l := &layouts[i]
		role, sig, conf := ClassifyCanonicalRole(l)
		if role == "" || conf < CanonicalConfidenceThreshold {
			continue
		}
		if IsCanonicalLayoutName(role, l.Name) {
			continue
		}

		canonical := CanonicalNameFor(role)
		checks = append(checks, ConformanceCheck{
			Category: "layout",
			Check:    fmt.Sprintf("Layout name matches canonical role: %s", role),
			Status:   ConformanceStatusWarn,
			Detail: fmt.Sprintf(
				"rename suggested: %q → %q (structurally a %s; signature=%s, confidence=%.2f)",
				l.Name, canonical, role, sig, conf,
			),
		})
	}

	return checks
}

// checkDuplicateLayoutSignatures flags layouts that share both canonical role
// AND structural signature.
func checkDuplicateLayoutSignatures(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck

	type bucket struct {
		role  string
		sig   string
		names []string
	}
	buckets := make(map[string]*bucket)
	order := make([]string, 0)
	for i := range layouts {
		l := &layouts[i]
		role, sig, conf := ClassifyCanonicalRole(l)
		if role == "" || conf < CanonicalConfidenceThreshold {
			continue
		}
		if sig == "" || sig == "blank" {
			continue
		}
		key := role + "|" + sig
		b, ok := buckets[key]
		if !ok {
			b = &bucket{role: role, sig: sig}
			buckets[key] = b
			order = append(order, key)
		}
		b.names = append(b.names, l.Name)
	}

	for _, key := range order {
		b := buckets[key]
		if len(b.names) < 2 {
			continue
		}
		checks = append(checks, ConformanceCheck{
			Category: "layout",
			Check:    "Duplicate layout signature",
			Status:   ConformanceStatusWarn,
			Detail: fmt.Sprintf(
				"%d layouts share role %s + signature %s: %v — consider removing duplicates and renaming the canonical one",
				len(b.names), b.role, b.sig, b.names,
			),
		})
	}

	return checks
}

// checkSectionNumber verifies the Section Number placeholder on section
// divider layouts.
func checkSectionNumber(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck

	var sectionLayout *types.LayoutMetadata
	for i, l := range layouts {
		for _, tag := range l.Tags {
			if tag == "section-header" {
				sectionLayout = &layouts[i]
				break
			}
		}
		if sectionLayout != nil {
			break
		}
	}

	if sectionLayout == nil {
		return nil
	}

	var snPh *types.PlaceholderInfo
	for i, ph := range sectionLayout.Placeholders {
		if strings.EqualFold(ph.ID, "section number") {
			snPh = &sectionLayout.Placeholders[i]
			break
		}
	}

	if snPh == nil {
		checks = append(checks, ConformanceCheck{
			Category: "placeholder",
			Check:    "Section Divider: has Section Number placeholder",
			Status:   ConformanceStatusFail,
			Detail:   fmt.Sprintf("Layout %q has no placeholder named \"Section Number\"", sectionLayout.Name),
		})
		return checks
	}

	checks = append(checks, ConformanceCheck{
		Category: "placeholder",
		Check:    "Section Divider: has Section Number placeholder",
		Status:   ConformanceStatusPass,
	})

	const minWidthEMU int64 = 2743200
	if snPh.Bounds.Width > 0 && snPh.Bounds.Width < minWidthEMU {
		checks = append(checks, ConformanceCheck{
			Category: "typography",
			Check:    "Section Number: minimum width (3 inches)",
			Status:   ConformanceStatusWarn,
			Detail:   fmt.Sprintf("Width is %.1f inches (need ≥3.0)", float64(snPh.Bounds.Width)/914400.0),
		})
	} else if snPh.Bounds.Width > 0 {
		checks = append(checks, ConformanceCheck{
			Category: "typography",
			Check:    "Section Number: minimum width (3 inches)",
			Status:   ConformanceStatusPass,
		})
	}

	return checks
}

// OOXML ST_SlideSizeCoordinate bounds for p:sldSz cx/cy.
const (
	minSlideSizeEMU int64 = 914400
	maxSlideSizeEMU int64 = 51206400
)

// checkSlideSize fails a missing or out-of-range p:sldSz, which the generator
// would otherwise copy verbatim into every output deck
// (go-slide-creator-csclk.39). It returns the slide size for the canvas checks,
// or zeros when the size is unusable.
func checkSlideSize(reader *Reader) (int64, int64, []ConformanceCheck) {
	const name = "Slide size: p:sldSz present and within OOXML range"
	w, h := ParseSlideDimensions(reader)
	if w == 0 && h == 0 {
		return 0, 0, []ConformanceCheck{{Category: "slide", Check: name, Status: ConformanceStatusFail,
			Detail: "ppt/presentation.xml declares no usable <p:sldSz>"}}
	}
	if w < minSlideSizeEMU || w > maxSlideSizeEMU || h < minSlideSizeEMU || h > maxSlideSizeEMU {
		return 0, 0, []ConformanceCheck{{Category: "slide", Check: name, Status: ConformanceStatusFail,
			Detail: fmt.Sprintf("sldSz cx=%d cy=%d; each must be %d..%d EMU", w, h, minSlideSizeEMU, maxSlideSizeEMU)}}
	}
	return w, h, []ConformanceCheck{{Category: "slide", Check: name, Status: ConformanceStatusPass,
		Detail: fmt.Sprintf("%.2fin x %.2fin", float64(w)/914400, float64(h)/914400)}}
}

// checkPlaceholdersOnCanvas warns when layout placeholders extend past the
// right or bottom slide edge, e.g. 16:9 master geometry in a 4:3 deck
// (go-slide-creator-csclk.39). Placeholders parked at negative coordinates are
// intentionally off-slide and are not judged; a 1% tolerance absorbs rounding.
func checkPlaceholdersOnCanvas(layouts []types.LayoutMetadata, slideW, slideH int64) []ConformanceCheck {
	if slideW <= 0 || slideH <= 0 {
		return nil
	}
	const name = "Placeholders within slide canvas"
	var off []string
	for _, l := range layouts {
		for _, ph := range l.Placeholders {
			b := ph.Bounds
			if b.Width <= 0 || b.Height <= 0 || b.X < 0 || b.Y < 0 {
				continue
			}
			if b.X+b.Width > slideW+slideW/100 || b.Y+b.Height > slideH+slideH/100 {
				off = append(off, fmt.Sprintf("%s/%s", l.Name, ph.ID))
			}
		}
	}
	if len(off) == 0 {
		return []ConformanceCheck{{Category: "slide", Check: name, Status: ConformanceStatusPass}}
	}
	return []ConformanceCheck{{Category: "slide", Check: name, Status: ConformanceStatusWarn,
		Detail: fmt.Sprintf("%d placeholder(s) extend past the %.2fin x %.2fin slide: %s",
			len(off), float64(slideW)/914400, float64(slideH)/914400, strings.Join(off, ", "))}}
}

// checkThemeSource validates the theme part itself: the slide master's theme
// relationship must resolve, the theme must parse, both font scheme typefaces
// must be present, and every scheme colour value must be six hex digits.
// ParseTheme silently substitutes defaults for all of these, so checks on its
// result could not fail (go-slide-creator-csclk.33, -.34). The bool reports
// whether a theme was read, so callers skip the parsed-theme checks otherwise.
func checkThemeSource(reader *Reader) ([]ConformanceCheck, bool) {
	var checks []ConformanceCheck
	masters, _ := reader.ListFiles("ppt/slideMasters/slideMaster*.xml")
	sort.Strings(masters)
	var dangling []string
	for _, master := range masters {
		relsPath := filepath.ToSlash(filepath.Join(filepath.Dir(master), "_rels", filepath.Base(master)+".rels"))
		data, err := reader.ReadFile(relsPath)
		if err != nil {
			continue
		}
		var rels pptx.RelationshipsXML
		if xml.Unmarshal(data, &rels) != nil {
			continue
		}
		for _, rel := range rels.Relationships {
			if rel.Type != pptx.RelTypeTheme {
				continue
			}
			target := ResolveRelativePath(filepath.Dir(master), rel.Target)
			if !reader.hasFile(target) {
				dangling = append(dangling, fmt.Sprintf("%s -> %s", master, target))
			}
		}
	}
	if len(dangling) > 0 {
		checks = append(checks, ConformanceCheck{Category: "theme", Check: "Theme: slide master theme relationship resolves", Status: ConformanceStatusFail,
			Detail: "dangling theme relationship: " + strings.Join(dangling, "; ")})
	}

	themeFiles, _ := reader.ListFiles("ppt/theme/theme*.xml")
	if len(themeFiles) == 0 {
		return append(checks, ConformanceCheck{Category: "theme", Check: "Theme: theme part present", Status: ConformanceStatusFail,
			Detail: "the package has no ppt/theme/theme*.xml"}), false
	}
	data, err := reader.ReadFile(themeFiles[0])
	var theme pptx.ThemeXML
	if err == nil {
		err = xml.Unmarshal(data, &theme)
	}
	if err != nil {
		return append(checks, ConformanceCheck{Category: "theme", Check: "Theme: theme part parses", Status: ConformanceStatusFail,
			Detail: fmt.Sprintf("%s: %v", themeFiles[0], err)}), false
	}

	fonts := theme.ThemeElements.FontScheme
	for _, f := range []struct{ check, typeface string }{
		{"Theme: major font (titles) defined", fonts.MajorFont.Latin.Typeface},
		{"Theme: minor font (body) defined", fonts.MinorFont.Latin.Typeface},
	} {
		c := ConformanceCheck{Category: "theme", Check: f.check, Status: ConformanceStatusPass, Detail: f.typeface}
		if strings.TrimSpace(f.typeface) == "" {
			c.Status = ConformanceStatusFail
			c.Detail = fmt.Sprintf("%s has no a:fontScheme latin typeface", themeFiles[0])
		}
		checks = append(checks, c)
	}

	var invalid []string
	for _, slot := range colorSlots {
		def := slot.getter(theme.ThemeElements.ColorScheme)
		raw := def.SRGBColor.Val
		if raw == "" {
			raw = def.SystemColor.LastClr
		}
		if raw != "" && !isHex6(strings.TrimSpace(raw)) {
			invalid = append(invalid, fmt.Sprintf("%s=%q", slot.name, raw))
		}
	}
	c := ConformanceCheck{Category: "theme", Check: "Theme: scheme colors are 6-digit hex", Status: ConformanceStatusPass}
	if len(invalid) > 0 {
		c.Status = ConformanceStatusFail
		c.Detail = "invalid color value(s): " + strings.Join(invalid, ", ")
	}
	checks = append(checks, c)
	return checks, true
}

// isHex6 reports whether s is exactly six hexadecimal digits.
func isHex6(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

// surfaceTintRoles are the four roles TEMPLATE_SPEC requires in surface_tints.
var surfaceTintRoles = []string{"subtle", "paper", "elevated", "inverse"}

// validMetadataColor accepts an OOXML scheme color name or a 6-digit hex
// value (optionally '#'-prefixed), the two forms fills resolve.
func validMetadataColor(v string) bool {
	return pptx.IsSchemeColor(v) || isHex6(strings.TrimPrefix(v, "#"))
}

// checkMetadataColors validates the embedded metadata's surface_tints and
// data_palette values, which are otherwise written verbatim into slide XML
// (go-slide-creator-csclk.36). Templates without metadata are not judged.
func checkMetadataColors(reader *Reader) []ConformanceCheck {
	md, err := ParseMetadata(reader)
	if err != nil || md == nil || (len(md.SurfaceTints) == 0 && len(md.DataPalette) == 0) {
		return nil
	}
	var invalid, missing []string
	for _, role := range surfaceTintRoles {
		v, ok := md.SurfaceTints[role]
		switch {
		case !ok && len(md.SurfaceTints) > 0:
			missing = append(missing, role)
		case ok && !validMetadataColor(v):
			invalid = append(invalid, fmt.Sprintf("surface_tints.%s=%q", role, v))
		}
	}
	// data_palette resolves only through the theme's scheme slots; any other
	// entry (hex included) is silently dropped from chart palettes.
	themeSlots := make(map[string]bool, len(colorSlots))
	for _, slot := range colorSlots {
		themeSlots[slot.name] = true
	}
	for i, v := range md.DataPalette {
		if !themeSlots[v] {
			invalid = append(invalid, fmt.Sprintf("data_palette[%d]=%q", i, v))
		}
	}
	valid := ConformanceCheck{Category: "metadata", Check: "Metadata: surface_tints and data_palette colors valid", Status: ConformanceStatusPass}
	if len(invalid) > 0 {
		valid.Status = ConformanceStatusFail
		valid.Detail = "surface_tints values must be scheme color names or 6-digit hex and data_palette entries theme slot names (dk1..accent6, hlink, folHlink): " +
			strings.Join(invalid, ", ")
	}
	checks := []ConformanceCheck{valid}
	if len(missing) > 0 {
		checks = append(checks, ConformanceCheck{Category: "metadata", Check: "Metadata: surface_tints defines all four roles", Status: ConformanceStatusWarn,
			Detail: "missing role(s): " + strings.Join(missing, ", ")})
	}
	return checks
}

// checkTheme verifies theme requirements.
func checkTheme(theme types.ThemeInfo) []ConformanceCheck {
	var checks []ConformanceCheck

	requiredColors := []string{"dk1", "dk2", "lt1", "lt2",
		"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
		"hlink", "folHlink"}

	colorMap := make(map[string]string, len(theme.Colors))
	for _, c := range theme.Colors {
		colorMap[c.Name] = c.RGB
	}

	var missingColors []string
	for _, name := range requiredColors {
		if _, ok := colorMap[name]; !ok {
			missingColors = append(missingColors, name)
		}
	}

	if len(missingColors) > 0 {
		checks = append(checks, ConformanceCheck{
			Category: "theme",
			Check:    "Theme: all 12 scheme colors defined",
			Status:   ConformanceStatusFail,
			Detail:   fmt.Sprintf("Missing: %s", strings.Join(missingColors, ", ")),
		})
	} else {
		checks = append(checks, ConformanceCheck{
			Category: "theme",
			Check:    "Theme: all 12 scheme colors defined",
			Status:   ConformanceStatusPass,
		})
	}

	// Fonts are checked on the raw theme XML by checkThemeSource: ParseTheme
	// defaults a missing or empty typeface to Calibri, so a check on the
	// parsed value could never fail (go-slide-creator-csclk.34).

	checks = append(checks, checkColorPolarity(colorMap)...)
	checks = append(checks, checkAccentVisibility(colorMap)...)

	return checks
}

// checkAccentVisibility warns when the first chart/data accent nearly
// disappears on the template's light canvas. It is a warning rather than a
// failure because accent1 can still be used intentionally on dark surfaces.
func checkAccentVisibility(colorMap map[string]string) []ConformanceCheck {
	const checkName = "Theme: accent1 visible on lt1 (contrast >= 2:1)"
	accentHex, accentOK := colorMap["accent1"]
	backgroundHex, backgroundOK := colorMap["lt1"]
	if !accentOK || !backgroundOK {
		return nil // missing scheme slots are reported by checkTheme
	}
	accent, accentErr := svggen.ParseColor(accentHex)
	background, backgroundErr := svggen.ParseColor(backgroundHex)
	if accentErr != nil || backgroundErr != nil {
		return nil
	}
	ratio := accent.ContrastWith(background)
	if ratio < 2 {
		return []ConformanceCheck{{
			Category: "theme", Check: checkName, Status: ConformanceStatusWarn,
			Detail: fmt.Sprintf("accent1 %s has only %.2f:1 contrast on lt1 %s; chart series and fills may be hard to see", accentHex, ratio, backgroundHex),
		}}
	}
	return []ConformanceCheck{{Category: "theme", Check: checkName, Status: ConformanceStatusPass}}
}

// checkColorPolarity verifies dark colors are dark and light colors are light.
func checkColorPolarity(colorMap map[string]string) []ConformanceCheck {
	var checks []ConformanceCheck

	for _, name := range []string{"dk1", "dk2"} {
		rgb, ok := colorMap[name]
		if !ok {
			continue
		}
		lum := hexLuminance(rgb)
		if lum > 0.5 {
			checks = append(checks, ConformanceCheck{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is dark (luminance < 50%%)", name),
				Status:   ConformanceStatusWarn,
				Detail:   fmt.Sprintf("%s has luminance %.0f%% — expected dark", rgb, lum*100),
			})
		} else {
			checks = append(checks, ConformanceCheck{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is dark (luminance < 50%%)", name),
				Status:   ConformanceStatusPass,
			})
		}
	}

	for _, name := range []string{"lt1", "lt2"} {
		rgb, ok := colorMap[name]
		if !ok {
			continue
		}
		lum := hexLuminance(rgb)
		if lum < 0.5 {
			checks = append(checks, ConformanceCheck{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is light (luminance > 50%%)", name),
				Status:   ConformanceStatusWarn,
				Detail:   fmt.Sprintf("%s has luminance %.0f%% — expected light", rgb, lum*100),
			})
		} else {
			checks = append(checks, ConformanceCheck{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is light (luminance > 50%%)", name),
				Status:   ConformanceStatusPass,
			})
		}
	}

	return checks
}

// hexLuminance computes relative luminance from a hex RGB string.
func hexLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0
	}

	r := hexByte(hex[0:2])
	g := hexByte(hex[2:4])
	b := hexByte(hex[4:6])

	rl := sRGBToLinear(float64(r) / 255.0)
	gl := sRGBToLinear(float64(g) / 255.0)
	bl := sRGBToLinear(float64(b) / 255.0)

	return 0.2126*rl + 0.7152*gl + 0.0722*bl
}

// hexByte converts a 2-character hex string to a byte.
func hexByte(s string) uint8 {
	var v uint8
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= uint8(c - '0')
		case c >= 'a' && c <= 'f':
			v |= uint8(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			v |= uint8(c - 'A' + 10)
		}
	}
	return v
}

// sRGBToLinear converts an sRGB channel value to linear.
func sRGBToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
