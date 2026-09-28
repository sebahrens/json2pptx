package patterns

import (
	"strings"
	"testing"
)

func TestArchStackDescriptionBudgets(t *testing.T) {
	p := &archStack{}
	v := &ArchStackValues{SideRails: []string{"Security", "Ops", "Data"}}
	for i := 0; i < 5; i++ {
		v.Tiers = append(v.Tiers, ArchStackTier{Label: "Tier", Description: "Brief"})
	}
	v.Tiers[2].Description = strings.Repeat("W", 40)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured unbroken target should fit: %v", got)
	}
	v.Tiers[2].Description += "W"
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "tiers[2].description") || !strings.Contains(got[0], "about 40 wide") {
		t.Fatalf("dense unbroken warning: %v", got)
	}
	v.Tiers[2].Description = strings.Repeat("word ", 9)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "about 40 readable") {
		t.Fatalf("worded warning: %v", got)
	}
	// Six tiers hold a label and no readable description.
	v.Tiers[2].Description = "Brief"
	v.Tiers = append(v.Tiers, ArchStackTier{Label: "Tier"})
	for i := range v.Tiers {
		v.Tiers[i].Description = ""
	}
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("six label-only tiers should fit: %v", got)
	}
	v.Tiers[2].Description = "Brief"
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "no readable description") {
		t.Fatalf("six-tier description warning: %v", got)
	}
	// Side rails hold about 26 wide characters.
	v.SideRails[0] = strings.Repeat("W", 27)
	v.Tiers[2].Description = ""
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "side_rails[0]") {
		t.Fatalf("rail unbroken warning: %v", got)
	}
	v.SideRails[0] = "Security"
	v.Tiers = v.Tiers[:4]
	v.Tiers[2].Description = strings.Repeat("W", 120)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("four-tier schema maximum should fit: %v", got)
	}
}
