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
