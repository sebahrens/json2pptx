package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ahrens/svggen"
	"github.com/sebahrens/json2pptx/internal/template"
)

type jsonPalette struct {
	Accent1     string `json:"accent1"`
	Accent2     string `json:"accent2"`
	Accent3     string `json:"accent3"`
	Accent4     string `json:"accent4"`
	Accent5     string `json:"accent5"`
	Accent6     string `json:"accent6"`
	Background  string `json:"background"`
	TextPrimary string `json:"text_primary"`
}

type jsonColor struct {
	Name string `json:"name"`
	RGB  string `json:"rgb"`
}

type jsonTemplate struct {
	Name      string      `json:"name"`
	TitleFont string      `json:"title_font"`
	BodyFont  string      `json:"body_font"`
	Colors    []jsonColor `json:"colors"`
	Palette   jsonPalette `json:"palette"`
}

// processTemplate opens a PPTX template, extracts theme info, and returns structured data.
// The file is closed before returning, avoiding resource leaks when called in a loop.
func processTemplate(name, path string) (jsonTemplate, error) {
	f, err := os.Open(path)
	if err != nil {
		return jsonTemplate{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return jsonTemplate{}, fmt.Errorf("stat %s: %w", path, err)
	}
	zr, err := zip.NewReader(f, stat.Size())
	if err != nil {
		return jsonTemplate{}, fmt.Errorf("reading zip %s: %w", path, err)
	}

	themeInfo := template.ParseThemeFromZip(zr)

	themeInputs := make([]svggen.ThemeColorInput, len(themeInfo.Colors))
	for i, tc := range themeInfo.Colors {
		themeInputs[i] = svggen.ThemeColorInput{Name: tc.Name, RGB: tc.RGB}
	}
	palette := svggen.NewPaletteFromThemeColors(themeInputs)

	jt := jsonTemplate{
		Name:      name,
		TitleFont: themeInfo.TitleFont,
		BodyFont:  themeInfo.BodyFont,
		Palette: jsonPalette{
			Accent1:     palette.Accent1.Hex(),
			Accent2:     palette.Accent2.Hex(),
			Accent3:     palette.Accent3.Hex(),
			Accent4:     palette.Accent4.Hex(),
			Accent5:     palette.Accent5.Hex(),
			Accent6:     palette.Accent6.Hex(),
			Background:  palette.Background.Hex(),
			TextPrimary: palette.TextPrimary.Hex(),
		},
	}
	for _, c := range themeInfo.Colors {
		jt.Colors = append(jt.Colors, jsonColor{Name: c.Name, RGB: c.RGB})
	}
	return jt, nil
}

func themeInputsFromJSON(jt jsonTemplate) []svggen.ThemeColorInput {
	inputs := make([]svggen.ThemeColorInput, len(jt.Colors))
	for i, c := range jt.Colors {
		inputs[i] = svggen.ThemeColorInput{Name: c.Name, RGB: c.RGB}
	}
	return inputs
}

func main() {
	templateFlag := flag.String("template", "", "Template name to debug (default: all templates)")
	jsonFlag := flag.Bool("json", false, "Output as JSON")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: debugcolors [options]\n\n")
		fmt.Fprintf(os.Stderr, "Display and debug color palettes extracted from PPTX templates.\n")
		fmt.Fprintf(os.Stderr, "Shows theme colors, accent colors, and computed palette for a given template.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	templates := []struct {
		name string
		path string
	}{
		{"modern-template", "templates/modern-template.pptx"},
		{"forest-green", "testdata/templates/forest-green.pptx"},
		{"warm-coral", "testdata/templates/warm-coral.pptx"},
	}

	// Filter templates if -template flag is set
	if *templateFlag != "" {
		var filtered []struct {
			name string
			path string
		}
		for _, t := range templates {
			if t.name == *templateFlag {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "Unknown template: %s\nAvailable templates:\n", *templateFlag)
			for _, t := range templates {
				fmt.Fprintf(os.Stderr, "  %s\n", t.name)
			}
			os.Exit(1)
		}
		templates = filtered
	}

	var jsonResults []jsonTemplate

	for _, t := range templates {
		if !*jsonFlag {
			fmt.Printf("\n=== %s ===\n", t.name)
		}

		jt, err := processTemplate(t.name, t.path)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			continue
		}

		if *jsonFlag {
			jsonResults = append(jsonResults, jt)
		} else {
			palette := svggen.NewPaletteFromThemeColors(themeInputsFromJSON(jt))
			fmt.Printf("  Theme name: %s\n", t.name)
			fmt.Printf("  Colors (%d):\n", len(jt.Colors))
			for _, c := range jt.Colors {
				fmt.Printf("    %-10s %s\n", c.Name, c.RGB)
			}
			fmt.Printf("  Resulting palette:\n")
			fmt.Printf("    Accent1/Primary:   %s\n", palette.Accent1.Hex())
			fmt.Printf("    Accent2/Secondary: %s\n", palette.Accent2.Hex())
			fmt.Printf("    Accent3/Tertiary:  %s\n", palette.Accent3.Hex())
			fmt.Printf("    Accent4:           %s\n", palette.Accent4.Hex())
			fmt.Printf("    Accent5:           %s\n", palette.Accent5.Hex())
			fmt.Printf("    Accent6:           %s\n", palette.Accent6.Hex())
			fmt.Printf("    Background:        %s\n", palette.Background.Hex())
			fmt.Printf("    TextPrimary:       %s\n", palette.TextPrimary.Hex())

			// Compare with default
			defPalette := svggen.DefaultPalette()
			fmt.Printf("  Default palette for reference:\n")
			fmt.Printf("    Accent1/Primary:   %s\n", defPalette.Accent1.Hex())
			fmt.Printf("    Accent2/Secondary: %s\n", defPalette.Accent2.Hex())
			fmt.Printf("    Accent3/Tertiary:  %s\n", defPalette.Accent3.Hex())
		}
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(jsonResults)
	}
}
