package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

func main() {
	var agent, out string
	flag.StringVar(&agent, "agent", "", "agent executable plus optional space-separated arguments")
	flag.StringVar(&out, "out", "output/quality-benchmark", "evidence directory")
	flag.Parse()
	if agent == "" {
		fmt.Fprintln(os.Stderr, "--agent is required; no provider calls occur by default")
		os.Exit(2)
	}
	briefData, err := os.ReadFile("tests/quality/agent_briefs.json")
	if err != nil {
		panic(err)
	}
	var briefs []qualitybench.Brief
	if err = json.Unmarshal(briefData, &briefs); err != nil {
		panic(err)
	}
	r := qualitybench.Runner{Agent: strings.Fields(agent), OutputDir: out, Briefs: briefs, Templates: []qualitybench.Template{{Name: "midnight-blue", Family: "corporate"}, {Name: "clean-white", Family: "editorial"}, {Name: "heldout-seven-layout", Family: "independent-seven-layout", HeldOut: true}}, Repetitions: 2}
	report, err := r.Run(context.Background())
	if err != nil {
		panic(err)
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(out+"/report.json", append(data, '\n'), 0o644)
	fmt.Printf("wrote %d run evidence files to %s\n", len(report.Evidence), out)
}
