package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// templateCheckResult is the JSON-serializable output of template-check.
type templateCheckResult struct {
	Template string               `json:"template"`
	Pass     bool                 `json:"pass"`
	Checks   []templateCheckEntry `json:"checks"`
}

// templateCheckEntry is a single check result.
type templateCheckEntry struct {
	Category string `json:"category"` // "layout", "placeholder", "theme", "typography"
	Check    string `json:"check"`
	Status   string `json:"status"` // "pass", "fail", "warn"
	Detail   string `json:"detail,omitempty"`
}

// mandatoryLayout defines the requirements for a mandatory layout.
type mandatoryLayout struct {
	role         string   // human-readable role
	names        []string // accepted layout names (case-insensitive)
	tags         []string // at least one tag must be present (alternative to name matching)
	placeholders []string // required placeholder types: "title", "subtitle", "body"
	minBody      int      // minimum number of body-type placeholders
}

// mandatoryLayouts lists all mandatory layouts per the template spec.
var mandatoryLayouts = []mandatoryLayout{
	{
		role:         "Title Slide",
		names:        []string{"title slide"},
		tags:         []string{"title-slide"},
		placeholders: []string{"title", "subtitle"},
	},
	{
		role:         "One Content",
		names:        []string{"one content", "content"},
		tags:         []string{"content"},
		placeholders: []string{"title", "body"},
	},
	{
		role:         "Two Content",
		names:        []string{"two content", "comparison"},
		tags:         []string{"two-column"},
		placeholders: []string{"title"},
		minBody:      2,
	},
	{
		role:         "Section Divider",
		names:        []string{"section divider", "section header"},
		tags:         []string{"section-header"},
		placeholders: []string{"title"},
	},
	{
		role:  "Blank",
		names: []string{"blank"},
		tags:  []string{"blank"},
	},
	{
		role:         "Blank + Title",
		names:        []string{"blank + title", "blank layout"},
		tags:         []string{"blank-title"},
		placeholders: []string{"title"},
	},
	{
		role:         "Closing",
		names:        []string{"closing", "thank you", "end slide"},
		tags:         []string{"closing"},
		placeholders: []string{"title"},
	},
}

// runTemplateCheck implements the template-check subcommand.
func runTemplateCheck() error {
	fs := flag.NewFlagSet("template-check", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "output as JSON")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) != 1 {
		return fmt.Errorf("usage: json2pptx template-check [--json] <template.pptx>")
	}
	templatePath := args[0]

	// Open and parse template
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()

	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return fmt.Errorf("failed to parse layouts: %w", err)
	}

	theme := template.ParseTheme(reader)

	// Run all checks
	var checks []templateCheckEntry
	checks = append(checks, checkMandatoryLayouts(layouts)...)
	checks = append(checks, checkSectionNumber(layouts)...)
	checks = append(checks, checkTheme(theme)...)

	// Determine pass/fail
	pass := true
	for _, c := range checks {
		if c.Status == "fail" {
			pass = false
			break
		}
	}

	result := templateCheckResult{
		Template: filepath.Base(templatePath),
		Pass:     pass,
		Checks:   checks,
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printTemplateCheckText(result)

	if !pass {
		return fmt.Errorf("template conformance check failed")
	}
	return nil
}

// checkMandatoryLayouts verifies all mandatory layouts are present with required placeholders.
func checkMandatoryLayouts(layouts []types.LayoutMetadata) []templateCheckEntry {
	var checks []templateCheckEntry

	for _, ml := range mandatoryLayouts {
		found, layout := findMandatoryLayout(layouts, ml)

		if !found {
			checks = append(checks, templateCheckEntry{
				Category: "layout",
				Check:    fmt.Sprintf("Mandatory layout: %s", ml.role),
				Status:   "fail",
				Detail:   fmt.Sprintf("No layout found matching names %v or tags %v", ml.names, ml.tags),
			})
			continue
		}

		// Layout found — check is pass
		checks = append(checks, templateCheckEntry{
			Category: "layout",
			Check:    fmt.Sprintf("Mandatory layout: %s", ml.role),
			Status:   "pass",
			Detail:   fmt.Sprintf("Found: %q (ID: %s)", layout.Name, layout.ID),
		})

		// Check required placeholders
		checks = append(checks, checkPlaceholders(ml, layout)...)
	}

	return checks
}

