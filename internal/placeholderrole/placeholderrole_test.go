package placeholderrole

import "testing"

func TestIsSectionNumberAlias(t *testing.T) {
	trueCases := []string{"section_number", "section_no", "large_number",
		"SECTION_NUMBER", " section_no ", "Large_Number"}
	for _, id := range trueCases {
		if !IsSectionNumberAlias(id) {
			t.Errorf("IsSectionNumberAlias(%q) = false, want true", id)
		}
	}

	falseCases := []string{"body", "title", "subtitle", "section", "number", ""}
	for _, id := range falseCases {
		if IsSectionNumberAlias(id) {
			t.Errorf("IsSectionNumberAlias(%q) = true, want false", id)
		}
	}
}

func TestContainsAnyHint(t *testing.T) {
	cases := []struct {
		id    string
		hints []string
		want  bool
	}{
		{"eyebrow text", EyebrowHints, true},
		{"kicker", EyebrowHints, true},
		{"subtitle", SubtitleHints, true},
		{"sub-title", SubtitleHints, true},
		{"footer left", FooterHints, true},
		{"slide_number", PageNumHints, true},
		{"date placeholder", DateHints, true},
		{"title", EyebrowHints, false},
		{"body", SubtitleHints, false},
	}
	for _, c := range cases {
		if got := ContainsAnyHint(c.id, c.hints); got != c.want {
			t.Errorf("ContainsAnyHint(%q, %v) = %v, want %v", c.id, c.hints, got, c.want)
		}
	}
}
