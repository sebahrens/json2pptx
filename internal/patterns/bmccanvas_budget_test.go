package patterns

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBMCCanvasCellBudgetsAndSchemaGuidance(t *testing.T) {
	pat := &bmcCanvas{}
	warn := func(v *BMCCanvasValues) []string {
		return pat.PostExpandWarnings(ExpandContext{}, v, nil)
	}
	base := pat.ExemplarValues().(*BMCCanvasValues)
	if got := warn(base); len(got) != 0 {
		t.Fatalf("exemplar unexpectedly warned: %v", got)
	}

	tests := []struct {
		name          string
		set           func(*BMCCanvasValues, []string)
		count, budget int
	}{
		{"tall", func(v *BMCCanvasValues, b []string) { v.KeyPartners.Bullets = b }, 3, 127},
		{"short", func(v *BMCCanvasValues, b []string) { v.KeyActivities.Bullets = b }, 3, 52},
		{"cost", func(v *BMCCanvasValues, b []string) { v.CostStructure.Bullets = b }, 3, 183},
		{"revenue", func(v *BMCCanvasValues, b []string) { v.RevenueStreams.Bullets = b }, 3, 121},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := *base
			bullets := []string{strings.Repeat("x", tc.budget), "short", "short"}
			tc.set(&v, bullets)
			if got := warn(&v); len(got) != 0 {
				t.Fatalf("at budget: %v", got)
			}
			bullets[0] += "x"
			got := warn(&v)
			if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) || !strings.Contains(got[0], fmt.Sprintf("about %d characters", tc.budget)) {
				t.Fatalf("above budget: %v", got)
			}
		})
	}

	v := *base
	v.Channels.Bullets = make([]string, 7)
	for i := range v.Channels.Bullets {
		v.Channels.Bullets[i] = "short"
	}
	if got := warn(&v); len(got) != 0 {
		t.Fatalf("seven short bullets should fit: %v", got)
	}
	v.Channels.Bullets = make([]string, 8)
	for i := range v.Channels.Bullets {
		v.Channels.Bullets[i] = "short"
	}
	got := warn(&v)
	if len(got) != 1 || !strings.Contains(got[0], "channels.bullets has 8 items") || !strings.Contains(got[0], "at most 7") {
		t.Fatalf("unreadable count warning: %v", got)
	}

	schema := pat.Schema()
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 6000 {
		t.Fatalf("schema is %d bytes; limit is 6000", len(data))
	}
	values := schema.raw.Properties["values"].raw.Properties
	for key, def := range map[string]string{"key_partners": "tallCell", "key_activities": "shortCell", "cost_structure": "costCell", "revenue_streams": "revenueCell"} {
		if values[key].raw.Ref != "#/$defs/"+def {
			t.Errorf("%s uses ref %q, want %s", key, values[key].raw.Ref, def)
		}
		bullets := schema.raw.Defs[def].raw.Properties["bullets"].raw
		if bullets.Items.raw.MaxLength == nil || *bullets.Items.raw.MaxLength != 200 || !strings.Contains(bullets.Description, "readable characters per bullet") {
			t.Errorf("%s lacks its 200-character schema cap and guidance: %+v", key, bullets)
		}
	}
	if schema.raw.Defs["tallCell"].raw.Properties["bullets"].raw.Description == schema.raw.Defs["shortCell"].raw.Properties["bullets"].raw.Description {
		t.Error("tall and short cells share guidance")
	}
}
