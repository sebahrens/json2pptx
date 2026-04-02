package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/visualqa"
)

// slideInputForQA mirrors just the fields we need from the JSON input.
type slideInputForQA struct {
	LayoutID  string `json:"layout_id,omitempty"`
	SlideType string `json:"slide_type,omitempty"`
	Content   []struct {
		PlaceholderID string          `json:"placeholder_id"`
		Type          string          `json:"type"`
		Value         json.RawMessage `json:"value,omitempty"`
		TextValue     *string         `json:"text_value,omitempty"`
	} `json:"content"`
	ShapeGrid json.RawMessage `json:"shape_grid,omitempty"`
}

type presentationForQA struct {
	Template string            `json:"template"`
	Slides   []slideInputForQA `json:"slides"`
}

func cmdQA(args []string) {
	fs := flag.NewFlagSet("qa", flag.ExitOnError)
	imagesDir := fs.String("images", "", "directory containing slide JPG images (slide-0.jpg, slide-1.jpg, ...)")
	jsonFile := fs.String("json", "", "JSON input file (for slide type metadata)")
	model := fs.String("model", "", "Claude model override (default: claude-haiku-4-5-20251001)")
	parallel := fs.Int("parallel", 4, "concurrent API calls")
	jsonOutput := fs.Bool("json-output", false, "output full JSON report")
	minSeverity := fs.String("min-severity", "P3", "minimum severity to report (P0, P1, P2, P3)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if *imagesDir == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --images is required (directory of slide JPG/PNG files)")
		os.Exit(1)
	}

	// Load slide images.
	slideImages, err := loadSlideImages(*imagesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR loading images: %v\n", err)
		os.Exit(1)
	}
	if len(slideImages) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: no slide images found in directory")
		os.Exit(1)
	}

	// Load slide metadata from JSON if provided.
	var pres presentationForQA
	if *jsonFile != "" {
		data, err := os.ReadFile(*jsonFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR reading JSON: %v\n", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &pres); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR parsing JSON: %v\n", err)
			os.Exit(1)
		}
	}

	// Build SlideImage list with type metadata.
	images := make([]visualqa.SlideImage, len(slideImages))
	for i, img := range slideImages {
		info := visualqa.SlideInfo{Index: i}
		if i < len(pres.Slides) {
			s := pres.Slides[i]
			info.Type = inferSlideType(s)
			info.Title = extractTitle(s)
		}
		if info.Type == "" {
			info.Type = "content" // default fallback
		}
		images[i] = visualqa.SlideImage{Info: info, Data: img}
	}

	// Create agent.
	var opts []visualqa.Option
	if *model != "" {
		opts = append(opts, visualqa.WithModel(*model))
	}
	if *parallel > 0 {
		opts = append(opts, visualqa.WithParallelism(*parallel))
	}

	agent, err := visualqa.NewAgent(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "qa: inspecting %d slides (parallel=%d)\n", len(images), *parallel)

	// Run inspection.
	report := agent.InspectAll(context.Background(), images)
	report.Template = pres.Template

	if *jsonOutput {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
	} else {
		printReport(report, severityThreshold(*minSeverity))
	}

	// Exit with non-zero if P0 or P1 issues found.
	if report.TotalByP0 > 0 || report.TotalByP1 > 0 {
		os.Exit(1)
	}
}

// loadSlideImages reads slide images from a directory, sorted by name.
func loadSlideImages(dir string) ([][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".png") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)

	images := make([][]byte, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		images = append(images, data)
	}
	return images, nil
}

// inferSlideType determines the slide type from JSON metadata.
func inferSlideType(s slideInputForQA) string {
	if s.SlideType != "" {
		return s.SlideType
	}
	if len(s.ShapeGrid) > 0 && string(s.ShapeGrid) != "null" {
		return "content" // shape_grid slides are content-type visually
	}
	// Infer from layout ID naming conventions.
	lid := strings.ToLower(s.LayoutID)
	switch {
	case strings.Contains(lid, "title") || lid == "slidelayout1":
		return "title"
	case strings.Contains(lid, "section") || lid == "slidelayout3":
		return "section"
	case strings.Contains(lid, "two") || strings.Contains(lid, "column"):
		return "two-column"
	case strings.Contains(lid, "blank"):
		return "blank"
	case strings.Contains(lid, "comparison"):
		return "comparison"
	}
	// Infer from content types.
	for _, c := range s.Content {
		switch c.Type {
		case "chart", "diagram":
			return c.Type
		case "table":
			return "table"
		case "image":
			return "image"
		}
	}
	return "content"
}

// extractTitle gets the title text from a slide's content items.
func extractTitle(s slideInputForQA) string {
	for _, c := range s.Content {
		if c.PlaceholderID == "title" || c.PlaceholderID == "ctrTitle" {
			if c.TextValue != nil {
				return *c.TextValue
			}
			// Try legacy value field.
			if len(c.Value) > 0 {
				var v string
				if json.Unmarshal(c.Value, &v) == nil {
					return v
				}
			}
		}
	}
	return ""
}

// severityThreshold returns severity values at or above the threshold.
func severityThreshold(min string) map[visualqa.Severity]bool {
	all := map[visualqa.Severity]bool{
		visualqa.SeverityP0: true,
		visualqa.SeverityP1: true,
		visualqa.SeverityP2: true,
		visualqa.SeverityP3: true,
	}
	switch min {
	case "P0":
		delete(all, visualqa.SeverityP1)
		delete(all, visualqa.SeverityP2)
		delete(all, visualqa.SeverityP3)
	case "P1":
		delete(all, visualqa.SeverityP2)
		delete(all, visualqa.SeverityP3)
	case "P2":
		delete(all, visualqa.SeverityP3)
	}
	return all
}

// printReport outputs a human-readable summary.
func printReport(r *visualqa.Report, show map[visualqa.Severity]bool) {
	fmt.Printf("\n=== Visual QA Report ===\n")
	if r.Template != "" {
		fmt.Printf("Template: %s\n", r.Template)
	}
	fmt.Printf("Slides inspected: %d\n\n", r.SlideCount)

	issuesShown := 0
	for _, sr := range r.Results {
		if sr.Error != "" {
			fmt.Printf("  Slide %d (%s): ERROR — %s\n", sr.SlideIndex, sr.SlideType, sr.Error)
			continue
		}
		for _, f := range sr.Findings {
			if !show[f.Severity] {
				continue
			}
			fmt.Printf("  %s\n", f.String())
			issuesShown++
		}
	}

	if issuesShown == 0 {
		fmt.Printf("  No issues found. All slides passed visual inspection.\n")
	}

	fmt.Printf("\nSummary: P0=%d  P1=%d  P2=%d  P3=%d  Total=%d\n",
		r.TotalByP0, r.TotalByP1, r.TotalByP2, r.TotalByP3, r.TotalIssues)

	if r.TotalByP0 > 0 {
		fmt.Println("RESULT: FAIL (P0 issues found)")
	} else if r.TotalByP1 > 0 {
		fmt.Println("RESULT: FAIL (P1 issues found)")
	} else {
		fmt.Println("RESULT: PASS")
	}
}
