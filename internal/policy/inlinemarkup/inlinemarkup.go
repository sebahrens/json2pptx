// Package inlinemarkup enforces the inline-markup vocabulary the engine
// actually renders.
//
// The run builder understands a small set of inline tags and passes anything
// else through to the text run verbatim, so a deck using <sup>, <a>, <color> or
// <code> rendered visible XML-ish garbage on the slide — and validate said
// nothing (go-slide-creator-510u). Superscript footnote markers are near
// universal in consulting decks, so an author reaches for one by reflex.
package inlinemarkup

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/policy/textwalk"
)

// SupportedTags is the inline markup the run builder renders. It must stay in
// step with pptx.SplitInlineTags and with the supports_inline_markup capability.
var SupportedTags = []string{"b", "i", "u", "sup", "sub"}

var supported = func() map[string]bool {
	m := make(map[string]bool, len(SupportedTags)*2)
	for _, t := range SupportedTags {
		m[t] = true
		m["/"+t] = true
	}
	return m
}()

// tagPattern matches anything that looks like an HTML/XML tag: a name,
// optionally closing, optionally carrying attributes.
var tagPattern = regexp.MustCompile(`</?([A-Za-z][A-Za-z0-9_-]*)(?:\s[^<>]*)?/?>`)

// Violation is one unsupported tag found in an authored string.
type Violation struct {
	// Path is a JSON-style accessor (e.g. "slides[2].content[0].text_value").
	Path string
	// Tags are the distinct unsupported tag names found, in sorted order.
	Tags []string
}

// markupFields carry markup by contract rather than authored prose, so a tag in
// them is not a mistake and never reaches the run builder. icon.svg_data is the
// documented way to supply an inline icon; scanning it reported every
// <svg>/<path>/<circle> as text that "prints literally on the slide", which is
// both false and unfixable — the finding fired on every deck using a harvey-ball
// table-highlight or any inline icon (go-slide-creator-6o1r).
var markupFields = map[string]bool{"svg_data": true}

// authoredText reports whether a walked path names a field whose value the run
// builder renders as text. The check is on the field name alone, so it holds at
// any depth (a nested grid cell's icon is as exempt as a top-level one).
func authoredText(path string) bool {
	field := path
	if i := strings.LastIndexByte(field, '.'); i >= 0 {
		field = field[i+1:]
	}
	if i := strings.IndexByte(field, '['); i >= 0 {
		field = field[:i]
	}
	return !markupFields[field]
}

// Scan returns one Violation per authored string containing a tag outside
// SupportedTags, in stable path order. Strings with no angle brackets are
// skipped, so the scan costs nothing on ordinary prose.
func Scan(input any) []Violation {
	var violations []Violation
	textwalk.Strings(input, func(value, path string) {
		if !strings.Contains(value, "<") || !authoredText(path) {
			return
		}
		seen := map[string]bool{}
		var tags []string
		for _, m := range tagPattern.FindAllStringSubmatch(value, -1) {
			name := strings.ToLower(m[1])
			if supported[name] || seen[name] {
				continue
			}
			seen[name] = true
			tags = append(tags, name)
		}
		if len(tags) == 0 {
			return
		}
		sort.Strings(tags)
		violations = append(violations, Violation{Path: path, Tags: tags})
	})
	sort.SliceStable(violations, func(i, j int) bool {
		return violations[i].Path < violations[j].Path
	})
	return violations
}

// Validate returns an UNSUPPORTED_INLINE_MARKUP fit finding per offending
// string. The findings are advisory (action "review"): the deck still renders,
// it just renders the tag as literal text, which is exactly what the author
// needs to be told.
func Validate(input any) []patterns.FitFinding {
	violations := Scan(input)
	if len(violations) == 0 {
		return nil
	}
	findings := make([]patterns.FitFinding, 0, len(violations))
	for _, v := range violations {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: "inline_markup",
				Path:    v.Path,
				Code:    patterns.ErrCodeUnsupportedInlineMarkup,
				Message: fmt.Sprintf(
					"%s uses inline tag(s) <%s> which the renderer does not support — they print literally on the slide; supported tags are <%s>",
					textwalk.DisplayPath(v.Path),
					strings.Join(v.Tags, ">, <"),
					strings.Join(SupportedTags, ">, <")),
				Fix: &patterns.FixSuggestion{
					Kind: "remove_key",
					Params: map[string]any{
						"path":        v.Path,
						"unsupported": v.Tags,
						"supported":   SupportedTags,
						"hint":        "remove the tag, or express the intent with a supported one (a footnote marker is <sup>1</sup>)",
					},
				},
			},
			Action: "review",
		})
	}
	return findings
}
