package template

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
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

	layouts, err := ParseLayouts(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layouts: %w", err)
	}

	theme := ParseTheme(reader)

	var checks []ConformanceCheck
	checks = append(checks, checkMandatoryLayouts(layouts)...)
	checks = append(checks, checkLayoutNameMismatches(layouts)...)
	checks = append(checks, checkDuplicateLayoutSignatures(layouts)...)
	checks = append(checks, checkSectionNumber(layouts)...)
	checks = append(checks, checkTheme(theme)...)

	pass := true
	for _, c := range checks {
		if c.Status == ConformanceStatusFail {
			pass = false
			break
		}
	}

	return &ConformanceReport{
		Template: filepath.Base(path),
		SHA256:   reader.Hash(),
		Pass:     pass,
		Checks:   checks,
	}, nil
}

// checkMandatoryLayouts verifies all mandatory layouts are present with
// required placeholders.
func checkMandatoryLayouts(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck

	for _, ml := range MandatoryLayouts {
		found, layout := findMandatoryLayout(layouts, ml)

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
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
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
			Status:   ConformanceStatusWarn,
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

	if theme.TitleFont == "" {
		checks = append(checks, ConformanceCheck{
			Category: "theme",
			Check:    "Theme: major font (titles) defined",
			Status:   ConformanceStatusFail,
		})
	} else {
		checks = append(checks, ConformanceCheck{
			Category: "theme",
			Check:    "Theme: major font (titles) defined",
			Status:   ConformanceStatusPass,
			Detail:   theme.TitleFont,
		})
	}

	if theme.BodyFont == "" {
		checks = append(checks, ConformanceCheck{
			Category: "theme",
			Check:    "Theme: minor font (body) defined",
			Status:   ConformanceStatusFail,
		})
	} else {
		checks = append(checks, ConformanceCheck{
			Category: "theme",
			Check:    "Theme: minor font (body) defined",
			Status:   ConformanceStatusPass,
			Detail:   theme.BodyFont,
		})
	}

	checks = append(checks, checkColorPolarity(colorMap)...)

	return checks
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