// findMandatoryLayout searches for a mandatory layout by name or tag.
func findMandatoryLayout(layouts []types.LayoutMetadata, ml mandatoryLayout) (bool, types.LayoutMetadata) {
	// First try name match
	for _, l := range layouts {
		lowerName := strings.ToLower(l.Name)
		for _, name := range ml.names {
			if lowerName == name {
				return true, l
			}
		}
	}

	// Fall back to tag match
	for _, l := range layouts {
		for _, reqTag := range ml.tags {
			for _, lt := range l.Tags {
				if lt == reqTag {
					return true, l
				}
			}
		}
	}

	return false, types.LayoutMetadata{}
}

// checkPlaceholders verifies a layout has required placeholders.
func checkPlaceholders(ml mandatoryLayout, layout types.LayoutMetadata) []templateCheckEntry {
	var checks []templateCheckEntry

	for _, reqType := range ml.placeholders {
		found := false
		for _, ph := range layout.Placeholders {
			if matchesPlaceholderRequirement(ph, reqType) {
				found = true
				break
			}
		}

		status := "pass"
		detail := ""
		if !found {
			status = "fail"
			detail = fmt.Sprintf("Layout %q missing required %q placeholder", layout.Name, reqType)
		}

		checks = append(checks, templateCheckEntry{
			Category: "placeholder",
			Check:    fmt.Sprintf("%s: has %s placeholder", ml.role, reqType),
			Status:   status,
			Detail:   detail,
		})
	}

	// Check minimum body count
	if ml.minBody > 0 {
		bodyCount := 0
		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
				bodyCount++
			}
		}

		status := "pass"
		detail := ""
		if bodyCount < ml.minBody {
			status = "fail"
			detail = fmt.Sprintf("Layout %q has %d body placeholder(s), need %d", layout.Name, bodyCount, ml.minBody)
		}

		checks = append(checks, templateCheckEntry{
			Category: "placeholder",
			Check:    fmt.Sprintf("%s: minimum %d body placeholders", ml.role, ml.minBody),
			Status:   status,
			Detail:   detail,
		})
	}

	return checks
}

// matchesPlaceholderRequirement checks if a placeholder matches a requirement string.
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

// checkSectionNumber verifies the Section Number placeholder on section divider layouts.
func checkSectionNumber(layouts []types.LayoutMetadata) []templateCheckEntry {
	var checks []templateCheckEntry

	// Find section divider layout
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
		// Already covered by mandatory layout check
		return nil
	}

	// Find Section Number placeholder
	var snPh *types.PlaceholderInfo
	for i, ph := range sectionLayout.Placeholders {
		if strings.EqualFold(ph.ID, "section number") {
			snPh = &sectionLayout.Placeholders[i]
			break
		}
	}

	if snPh == nil {
		checks = append(checks, templateCheckEntry{
			Category: "placeholder",
			Check:    "Section Divider: has Section Number placeholder",
			Status:   "warn",
			Detail:   fmt.Sprintf("Layout %q has no placeholder named \"Section Number\"", sectionLayout.Name),
		})
		return checks
	}

	checks = append(checks, templateCheckEntry{
		Category: "placeholder",
		Check:    "Section Divider: has Section Number placeholder",
		Status:   "pass",
	})

	// Check width (minimum 3 inches = 2,743,200 EMU)
	const minWidthEMU int64 = 2743200
	if snPh.Bounds.Width > 0 && snPh.Bounds.Width < minWidthEMU {
		checks = append(checks, templateCheckEntry{
			Category: "typography",
			Check:    "Section Number: minimum width (3 inches)",
			Status:   "warn",
			Detail:   fmt.Sprintf("Width is %.1f inches (need ≥3.0)", float64(snPh.Bounds.Width)/914400.0),
		})
	} else if snPh.Bounds.Width > 0 {
		checks = append(checks, templateCheckEntry{
			Category: "typography",
			Check:    "Section Number: minimum width (3 inches)",
			Status:   "pass",
		})
	}

	return checks
}

