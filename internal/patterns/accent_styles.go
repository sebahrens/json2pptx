package patterns

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Restrained accent defaults (go-slide-creator-fl11f): structural cells
// (roadmap activities, process steps, KPI tiles, detail cards) take a neutral
// tint with dark text by default, and a pattern's accent marks structure with
// rules and markers. A solid accent fill is kept for the slide's one
// emphasised element, or behind an explicit opt-in style.

// recolorTextInk rewrites every paragraph (and the text-level default) whose
// colour is from to to, for text built for an accent fill that now sits on a
// neutral tint. The payload is returned unchanged when it is not a
// {paragraphs} object.
func recolorTextInk(text json.RawMessage, from, to string) json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(text, &obj) != nil || obj == nil {
		return text
	}
	fromJSON, _ := json.Marshal(from)
	toJSON, _ := json.Marshal(to)
	if c, ok := obj["color"]; ok && string(c) == string(fromJSON) {
		obj["color"] = toJSON
	}
	if raw, ok := obj["paragraphs"]; ok {
		var paras []map[string]json.RawMessage
		if json.Unmarshal(raw, &paras) != nil {
			return text
		}
		for _, p := range paras {
			if c, ok := p["color"]; ok && string(c) == string(fromJSON) {
				p["color"] = toJSON
			}
		}
		obj["paragraphs"], _ = json.Marshal(paras)
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return text
	}
	return out
}

// errInvalidEnum reports an override value outside its enum.
func errInvalidEnum(pattern, path, got string, allowed []string) *ValidationError {
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    "invalid_enum",
		Message: fmt.Sprintf("%s: %s must be one of %s; got %q", pattern, path, strings.Join(allowed, ", "), got),
	}
}

// accentInkOnTone returns accent when it clears minContrast against tone as
// painted, else the first theme ink that reads there (lt1 / dk2 / dk1). An
// accent numeral or keyword on a neutral tint keeps the brand where it reads
// and never ships below the bar (the render-time fixer would otherwise swap
// it, go-slide-creator-fl11f). Without a theme it returns accent.
func accentInkOnTone(ctx ExpandContext, accent string, tone fillTone, minContrast float64) string {
	if r, ok := fillContrast(ctx, fillTone{Color: accent}, tone); !ok || r >= minContrast {
		return accent
	}
	return readableInkOn(ctx, tone, accent, minContrast)
}

// AccentInkOnNeutral is accentInkOnTone for a caller outside a pattern
// expansion (the native diagram builders, go-slide-creator-amtkg): the ink an
// accent title takes on the neutral surface at pct% coverage, given the
// template's theme colours. With no colours it returns accent.
func AccentInkOnNeutral(colors []types.ThemeColor, accent string, pct int, minContrast float64) string {
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
	return accentInkOnTone(ctx, accent, neutralTone(pct), minContrast)
}
