package pipeline

import "fmt"

// ParseError indicates a failure during markdown parsing.
type ParseError struct {
	Err     error
	Line    int
	Field   string
	Message string
}

func (e *ParseError) Error() string {
	if e.Message != "" {
		if e.Line > 0 {
			return fmt.Sprintf("line %d: %s", e.Line, e.Message)
		}
		return e.Message
	}
	return fmt.Sprintf("parse error: %v", e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// ValidationError indicates a validation failure on the parsed presentation.
type ValidationError struct {
	Line    int
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return e.Message
}

// LayoutError indicates a failure during layout selection.
type LayoutError struct {
	Err error
}

func (e *LayoutError) Error() string {
	return fmt.Sprintf("layout selection error: %v", e.Err)
}

func (e *LayoutError) Unwrap() error {
	return e.Err
}

// GenerationError indicates a failure during PPTX generation.
type GenerationError struct {
	Err error
}

func (e *GenerationError) Error() string {
	return fmt.Sprintf("generation error: %v", e.Err)
}

func (e *GenerationError) Unwrap() error {
	return e.Err
}
