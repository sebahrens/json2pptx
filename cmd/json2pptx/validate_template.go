package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// validateTemplateResult is the JSON-serializable output of validate-template.
type validateTemplateResult struct {
	Valid        bool                        `json:"valid"`
	Template     string                      `json:"template"`
	AspectRatio  string                      `json:"aspect_ratio"`
	Theme        validateTemplateTheme       `json:"theme"`
	Layouts      []validateTemplateLayout    `json:"layouts"`
	Capabilities validateTemplateCapabilites `json:"capabilities"`
	Warnings     []string                    `json:"warnings"`
	Errors       []string                    `json:"errors"`
}

// validateTemplateTheme is the theme section of the JSON output.
type validateTemplateTheme struct {
	Name      string            `json:"name"`
	TitleFont string            `json:"title_font"`
	BodyFont  string            `json:"body_font"`
	Colors    map[string]string `json:"colors"`
}

// validateTemplateLayout is a single layout in the JSON output.
type validateTemplateLayout struct {
	Index        int                              `json:"index"`
	Name         string                           `json:"name"`
	ID           string                           `json:"id"`
	Tags         []string                         `json:"tags"`
	Placeholders []validateTemplatePlaceholder     `json:"placeholders,omitempty"`
	Capacity     validateTemplateCapacityEstimate  `json:"capacity"`
}

// validateTemplatePlaceholder is a placeholder in the JSON output.
type validateTemplatePlaceholder struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Index    int    `json:"index"`
	MaxChars int    `json:"max_chars"`
}

// validateTemplateCapacityEstimate is the capacity estimate in the JSON output.
type validateTemplateCapacityEstimate struct {
	MaxBullets    int  `json:"max_bullets"`
	MaxTextLines  int  `json:"max_text_lines"`
	HasImageSlot  bool `json:"has_image_slot"`
	HasChartSlot  bool `json:"has_chart_slot"`
	TextHeavy     bool `json:"text_heavy"`
	VisualFocused bool `json:"visual_focused"`
}

// validateTemplateCapabilites summarizes what the template can do.
type validateTemplateCapabilites struct {
	ChartCapable bool `json:"chart_capable"`
	ImageCapable bool `json:"image_capable"`
	TwoColumn    bool `json:"two-column"`
	Synthesized  bool `json:"synthesized"`
}

// runValidateTemplate implements the validate-template subcommand.
func runValidateTemplate() error {
	fs := flag.NewFlagSet("validate-template", flag.ContinueOnError)
	strict := fs.Bool("strict", false, "fail on warnings, not just errors")
	jsonOutput := fs.Bool("json", false, "output as JSON")
	verbose := fs.Bool("verbose", false, "show placeholder details")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) != 1 {
		return fmt.Errorf("usage: json2pptx validate-template [--strict] [--json] [--verbose] <template.pptx>")
	}
	templatePath := args[0]

	// Open template
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Parse layouts
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		return fmt.Errorf("failed to parse layouts: %w", err)
	}

	// Parse theme
	theme := template.ParseTheme(reader)

	// Validate metadata
	validationResult := template.ValidateTemplateMetadata(reader, *strict)

	// Apply metadata hints to layouts
	template.ApplyMetadataHints(layouts, validationResult.Metadata)

	// Determine aspect ratio
	aspectRatio := "16:9"
	if validationResult.Metadata != nil && validationResult.Metadata.AspectRatio != "" {
		aspectRatio = validationResult.Metadata.AspectRatio
	}

	// Build analysis for synthesis
	analysis := &types.TemplateAnalysis{
		TemplatePath: templatePath,
		Hash:         reader.Hash(),
		AspectRatio:  aspectRatio,
		Layouts:      layouts,
		Theme:        theme,
		Metadata:     validationResult.Metadata,
	}

	// Synthesize missing layout capabilities
	_ = template.SynthesizeIfNeeded(reader, analysis)
	layouts = analysis.Layouts

	// Determine capabilities
	caps := detectCapabilities(layouts, analysis.Synthesis)

	// Run heuristic checks
	sectionNumberWarnings := checkSectionNumberNaming(layouts)

	// Collect all warnings and errors
	var warnings []string
	var errors []string
	warnings = append(warnings, validationResult.Warnings...)
	warnings = append(warnings, sectionNumberWarnings...)
	errors = append(errors, validationResult.Errors...)

	valid := validationResult.Valid

	// Build result
	result := validateTemplateResult{
		Valid:       valid,
		Template:    filepath.Base(templatePath),
		AspectRatio: aspectRatio,
		Theme:       buildThemeOutput(theme),
		Layouts:     buildLayoutsOutput(layouts, *verbose),
		Capabilities: caps,
		Warnings:    warnings,
		Errors:      errors,
	}

	// Output
	if *jsonOutput {
		return outputJSON(result)
	}
	outputText(result, *verbose)

	// Return error for invalid templates so the caller (dispatch) sets exit code
	if !valid {
		return fmt.Errorf("template validation failed")
	}
	return nil
}

