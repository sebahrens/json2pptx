package semantic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/safeyaml"
)

// maxSpecSize bounds a semantic source document. It is generous relative to the
// safeyaml chart/diagram default because a full deck spec is larger than a
// single chart definition, while still guarding against runaway input.
const maxSpecSize = 1 << 20 // 1 MiB

// rawDeck is the lenient decode target. Slides are decoded as generic maps so
// that an unknown or malformed slide kind surfaces as a path-scoped semantic
// diagnostic rather than an opaque decoder error.
type rawDeck struct {
	Meta   DeckMeta         `json:"meta" yaml:"meta"`
	Slides []map[string]any `json:"slides" yaml:"slides"`
}

// Parse decodes a semantic deck document, dispatching on the filename
// extension: ".json" is parsed as JSON, everything else (including ".yaml" and
// ".yml") as YAML. It returns a best-effort DeckSpec together with any
// diagnostics; callers should check Diagnostics.HasErrors before using the spec.
func Parse(filename string, data []byte) (*DeckSpec, Diagnostics) {
	if strings.HasSuffix(strings.ToLower(filename), ".json") {
		return ParseJSON(data)
	}
	return ParseYAML(data)
}

// ParseYAML decodes a semantic deck document from YAML.
func ParseYAML(data []byte) (*DeckSpec, Diagnostics) {
	limits := safeyaml.DefaultLimits()
	limits.MaxSize = maxSpecSize

	var raw rawDeck
	if err := safeyaml.UnmarshalWithLimits(data, &raw, limits); err != nil {
		return nil, parseErrorDiagnostics(err)
	}
	return buildDeckSpec(raw)
}

// ParseJSON decodes a semantic deck document from JSON.
func ParseJSON(data []byte) (*DeckSpec, Diagnostics) {
	if len(data) > maxSpecSize {
		return nil, parseErrorDiagnostics(fmt.Errorf("document exceeds maximum size of %d bytes", maxSpecSize))
	}
	var raw rawDeck
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, parseErrorDiagnostics(err)
	}
	return buildDeckSpec(raw)
}

// parseErrorDiagnostics wraps a decode error as a root-scoped diagnostic.
func parseErrorDiagnostics(err error) Diagnostics {
	var ds Diagnostics
	ds.add("", CodeParseError, err.Error())
	return ds
}

// buildDeckSpec converts the lenient rawDeck into a DeckSpec, validating the
// archetype and each slide's kind discriminator and emitting path-scoped
// diagnostics for anything unrecognized.
func buildDeckSpec(raw rawDeck) (*DeckSpec, Diagnostics) {
	var ds Diagnostics

	spec := &DeckSpec{Meta: raw.Meta}

	if raw.Meta.Archetype != "" && !raw.Meta.Archetype.Valid() {
		ds.add("meta.archetype", CodeUnknownArchetype,
			fmt.Sprintf("unknown archetype %q; expected one of %s",
				raw.Meta.Archetype, joinArchetypes()))
	}

	spec.Slides = make([]SlideSpec, 0, len(raw.Slides))
	for i, m := range raw.Slides {
		path := fmt.Sprintf("slides[%d]", i)
		slide := buildSlideSpec(path, m, &ds)
		spec.Slides = append(spec.Slides, slide)
	}

	return spec, ds
}

// buildSlideSpec validates and converts a single raw slide map.
func buildSlideSpec(path string, m map[string]any, ds *Diagnostics) SlideSpec {
	var slide SlideSpec

	kindRaw, ok := m["kind"]
	switch {
	case !ok:
		ds.add(path, CodeMissingKind,
			fmt.Sprintf("slide is missing the required \"kind\" field; expected one of %s", joinKinds()))
	default:
		kindStr, isStr := kindRaw.(string)
		if !isStr {
			ds.add(path+".kind", CodeInvalidKindType,
				fmt.Sprintf("kind must be a string, got %T", kindRaw))
		} else {
			slide.Kind = SlideKind(kindStr)
			if !slide.Kind.Valid() {
				ds.add(path+".kind", CodeUnknownKind,
					fmt.Sprintf("unknown slide kind %q; expected one of %s", kindStr, joinKinds()))
			}
		}
	}

	// Retain the kind-specific payload (everything except the discriminator).
	if len(m) > 0 {
		body := make(map[string]any, len(m))
		for k, v := range m {
			if k == "kind" {
				continue
			}
			body[k] = v
		}
		if len(body) > 0 {
			slide.Body = body
		}
	}

	return slide
}

// joinKinds renders the registered slide kinds for diagnostic messages.
func joinKinds() string {
	kinds := AllSlideKinds()
	parts := make([]string, len(kinds))
	for i, k := range kinds {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}

// joinArchetypes renders the registered archetypes for diagnostic messages.
func joinArchetypes() string {
	arches := AllArchetypes()
	parts := make([]string, len(arches))
	for i, a := range arches {
		parts[i] = string(a)
	}
	return strings.Join(parts, ", ")
}
