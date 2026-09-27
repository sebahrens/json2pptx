package patterns

import (
	"strings"
	"testing"
)

func TestArchStackDescriptionBudgets(t *testing.T) {
	p := &archStack{}
	v := &ArchStackValues{SideRails: []string{"Security", "Ops", "Data"}}
	for i := 0; i < 6; i++ {
		v.Tiers = append(v.Tiers, ArchStackTier{Label: "Tier", Description: "Brief"})
	}
	v.Tiers[2].Description = strings.Repeat("W", 53)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured unbroken target should fit: %v", got)
	}
	v.Tiers[2].Description += "W"
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "tiers[2].description") || !strings.Contains(got[0], "about 53 wide") {
		t.Fatalf("dense unbroken warning: %v", got)
	}
	v.Tiers[2].Description = strings.Repeat("word ", 24)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "about 117 readable") {
		t.Fatalf("worded warning: %v", got)
	}
	v.Tiers = v.Tiers[:4]
	v.Tiers[2].Description = strings.Repeat("W", 120)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("four-tier schema maximum should fit: %v", got)
	}
}