// detectCapabilities scans layouts for known capability tags.
func detectCapabilities(layouts []types.LayoutMetadata, synthesis *types.SynthesisManifest) validateTemplateCapabilites {
	caps := validateTemplateCapabilites{}
	synthesized := synthesis != nil && len(synthesis.SyntheticFiles) > 0

	for _, layout := range layouts {
		for _, ph := range layout.Placeholders {
			if ph.Type == types.PlaceholderChart {
				caps.ChartCapable = true
			}
			if ph.Type == types.PlaceholderImage {
				caps.ImageCapable = true
			}
		}
		for _, tag := range layout.Tags {
			if tag == "two-column" {
				caps.TwoColumn = true
			}
		}
	}

	caps.Synthesized = synthesized
	return caps
}

// buildThemeOutput converts ThemeInfo to the output format.
func buildThemeOutput(theme types.ThemeInfo) validateTemplateTheme {
	colors := make(map[string]string, len(theme.Colors))
	for _, c := range theme.Colors {
		colors[c.Name] = c.RGB
	}
	return validateTemplateTheme{
		Name:      theme.Name,
		TitleFont: theme.TitleFont,
		BodyFont:  theme.BodyFont,
		Colors:    colors,
	}
}

// buildLayoutsOutput converts layouts to the output format.
func buildLayoutsOutput(layouts []types.LayoutMetadata, verbose bool) []validateTemplateLayout {
	out := make([]validateTemplateLayout, len(layouts))
	for i, l := range layouts {
		vl := validateTemplateLayout{
			Index: l.Index,
			Name:  l.Name,
			ID:    l.ID,
			Tags:  l.Tags,
			Capacity: validateTemplateCapacityEstimate{
				MaxBullets:    l.Capacity.MaxBullets,
				MaxTextLines:  l.Capacity.MaxTextLines,
				HasImageSlot:  l.Capacity.HasImageSlot,
				HasChartSlot:  l.Capacity.HasChartSlot,
				TextHeavy:     l.Capacity.TextHeavy,
				VisualFocused: l.Capacity.VisualFocused,
			},
		}
		if verbose {
			phs := make([]validateTemplatePlaceholder, len(l.Placeholders))
			for j, ph := range l.Placeholders {
				phs[j] = validateTemplatePlaceholder{
					ID:       ph.ID,
					Type:     string(ph.Type),
					Index:    ph.Index,
					MaxChars: ph.MaxChars,
				}
			}
			vl.Placeholders = phs
		}
		if vl.Tags == nil {
			vl.Tags = []string{}
		}
		out[i] = vl
	}
	return out
}

