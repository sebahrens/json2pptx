package semantic

import (
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

func TestRegionsDocumentationListsLiveKinds(t *testing.T) {
	data, err := os.ReadFile("../../docs/SEMANTIC_COMPILER.md")
	if err != nil {
		t.Fatal(err)
	}
	row := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "| `regions` |") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("semantic compiler catalogue has no regions row")
	}
	for _, kind := range slides.RegionKinds {
		if !strings.Contains(row, "`"+kind+"`") {
			t.Errorf("regions catalogue row omits %q", kind)
		}
	}
	if !strings.Contains(row, "3–6") || !strings.Contains(row, "3–8") {
		t.Error("regions catalogue must state process3–6 and cycle3–8 budgets")
	}
}
