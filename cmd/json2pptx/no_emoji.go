package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// noEmojiPolicyMessage is the canonical guidance appended to no-emoji
// diagnostics so agents understand both the policy and the remediation path:
// switch the offending text to a bundled SVG icon name or supply a
// user-provided icon (URL, data URI, inline SVG, or file path).
const noEmojiPolicyMessage = "decks must not contain emoji codepoints — use a bundled SVG icon (e.g. \"chart-pie\", \"filled:check\") or a user-provided icon (URL / data URI / inline SVG / file path). Run list_patterns or browse svggen/icons/{outline,filled}/ for the bundled icon set."

// isEmoji reports whether r is a Unicode emoji or symbol codepoint that
// requires an emoji-capable font to render. The ranges cover the most
// common emoji blocks plus variation selectors and ZWJ so composed emoji
// sequences are caught even when individual codepoints look benign.
func isEmoji(r rune) bool {
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

// containsEmoji reports whether s contains any emoji codepoints.
func containsEmoji(s string) bool {
	for _, r := range s {
		if isEmoji(r) {
			return true
		}
	}
	return false
}

// extractEmojiSample returns up to maxRunes distinct emoji codepoints found
// in s, in order of first appearance. Used in diagnostic messages so the
// caller sees exactly which codepoints triggered the violation.
func extractEmojiSample(s string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 3
	}
	seen := make(map[rune]bool)
	var b strings.Builder
	for _, r := range s {
		if !isEmoji(r) || unicode.IsSpace(r) {
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

// ValidateNoEmojiInText scans every string value in the presentation input
// for emoji codepoints and returns a refuse-class finding for each hit. The
// scan is JSON-based (marshal + recursive walk) so it covers typed fields,
// raw json.RawMessage overrides, shape grids, pattern overrides, and any
// future schema additions without per-field plumbing.
//
// Returned findings carry no_emoji_violation diagnostics with path info
// (e.g. slides[3].content[1].text_value) and a fix suggestion that points
// authors at the bundled icon set.
func ValidateNoEmojiInText(input *PresentationInput) []patterns.FitFinding {
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

	var findings []patterns.FitFinding
	walkJSONStringsForEmoji(root, "", func(value, path string) {
		sample := extractEmojiSample(value, 3)
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Pattern: "no_emoji",
				Path:    path,
				Code:    "no_emoji_violation",
				Message: fmt.Sprintf("%s contains emoji codepoint(s) %q — %s",
					displayPath(path), sample, noEmojiPolicyMessage),
				Fix: &patterns.FixSuggestion{
					Kind: "remove_emoji",
					Params: map[string]any{
						"path":  path,
						"hint":  "remove the emoji codepoint(s) or replace with a bundled SVG icon name (see list_patterns and svggen/icons/{outline,filled})",
					},
				},
			},
			Action: "refuse",
		})
	})

	// Stable order for deterministic diagnostics (JSON-walk visits map keys
	// in nondeterministic order).
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].Path < findings[j].Path
	})
	return findings
}

// walkJSONStringsForEmoji recursively visits every string value in a decoded
// JSON tree and calls visit when the string contains an emoji codepoint. The
// path argument is a JSON-style accessor (e.g. "slides[2].content[0].text_value").
func walkJSONStringsForEmoji(v any, path string, visit func(value, path string)) {
	switch n := v.(type) {
	case string:
		if containsEmoji(n) {
			visit(n, path)
		}
	case map[string]any:
		for k, child := range n {
			next := k
			if path != "" {
				next = path + "." + k
			}
			walkJSONStringsForEmoji(child, next, visit)
		}
	case []any:
		for i, child := range n {
			walkJSONStringsForEmoji(child, fmt.Sprintf("%s[%d]", path, i), visit)
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

// noEmojiDiagnostics converts no-emoji FitFindings into Diagnostics with a
// next_tool_call hint that points authors back to validate_input so they can
// re-check after stripping emoji.
func noEmojiDiagnostics(violations []patterns.FitFinding) []diagnostics.Diagnostic {
	diags := make([]diagnostics.Diagnostic, 0, len(violations))
	for _, v := range violations {
		d := diagnostics.Diagnostic{
			Code:     "no_emoji_violation",
			Path:     v.Path,
			Message:  v.Message,
			Severity: diagnostics.SeverityError,
			NextToolCall: &patterns.ToolCallSuggestion{
				Tool: "validate_input",
				ArgsTemplate: map[string]any{
					"presentation": "<re-submit after removing emoji codepoints>",
				},
			},
		}
		if v.Fix != nil {
			d.Fix = &diagnostics.Fix{Kind: v.Fix.Kind, Params: v.Fix.Params}
		}
		diags = append(diags, d)
	}
	return diags
}
