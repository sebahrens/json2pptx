// Package parser provides presentation validation helpers.
// Markdown content extraction functions have been removed; JSON mode builds
// PresentationDefinition directly from the JSON schema.
package parser

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Parse is a deprecated stub. Markdown parsing has been removed; use JSON mode.
func Parse(_ string) (*types.PresentationDefinition, error) {
	return nil, fmt.Errorf("markdown parsing has been removed; use JSON mode")
}

// ParseFrontmatter is a deprecated stub. Markdown frontmatter parsing has been removed.
func ParseFrontmatter(_ string) (types.Metadata, string, []types.ParseError) {
	return types.Metadata{}, "", nil
}

// ResolveSlotContent is a deprecated no-op. JSON mode populates slot fields directly.
func ResolveSlotContent(_ map[int]*types.SlotContent) {}

// ResolveSlideTable is a deprecated no-op. JSON mode populates Table directly.
func ResolveSlideTable(_ *types.SlideDefinition) {}

// ParseMarkdownTable is a deprecated stub. Use types.TableSpec directly in JSON mode.
func ParseMarkdownTable(_ string) (*types.TableSpec, error) {
	return nil, fmt.Errorf("markdown table parsing has been removed; use JSON mode")
}

// hasFatalErrors checks if any errors in the list are fatal (ErrorLevelError).
func hasFatalErrors(errors []types.ParseError) bool {
	for _, err := range errors {
		if err.Level == types.ErrorLevelError {
			return true
		}
	}
	return false
}

// ValidatePresentation performs validation on a parsed presentation.
// This checks for common issues and returns validation errors.
func ValidatePresentation(presentation *types.PresentationDefinition) []types.ParseError {
	var errors []types.ParseError

	// Check for missing required metadata
	if presentation.Metadata.Title == "" {
		errors = append(errors, types.ParseError{
			Line:    0,
			Message: "presentation title is required",
			Field:   "title",
			Level:   types.ErrorLevelError,
		})
	}

	if presentation.Metadata.Template == "" {
		errors = append(errors, types.ParseError{
			Line:    0,
			Message: "template is required",
			Field:   "template",
			Level:   types.ErrorLevelError,
		})
	}

	// Check for empty presentation
	if len(presentation.Slides) == 0 {
		errors = append(errors, types.ParseError{
			Line:    0,
			Message: "presentation must contain at least one slide",
			Field:   "slides",
			Level:   types.ErrorLevelError,
		})
	}

	return errors
}

// HasErrors checks if the presentation has any fatal errors.
func HasErrors(presentation *types.PresentationDefinition) bool {
	return hasFatalErrors(presentation.Errors)
}

// GetErrorsByLevel returns errors filtered by severity level.
func GetErrorsByLevel(presentation *types.PresentationDefinition, level types.ErrorLevel) []types.ParseError {
	var filtered []types.ParseError

	for _, err := range presentation.Errors {
		if err.Level == level {
			filtered = append(filtered, err)
		}
	}

	return filtered
}
