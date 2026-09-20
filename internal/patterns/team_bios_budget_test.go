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
			want = 141
		}
		if got := teamBiosReadableBioBudget(members); got != want {
			t.Errorf("budget(%d)=%d, want %d", members, got, want)
		}
	}
	schema := (&teamBios{}).Schema()
	bio := schema.raw.Properties["values"].raw.Properties["members"].raw.Items.raw.Properties["bio"]
	if bio.raw.MaxLength == nil || *bio.raw.MaxLength != 220 || !strings.Contains(bio.raw.Description, "141 with 5-8") {
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
	v.Members[0].Bio = strings.Repeat("B", 141)
	if got := pat.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("five members at budget warned: %v", got)
	}
	v.Members[0].Bio += "x"
	got := pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "members[0].bio") || !strings.Contains(got[0], "about 141") {
		t.Fatalf("dense budget warning: %v", got)
	}
	v.Members[0].Photo = &jsonschema.GridImageInput{Path: "/tmp/headshot.png"}
	got = pat.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "about 141") {
		t.Fatalf("photo changed text-zone budget: %v", got)
	}
}
