package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ahrens/go-slide-creator/internal/testrand"
)

func cmdSVGStress(args []string) {
	fs := flag.NewFlagSet("svg-stress", flag.ExitOnError)
	seed := fs.Uint64("seed", 0, "random seed (0 = use current time)")
	filterType := fs.String("type", "", "test only this diagram type")
	jsonOut := fs.Bool("json", false, "output full JSON report")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}

	fmt.Fprintf(os.Stderr, "svg-stress: seed=%d\n", *seed)

	runner := testrand.NewSVGStressRunner(*seed)
	report := runner.Run(*filterType)

	if *jsonOut {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Fprintf(os.Stderr, "total=%d passed=%d failed=%d\n",
			report.Total, report.Passed, report.Failed)

		for _, f := range report.Failures {
			fmt.Fprintf(os.Stderr, "  FAIL: %s/%s: %s\n", f.DiagramType, f.Variant, f.Error)
		}
	}

	if report.Failed > 0 {
		os.Exit(1)
	}
}
