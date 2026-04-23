package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// runPatterns implements the "patterns" subcommand with sub-subcommands:
// list, show, validate, expand.
func runPatterns() error {
	if len(os.Args) < 2 {
		printPatternsUsage()
		return nil
	}

	subcmd := os.Args[1]
	// Shift args for the sub-subcommand's flags
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcmd {
	case "list":
		return runPatternsList()
	case "show":
		return runPatternsShow()
	case "validate":
		return runPatternsValidate()
	case "expand":
		return runPatternsExpand()
	case "help", "-h", "--help":
		printPatternsUsage()
		return nil
	default:
		printPatternsUsage()
		return fmt.Errorf("unknown patterns subcommand %q", subcmd)
	}
}

func printPatternsUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx patterns <command> [options]

Commands:
  list                         List all available patterns
  show <name>                  Show full schema and details for a pattern
  validate <name> <file.json>  Validate pattern values without generating
  expand <name> <file.json>    Expand pattern to shape_grid JSON

Run 'json2pptx patterns <command> -h' for command-specific help.
`)
}

// ---------------------------------------------------------------------------
// patterns list
// ---------------------------------------------------------------------------

func runPatternsList() error {
	fs := flag.NewFlagSet("patterns list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns list [--json]\n\n")
		fmt.Fprintf(os.Stderr, "List all available named patterns.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	reg := patterns.Default()
	all := reg.List()

	if *jsonOut {
		entries := make([]skillPatternCompact, len(all))
		for i, p := range all {
			cells := ""
			if cd, ok := p.(patterns.CellDescriber); ok {
				cells = cd.CellsHint()
			}
			var sizeBytes int
			if ex, ok := p.(patterns.Exemplar); ok {
				sizeBytes, _ = patterns.CanonicalSizeBytes(p, ex.ExemplarValues())
			}
			entries[i] = skillPatternCompact{
				Name:                     p.Name(),
				Cells:                    cells,
				UseWhen:                  p.UseWhen(),
				EstimatedPromptSizeBytes: sizeBytes,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable table
	fmt.Printf("%-25s %-10s %s\n", "NAME", "CELLS", "USE WHEN")
	fmt.Printf("%-25s %-10s %s\n", strings.Repeat("-", 25), strings.Repeat("-", 10), strings.Repeat("-", 40))
	for _, p := range all {
		cells := ""
		if cd, ok := p.(patterns.CellDescriber); ok {
			cells = cd.CellsHint()
		}
		fmt.Printf("%-25s %-10s %s\n", p.Name(), cells, p.UseWhen())
	}
	return nil
}

// ---------------------------------------------------------------------------
// patterns show
// ---------------------------------------------------------------------------

func runPatternsShow() error {
	fs := flag.NewFlagSet("patterns show", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns show [--json] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Show full schema and details for a named pattern.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("pattern name is required")
	}
	name := args[0]

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		return unknownPatternError(name, reg)
	}

	if *jsonOut {
		schemaJSON, _ := json.Marshal(pat.Schema())
		result := skillPatternFull{
			Name:        pat.Name(),
			Description: pat.Description(),
			UseWhen:     pat.UseWhen(),
			Version:     pat.Version(),
			Schema:      schemaJSON,
		}
		if cd, ok := pat.(patterns.CellDescriber); ok {
			result.Cells = cd.CellsHint()
		}
		if cs, ok := pat.(patterns.CalloutSupport); ok {
			result.SupportsCallout = cs.SupportsCallout()
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	fmt.Printf("Pattern: %s (v%d)\n", pat.Name(), pat.Version())
	fmt.Printf("Description: %s\n", pat.Description())
	fmt.Printf("Use when: %s\n", pat.UseWhen())
	if cd, ok := pat.(patterns.CellDescriber); ok {
		fmt.Printf("Cells: %s\n", cd.CellsHint())
	}
	if cs, ok := pat.(patterns.CalloutSupport); ok && cs.SupportsCallout() {
		fmt.Printf("Supports callout: yes\n")
	}
	fmt.Println()
	schemaJSON, _ := json.MarshalIndent(pat.Schema(), "", "  ")
	fmt.Printf("Schema:\n%s\n", string(schemaJSON))
	return nil
}

// ---------------------------------------------------------------------------
// patterns validate
// ---------------------------------------------------------------------------

func runPatternsValidate() error {
	fs := flag.NewFlagSet("patterns validate", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON (D10 structured errors)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns validate [--json] <name> <values.json>\n\n")
		fmt.Fprintf(os.Stderr, "Validate pattern values without generating a deck.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) < 2 {
		fs.Usage()
		return fmt.Errorf("pattern name and values file are required")
	}
	name := args[0]
	valuesFile := args[1]

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		return unknownPatternError(name, reg)
	}

	// Read and parse values file — expects a PatternInput-shaped JSON
	content, err := os.ReadFile(valuesFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", valuesFile, err)
	}

	pi, err := parsePatternInputFile(content, name)
	if err != nil {
		return err
	}

	// Unmarshal values
	values := pat.NewValues()
	if err := json.Unmarshal(pi.Values, values); err != nil {
		return emitValidationResult(name, *jsonOut, fmt.Errorf("invalid values: %w", err))
	}

	// Unmarshal overrides
	var overrides any
	if len(pi.Overrides) > 0 {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal(pi.Overrides, overrides); err != nil {
				return emitValidationResult(name, *jsonOut, fmt.Errorf("invalid overrides: %w", err))
			}
		}
	}

	// Unmarshal cell_overrides
	cellOverrides, err := unmarshalCellOverrides(pat, pi.CellOverrides)
	if err != nil {
		return emitValidationResult(name, *jsonOut, err)
	}

	// Validate
	if err := pat.Validate(values, overrides, cellOverrides); err != nil {
		return emitValidationResult(name, *jsonOut, err)
	}

	// Success
	if *jsonOut {
		result := struct {
			OK      bool   `json:"ok"`
			Pattern string `json:"pattern"`
		}{OK: true, Pattern: name}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Pattern %q: valid\n", name)
	}
	return nil
}

// ---------------------------------------------------------------------------
// patterns expand
// ---------------------------------------------------------------------------

func runPatternsExpand() error {
	fs := flag.NewFlagSet("patterns expand", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON (always JSON, flag is for consistency)")
	_ = jsonOut // expand always outputs JSON; flag accepted for consistency

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx patterns expand [--json] <name> <values.json>\n\n")
		fmt.Fprintf(os.Stderr, "Expand a pattern to its shape_grid equivalent.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) < 2 {
		fs.Usage()
		return fmt.Errorf("pattern name and values file are required")
	}
	name := args[0]
	valuesFile := args[1]

	reg := patterns.Default()
	pat, ok := reg.Get(name)
	if !ok {
		return unknownPatternError(name, reg)
	}

	// Read values file
	content, err := os.ReadFile(valuesFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", valuesFile, err)
	}

	pi, err := parsePatternInputFile(content, name)
	if err != nil {
		return err
	}

	// Build a minimal ExpandContext (no template context in CLI)
	expandCtx := patterns.ExpandContext{
		SlideWidth:  9144000,
		SlideHeight: 5143500,
		LayoutBounds: patterns.LayoutBounds{
			X: 457200, Y: 457200,
			Width: 8229600, Height: 4229100,
		},
	}

	grid, _, err := expandPattern(pi, expandCtx, reg)
	if err != nil {
		return err
	}

	result := struct {
		Pattern   string                     `json:"pattern"`
		Version   int                        `json:"version"`
		ShapeGrid *jsonschema.ShapeGridInput `json:"shape_grid"`
	}{
		Pattern:   pat.Name(),
		Version:   pat.Version(),
		ShapeGrid: grid,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parsePatternInputFile parses a values file. It accepts two formats:
// 1. A full PatternInput object (with "values", "overrides", "cell_overrides" keys)
// 2. A bare values array/object (treated as the "values" field directly)
func parsePatternInputFile(content []byte, name string) (*PatternInput, error) {
	// Try full PatternInput first
	var pi PatternInput
	if err := json.Unmarshal(content, &pi); err == nil && len(pi.Values) > 0 {
		if pi.Name == "" {
			pi.Name = name
		}
		return &pi, nil
	}

	// Fall back to bare values
	pi = PatternInput{
		Name:   name,
		Values: json.RawMessage(content),
	}
	return &pi, nil
}

// unmarshalCellOverrides converts raw cell override JSON to typed overrides.
func unmarshalCellOverrides(pat patterns.Pattern, rawCO map[string]json.RawMessage) (map[int]any, error) {
	if len(rawCO) == 0 {
		return nil, nil
	}

	result := make(map[int]any, len(rawCO))
	for key, raw := range rawCO {
		idx, err := strconv.Atoi(key)
		if err != nil {
			return nil, fmt.Errorf("cell_overrides key %q is not an integer", key)
		}
		co := pat.NewCellOverride()
		if co == nil {
			return nil, fmt.Errorf("pattern %q does not support cell_overrides", pat.Name())
		}
		if err := json.Unmarshal(raw, co); err != nil {
			return nil, fmt.Errorf("invalid cell_overrides[%d]: %w", idx, err)
		}
		result[idx] = co
	}
	return result, nil
}

// emitValidationResult outputs a validation failure and returns an error to
// signal non-zero exit. In --json mode it uses D10 structured errors.
func emitValidationResult(name string, jsonMode bool, validationErr error) error {
	if jsonMode {
		result := struct {
			OK      bool                     `json:"ok"`
			Pattern string                   `json:"pattern"`
			Errors  []patternValidationError `json:"errors"`
		}{
			OK:      false,
			Pattern: name,
			Errors:  splitValidationErrors(validationErr),
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Fprintf(os.Stderr, "Pattern %q: validation failed\n  %s\n", name, validationErr)
	}
	return fmt.Errorf("validation failed")
}

// unknownPatternError returns a helpful error when a pattern name is not found.
func unknownPatternError(name string, reg *patterns.Registry) error {
	all := reg.List()
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = p.Name()
	}
	return fmt.Errorf("unknown pattern %q; available: %s\nHint: use `json2pptx patterns list` to see all patterns", name, strings.Join(names, ", "))
}
