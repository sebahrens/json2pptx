package data

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// varPattern matches {{ variable.path }} with optional whitespace.
var varPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}`)

// ResolveVariables performs variable interpolation on all string fields
// of the PresentationDefinition. Undefined variables produce warnings
// and remain unchanged in the output.
func ResolveVariables(pres *types.PresentationDefinition, ctx *Context) []string {
	if ctx == nil || len(ctx.Vars) == 0 {
		return nil
	}

	var warnings []string

	// Resolve metadata fields
	pres.Metadata.Title, warnings = resolveField(pres.Metadata.Title, ctx, warnings)
	pres.Metadata.Author, warnings = resolveField(pres.Metadata.Author, ctx, warnings)
	pres.Metadata.Date, warnings = resolveField(pres.Metadata.Date, ctx, warnings)

	// Resolve each slide
	for i := range pres.Slides {
		warnings = resolveSlide(&pres.Slides[i], ctx, warnings)
	}

	return warnings
}

func resolveSlide(slide *types.SlideDefinition, ctx *Context, warnings []string) []string {
	slide.Title, warnings = resolveField(slide.Title, ctx, warnings)
	slide.SpeakerNotes, warnings = resolveField(slide.SpeakerNotes, ctx, warnings)
	slide.Source, warnings = resolveField(slide.Source, ctx, warnings)
	slide.RawContent, warnings = resolveField(slide.RawContent, ctx, warnings)

	// Resolve structured content
	warnings = resolveContent(&slide.Content, ctx, warnings)

	// Resolve slot content
	for _, slot := range slide.Slots {
		if slot == nil {
			continue
		}
		slot.RawContent, warnings = resolveField(slot.RawContent, ctx, warnings)
		slot.Text, warnings = resolveField(slot.Text, ctx, warnings)
		for j := range slot.Bullets {
			slot.Bullets[j], warnings = resolveField(slot.Bullets[j], ctx, warnings)
		}
	}

	return warnings
}

func resolveContent(content *types.SlideContent, ctx *Context, warnings []string) []string {
	content.Body, warnings = resolveField(content.Body, ctx, warnings)
	content.ImagePath, warnings = resolveField(content.ImagePath, ctx, warnings)
	content.TableRaw, warnings = resolveField(content.TableRaw, ctx, warnings)

	for i := range content.Bullets {
		content.Bullets[i], warnings = resolveField(content.Bullets[i], ctx, warnings)
	}
	for i := range content.BulletGroups {
		content.BulletGroups[i].Header, warnings = resolveField(content.BulletGroups[i].Header, ctx, warnings)
		for j := range content.BulletGroups[i].Bullets {
			content.BulletGroups[i].Bullets[j], warnings = resolveField(content.BulletGroups[i].Bullets[j], ctx, warnings)
		}
	}

	for i := range content.Left {
		content.Left[i], warnings = resolveField(content.Left[i], ctx, warnings)
	}
	for i := range content.Right {
		content.Right[i], warnings = resolveField(content.Right[i], ctx, warnings)
	}

	return warnings
}

// resolveField replaces all {{ var }} expressions in a string.
// Undefined variables are left unchanged and a warning is appended.
func resolveField(s string, ctx *Context, warnings []string) (string, []string) {
	if !strings.Contains(s, "{{") {
		return s, warnings
	}

	result := varPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the variable path from {{ path }}
		sub := varPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		path := sub[1]

		val, found := lookupPath(ctx.Vars, path)
		if !found {
			warnings = append(warnings, fmt.Sprintf("data: undefined variable %q", path))
			return match
		}

		return formatValue(val)
	})

	return result, warnings
}

// lookupPath resolves a dot-separated path against the variable tree.
// It supports nested maps and array indexing: "revenue.2025.q4" or "items.0.name".
func lookupPath(vars map[string]any, path string) (any, bool) {
	// Try direct key first (handles non-nested keys like "company")
	if val, ok := vars[path]; ok {
		return val, true
	}

	// Split path and traverse
	parts := strings.Split(path, ".")
	var current any = vars

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val

		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]

		case []map[string]string:
			// CSV rows
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]

		case map[string]string:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val

		default:
			return nil, false
		}
	}

	return current, true
}

// formatValue converts a resolved value to its string representation.
func formatValue(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
