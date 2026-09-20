package template

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-j4364: a layout that leaves its placeholder colour to the
// master's txStyles reported an empty FontColor, so the contrast preflight
// skipped it and validate stayed silent while generate swapped the colour.

func TestInheritedFontColorIsResolvedForAMasterStyledPlaceholder(t *testing.T) {
	layouts := parseBundledLayouts(t, "midnight-blue.pptx")
	title := placeholderOn(t, layouts, "slideLayout4", "title")
	if title.FontColor != "" {
		t.Fatalf("this test assumes the Section Divider title declares no colour of its own, got %q", title.FontColor)
	}
	if title.InheritedFontColor == "" {
		t.Error("InheritedFontColor is empty; the preflight has nothing to check and generate will swap silently")
	}
}

// FontColor keeps its narrower meaning — examine_template reports it to agents
// as what the template itself declares.
func TestFontColorStillMeansTheLayoutDeclaredIt(t *testing.T) {
	layouts := parseBundledLayouts(t, "midnight-blue.pptx")
	title := placeholderOn(t, layouts, "slideLayout4", "title")
	if title.FontColor != "" {
		t.Errorf("FontColor = %q; widening it would change what examine_template reports", title.FontColor)
	}
	// A placeholder that DOES declare one keeps it, and the inherited value
	// agrees rather than contradicting.
	declared := placeholderOn(t, layouts, "slideLayout2", "title")
	if declared.FontColor == "" {
		t.Fatal("this test assumes slideLayout2's title declares a colour")
	}
	if declared.InheritedFontColor != declared.FontColor {
		t.Errorf("declared %q but inherited resolves to %q", declared.FontColor, declared.InheritedFontColor)
	}
}

// The colour map is the part that cannot be skipped: modern-template's Section
// Divider maps tx1 -> lt1, so resolving against the theme alone would predict
// the opposite of what renders.
func TestColorMapOverrideIsApplied(t *testing.T) {
	override := map[string]string{"tx1": "lt1", "bg1": "dk1"}
	cases := []struct{ in, want string }{
		{"tx1", "lt1"},         // remapped
		{"accent1", "accent1"}, // not mentioned, passes through
		{"#123456", "#123456"}, // a literal has no scheme slot to remap
		{"", ""},               // nothing to resolve
	}
	for _, c := range cases {
		if got := inheritedPlaceholderColor(c.in, override); got != c.want {
			t.Errorf("inheritedPlaceholderColor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// No override at all is the common case and must not change anything.
	if got := inheritedPlaceholderColor("tx1", nil); got != "tx1" {
		t.Errorf("with no override, tx1 resolved to %q", got)
	}
}

// The parser must read the same element internal/generator reads, or the
// preflight and the render disagree about an inverted layout.
func TestParseLayoutColorMapOverrideReadsAnInvertedLayout(t *testing.T) {
	layouts := parseBundledLayouts(t, "modern-template.pptx")
	var sectionTitle string
	for i := range layouts {
		if layouts[i].ID != "slideLayout2" {
			continue
		}
		for pi := range layouts[i].Placeholders {
			if layouts[i].Placeholders[pi].Type == types.PlaceholderTitle {
				sectionTitle = layouts[i].Placeholders[pi].InheritedFontColor
			}
		}
	}
	// modern-template's Section Divider declares overrideClrMapping tx1="lt1".
	// Whatever the master states, the resolved value must not come back as a
	// tx/dk slot the override renames away from.
	if sectionTitle == "tx1" {
		t.Error("inherited colour came back as tx1; the layout's clrMapOvr renames it to lt1")
	}
}

func parseBundledLayouts(t *testing.T, name string) []types.LayoutMetadata {
	t.Helper()
	r, err := OpenTemplate("../../templates/" + name)
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer func() { _ = r.Close() }()
	layouts, err := ParseLayouts(r)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return layouts
}

func placeholderOn(t *testing.T, layouts []types.LayoutMetadata, layoutID, phID string) types.PlaceholderInfo {
	t.Helper()
	for i := range layouts {
		if layouts[i].ID != layoutID {
			continue
		}
		for pi := range layouts[i].Placeholders {
			if layouts[i].Placeholders[pi].ID == phID {
				return layouts[i].Placeholders[pi]
			}
		}
	}
	t.Fatalf("placeholder %q not found on %s", phID, layoutID)
	return types.PlaceholderInfo{}
}
