package slides

import (
	"strings"
	"testing"
)

func person(name, role, bio string) map[string]any {
	m := map[string]any{"name": name}
	if role != "" {
		m["role"] = role
	}
	if bio != "" {
		m["bio"] = bio
	}
	return m
}

func teamBody(members ...any) map[string]any {
	return map[string]any{"title": "Who you will be working with", "members": members}
}

// "Our people" is a fixture of every proposal deck and the DeckSpec had no kind
// for it, so an agent dropped to raw_json2pptx or listed the team as bullets
// (go-slide-creator-13lj).
func TestCompileTeamUsesTheCardsWhenTheRosterFits(t *testing.T) {
	body := teamBody(
		person("Amara Okafor", "Engagement partner", "Led the settlement migration."),
		person("Jonas Weber", "Delivery lead", "Runs the cutover rehearsals."),
		person("Priya Raman", "Data lead", ""),
	)
	if got := TeamPattern(body); got != "team-bios" {
		t.Fatalf("TeamPattern = %q, want team-bios", got)
	}
	slide, _, err := CompileTeam(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "team-bios" {
		t.Fatalf("pattern = %+v, want team-bios", slide.Pattern)
	}
	values := string(slide.Pattern.Values)
	for _, want := range []string{"Amara Okafor", "Engagement partner", "Led the settlement migration."} {
		if !strings.Contains(values, want) {
			t.Errorf("values lost %q: %s", want, values)
		}
	}
}

// Past the pattern's budgets the roster degrades to bullets rather than being
// truncated into the cards, and the finding says which budget broke.
func TestTeamDegradesWithAReason(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		reason string
	}{
		{
			name: "nine people",
			body: teamBody(
				person("A", "Partner", ""), person("B", "Partner", ""), person("C", "Partner", ""),
				person("D", "Partner", ""), person("E", "Partner", ""), person("F", "Partner", ""),
				person("G", "Partner", ""), person("H", "Partner", ""), person("I", "Partner", ""),
			),
			reason: "at most 8",
		},
		{
			name:   "a member with no role",
			body:   teamBody(person("Amara Okafor", "", "Led the migration."), person("Jonas Weber", "Delivery lead", "")),
			reason: "has no role",
		},
		{
			name:   "an over-long bio",
			body:   teamBody(person("Amara Okafor", "Partner", strings.Repeat("a", 260))),
			reason: "bio is 260 characters",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TeamPattern(c.body); got != "" {
				t.Errorf("TeamPattern = %q, want the bullet fallback", got)
			}
			if over := TeamOverBudget(c.body); !strings.Contains(over, c.reason) {
				t.Errorf("TeamOverBudget = %q, want it to mention %q", over, c.reason)
			}
			slide, _, err := CompileTeam(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if slide.Pattern != nil {
				t.Errorf("expected the bullet fallback, got pattern %s", slide.Pattern.Name)
			}
			if len(slide.Content) == 0 {
				t.Error("the fallback lost the roster entirely")
			}
		})
	}
}

// A roster inside the budgets reports nothing.
func TestTeamWithinBudgetsReportsNothing(t *testing.T) {
	body := teamBody(person("Amara Okafor", "Engagement partner", "Led the migration."))
	if over := TeamOverBudget(body); over != "" {
		t.Errorf("a one-person roster reported %q", over)
	}
	// An empty roster is the required-field gate's business, not the budget's.
	if over := TeamOverBudget(map[string]any{"members": []any{}}); over != "" {
		t.Errorf("an empty roster reported a budget problem: %q", over)
	}
}

// The aliases an author reaches for resolve, and an entry with no name is
// dropped rather than rendering a blank card.
func TestTeamMemberAliases(t *testing.T) {
	for _, field := range []string{"members", "people", "team"} {
		body := map[string]any{field: []any{person("Amara Okafor", "Partner", "")}}
		if n := len(TeamMembers(body)); n != 1 {
			t.Errorf("%s: resolved %d members, want 1", field, n)
		}
	}
	for _, key := range []string{"role", "position", "job_title"} {
		body := map[string]any{"members": []any{map[string]any{"name": "Amara", key: "Partner"}}}
		got := TeamMembers(body)
		if len(got) != 1 || got[0].Role != "Partner" {
			t.Errorf("%s: resolved %+v", key, got)
		}
	}
	body := map[string]any{"members": []any{person("Amara", "Partner", ""), map[string]any{"role": "orphan"}}}
	if n := len(TeamMembers(body)); n != 1 {
		t.Errorf("resolved %d members, want the 1 with a name", n)
	}
}

// A photo label the badge cannot hold is dropped rather than clipped: half a
// set of initials means nothing.
func TestTeamPhotoLabelTooLongIsDropped(t *testing.T) {
	body := map[string]any{"members": []any{
		map[string]any{"name": "Amara Okafor", "role": "Partner", "photo_label": "AMARA OKAFOR"},
		map[string]any{"name": "Jonas Weber", "role": "Lead", "initials": "JW"},
	}}
	got := TeamMembers(body)
	if len(got) != 2 {
		t.Fatalf("resolved %d members", len(got))
	}
	if got[0].PhotoLabel != "" {
		t.Errorf("an over-long photo label survived as %q", got[0].PhotoLabel)
	}
	if got[1].PhotoLabel != "JW" {
		t.Errorf("initials = %q, want JW", got[1].PhotoLabel)
	}
}
