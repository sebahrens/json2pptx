package slides

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

type pillarItem struct {
	Title string   `json:"title"`
	Body  []string `json:"body,omitempty"`
}

// strategyHouseValues is the strategy-house pattern's values. Foundation is a
// string (one band) or a list of levels, each a string or a list of cells —
// the shape the DeckSpec carries, passed through as authored.
type strategyHouseValues struct {
	Objective  string       `json:"objective"`
	Beam       string       `json:"beam,omitempty"`
	Pillars    []pillarItem `json:"pillars"`
	Foundation any          `json:"foundation"`
	RoofBadges []string     `json:"roof_badges,omitempty"`
}

type pillarsResolution struct {
	pattern       string
	items         []pillarItem
	badges        []string
	foundation    any        // string, or []any of string / []string levels
	layers        [][]string // the foundation as levels of cells
	listed        bool       // foundation was authored as a list
	beam          string
	path, problem string
	hard          bool
}

// House limits, mirroring the strategy-house pattern's validation.
const (
	houseMaxLayers    = 3
	houseMaxCells     = patterns.HouseMaxLevelCells
	houseBandMaxChars = 140
	houseCellMaxChars = 40
)

// parseFoundation reads foundation as a string or a list of levels, each a
// string (a full-width band) or a list of short strings (a row of cells).
func parseFoundation(v any) (pillarsResolution, pillarsResolution) {
	var r pillarsResolution
	switch f := v.(type) {
	case string:
		if s := strings.TrimSpace(f); s != "" {
			r.foundation, r.layers = s, [][]string{{s}}
		}
		return r, pillarsResolution{}
	case []any:
		r.listed = true
		levels := make([]any, 0, len(f))
		for i, level := range f {
			path := fmt.Sprintf("foundation[%d]", i)
			switch l := level.(type) {
			case string:
				if strings.TrimSpace(l) == "" {
					return r, pillarsResolution{path: path, problem: "foundation level must be a non-empty string or a list of strings", hard: true}
				}
				levels = append(levels, strings.TrimSpace(l))
				r.layers = append(r.layers, []string{strings.TrimSpace(l)})
			case []any:
				cells := make([]string, 0, len(l))
				for j, cell := range l {
					c, ok := cell.(string)
					if !ok || strings.TrimSpace(c) == "" {
						return r, pillarsResolution{path: fmt.Sprintf("%s[%d]", path, j), problem: "foundation cell must be a non-empty string", hard: true}
					}
					cells = append(cells, strings.TrimSpace(c))
				}
				if len(cells) == 0 {
					return r, pillarsResolution{path: path, problem: "foundation level must be a non-empty string or a list of strings", hard: true}
				}
				levels = append(levels, cells)
				r.layers = append(r.layers, cells)
			default:
				return r, pillarsResolution{path: path, problem: "foundation level must be a string or a list of strings", hard: true}
			}
		}
		if len(levels) > 0 {
			r.foundation = levels
		}
		return r, pillarsResolution{}
	}
	return r, pillarsResolution{path: "foundation", problem: "foundation must be a string or a list of levels", hard: true}
}

// readHouseLevels reads the foundation and the beam into r.
func (r *pillarsResolution) readHouseLevels(body map[string]any) pillarsResolution {
	if v, present := body["foundation"]; present {
		f, issue := parseFoundation(v)
		if issue.problem != "" {
			return issue
		}
		r.foundation, r.layers, r.listed = f.foundation, f.layers, f.listed
	}
	if v, present := body["beam"]; present {
		beam, ok := v.(string)
		if !ok {
			return pillarsResolution{path: "beam", problem: "beam must be a string", hard: true}
		}
		r.beam = strings.TrimSpace(beam)
	}
	return pillarsResolution{}
}

