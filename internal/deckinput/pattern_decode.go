package deckinput

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// One decoder for a pattern block (go-slide-creator-iqknz).
//
// The renderer read a pattern block in expandPattern — look the pattern up,
// inspect the raw payload against the pattern's own decoder, unmarshal the
// values / overrides / cell_overrides, run the pattern's Validate — and the
// DeckSpec static check read it again in a copy of that code that had no
// inspection step. The two disagreed on the code and the path of the same
// fault: `values: "nope"` was invalid_shape at …/pattern/values for a raw
// deck and one PATTERN_ERROR at …/pattern for the same slide inside a
// DeckSpec. DecodePattern is the one reading; both callers use it.

// DecodedPattern is a pattern block read by its pattern's own decoder and
// accepted by the pattern's input gate.
type DecodedPattern struct {
	Pattern patterns.Pattern
	// Values, Overrides and CellOverrides are the pattern's typed inputs, as
	// Pattern.Validate and Pattern.Expand take them. Overrides is nil when
	// the block has none (or the pattern takes none).
	Values        any
	Overrides     any
	CellOverrides map[int]any
	// TypeScale is the block's type-scale mode: overrides.type_scale, else
	// the block's default_type_scale. Empty when neither is set.
	TypeScale string
}

// PatternError is a pattern block its pattern refuses, one finding per fault.
//
// Each finding's Path is relative to the pattern block and names one of its
// own keys first — "name", "values[0].big", "overrides.gap",
// "cell_overrides[2]", "callout" — so a caller holding the block's location
// can address the field. An empty Path is the block itself: a fault the
// decoder cannot pin to one key (vertical_align outside its set, a
// cell_overrides key that is not an index).
type PatternError struct {
	Pattern  string
	Findings []*patterns.ValidationError
}

// Error joins the findings' messages, one per line.
func (e *PatternError) Error() string {
	msgs := make([]string, len(e.Findings))
	for i, f := range e.Findings {
		msgs[i] = f.Message
	}
	return strings.Join(msgs, "\n")
}

// Unwrap exposes the findings to errors.As / errors.Is and to
// diagnostics.FromJoinedError.
func (e *PatternError) Unwrap() []error {
	out := make([]error, len(e.Findings))
	for i, f := range e.Findings {
		out[i] = f
	}
	return out
}

// IsBlockFault reports whether the error is a single fault of the block as a
// whole (PATTERN_ERROR with no field path), and returns its message.
func (e *PatternError) IsBlockFault() (string, bool) {
	if len(e.Findings) == 1 && e.Findings[0].Path == "" && e.Findings[0].Code == diagnostics.CodePatternError {
		return e.Findings[0].Message, true
	}
	return "", false
}

func patternBlockFault(name, format string, args ...any) error {
	return &PatternError{Pattern: name, Findings: []*patterns.ValidationError{{
		Pattern: name,
		Code:    diagnostics.CodePatternError,
		Message: fmt.Sprintf(format, args...),
	}}}
}

// UnknownPatternFinding is the finding for a pattern name the registry does
// not have, with the closest registered name when there is one.
func UnknownPatternFinding(reg *patterns.Registry, name string) *patterns.ValidationError {
	ve := &patterns.ValidationError{
		Path: "name",
		Code: diagnostics.CodeUnknownPattern,
	}
	if suggestion, ok := reg.Suggest(name); ok {
		ve.Message = fmt.Sprintf("unknown pattern %q; did you mean %q?", name, suggestion)
		ve.Fix = &patterns.FixSuggestion{Kind: "swap_pattern", Params: map[string]any{
			"from": name, "to": suggestion, "did_you_mean": suggestion,
		}}
	} else {
		ve.Message = fmt.Sprintf("unknown pattern %q; call list_patterns for the registered names", name)
		ve.Fix = &patterns.FixSuggestion{Kind: "swap_pattern", Params: map[string]any{"from": name}}
	}
	return ve
}

// lookupPattern resolves the block's pattern and checks the block's own
// enum field (vertical_align). The error is a *PatternError.
func lookupPattern(p *PatternInput, reg *patterns.Registry) (patterns.Pattern, error) {
	pat, ok := reg.Get(p.Name)
	if !ok {
		return nil, &PatternError{Pattern: p.Name, Findings: []*patterns.ValidationError{UnknownPatternFinding(reg, p.Name)}}
	}
	if _, ok := shapegrid.ParseVerticalAlign(p.VerticalAlign); !ok {
		return nil, patternBlockFault(p.Name, "pattern %q: vertical_align must be one of \"auto\", \"top\", \"center\", \"bottom\", \"stretch\", got %q", p.Name, p.VerticalAlign)
	}
	return pat, nil
}

