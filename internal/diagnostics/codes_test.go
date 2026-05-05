package diagnostics

import (
	"regexp"
	"testing"
)

// TestAllCodesAreScreamingSnake verifies that every declared code follows
// SCREAMING_SNAKE_CASE convention.
func TestAllCodesAreScreamingSnake(t *testing.T) {
	re := regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)
	for _, code := range AllCodes() {
		if !re.MatchString(code) {
			t.Errorf("code %q is not SCREAMING_SNAKE_CASE", code)
		}
	}
}

// TestAllCodesUnique ensures no duplicate codes in AllCodes().
func TestAllCodesUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, code := range AllCodes() {
		if seen[code] {
			t.Errorf("duplicate code: %q", code)
		}
		seen[code] = true
	}
}
