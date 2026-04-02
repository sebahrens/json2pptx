package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ahrens/svggen"
	"github.com/ahrens/go-slide-creator/internal/template"
)

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

	type jsonPalette struct {
		Accent1    string `json:"accent1"`
		Accent2    string `json:"accent2"`
		Accent3    string `json:"accent3"`
		Accent4    string `json:"accent4"`
		Accent5    string `json:"accent5"`
		Accent6    string `json:"accent6"`
		Background string `json:"background"`
		TextPrimary string `json:"text_primary"`
	}
	type jsonColor struct {
		Name string `json:"name"`
		RGB  string `json:"rgb"`
	}
	type jsonTemplate struct {
		Name      string       `json:"name"`
		TitleFont string       `json:"title_font"`
		BodyFont  string       `json:"body_font"`
		Colors    []jsonColor  `json:"colors"`
		Palette   jsonPalette  `json:"palette"`
	}
	var jsonResults []jsonTemplate

	for _, t := range templates {
		if !*jsonFlag {
			fmt.Printf("\n=== %s ===\n", t.name)
		}

		f, err := os.Open(t.path)
		if err != nil {
			fmt.Printf("  ERROR opening: %v\n", err)
			continue
		}
		defer f.Close()

		stat, _ := f.Stat()
		zr, err := zip.NewReader(f, stat.Size())
		if err != nil {
			fmt.Printf("  ERROR reading zip: %v\n", err)
			continue
		}

		// Use ParseThemeFromZip (same as generator does)
		themeInfo := template.ParseThemeFromZip(zr)

		// Now simulate what diagramSpecToSVGGen does
		themeInputs := make([]svggen.ThemeColorInput, len(themeInfo.Colors))
		for i, tc := range themeInfo.Colors {
			themeInputs[i] = svggen.ThemeColorInput{
				Name: tc.Name,
				RGB:  tc.RGB,
			}
		}

		// Build palette (same as StyleGuideFromSpec does)
		palette := svggen.NewPaletteFromThemeColors(themeInputs)

		if *jsonFlag {
			jt := jsonTemplate{
				Name:      t.name,
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
			jsonResults = append(jsonResults, jt)
		} else {
			fmt.Printf("  Theme name: %s\n", themeInfo.Name)
			fmt.Printf("  Title font: %s\n", themeInfo.TitleFont)
			fmt.Printf("  Body font: %s\n", themeInfo.BodyFont)
			fmt.Printf("  Colors (%d):\n", len(themeInfo.Colors))
			for _, c := range themeInfo.Colors {
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
