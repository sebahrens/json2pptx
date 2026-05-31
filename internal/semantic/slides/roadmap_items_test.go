package slides

import "testing"

func TestFoldPhaseItems(t *testing.T) {
	cases := []struct {
		name  string
		desc  string
		items []string
		want  string
	}{
		{"none", "Define scope", nil, "Define scope"},
		{"itemsOnly", "", []string{"a", "b"}, "a · b"},
		{"both", "Lead-in", []string{"a", "b"}, "Lead-in · a · b"},
	}
	for _, c := range cases {
		if got := foldPhaseItems(c.desc, c.items); got != c.want {
			t.Errorf("%s: foldPhaseItems(%q,%v)=%q want %q", c.name, c.desc, c.items, got, c.want)
		}
	}
}

func TestPhaseItemsExtract(t *testing.T) {
	got := phaseItems(map[string]any{"items": []any{"a", " b ", "", 3}})
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
	if a := phaseItems(map[string]any{"bullets": []any{"x"}}); len(a) != 1 || a[0] != "x" {
		t.Errorf("bullets alias got %v want [x]", a)
	}
	if n := phaseItems(map[string]any{}); n != nil {
		t.Errorf("empty got %v want nil", n)
	}
}

func TestRoadmapPhasesThreadsItems(t *testing.T) {
	body := map[string]any{
		"phases": []any{
			map[string]any{"name": "Discovery", "items": []any{"Interviews", "Surveys"}},
			map[string]any{"name": "Build", "description": "Core build"},
		},
	}
	phases := roadmapPhases(body)
	if len(phases) != 2 {
		t.Fatalf("got %d phases want 2", len(phases))
	}
	if phases[0].Description != "Interviews · Surveys" {
		t.Errorf("phase0 desc = %q want %q", phases[0].Description, "Interviews · Surveys")
	}
	if phases[1].Description != "Core build" {
		t.Errorf("phase1 desc = %q want %q", phases[1].Description, "Core build")
	}
}
