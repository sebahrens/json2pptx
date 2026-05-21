// Package themeinfo resolves a template's theme palette and fonts, applying
// optional per-deck overrides. It is the reusable core behind the resolve-theme
// CLI subcommand and the resolve_theme MCP tool: callers in cmd/json2pptx adapt
// their CLI flags / MCP arguments into Options and shape Result into their
// response envelope.
package themeinfo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Options configures a Resolve call.
type Options struct {
	// ColorNames restricts the resolved palette to these scheme names, in this
	// order. Empty returns the full theme palette in theme-defined order.
	ColorNames []string
	// Override is applied to the parsed theme before colors/fonts are read, so
	// the result mirrors what the singlepass generator would see for the same
	// frontmatter. Nil leaves the template theme untouched.
	Override *types.ThemeOverride
}

// ColorEntry mirrors svggen-mcp's StyleSpec.theme_colors entry shape
// ({name, rgb}) so a caller can forward the slice straight into render_diagram's
// style.theme_colors without pivoting the colors map by hand.
type ColorEntry struct {
	Name string `json:"name"`
	RGB  string `json:"rgb"`
}

// Fonts describes the font families in the resolved theme.
type Fonts struct {
	Major FontEntry `json:"major"`
	Minor FontEntry `json:"minor"`
}

// FontEntry describes a single font slot.
type FontEntry struct {
	Latin string `json:"latin"`
}

// UnknownColor is reported for requested color names absent from the theme.
type UnknownColor struct {
	Name       string `json:"name"`
	DidYouMean string `json:"did_you_mean,omitempty"`
}

// Result is the resolved theme view returned by Resolve.
type Result struct {
	// Template is the template basename without the .pptx suffix.
	Template string
	// Colors maps scheme name -> hex. Filtered to Options.ColorNames when set,
	// otherwise the full palette.
	Colors map[string]string
	// ThemeColors mirrors Colors as an ordered [{name,rgb}] slice: theme-defined
	// order when unfiltered, request order when filtered.
	ThemeColors []ColorEntry
	// Fonts holds the resolved major/minor latin font families.
	Fonts Fonts
	// ResolvedFor echoes Options.ColorNames; nil when unfiltered.
	ResolvedFor []string
	// Unknown lists requested names not found in the theme, with a did-you-mean.
	Unknown []UnknownColor
	// Warnings carries override warnings (non-embedded fonts, unknown keys).
	Warnings []string
	// AllColors is the full resolved theme palette regardless of filtering, so
	// callers can derive theme-wide views (e.g. color roles) from one call.
	AllColors []types.ThemeColor
}

// Resolve parses the template at templatePath, applies opts.Override, and
// returns the resolved palette and fonts. The error wraps OpenTemplate failures
// ("failed to open template: ...") so callers can surface a template-error
// response unchanged.
func Resolve(templatePath string, opts Options) (Result, error) {
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		return Result{}, fmt.Errorf("failed to open template: %w", err)
	}
	defer func() { _ = reader.Close() }()

	theme := template.ParseTheme(reader)

	// Apply optional theme override before reading colors/fonts, so the result
	// mirrors what the singlepass generator would see for the same frontmatter.
	var warnings []string
	if opts.Override != nil {
		theme, warnings = theme.ApplyOverride(opts.Override)
	}

	// Build the full color map and name list (used for filtering and did-you-mean).
	allColors := make(map[string]string, len(theme.Colors))
	allColorNames := make([]string, 0, len(theme.Colors))
	for _, c := range theme.Colors {
		allColors[c.Name] = c.RGB
		allColorNames = append(allColorNames, c.Name)
	}

	colors := allColors
	var unknown []UnknownColor
	var resolvedFor []string
	// themeColors mirrors the colors map as the [{name,rgb}] array. Built in
	// stable order: theme-defined order when unfiltered, request order when filtered.
	themeColors := make([]ColorEntry, 0, len(theme.Colors))

	if len(opts.ColorNames) > 0 {
		colors = make(map[string]string, len(opts.ColorNames))
		resolvedFor = opts.ColorNames
		for _, name := range opts.ColorNames {
			if hex, ok := allColors[name]; ok {
				colors[name] = hex
				themeColors = append(themeColors, ColorEntry{Name: name, RGB: hex})
			} else {
				entry := UnknownColor{Name: name}
				if match, _ := generator.ClosestMatch(name, allColorNames, 3); match != "" {
					entry.DidYouMean = match
				}
				unknown = append(unknown, entry)
			}
		}
	} else {
		for _, c := range theme.Colors {
			themeColors = append(themeColors, ColorEntry{Name: c.Name, RGB: c.RGB})
		}
	}

	return Result{
		Template:    strings.TrimSuffix(filepath.Base(templatePath), ".pptx"),
		Colors:      colors,
		ThemeColors: themeColors,
		Fonts: Fonts{
			Major: FontEntry{Latin: theme.TitleFont},
			Minor: FontEntry{Latin: theme.BodyFont},
		},
		ResolvedFor: resolvedFor,
		Unknown:     unknown,
		Warnings:    warnings,
		AllColors:   theme.Colors,
	}, nil
}