// outputJSON writes the result as JSON to stdout.
func outputJSON(result validateTemplateResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// outputText writes the result as human-readable text to stdout.
func outputText(result validateTemplateResult, verbose bool) {
	fmt.Printf("Template: %s\n", result.Template)
	fmt.Printf("Aspect Ratio: %s\n", result.AspectRatio)
	fmt.Printf("Valid: %t\n", result.Valid)

	// Theme
	fmt.Println()
	fmt.Println("Theme:")
	fmt.Printf("  Name: %s\n", result.Theme.Name)
	fmt.Printf("  Title Font: %s\n", result.Theme.TitleFont)
	fmt.Printf("  Body Font: %s\n", result.Theme.BodyFont)
	if len(result.Theme.Colors) > 0 {
		colorStrs := make([]string, 0, len(result.Theme.Colors))
		for _, name := range themeColorOrder() {
			if rgb, ok := result.Theme.Colors[name]; ok {
				colorStrs = append(colorStrs, rgb)
			}
		}
		fmt.Printf("  Colors: %s\n", strings.Join(colorStrs, ", "))
	}

	// Layouts
	fmt.Println()
	fmt.Printf("Layouts (%d):\n", len(result.Layouts))
	for i, l := range result.Layouts {
		tags := ""
		if len(l.Tags) > 0 {
			tags = " [" + strings.Join(l.Tags, ", ") + "]"
		}
		phCount := len(l.Placeholders)
		if !verbose {
			// Count from capacity hints when placeholders not included
			phCount = countPlaceholdersFromCapacity(l.Capacity)
		}
		fmt.Printf("  %d. %s%s", i+1, l.Name, tags)
		if verbose {
			fmt.Printf(" -- %d placeholders\n", len(l.Placeholders))
			for _, ph := range l.Placeholders {
				fmt.Printf("       [%s] %s (idx=%d, max_chars=%d)\n",
					ph.Type, ph.ID, ph.Index, ph.MaxChars)
			}
		} else {
			fmt.Printf(" -- %d placeholders\n", phCount)
		}
	}

	// Capabilities
	fmt.Println()
	fmt.Println("Capabilities:")
	fmt.Printf("  Chart-capable: %s\n", boolYesNo(result.Capabilities.ChartCapable))
	fmt.Printf("  Image-capable: %s\n", boolYesNo(result.Capabilities.ImageCapable))
	twoCol := boolYesNo(result.Capabilities.TwoColumn)
	if result.Capabilities.TwoColumn && result.Capabilities.Synthesized {
		twoCol += " (synthesized)"
	}
	fmt.Printf("  Two-column: %s\n", twoCol)

	// Warnings and errors
	fmt.Println()
	if len(result.Errors) > 0 {
		fmt.Printf("Errors (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings (%d):\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	if len(result.Errors) == 0 && len(result.Warnings) == 0 {
		fmt.Println("Warnings: none")
	}
}

// themeColorOrder returns the standard order for theme color names.
func themeColorOrder() []string {
	return []string{
		"dk1", "dk2", "lt1", "lt2",
		"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
		"hlink", "folHlink",
	}
}

// countPlaceholdersFromCapacity estimates placeholder count from capacity info.
// This is a rough heuristic used when verbose mode is off and placeholders aren't serialized.
func countPlaceholdersFromCapacity(cap validateTemplateCapacityEstimate) int {
	count := 0
	if cap.MaxBullets > 0 || cap.MaxTextLines > 0 || cap.TextHeavy {
		count++ // at least one text placeholder
	}
	if cap.HasImageSlot {
		count++
	}
	if cap.HasChartSlot {
		count++
	}
	// Title is almost always present
	count++
	return count
}

// boolYesNo returns "yes" or "no" for a boolean value.
func boolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// checkSectionNumberNaming inspects section-header layouts for body-typed
// placeholders that look like decorative number frames but are not named
// "Section Number". The engine's normalizer (normalizer.go) skips shapes
// named "Section Number" during body normalization; a misnamed shape gets
// treated as body text, corrupting the visual.
//
// Heuristic thresholds (all must be true to trigger):
//   - Placeholder type is "body"
//   - MaxChars < 5  (tiny text capacity — typical of a 1-2 digit number frame)
//   - FontSize > 4000 (> 40pt in hundredths-of-a-point)
//   - Y position in upper third of slide (Y < slideHeight/3 ≈ 2_286_000 EMU for 16:9)
//   - Name is NOT "Section Number" (case-insensitive)
func checkSectionNumberNaming(layouts []types.LayoutMetadata) []string {
	const (
		maxCharsThreshold = 5        // fewer than 5 chars expected
		minFontSize       = 4000     // 40pt in hundredths-of-a-point
		upperThirdY       = 2286000  // ~1/3 of 6858000 EMU (16:9 slide height)
	)

	var warnings []string
	for _, layout := range layouts {
		if !hasSectionHeaderTag(layout.Tags) {
			continue
		}
		for _, ph := range layout.Placeholders {
			if ph.Type != types.PlaceholderBody {
				continue
			}
			if strings.EqualFold(ph.ID, "Section Number") {
				continue
			}
			if ph.MaxChars >= maxCharsThreshold {
				continue
			}
			if ph.FontSize < minFontSize {
				continue
			}
			if ph.Bounds.Y >= upperThirdY {
				continue
			}
			warnings = append(warnings, fmt.Sprintf(
				"Layout %q has a placeholder that looks like a decorative section number but is named %q. "+
					"Rename to \"Section Number\" so the engine preserves it correctly.",
				layout.Name, ph.ID,
			))
		}
	}
	return warnings
}

// hasSectionHeaderTag returns true if the tags slice contains "section-header".
func hasSectionHeaderTag(tags []string) bool {
	for _, t := range tags {
		if t == "section-header" {
			return true
		}
	}
	return false
}