// houseBudget checks the house's levels against the pattern's limits.
func houseBudget(r pillarsResolution) (string, string) {
	if utf8.RuneCountInString(r.beam) > houseBandMaxChars {
		return "beam", fmt.Sprintf("beam exceeds %d characters", houseBandMaxChars)
	}
	if len(r.layers) > houseMaxLayers {
		return "foundation", fmt.Sprintf("%d foundation levels exceed the %d a house keeps readable", len(r.layers), houseMaxLayers)
	}
	counts := []int{len(r.items)}
	for i, layer := range r.layers {
		path := "foundation"
		if r.listed {
			path = fmt.Sprintf("foundation[%d]", i)
		}
		if len(layer) > houseMaxCells {
			return path, fmt.Sprintf("%d cells in one foundation level exceed %d", len(layer), houseMaxCells)
		}
		counts = append(counts, len(layer))
		limit, what := houseBandMaxChars, "foundation"
		if len(layer) > 1 {
			limit, what = houseCellMaxChars, "foundation cell"
		}
		for j, cell := range layer {
			if utf8.RuneCountInString(cell) > limit {
				if len(layer) > 1 {
					path = fmt.Sprintf("foundation[%d][%d]", i, j)
				}
				return path, fmt.Sprintf("%s exceeds %d characters", what, limit)
			}
		}
	}
	if !patterns.HouseColumnsFit(counts...) {
		return "foundation", fmt.Sprintf("%d pillars and the foundation levels' cell counts cannot share one column grid", len(r.items))
	}
	return "", ""
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
	if _, present := body["objective"]; present && !objectivePresent {
		return pillarsResolution{path: "objective", problem: "objective must be a string", hard: true}
	}
	if issue := r.readHouseLevels(body); issue.problem != "" {
		return issue
	}
	objective = strings.TrimSpace(objective)
	house := objective != "" && r.foundation != nil
	if (objective != "") != (r.foundation != nil) {
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
	if !house && r.beam != "" {
		return pillarsResolution{path: "beam", problem: "a beam needs both objective and foundation"}
	}
	if house {
		if utf8.RuneCountInString(objective) > houseBandMaxChars {
			return pillarsResolution{path: "objective", problem: "objective exceeds 140 characters"}
		}
		if field, problem := houseBudget(r); problem != "" {
			return pillarsResolution{path: field, problem: problem}
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
		values = strategyHouseValues{Objective: strField(in.Body, "objective"), Beam: r.beam, Pillars: r.items, Foundation: r.foundation, RoofBadges: r.badges}
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
	for _, field := range []string{"objective", "beam", "foundation", "roof_badges"} {
		links = append(links, SourceLink{RawPath: in.rawSlide() + ".pattern.values." + field, SemanticPath: in.semSlide() + "." + field})
	}
	// A listed foundation maps level by level and cell by cell, so a finding
	// on one enabler box names that box in the DeckSpec.
	if r.pattern == "strategy-house" && r.listed {
		for i, layer := range r.layers {
			level := fmt.Sprintf(".foundation[%d]", i)
			links = append(links, SourceLink{RawPath: in.rawSlide() + ".pattern.values" + level, SemanticPath: in.semSlide() + level})
			if len(layer) < 2 {
				continue
			}
			for j := range layer {
				cell := fmt.Sprintf("%s[%d]", level, j)
				links = append(links, SourceLink{RawPath: in.rawSlide() + ".pattern.values" + cell, SemanticPath: in.semSlide() + cell})
			}
		}
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
	if v, ok := in.Body["beam"]; ok {
		bullets = append(bullets, "Across the pillars: "+fmt.Sprint(v))
	}
	if v, ok := in.Body["foundation"]; ok {
		bullets = append(bullets, foundationBullets(v)...)
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

// foundationBullets writes the foundation as bullets, one per level, so a
// house that falls back to a bullet slide keeps every enabler.
func foundationBullets(v any) []string {
	levels, ok := v.([]any)
	if !ok {
		return []string{"Foundation: " + fmt.Sprint(v)}
	}
	out := make([]string, 0, len(levels))
	for _, level := range levels {
		if cells, ok := level.([]any); ok {
			parts := make([]string, len(cells))
			for i, c := range cells {
				parts[i] = fmt.Sprint(c)
			}
			out = append(out, "Foundation: "+strings.Join(parts, " · "))
			continue
		}
		out = append(out, "Foundation: "+fmt.Sprint(level))
	}
	return out
}
