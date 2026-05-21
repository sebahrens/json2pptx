// Package emoji is the single source of truth for the no-emoji content
// policy. It exposes the emoji-detection predicate, a sanitizer, and the
// presentation-input validator so that every producer (the cmd/json2pptx
// generate/validate boundary, internal/testrand corpora, and any future
// generator) and the enforcer share one implementation. Keeping the
// predicate in an importable package means producers stay consistent with
// the validator by construction instead of drifting apart.
package emoji

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// PolicyMessage is the canonical guidance appended to no-emoji diagnostics so
// agents understand both the policy and the remediation path: switch the
// offending text to a bundled SVG icon name or supply a user-provided icon
// (URL, data URI, inline SVG, or file path).
const PolicyMessage = "decks must not contain emoji codepoints — use a bundled SVG icon (e.g. \"chart-pie\", \"filled:check\") or a user-provided icon (URL / data URI / inline SVG / file path). Run list_patterns or browse svggen/icons/{outline,filled}/ for the bundled icon set."

// IsEmoji reports whether r is a Unicode emoji or symbol codepoint that
// requires an emoji-capable font to render. The ranges cover the most common
// emoji blocks plus variation selectors and ZWJ so composed emoji sequences
// are caught even when individual codepoints look benign.
func IsEmoji(r rune) bool {
	// Miscellaneous Symbols
	if r >= 0x2600 && r <= 0x26FF {
		return true
	}
	// Dingbats
	if r >= 0x2700 && r <= 0x27BF {
		return true
	}
	// Variation Selectors (keep with adjacent emoji)
	if r >= 0xFE00 && r <= 0xFE0F {
		return true
	}
	// Supplemental Symbols and Pictographs, Emoticons, Transport, etc.
	if r >= 0x1F300 && r <= 0x1FAFF {
		return true
	}
	// Regional Indicator Symbols (flags)
	if r >= 0x1F1E0 && r <= 0x1F1FF {
		return true
	}
	// Zero Width Joiner (used in composite emoji sequences)
	if r == 0x200D {
		return true
	}
	// Combining Enclosing Keycap
	if r == 0x20E3 {
		return true
	}
	// Miscellaneous Symbols and Arrows
	if r >= 0x2B05 && r <= 0x2B55 {
		return true
	}
	// Copyright, Registered, Trade Mark
	if r == 0x00A9 || r == 0x00AE || r == 0x2122 {
		return true
	}
	return false
}

// Contains reports whether s contains any emoji codepoints.
func Contains(s string) bool {
	for _, r := range s {
		if IsEmoji(r) {
			return true
		}
	}
	return false
}

// Sanitize returns s with every emoji codepoint removed. Clean strings are
// returned unchanged (including their leading/trailing whitespace); strings
// that contained emoji have the offending runes dropped and any whitespace
// runs left behind collapsed and trimmed so producers can route literal
// corpora through this function and stay policy-clean by construction.
func Sanitize(s string) string {
	if !Contains(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if IsEmoji(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// ExtractSample returns up to maxRunes distinct emoji codepoints found in s,
// in order of first appearance. Used in diagnostic messages so the caller
// sees exactly which codepoints triggered the violation.
func ExtractSample(s string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 3
	}
	seen := make(map[rune]bool)
	var b strings.Builder
	for _, r := range s {
		if !IsEmoji(r) || unicode.IsSpace(r) {
			continue
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		b.WriteRune(r)
		if len(seen) >= maxRunes {
			break
		}
	}
	return b.String()
}

// Violation is a single emoji hit located by Scan: the JSON-style path to the
// offending string and a short sample of the emoji codepoints it contained.
type Violation struct {
	// Path is a JSON-style accessor (e.g. "slides[2].content[0].text_value").
	Path string
	// Value is the full offending string.
	Value string
	// Sample is up to three distinct emoji codepoints from Value.
	Sample string
}

// Scan marshals input to JSON and returns one Violation for every string
// value that contains an emoji codepoint. Working from JSON (marshal +
// recursive walk) means the scan covers typed fields, raw json.RawMessage
// overrides, shape grids, pattern overrides, and any future schema additions
// without per-field plumbing. Violations are returned in stable path order.
func Scan(input any) []Violation {
	if input == nil {
		return nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}

	var violations []Violation
	walkJSONStrings(root, "", func(value, path string) {
		violations = append(violations, Violation{
			Path:   path,
			Value:  value,
			Sample: ExtractSample(value, 3),
		})
	})

	// Stable order for deterministic diagnostics (JSON-walk visits map keys
	// in nondeterministic order).
	sort.SliceStable(violations, func(i, j int) bool {
		return violations[i].Path < violations[j].Path
	})
	return violations
}

// ValidateNoEmojiInText scans every string value in a presentation input for
// emoji codepoints and returns a refuse-class FitFinding for each hit. input
// may be any producer's presentation type (cmd/json2pptx, internal/testrand,
// …) because the scan is JSON-based.
//
// Returned findings carry no_emoji_violation diagnostics with path info
// (e.g. slides[3].content[1].text_value) and a fix suggestion that points
// authors at the bundled icon set.
func ValidateNoEmojiInText(input any) []patterns.FitFinding {
	violations := Scan(input)
	if len(violations) == 0 {
		return nil
	}
	findings := make([]patterns.FitFinding, 0, len(violations))
	for _, v := range violations {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: "no_emoji",
				Path:    v.Path,
				Code:    "no_emoji_violation",
				Message: fmt.Sprintf("%s contains emoji codepoint(s) %q — %s",
					displayPath(v.Path), v.Sample, PolicyMessage),
				Fix: &patterns.FixSuggestion{
					Kind: "remove_emoji",
					Params: map[string]any{
						"path": v.Path,
						"hint": "remove the emoji codepoint(s) or replace with a bundled SVG icon name (see list_patterns and svggen/icons/{outline,filled})",
					},
				},
			},
			Action: "refuse",
		})
	}
	return findings
}

// walkJSONStrings recursively visits every string value in a decoded JSON
// tree and calls visit when the string contains an emoji codepoint. The path
// argument is a JSON-style accessor (e.g. "slides[2].content[0].text_value").
func walkJSONStrings(v any, path string, visit func(value, path string)) {
	switch n := v.(type) {
	case string:
		if Contains(n) {
			visit(n, path)
		}
	case map[string]any:
		for k, child := range n {
			next := k
			if path != "" {
				next = path + "." + k
			}
			walkJSONStrings(child, next, visit)
		}
	case []any:
		for i, child := range n {
			walkJSONStrings(child, fmt.Sprintf("%s[%d]", path, i), visit)
		}
	}
}

// displayPath returns a human-friendly variant of a JSON path for messages.
// Empty paths render as "<root>" so the message is never blank.
func displayPath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
