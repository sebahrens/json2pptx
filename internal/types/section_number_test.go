package types

import "testing"

func TestIsUnnumberedSectionTitle(t *testing.T) {
	cases := map[string]bool{
		"Appendix":                        true,
		"appendix A: detailed financials": true,
		"Backup slides":                   true,
		"Back-up":                         true,
		"Annex":                           true,
		"Q&A":                             true,
		"Questions?":                      true,
		"Thank you":                       true,
		"Appendix — methodology":          true,
		"Annexation strategy":             false,
		"Market overview":                 false,
		"Questionnaire results":           false,
		"":                                false,
	}
	for title, want := range cases {
		if got := IsUnnumberedSectionTitle(title); got != want {
			t.Errorf("IsUnnumberedSectionTitle(%q) = %v, want %v", title, got, want)
		}
	}
}

func TestIsAppendixSectionTitle(t *testing.T) {
	cases := map[string]bool{
		"Appendix":                      true,
		"Appendix: Detailed financials": true,
		"Backup slides":                 true,
		"Annex":                         true,
		"Q&A":                           false,
		"Thank you":                     false,
		"Annexation strategy":           false,
		"Appendixes are long":           false,
		"Market overview":               false,
		"":                              false,
	}
	for title, want := range cases {
		if got := IsAppendixSectionTitle(title); got != want {
			t.Errorf("IsAppendixSectionTitle(%q) = %v, want %v", title, got, want)
		}
	}
}

func TestBackMatterMask(t *testing.T) {
	probes := []BackMatterProbe{
		{},                                     // cover
		{Divider: true},                        // chapter divider
		{Crumb: "Where we stand"},              // body
		{Divider: true, AppendixDivider: true}, // appendix divider
		{},                                     // backup page (flat deck)
		{Crumb: "Appendix: Detailed data"},     // backup page (structure deck)
		{Divider: true},                        // a later chapter ends back matter
		{},
	}
	got := BackMatterMask(probes)
	want := []bool{false, false, false, true, true, true, false, false}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slide %d: back matter = %v, want %v (mask %v)", i, got[i], want[i], got)
		}
	}
}
