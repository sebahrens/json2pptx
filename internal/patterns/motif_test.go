package patterns

import (
	"encoding/json"
	"testing"
)

// TestEveryPatternDeclaresMotif fails when a pattern is registered without a
// motif, or the table names a pattern that does not exist
// (go-slide-creator-rd7oj).
func TestEveryPatternDeclaresMotif(t *testing.T) {
	registered := map[string]bool{}
	for _, p := range Default().List() {
		registered[p.Name()] = true
		m := PatternMotif(p.Name())
		if m == "" {
			t.Errorf("pattern %q declares no motif: add it to patternMotifs in motif.go (render its exemplar and name what it draws)", p.Name())
			continue
		}
		if !m.Valid() {
			t.Errorf("pattern %q declares unknown motif %q", p.Name(), m)
		}
	}
	for name, decl := range patternMotifs {
		if !registered[name] {
			t.Errorf("patternMotifs names %q, which is not a registered pattern", name)
		}
		for style, m := range decl.overrideStyles {
			if !m.Valid() {
				t.Errorf("%s overrides.style %q maps to unknown motif %q", name, style, m)
			}
		}
		for style, m := range decl.valueStyles {
			if !m.Valid() {
				t.Errorf("%s values.style %q maps to unknown motif %q", name, style, m)
			}
		}
	}
}

// TestMotifStyleKeysAreSchemaStyles keeps the style keys honest: every style
// a declaration switches on must be a value the pattern's schema accepts, so a
// renamed style cannot silently stop changing the motif.
func TestMotifStyleKeysAreSchemaStyles(t *testing.T) {
	for name, decl := range patternMotifs {
		p, ok := Default().Get(name)
		if !ok {
			continue
		}
		raw, err := json.Marshal(p.Schema())
		if err != nil {
			t.Fatalf("%s: marshal schema: %v", name, err)
		}
		var schema struct {
			Properties map[string]struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("%s: decode schema: %v", name, err)
		}
		check := func(section string, styles map[string]Motif) {
			if len(styles) == 0 {
				return
			}
			enum := schema.Properties[section].Properties["style"].Enum
			for style := range styles {
				found := false
				for _, e := range enum {
					found = found || e == style
				}
				if !found {
					t.Errorf("%s: %s.style %q is not in the schema enum %v", name, section, style, enum)
				}
			}
		}
		check("overrides", decl.overrideStyles)
		check("values", decl.valueStyles)
	}
}

func TestMotifForFollowsExplicitStyle(t *testing.T) {
	cases := []struct {
		name, values, overrides string
		want                    Motif
	}{
		{"stylish-panels", "", "", MotifOpenColumns},
		{"stylish-panels", "", `{"style":"ribbon"}`, MotifTiles},
		{"stylish-panels", "", `{"style":"open"}`, MotifOpenColumns},
		{"kpi-4up", "", "", MotifOpenColumns},
		{"kpi-4up", "", `{"style":"tiles"}`, MotifTiles},
		{"card-grid", "", "", MotifTiles},
		{"comparison-2col", "", `{"style":"tiles"}`, MotifTiles},
		{"numbered-step-strip", `{"style":"chevron"}`, "", MotifFlow},
		{"numbered-step-strip", `{"style":"toc"}`, "", MotifOpenList},
		{"table-highlight", "", "", MotifTable},
		{"waterfall-bridge", "", "", MotifChart},
		{"stat-hero", "", "", MotifHeroNumber},
		{"no-such-pattern", "", "", ""},
	}
	for _, c := range cases {
		got := MotifFor(c.name, json.RawMessage(c.values), json.RawMessage(c.overrides))
		if got != c.want {
			t.Errorf("MotifFor(%s, values=%s, overrides=%s) = %q, want %q", c.name, c.values, c.overrides, got, c.want)
		}
	}
}
