package generator

import (
	"regexp"
	"strings"
	"testing"
)

// panel_layout's documented shape is {title, body}, so an agent following the
// data-format hints writes {title: "ARR", body: "EUR 184m"} — which drew "ARR"
// at 32pt and the money as the small caption, the exact inverse of the sibling
// stat_cards diagram type fed the same content (go-slide-creator-3j88).
func TestStatCardParts(t *testing.T) {
	cases := []struct {
		name        string
		panel       nativePanelData
		hero        string
		caption     string
		bodyOrDelta string
	}{
		{
			name:    "a documented {title, body} card puts the number first",
			panel:   nativePanelData{title: "ARR", body: "EUR 184m"},
			hero:    "EUR 184m",
			caption: "ARR",
		},
		{
			name:        "an explicit value still wins and keeps its body line",
			panel:       nativePanelData{title: "ARR", value: "EUR 184m", body: "+12% YoY"},
			hero:        "EUR 184m",
			caption:     "ARR",
			bodyOrDelta: "+12% YoY",
		},
		{
			name:        "a prose body stays under the title",
			panel:       nativePanelData{title: "Discovery", body: "Interviews with twelve operators across three regions"},
			hero:        "Discovery",
			bodyOrDelta: "Interviews with twelve operators across three regions",
		},
		{
			name:        "a short body with no digit is a label, not a statistic",
			panel:       nativePanelData{title: "Phase 1", body: "Discovery"},
			hero:        "Phase 1",
			bodyOrDelta: "Discovery",
		},
		{
			name:  "a title alone is the hero",
			panel: nativePanelData{title: "ARR"},
			hero:  "ARR",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hero, caption, body := statCardParts(tc.panel)
			if hero != tc.hero || caption != tc.caption || body != tc.bodyOrDelta {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)",
					hero, caption, body, tc.hero, tc.caption, tc.bodyOrDelta)
			}
		})
	}
}

// heroRunRe captures the text of every run drawn at the stat card's hero size.
var heroRunRe = regexp.MustCompile(`sz="3200"[^>]*/?>(?:<[^>]+>)*?<a:t>([^<]*)</a:t>`)

// The two paths that render a stat card — panel_layout's {title, body} and the
// stat_cards type's {title, value} — must agree about which field is the 32pt
// number.
func TestStatCardPathsAgreeOnTheHero(t *testing.T) {
	fromBody := generateStatCardXML(nativePanelData{title: "ARR", body: "EUR 184m"}, 0, 0, 2000000, 1000000, 10)
	fromValue := generateStatCardXML(nativePanelData{title: "ARR", value: "EUR 184m"}, 0, 0, 2000000, 1000000, 10)

	for name, xml := range map[string]string{"panel_layout {title, body}": fromBody, "stat_cards {title, value}": fromValue} {
		hero := heroRunRe.FindStringSubmatch(xml)
		if hero == nil {
			t.Fatalf("%s: no hero run at 32pt in %q", name, xml)
		}
		if hero[1] != "EUR 184m" {
			t.Errorf("%s: hero is %q, want the number", name, hero[1])
		}
		if !strings.Contains(xml, `<a:t>ARR</a:t>`) {
			t.Errorf("%s: the caption is missing", name)
		}
	}
}
