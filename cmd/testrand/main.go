// Command testrand generates random JSON decks for E2E fuzz testing of json2pptx.
//
// Usage:
//
//	testrand generate --seed=N --output=path.json
//	testrand validate --pptx=path.pptx --expected-slides=N
//
// All randomization is seed-based. Every run prints the seed so failures
// are 100% reproducible.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sebahrens/json2pptx/internal/testrand"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		cmdGenerate(os.Args[2:])
	case "visual":
		cmdVisual(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	case "svg-stress":
		cmdSVGStress(os.Args[2:])
	case "qa":
		cmdQA(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `testrand — random E2E test generator for json2pptx

Commands:
  generate    Generate a random JSON deck
  visual      Generate a systematic visual stress test deck
  validate    Validate a generated PPTX file
  svg-stress  Run SVG stress tests across all diagram types
  qa          Run AI-powered visual QA on slide images (requires ANTHROPIC_API_KEY)

Generate options:
  --seed=N        Seed for random generation (default: unix timestamp)
  --output=FILE   Output JSON file (default: stdout)
  --svg-dir=DIR   SVG icon directory for random icon testing (e.g. svg/)

Visual options:
  --template=NAME   Template name (default: warm-coral)
  --output=FILE     Output JSON file (default: stdout)

Validate options:
  --pptx=FILE             PPTX file to validate
  --expected-slides=N     Expected slide count (0 = skip check)

SVG stress options:
  --seed=N        Seed for random generation (default: unix timestamp)
  --type=TYPE     Test only this diagram type (default: all)
  --json          Output full JSON report

QA options:
  --images=DIR        Directory containing slide images (slide-0.jpg, etc.) [required]
  --json=FILE         JSON input file for slide type metadata
  --model=MODEL       Claude model override (default: claude-haiku-4-5-20251001)
  --parallel=N        Concurrent API calls (default: 4)
  --json-output       Output full JSON report
  --min-severity=SEV  Minimum severity to report: P0, P1, P2, P3 (default: P3)
`)
}

func cmdGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	seed := fs.Uint64("seed", 0, "random seed (0 = use current time)")
	output := fs.String("output", "", "output file (empty = stdout)")
	svgDir := fs.String("svg-dir", "", "SVG icon directory for random icon testing (e.g. svg/)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}

	g := testrand.New(*seed)
	if *svgDir != "" {
		g.WithSVGDir(*svgDir, 40)
	}
	data, err := g.GenerateJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Always print seed for reproducibility
	fmt.Fprintf(os.Stderr, "seed=%d template=%s\n", *seed, extractTemplate(data))

	if *output != "" {
		if err := os.WriteFile(*output, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR writing %s: %v\n", *output, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", *output, len(data))
	} else {
		os.Stdout.Write(data)
		os.Stdout.Write([]byte("\n"))
	}
}

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	pptx := fs.String("pptx", "", "PPTX file to validate")
	expected := fs.Int("expected-slides", 0, "expected slide count")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if *pptx == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --pptx is required")
		os.Exit(1)
	}

	result, err := testrand.ValidatePPTX(*pptx, *expected)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))

	if !result.Valid {
		os.Exit(1)
	}
}

func extractTemplate(data []byte) string {
	var p struct {
		Template string `json:"template"`
	}
	_ = json.Unmarshal(data, &p)
	return p.Template
}
