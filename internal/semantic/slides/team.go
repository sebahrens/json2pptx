package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Team slides (go-slide-creator-13lj).
//
// "Our people" is a fixture of every proposal and engagement deck, and the
// DeckSpec had no kind for it: an agent either dropped to raw_json2pptx or
// listed the team as bullets. The payload here is what an author writes — a
// list of people with a role and, optionally, a line about them — and the
// compiler maps it onto the team-bios pattern.

const (
	// teamMaxMembers mirrors the pattern's card limit.
	teamMaxMembers = 8
	// teamNameMax / teamRoleMax / teamBioMax mirror its string budgets.
	teamNameMax = 60
	teamRoleMax = 80
	teamBioMax  = 220
	// teamPhotoLabelMax is the initials badge; longer labels are dropped rather
	// than truncated, since a clipped set of initials is meaningless.
	teamPhotoLabelMax = 8
)

// teamMember is one resolved person.
type teamMember struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Bio        string `json:"bio,omitempty"`
	PhotoLabel string `json:"photo_label,omitempty"`
}

// teamBiosValues is the team-bios pattern's values object.
type teamBiosValues struct {
	Members []teamMember `json:"members"`
}

// CompileTeam compiles a team payload onto the team-bios pattern, falling back
// to a bullet list when the payload does not fit the pattern's budgets.
func CompileTeam(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	members := TeamMembers(in.Body)
	if !teamBiosFits(members) {
		return compileTeamFallback(in, members)
	}

	encoded, err := json.Marshal(teamBiosValues{Members: members})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal team-bios values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "team-bios", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.members",
		SemanticPath: in.semSlide() + "." + teamMembersField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileTeamFallback renders the team as "Name — Role: bio" bullets so a large
// or over-long roster is not truncated into the cards.
func compileTeamFallback(in Input, members []teamMember) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := make([]string, 0, len(members))
	for _, m := range members {
		line := m.Name
		if m.Role != "" {
			line += " — " + m.Role
		}
		if m.Bio != "" {
			line += ": " + m.Bio
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{
		RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + "." + teamMembersField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// TeamMembers resolves a team payload's people. A member is a string (a bare
// name), or an object carrying a name and optionally a role, a bio and a photo
// label. Entries with no usable name are dropped.
func TeamMembers(body map[string]any) []teamMember {
	raw, ok := firstList(body, "members", "people", "team")
	if !ok {
		return nil
	}
	var out []teamMember
	for _, e := range raw {
		switch m := e.(type) {
		case string:
			if n := strings.TrimSpace(m); n != "" {
				out = append(out, teamMember{Name: n})
			}
		case map[string]any:
			name := firstNonEmpty(strField(m, "name"), strField(m, "title"), strField(m, "person"))
			if name == "" {
				continue
			}
			out = append(out, teamMember{
				Name:       name,
				Role:       firstNonEmpty(strField(m, "role"), strField(m, "position"), strField(m, "job_title")),
				Bio:        firstNonEmpty(strField(m, "bio"), strField(m, "description"), strField(m, "summary")),
				PhotoLabel: teamPhotoLabel(m),
			})
		}
	}
	return out
}

// teamPhotoLabel reads the initials badge, dropping a value the badge cannot
// hold rather than truncating it into nonsense.
func teamPhotoLabel(m map[string]any) string {
	label := firstNonEmpty(strField(m, "photo_label"), strField(m, "initials"))
	if runeLen(label) > teamPhotoLabelMax {
		return ""
	}
	return label
}

// teamMembersField names the payload field the members came from.
func teamMembersField(body map[string]any) string {
	for _, key := range []string{"members", "people", "team"} {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "members"
}

// teamBiosFits reports whether the roster fits the pattern: at least one
// person, at most eight, each inside the card's text budgets.
func teamBiosFits(members []teamMember) bool {
	if len(members) == 0 || len(members) > teamMaxMembers {
		return false
	}
	for _, m := range members {
		if m.Role == "" {
			// The pattern requires a role: a card with a blank line where the
			// title belongs reads as missing data.
			return false
		}
		if runeLen(m.Name) > teamNameMax || runeLen(m.Role) > teamRoleMax || runeLen(m.Bio) > teamBioMax {
			return false
		}
	}
	return true
}

// TeamPattern returns the pattern a team payload compiles to, or "" when it
// degrades to bullets.
func TeamPattern(body map[string]any) string {
	if teamBiosFits(TeamMembers(body)) {
		return "team-bios"
	}
	return ""
}

// TeamOverBudget explains why a roster cannot take the cards, for the finding
// that reports the degrade. Returns "" when it fits.
func TeamOverBudget(body map[string]any) string {
	members := TeamMembers(body)
	switch {
	case len(members) == 0:
		return ""
	case len(members) > teamMaxMembers:
		return fmt.Sprintf("has %d people; team-bios draws at most %d", len(members), teamMaxMembers)
	}
	for i, m := range members {
		switch {
		case m.Role == "":
			return fmt.Sprintf("member %d (%s) has no role; every card needs one", i+1, m.Name)
		case runeLen(m.Name) > teamNameMax:
			return fmt.Sprintf("member %d's name is %d characters; the card holds %d", i+1, runeLen(m.Name), teamNameMax)
		case runeLen(m.Role) > teamRoleMax:
			return fmt.Sprintf("member %d's role is %d characters; the card holds %d", i+1, runeLen(m.Role), teamRoleMax)
		case runeLen(m.Bio) > teamBioMax:
			return fmt.Sprintf("member %d's bio is %d characters; the card holds %d", i+1, runeLen(m.Bio), teamBioMax)
		}
	}
	return ""
}

// UsableTeamMemberCount returns how many people survive extraction.
func UsableTeamMemberCount(body map[string]any) int { return len(TeamMembers(body)) }
