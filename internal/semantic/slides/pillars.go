package slides

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

type pillarItem struct {
	Title string   `json:"title"`
	Body  []string `json:"body,omitempty"`
}

type strategyHouseValues struct {
	Objective  string       `json:"objective"`
	Pillars    []pillarItem `json:"pillars"`
	Foundation string       `json:"foundation"`
	RoofBadges []string     `json:"roof_badges,omitempty"`
}

type pillarsResolution struct {
	pattern       string
	items         []pillarItem
	badges        []string
	path, problem string
	hard          bool
}

// resolvePillars shares the visual decision among plan, validation, and compile.
func resolvePillars(body map[string]any) pillarsResolution {
	raw, ok := body["pillars"].([]any)
	if !ok {
		return pillarsResolution{path: "pillars", problem: "pillars must be an array", hard: true}
	}
	if len(raw) == 0 {
		return pillarsResolution{path: "pillars", problem: "provide at least one named pillar", hard: true}
	}
	r := pillarsResolution{}
	for i, entry := range raw {
		item, path, problem, hard := parsePillar(entry, i)
		if problem != "" {
			return pillarsResolution{path: path, problem: problem, hard: hard}
		}
		r.items = append(r.items, item)
	}
	if len(r.items) < 3 || len(r.items) > 5 {
		return pillarsResolution{path: "pillars", problem: fmt.Sprintf("%d pillars exceed the 3–5 visual range", len(r.items))}
	}
	objective, objectivePresent := body["objective"].(string)
	foundation, foundationPresent := body["foundation"].(string)
	if _, present := body["objective"]; present && !objectivePresent {
		return pillarsResolution{path: "objective", problem: "objective must be a string", hard: true}
	}
	if _, present := body["foundation"]; present && !foundationPresent {
		return pillarsResolution{path: "foundation", problem: "foundation must be a string", hard: true}
	}
	objective = strings.TrimSpace(objective)
	foundation = strings.TrimSpace(foundation)
	house := objective != "" && foundation != ""
	if (objective != "") != (foundation != "") {
		return pillarsResolution{path: "objective", problem: "objective and foundation must be supplied together for a strategy house"}
	}
	if v, present := body["roof_badges"]; present {
		badges, badgeIssue := parseRoofBadges(v)
		if badgeIssue.problem != "" {
			return badgeIssue
		}
		r.badges = badges
	}
	if !house && len(r.badges) > 0 {
		return pillarsResolution{path: "roof_badges", problem: "roof badges need both objective and foundation"}
	}
	if house {
		if utf8.RuneCountInString(objective) > 140 {
			return pillarsResolution{path: "objective", problem: "objective exceeds 140 characters"}
		}
		if utf8.RuneCountInString(foundation) > 140 {
			return pillarsResolution{path: "foundation", problem: "foundation exceeds 140 characters"}
		}
		for i, item := range r.items {
			if field, problem := pillarBudget(item, i, 60, 5, 120, false); problem != "" {
				return pillarsResolution{path: field, problem: problem}
			}
		}
		r.pattern = "strategy-house"
	} else {
		for i, item := range r.items {
			if field, problem := pillarBudget(item, i, 80, 8, 200, true); problem != "" {
				return pillarsResolution{path: field, problem: problem}
			}
		}
		r.pattern = "stylish-panels"
	}
	return r
}

func parseRoofBadges(v any) ([]string, pillarsResolution) {
	raw, ok := v.([]any)
	if !ok {
		return nil, pillarsResolution{path: "roof_badges", problem: "roof_badges must be an array", hard: true}
	}
	if len(raw) > 3 {
		return nil, pillarsResolution{path: "roof_badges", problem: "roof_badges exceed 3 items"}
	}
	var badges []string
	for i, entry := range raw {
		s, ok := entry.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, pillarsResolution{path: fmt.Sprintf("roof_badges[%d]", i), problem: "badge must be a non-empty string", hard: true}
		}
		if utf8.RuneCountInString(s) > 24 {
			return nil, pillarsResolution{path: fmt.Sprintf("roof_badges[%d]", i), problem: "badge exceeds 24 characters"}
		}
		badges = append(badges, strings.TrimSpace(s))
	}
	return badges, pillarsResolution{}
}

