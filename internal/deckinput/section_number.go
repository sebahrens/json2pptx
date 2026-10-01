package deckinput

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxSectionNumberLabelRunes bounds an authored section_number label: the
// divider renders it in a 150–350pt numeral slot, where anything longer than
// a short chapter mark ("A", "02", "IV") overflows.
const MaxSectionNumberLabelRunes = 4

// SectionNumberInput is slide.section_number: either the boolean false
// (suppress the automatic number) or a short label rendered verbatim
// (go-slide-creator-7ldh9). The boolean true is accepted and means the
// default automatic behaviour.
type SectionNumberInput struct {
	// Suppress is true for `false`: the divider shows no number and does not
	// advance the automatic counter.
	Suppress bool
	// Label is the authored label ("A", "02"). Empty for the boolean forms.
	Label string
}

// UnmarshalJSON accepts false, true, or a non-empty string of at most
// MaxSectionNumberLabelRunes runes.
func (s *SectionNumberInput) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	switch string(trimmed) {
	case "false":
		*s = SectionNumberInput{Suppress: true}
		return nil
	case "true", "null":
		*s = SectionNumberInput{}
		return nil
	}
	var label string
	if err := json.Unmarshal(trimmed, &label); err != nil {
		return fmt.Errorf("section_number must be false or a short label string such as \"A\" or \"02\"")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return fmt.Errorf("section_number label must not be empty; use false to suppress the number")
	}
	if utf8.RuneCountInString(label) > MaxSectionNumberLabelRunes {
		return fmt.Errorf("section_number label %q is longer than %d characters; the divider renders it in the oversized numeral slot", label, MaxSectionNumberLabelRunes)
	}
	*s = SectionNumberInput{Label: label}
	return nil
}

// MarshalJSON writes the authored form back (false, a label, or true).
func (s SectionNumberInput) MarshalJSON() ([]byte, error) {
	switch {
	case s.Suppress:
		return []byte("false"), nil
	case s.Label != "":
		return json.Marshal(s.Label)
	default:
		return []byte("true"), nil
	}
}
