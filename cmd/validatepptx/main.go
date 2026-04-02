// Command validatepptx validates PPTX structure and embedded media.
//
// It performs structural validation and reports statistics about slides,
// SVG files, PNG files, and chart embedding.
//
// Usage:
//
//	validatepptx [options] <pptx-file>
//
// Options:
//
//	-json           Output results as JSON
//	-require-svg    Exit with error if no SVG files are found
//	-min-slides N   Minimum expected slide count
//
// Exit codes:
//
//	0 - Validation passed
//	1 - Validation failed (structural issues or requirements not met)
//	2 - Error (file not found, invalid PPTX, etc.)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ValidationResult holds the validation outcome.
type ValidationResult struct {
	FilePath    string            `json:"file_path"`
	IsValid     bool              `json:"is_valid"`
	SlideCount  int               `json:"slide_count"`
	MediaStats  pptx.MediaStats   `json:"media_stats"`
	Errors      []string          `json:"errors,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
	PassMessage string            `json:"pass_message,omitempty"`
}

func main() {
	jsonOutput := flag.Bool("json", false, "Output results as JSON")
	requireSVG := flag.Bool("require-svg", false, "Require at least one SVG file")
	minSlides := flag.Int("min-slides", 0, "Minimum expected slide count")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: validatepptx [options] <pptx-file>")
		flag.PrintDefaults()
		os.Exit(2)
	}

	filePath := flag.Arg(0)

	result := validate(filePath, *requireSVG, *minSlides)

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(2)
		}
	} else {
		printHumanOutput(result)
	}

	if !result.IsValid {
		os.Exit(1)
	}
}

func validate(filePath string, requireSVG bool, minSlides int) ValidationResult {
	result := ValidationResult{
		FilePath: filePath,
		IsValid:  true,
	}

	// Read file (path comes from CLI argument, not user input)
	data, err := os.ReadFile(filePath) //nolint:gosec // CLI tool - path from command-line argument
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to read file: %v", err))
		return result
	}

	// Create validator
	v, err := pptx.NewValidator(data)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid PPTX: %v", err))
		return result
	}

	// Run structural validation
	if err := v.Validate(); err != nil {
		result.IsValid = false
		if verrs, ok := err.(pptx.ValidationErrors); ok {
			for _, verr := range verrs {
				result.Errors = append(result.Errors, verr.Error())
			}
		} else {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// Get counts
	result.SlideCount = v.CountSlides()
	result.MediaStats = v.MediaStats()

	// Check requirements
	if requireSVG && result.MediaStats.SVG == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "No SVG files found (--require-svg specified)")
	}

	if minSlides > 0 && result.SlideCount < minSlides {
		result.IsValid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Slide count %d is less than minimum %d", result.SlideCount, minSlides))
	}

	// Add warnings
	if result.MediaStats.SVG == 0 && result.MediaStats.PNG > 0 {
		result.Warnings = append(result.Warnings,
			"PPTX contains PNG files but no SVG files - charts may be rasterized only")
	}

	if result.IsValid {
		result.PassMessage = fmt.Sprintf("PPTX valid: %d slides, %d SVG, %d PNG",
			result.SlideCount, result.MediaStats.SVG, result.MediaStats.PNG)
	}

	return result
}

func printHumanOutput(result ValidationResult) {
	fmt.Printf("PPTX Validation: %s\n", result.FilePath)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Slides:      %d\n", result.SlideCount)
	fmt.Printf("  Media Total: %d\n", result.MediaStats.Total)
	fmt.Printf("    SVG:       %d\n", result.MediaStats.SVG)
	fmt.Printf("    PNG:       %d\n", result.MediaStats.PNG)
	fmt.Printf("    Other:     %d\n", result.MediaStats.Other)
	fmt.Println()

	if len(result.Warnings) > 0 {
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
		fmt.Println()
	}

	if len(result.Errors) > 0 {
		fmt.Println("Errors:")
		for _, e := range result.Errors {
			fmt.Printf("  ✗ %s\n", e)
		}
		fmt.Println()
	}

	if result.IsValid {
		fmt.Printf("✓ VALID: %s\n", result.PassMessage)
	} else {
		fmt.Println("✗ INVALID: See errors above")
	}
}
