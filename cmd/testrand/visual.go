package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/testrand"
)

func cmdVisual(args []string) {
	fs := flag.NewFlagSet("visual", flag.ExitOnError)
	template := fs.String("template", "warm-coral", "template name (without .pptx)")
	output := fs.String("output", "", "output file (empty = stdout)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	g := testrand.NewVisualDeckGenerator(*template)
	data, err := g.GenerateJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	p := g.Generate()
	fmt.Fprintf(os.Stderr, "visual: template=%s slides=%d\n", *template, len(p.Slides))

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
