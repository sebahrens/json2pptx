package generator

import (
	"strings"
	"testing"
)

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"fade", true},
		{"push", true},
		{"wipe", true},
		{"cover", true},
		{"uncover", true},
		{"cut", true},
		{"dissolve", true},
		{"none", false},
		{"slide", false},
		{"", false},
		{"FADE", true},  // case insensitive
		{"Push", true},  // case insensitive
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidTransition(tt.name)
			if got != tt.want {
				t.Errorf("IsValidTransition(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestNormalizeTransitionSpeed(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"slow", "slow"},
		{"fast", "fast"},
		{"med", "med"},
		{"medium", "med"},
		{"", "med"},
		{"unknown", "med"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeTransitionSpeed(tt.input)
			if got != tt.want {
				t.Errorf("normalizeTransitionSpeed(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildTransitionXML(t *testing.T) {
	tests := []struct {
		name      string
		transType string
		speed     string
		wantEmpty bool
		contains  []string
	}{
		{
			name:      "fade transition",
			transType: "fade",
			speed:     "med",
			contains:  []string{`<p:transition spd="med">`, `<p:fade/>`, `</p:transition>`},
		},
		{
			name:      "push transition slow",
			transType: "push",
			speed:     "slow",
			contains:  []string{`<p:transition spd="slow">`, `<p:push dir="l"/>`},
		},
		{
			name:      "wipe transition fast",
			transType: "wipe",
			speed:     "fast",
			contains:  []string{`<p:transition spd="fast">`, `<p:wipe dir="d"/>`},
		},
		{
			name:      "cover transition",
			transType: "cover",
			speed:     "",
			contains:  []string{`<p:transition spd="med">`, `<p:cover dir="l"/>`},
		},
		{
			name:      "cut transition",
			transType: "cut",
			speed:     "",
			contains:  []string{`<p:transition spd="med">`, `<p:cut/>`},
		},
		{
			name:      "dissolve transition",
			transType: "dissolve",
			speed:     "slow",
			contains:  []string{`<p:transition spd="slow">`, `<p:dissolve/>`},
		},
		{
			name:      "none returns empty",
			transType: "none",
			speed:     "",
			wantEmpty: true,
		},
		{
			name:      "empty returns empty",
			transType: "",
			speed:     "",
			wantEmpty: true,
		},
		{
			name:      "invalid returns empty",
			transType: "invalid",
			speed:     "",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTransitionXML(tt.transType, tt.speed)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("buildTransitionXML(%q, %q) = %q, want empty", tt.transType, tt.speed, got)
				}
				return
			}
			for _, c := range tt.contains {
				if !strings.Contains(got, c) {
					t.Errorf("buildTransitionXML(%q, %q) = %q, missing %q", tt.transType, tt.speed, got, c)
				}
			}
		})
	}
}

func TestBuildBulletBuildTimingXML(t *testing.T) {
	got := buildBulletBuildTimingXML()
	if got == "" {
		t.Fatal("buildBulletBuildTimingXML() returned empty string")
	}

	// Verify key elements are present
	required := []string{
		"<p:timing>",
		"</p:timing>",
		"<p:tnLst>",
		"<p:bldLst>",
		`<p:bldP spId="3" grpId="0" build="p"/>`,
		`nodeType="tmRoot"`,
		`presetClass="entr"`,
	}
	for _, r := range required {
		if !strings.Contains(got, r) {
			t.Errorf("buildBulletBuildTimingXML() missing %q", r)
		}
	}
}

func TestInsertTransitionAndBuild(t *testing.T) {
	baseSlide := []byte(`<p:sld>
  <p:cSld>
    <p:spTree></p:spTree>
  </p:cSld>
  <p:clrMapOvr>
    <a:masterClrMapping/>
  </p:clrMapOvr>
</p:sld>`)

	t.Run("no transition or build", func(t *testing.T) {
		got := insertTransitionAndBuild(baseSlide, "", "", "")
		if string(got) != string(baseSlide) {
			t.Errorf("expected no change, got %q", got)
		}
	})

	t.Run("fade transition only", func(t *testing.T) {
		got := insertTransitionAndBuild(baseSlide, "fade", "med", "")
		s := string(got)
		if !strings.Contains(s, `<p:transition spd="med">`) {
			t.Error("missing transition element")
		}
		if !strings.Contains(s, `<p:fade/>`) {
			t.Error("missing fade element")
		}
		// Verify transition comes before clrMapOvr
		transIdx := strings.Index(s, "<p:transition")
		clrIdx := strings.Index(s, "<p:clrMapOvr")
		if transIdx >= clrIdx {
			t.Error("transition should come before clrMapOvr")
		}
	})

	t.Run("build only", func(t *testing.T) {
		got := insertTransitionAndBuild(baseSlide, "", "", "bullets")
		s := string(got)
		if !strings.Contains(s, "<p:timing>") {
			t.Error("missing timing element")
		}
		if !strings.Contains(s, "<p:bldLst>") {
			t.Error("missing bldLst element")
		}
	})

	t.Run("both transition and build", func(t *testing.T) {
		got := insertTransitionAndBuild(baseSlide, "push", "fast", "bullets")
		s := string(got)
		if !strings.Contains(s, `<p:transition spd="fast">`) {
			t.Error("missing transition element")
		}
		if !strings.Contains(s, "<p:timing>") {
			t.Error("missing timing element")
		}
		// Transition should come before timing
		transIdx := strings.Index(s, "<p:transition")
		timingIdx := strings.Index(s, "<p:timing>")
		if transIdx >= timingIdx {
			t.Error("transition should come before timing")
		}
	})

	t.Run("fallback to closing sld tag", func(t *testing.T) {
		noClrMap := []byte(`<p:sld>
  <p:cSld>
    <p:spTree></p:spTree>
  </p:cSld>
</p:sld>`)
		got := insertTransitionAndBuild(noClrMap, "fade", "", "")
		s := string(got)
		if !strings.Contains(s, "<p:transition") {
			t.Error("missing transition element in fallback case")
		}
	})
}
