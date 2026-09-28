package patterns

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func TestTeamBiosReadableBioBudgetScalesAtSecondRow(t *testing.T) {
	for members := 1; members <= 8; members++ {
		want := 220
		if members >= 5 {
			want = 40
		}
		if got := teamBiosReadableBioBudget(members); got != want {
			t.Errorf("budget(%d)=%d, want %d", members, got, want)
		}
	}
	schema := (&teamBios{}).Schema()
	bio := schema.raw.Properties["values"].raw.Properties["members"].raw.Items.raw.Properties["bio"]
	if bio.raw.MaxLength == nil || *bio.raw.MaxLength != 220 || !strings.Contains(bio.raw.Description, "40 with 5-8") {
		t.Errorf("schema loses sparse maximum or dense guidance: %+v", bio.raw)
	}
}

func TestTeamBiosWarningsUseMemberCountWithOrWithoutPhoto(t *testing.T) {
	pat := &teamBios{}
	v := &TeamBiosValues{}
	for i := 0; i < 4; i++ {
		v.Members = append(v.Members, TeamBiosMember{Name: "Jane Doe", Role: "Lead", Bio: "Short"})
	}
	v.Members[0].Bio = strings.Repeat("B", 220)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("four members at schema max warned: %v", got)
	}
	v.Members = append(v.Members, TeamBiosMember{Name: "Maya", Role: "Analyst", Bio: "Short"})
	v.Members[0].Bio = strings.Repeat("B", 40)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("five members at budget warned: %v", got)
	}
	v.Members[0].Bio += "x"
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "members[0].bio") || !strings.Contains(got[0], "about 40") {
		t.Fatalf("dense budget warning: %v", got)
	}
	v.Members[0].Photo = &jsonschema.GridImageInput{Path: "/tmp/headshot.png"}
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "about 40") {
		t.Fatalf("photo changed text-zone budget: %v", got)
	}
	// Two card rows also tighten the name and role lines.
	v.Members[0].Bio = "Short"
	v.Members[1].Name = strings.Repeat("N", teamBiosTwoRowNameBudget)
	v.Members[1].Role = strings.Repeat("R", teamBiosTwoRowRoleBudget)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("two-row name/role at budget warned: %v", got)
	}
	v.Members[1].Name += "N"
	v.Members[1].Role += "R"
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 2 || !strings.Contains(got[0], "members[1].name") || !strings.Contains(got[1], "members[1].role") {
		t.Fatalf("two-row name/role warnings: %v", got)
	}
}