// DecodePattern reads a pattern block: it looks the pattern up, inspects the
// raw payload against the pattern's own decoder (so a shape the decoder
// rejects, or a key it silently discards, is a per-field finding), unmarshals
// the typed values, overrides and per-cell overrides, runs the pattern's
// Validate, and checks that a callout is one the pattern supports. It needs no
// template or theme.
//
// A block the pattern refuses is returned as a *PatternError.
func DecodePattern(p *PatternInput, reg *patterns.Registry) (*DecodedPattern, error) {
	pat, err := lookupPattern(p, reg)
	if err != nil {
		return nil, err
	}
	mode, cleanOverrides, _ := patterns.SplitTypeScaleOverride(p.Name, p.Overrides)
	if mode == "" {
		mode = p.DefaultTypeScale
	}

	// Inspect the raw payload BEFORE anything else reads it
	// (go-slide-creator-20jm). Two failures are invisible further down: a
	// shape encoding/json rejects (whose error names Go types), and a key the
	// decoder silently discards.
	if inputErrs := patterns.InspectPatternInput(pat, p.Values, p.Overrides, p.CellOverrides); len(inputErrs) > 0 {
		return nil, &PatternError{Pattern: p.Name, Findings: RootPatternFindingPaths(inputErrs)}
	}

	values := pat.NewValues()
	if err := json.Unmarshal(p.Values, values); err != nil {
		return nil, patternBlockFault(p.Name, "pattern %q: invalid values: %v", p.Name, err)
	}

	var overrides any
	if len(cleanOverrides) > 0 {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal(cleanOverrides, overrides); err != nil {
				return nil, patternBlockFault(p.Name, "pattern %q: invalid overrides: %v", p.Name, err)
			}
		}
	}

	// cell_overrides: string keys → int keys.
	var cellOverrides map[int]any
	if len(p.CellOverrides) > 0 {
		cellOverrides = make(map[int]any, len(p.CellOverrides))
		for key, raw := range p.CellOverrides {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return nil, patternBlockFault(p.Name, "pattern %q: cell_overrides key %q is not an integer", p.Name, key)
			}
			co := pat.NewCellOverride()
			if co == nil {
				return nil, patternBlockFault(p.Name, "pattern %q: does not support cell_overrides", p.Name)
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return nil, patternBlockFault(p.Name, "pattern %q: invalid cell_overrides[%d]: %v", p.Name, idx, err)
			}
			cellOverrides[idx] = co
		}
	}

	// The pattern's own findings are *patterns.ValidationError with a path;
	// they are kept one per field rather than joined into one message.
	if err := pat.Validate(values, overrides, cellOverrides); err != nil {
		if ves := PatternValidationFindings(err); len(ves) > 0 {
			return nil, &PatternError{Pattern: p.Name, Findings: RootPatternFindingPaths(ves)}
		}
		return nil, patternBlockFault(p.Name, "pattern %q: validation failed: %v", p.Name, err)
	}

	// Callout support (D18): refused before expansion, so validate and expand
	// agree (0kyd).
	if p.Callout != nil {
		cs, ok := pat.(patterns.CalloutSupport)
		if !ok || !cs.SupportsCallout() {
			return nil, &PatternError{Pattern: p.Name, Findings: []*patterns.ValidationError{
				patterns.ErrCalloutUnsupportedFor(p.Name, reg.CalloutSupportedPatterns()),
			}}
		}
	}

	return &DecodedPattern{Pattern: pat, Values: values, Overrides: overrides, CellOverrides: cellOverrides, TypeScale: mode}, nil
}

// ValidatePattern reports whether a pattern block survives its pattern's input
// gate — DecodePattern without the decoded result — so a caller can predict
// whether a PatternInput will render without building a ShapeGridInput or
// needing any template/theme context. It returns nil for a nil block and for
// one the pattern accepts, a *PatternError otherwise; callers that want
// structured diagnostics split it with diagnostics.FromJoinedError.
func ValidatePattern(p *PatternInput, reg *patterns.Registry) error {
	if p == nil {
		return nil
	}
	_, err := DecodePattern(p, reg)
	return err
}

// PatternValidationFindings returns the per-field findings an error from a
// pattern carries, or nil when it is not fully made of them. A pattern's own
// Validate returns errors.Join of *ValidationError.
func PatternValidationFindings(err error) []*patterns.ValidationError {
	var pe *PatternError
	if errors.As(err, &pe) {
		return pe.Findings
	}
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		var out []*patterns.ValidationError
		for _, e := range joined.Unwrap() {
			var ve *patterns.ValidationError
			if errors.As(e, &ve) {
				out = append(out, ve)
				continue
			}
			// One non-structured member means the set is not fully
			// addressable; fall back rather than report a partial picture.
			return nil
		}
		return out
	}
	var ve *patterns.ValidationError
	if errors.As(err, &ve) {
		return []*patterns.ValidationError{ve}
	}
	return nil
}

// patternInputSections are PatternInput's own JSON keys. A finding path that
// starts with one of them is already rooted at the pattern block.
var patternInputSections = map[string]bool{
	"name": true, "values": true, "overrides": true, "cell_overrides": true,
	"callout": true, "bounds": true, "max_height_pct": true,
}

// RootPatternFindingPaths returns copies of ves whose paths are rooted at the
// pattern block. Pattern.Validate reports values-relative paths
// ("members[0].role", "values[0].small"), and only the code that harvests them
// knows that; rooting happens once, here, so every consumer downstream can
// treat a finding path as block-relative without special cases. A finding
// about the choice of pattern itself (wrong_pattern, reported at "pattern")
// is addressed at the block's name, the field the fix changes.
func RootPatternFindingPaths(ves []*patterns.ValidationError) []*patterns.ValidationError {
	out := make([]*patterns.ValidationError, len(ves))
	for i, ve := range ves {
		copied := *ve
		switch {
		case copied.Path == "":
			copied.Path = "values"
		case copied.Path == "pattern":
			copied.Path = "name"
		case !patternInputSections[firstPathSegment(copied.Path)]:
			copied.Path = "values." + copied.Path
		}
		out[i] = &copied
	}
	return out
}

// firstPathSegment returns the leading field name of a dotted path, without
// any index suffix ("values[0].big" → "values").
func firstPathSegment(dotted string) string {
	seg := dotted
	if i := strings.IndexAny(seg, ".["); i >= 0 {
		seg = seg[:i]
	}
	return seg
}
