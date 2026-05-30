package semantic

// DeckSpec is the top-level semantic authoring model. It is intentionally
// compact: deck-level intent lives in Meta, and each slide is a kind-tagged
// payload. Later compiler phases validate and compile a DeckSpec into the raw
// internal/deckinput.PresentationInput model consumed by the generator.
type DeckSpec struct {
	Meta   DeckMeta    `json:"meta" yaml:"meta"`
	Slides []SlideSpec `json:"slides" yaml:"slides"`
}

// DeckMeta carries deck-level intent and presentation context.
type DeckMeta struct {
	// Title is the deck title.
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
	// Subtitle is an optional deck subtitle.
	Subtitle string `json:"subtitle,omitempty" yaml:"subtitle,omitempty"`
	// Archetype names the deck's overall purpose (see AllArchetypes).
	Archetype Archetype `json:"archetype,omitempty" yaml:"archetype,omitempty"`
	// Template optionally pins a json2pptx template name.
	Template string `json:"template,omitempty" yaml:"template,omitempty"`
	// Audience describes the intended audience (advisory).
	Audience string `json:"audience,omitempty" yaml:"audience,omitempty"`
	// Author is the deck author (advisory).
	Author string `json:"author,omitempty" yaml:"author,omitempty"`
	// Date is a free-form date string shown in chrome (advisory).
	Date string `json:"date,omitempty" yaml:"date,omitempty"`
}

// SlideSpec is a single semantic slide: a kind discriminator plus a
// kind-specific payload. The payload is retained as a generic map (Body) in
// this scaffold; later phases decode it into typed payloads per Kind.
type SlideSpec struct {
	// Kind selects the payload shape (see AllSlideKinds).
	Kind SlideKind `json:"kind" yaml:"kind"`
	// Body holds the kind-specific fields (everything except "kind"). It is
	// nil when the slide carried no payload fields.
	Body map[string]any `json:"body,omitempty" yaml:"body,omitempty"`
}

// String returns the string value of a Body field, or "" if it is absent or
// not a string. Convenience for callers inspecting common payload fields.
func (s SlideSpec) String(key string) string {
	if s.Body == nil {
		return ""
	}
	if v, ok := s.Body[key].(string); ok {
		return v
	}
	return ""
}
