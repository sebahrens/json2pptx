package rhythm

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestBreakPreferencesAreRegistered guards against a preference naming a
// pattern the registry does not know: suggestBreakPatterns ranks registry
// names only, so a misspelt preference is silently skipped and the next
// fallback moves up (go-slide-creator-s1uvj.16).
func TestBreakPreferencesAreRegistered(t *testing.T) {
	intents := []string{"chart", "table", "diagram", "process-flow", "image", "kpi", "card-grid", "comparison", ""}
	for _, intent := range intents {
		for _, name := range breakPreferences(intent) {
			if _, ok := patterns.Default().Get(name); !ok {
				t.Errorf("breakPreferences(%q) names unregistered pattern %q", intent, name)
			}
		}
	}
}