// checkTheme verifies theme requirements.
func checkTheme(theme types.ThemeInfo) []templateCheckEntry {
	var checks []templateCheckEntry

	// Check all 12 scheme colors
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
		checks = append(checks, templateCheckEntry{
			Category: "theme",
			Check:    "Theme: all 12 scheme colors defined",
			Status:   "fail",
			Detail:   fmt.Sprintf("Missing: %s", strings.Join(missingColors, ", ")),
		})
	} else {
		checks = append(checks, templateCheckEntry{
			Category: "theme",
			Check:    "Theme: all 12 scheme colors defined",
			Status:   "pass",
		})
	}

	// Check major and minor fonts
	if theme.TitleFont == "" {
		checks = append(checks, templateCheckEntry{
			Category: "theme",
			Check:    "Theme: major font (titles) defined",
			Status:   "fail",
		})
	} else {
		checks = append(checks, templateCheckEntry{
			Category: "theme",
			Check:    "Theme: major font (titles) defined",
			Status:   "pass",
			Detail:   theme.TitleFont,
		})
	}

	if theme.BodyFont == "" {
		checks = append(checks, templateCheckEntry{
			Category: "theme",
			Check:    "Theme: minor font (body) defined",
			Status:   "fail",
		})
	} else {
		checks = append(checks, templateCheckEntry{
			Category: "theme",
			Check:    "Theme: minor font (body) defined",
			Status:   "pass",
			Detail:   theme.BodyFont,
		})
	}

	// Check dk/lt color polarity
	checks = append(checks, checkColorPolarity(colorMap)...)

	return checks
}

// checkColorPolarity verifies dark colors are dark and light colors are light.
func checkColorPolarity(colorMap map[string]string) []templateCheckEntry {
	var checks []templateCheckEntry

	// Check dark colors
	for _, name := range []string{"dk1", "dk2"} {
		rgb, ok := colorMap[name]
		if !ok {
			continue
		}
		lum := hexLuminance(rgb)
		if lum > 0.5 {
			checks = append(checks, templateCheckEntry{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is dark (luminance < 50%%)", name),
				Status:   "warn",
				Detail:   fmt.Sprintf("%s has luminance %.0f%% — expected dark", rgb, lum*100),
			})
		} else {
			checks = append(checks, templateCheckEntry{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is dark (luminance < 50%%)", name),
				Status:   "pass",
			})
		}
	}

	// Check light colors
	for _, name := range []string{"lt1", "lt2"} {
		rgb, ok := colorMap[name]
		if !ok {
			continue
		}
		lum := hexLuminance(rgb)
		if lum < 0.5 {
			checks = append(checks, templateCheckEntry{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is light (luminance > 50%%)", name),
				Status:   "warn",
				Detail:   fmt.Sprintf("%s has luminance %.0f%% — expected light", rgb, lum*100),
			})
		} else {
			checks = append(checks, templateCheckEntry{
				Category: "theme",
				Check:    fmt.Sprintf("Theme: %s is light (luminance > 50%%)", name),
				Status:   "pass",
			})
		}
	}

	return checks
}

// hexLuminance computes relative luminance from a hex RGB string like "#2E5090" or "2E5090".
func hexLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0
	}

	r := hexByte(hex[0:2])
	g := hexByte(hex[2:4])
	b := hexByte(hex[4:6])

	// sRGB to linear
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

// printTemplateCheckText outputs the check result as human-readable text.
func printTemplateCheckText(result templateCheckResult) {
	if result.Pass {
		fmt.Printf("PASS: %s\n", result.Template)
	} else {
		fmt.Printf("FAIL: %s\n", result.Template)
	}
	fmt.Println()

	for _, c := range result.Checks {
		var icon string
		switch c.Status {
		case "pass":
			icon = "  [OK]  "
		case "fail":
			icon = "  [FAIL]"
		case "warn":
			icon = "  [WARN]"
		}

		line := fmt.Sprintf("%s %s", icon, c.Check)
		if c.Detail != "" {
			line += " — " + c.Detail
		}
		fmt.Println(line)
	}
}