func parsePillar(entry any, i int) (pillarItem, string, string, bool) {
	path := fmt.Sprintf("pillars[%d]", i)
	obj, ok := entry.(map[string]any)
	if !ok {
		return pillarItem{}, path, "pillar must be an object", true
	}
	title, ok := obj["title"].(string)
	if !ok || strings.TrimSpace(title) == "" {
		return pillarItem{}, path + ".title", "title must be a non-empty string", true
	}
	item := pillarItem{Title: strings.TrimSpace(title)}
	if v, present := obj["body"]; present {
		raw, ok := v.([]any)
		if !ok {
			return pillarItem{}, path + ".body", "body must be an array", true
		}
		for j, entry := range raw {
			s, ok := entry.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return pillarItem{}, fmt.Sprintf("%s.body[%d]", path, j), "body item must be a non-empty string", true
			}
			item.Body = append(item.Body, strings.TrimSpace(s))
		}
	}
	return item, "", "", false
}

func pillarBudget(item pillarItem, i, titleMax, bodyMax, bulletMax int, bodyRequired bool) (string, string) {
	base := fmt.Sprintf("pillars[%d]", i)
	if utf8.RuneCountInString(item.Title) > titleMax {
		return base + ".title", fmt.Sprintf("pillar title exceeds %d characters", titleMax)
	}
	if bodyRequired && len(item.Body) == 0 {
		return base + ".body", "panel pillar needs at least one body item"
	}
	if len(item.Body) > bodyMax {
		return base + ".body", fmt.Sprintf("pillar body exceeds %d items", bodyMax)
	}
	for j, s := range item.Body {
		if utf8.RuneCountInString(s) > bulletMax {
			return fmt.Sprintf("%s.body[%d]", base, j), fmt.Sprintf("pillar body item exceeds %d characters", bulletMax)
		}
	}
	return "", ""
}

func PillarsPattern(body map[string]any) string { return resolvePillars(body).pattern }
func PillarsIssue(body map[string]any) (string, string, bool) {
	r := resolvePillars(body)
	return r.path, r.problem, r.hard
}

func CompilePillars(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	if in.Layout == "content" {
		return compilePillarsFallback(in)
	}
	r := resolvePillars(in.Body)
	if r.pattern == "" {
		return compilePillarsFallback(in)
	}
	var values any = r.items
	if r.pattern == "strategy-house" {
		values = strategyHouseValues{Objective: strField(in.Body, "objective"), Pillars: r.items, Foundation: strField(in.Body, "foundation"), RoofBadges: r.badges}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal %s values: %w", r.pattern, err)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: r.pattern, Values: encoded}
	links = append(links, SourceLink{RawPath: in.rawSlide() + ".pattern.values.pillars", SemanticPath: in.semSlide() + ".pillars"})
	if r.pattern == "stylish-panels" {
		links[len(links)-1].RawPath = in.rawSlide() + ".pattern.values"
	}
	for _, field := range []string{"objective", "foundation", "roof_badges"} {
		links = append(links, SourceLink{RawPath: in.rawSlide() + ".pattern.values." + field, SemanticPath: in.semSlide() + "." + field})
	}
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

func compilePillarsFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := []string{}
	if v, ok := in.Body["objective"]; ok {
		bullets = append(bullets, "Objective: "+fmt.Sprint(v))
	}
	if badges, ok := in.Body["roof_badges"]; ok {
		bullets = append(bullets, "Roof badges: "+fmt.Sprint(badges))
	}
	if raw, ok := in.Body["pillars"].([]any); ok {
		for _, entry := range raw {
			if obj, ok := entry.(map[string]any); ok {
				line := fmt.Sprint(obj["title"])
				if body, ok := obj["body"].([]any); ok {
					for _, b := range body {
						line += " — " + fmt.Sprint(b)
					}
				} else if body, present := obj["body"]; present {
					line += " — " + fmt.Sprint(body)
				}
				bullets = append(bullets, line)
			} else {
				bullets = append(bullets, fmt.Sprint(entry))
			}
		}
	} else if raw, present := in.Body["pillars"]; present {
		bullets = append(bullets, fmt.Sprint(raw))
	}
	if v, ok := in.Body["foundation"]; ok {
		bullets = append(bullets, "Foundation: "+fmt.Sprint(v))
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{RawPath: fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx), SemanticPath: in.semSlide() + ".pillars"})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}
