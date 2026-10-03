package semantic

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// DeckSpec is the top-level semantic authoring model. It is intentionally
// compact: deck-level intent lives in Meta, and each slide is a kind-tagged
// payload. Later compiler phases validate and compile a DeckSpec into the raw
// internal/deckinput.PresentationInput model consumed by the generator.
type DeckSpec struct {
	Meta      DeckMeta       `json:"meta" yaml:"meta"`
	Slides    []SlideSpec    `json:"slides,omitempty" yaml:"slides,omitempty"`
	Structure *DeckStructure `json:"structure,omitempty" yaml:"structure,omitempty"`

	// slidesPresent preserves the distinction between an absent slides field
	// and an explicitly authored empty array. JSON/YAML omitempty cannot carry
	// that distinction after decoding, but the slides/structure XOR contract
	// depends on field presence.
	slidesPresent bool
}

// DeckStructure is the chapter-oriented alternative to flat Slides.
type DeckStructure struct {
	Cover      *SlideSpec    `json:"cover,omitempty" yaml:"cover,omitempty"`
	AutoAgenda bool          `json:"auto_agenda,omitempty" yaml:"auto_agenda,omitempty"`
	Sections   []DeckSection `json:"sections" yaml:"sections"`
	Closing    *SlideSpec    `json:"closing,omitempty" yaml:"closing,omitempty"`
}

type DeckSection struct {
	Title  string      `json:"title" yaml:"title"`
	Slides []SlideSpec `json:"slides" yaml:"slides"`
	// Appendix marks back matter (go-slide-creator-deb2h): the divider is
	// unnumbered, the section is left out of auto_agenda, and its slides'
	// tracker / crumb read "Appendix".
	Appendix bool `json:"appendix,omitempty" yaml:"appendix,omitempty"`
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
	// Date is a free-form date string shown in chrome (advisory). When Chrome
	// is set without its own footer_date, this value fills it.
	Date string `json:"date,omitempty" yaml:"date,omitempty"`
	// Source is the deck-level default source line, rendered on every data
	// slide (chart, table of figures, KPI / stat) that sets no source of its
	// own (go-slide-creator-mp2p4).
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	// Chrome carries deck furniture: confidentiality stamp, client name,
	// project code, footer date and page numbers. Every board deck has some of
	// it, and before go-slide-creator-zmjs the semantic path could not express
	// any of it — an agent on the recommended path silently shipped none.
	Chrome *ChromeSpec `json:"chrome,omitempty" yaml:"chrome,omitempty"`
	// ViewingMode is the readability policy for the deck ("present" or "read").
	ViewingMode string `json:"viewing_mode,omitempty" yaml:"viewing_mode,omitempty"`
	// TypeScale controls measured grow-to-fill for shape-grid text. Semantic
	// decks default to "comfortable"; "compact" preserves authored sizes.
	TypeScale string `json:"type_scale,omitempty" yaml:"type_scale,omitempty"`
	// AccentStrategy controls accent rotation ("primary", "rotate",
	// "section-keyed"). It overrides the compile-option default.
	AccentStrategy string `json:"accent_strategy,omitempty" yaml:"accent_strategy,omitempty"`
	// DesignMode is "constrained" (default) or "free". A compiled deck is
	// constrained: the template owns sizes and colours. The raw_json2pptx
	// escape hatch carries author-authored slide payloads through unchanged,
	// and those can hand-set both — so a spec that uses it needs the same
	// opt-out the raw path has, or its slides are refused with no way to say
	// they are deliberate (go-slide-creator-rs4h).
	DesignMode string `json:"design_mode,omitempty" yaml:"design_mode,omitempty"`
	// RequiredLayouts constrains planning to cover these canonical template
	// layout IDs without turning the list into a one-slide-per-layout outline.
	RequiredLayouts []string `json:"required_layouts,omitempty" yaml:"required_layouts,omitempty"`
	// Waivers turn named storyline findings into advisories for a deck whose
	// requested shape breaks the rule on purpose (a seven-slide pitch has no
	// executive summary). Each waiver names one finding code and says why; the
	// render and validate results record it (go-slide-creator-oh3qr).
	Waivers []FindingWaiver `json:"waivers,omitempty" yaml:"waivers,omitempty"`
}

// FindingWaiver waives one storyline finding code for the whole deck.
type FindingWaiver struct {
	// Code is the finding code to waive (see WaivableFindingCodes).
	Code string `json:"code" yaml:"code"`
	// Reason says why the rule does not apply to this deck. Required.
	Reason string `json:"reason" yaml:"reason"`
}

// waivableFindingCodes are the storyline rules a deck may waive: they judge the
// deck's shape against a consulting storyline, which a user's brief can
// legitimately override. Fit, readability and contrast findings describe a
// broken slide and are never waivable.
var waivableFindingCodes = []string{
	patterns.ErrCodeClosingWithoutNextSteps,
	patterns.ErrCodeNoExecutiveSummary,
	patterns.ErrCodeTitleNotAction,
	patterns.ErrCodeTakeawayMissing,
}

// WaivableFindingCodes returns the finding codes meta.waivers accepts, sorted.
func WaivableFindingCodes() []string {
	return append([]string(nil), waivableFindingCodes...)
}

// IsWaivableFindingCode reports whether meta.waivers may name code.
func IsWaivableFindingCode(code string) bool {
	_, ok := CanonicalWaivableCode(code)
	return ok
}

// CanonicalWaivableCode returns the finding code a waiver names, in the
// spelling findings carry. Codes are matched without regard to case: the
// catalogue mixes TITLE_NOT_ACTION with takeaway_missing, and a waiver should
// not fail on that.
func CanonicalWaivableCode(code string) (string, bool) {
	code = strings.TrimSpace(code)
	for _, c := range waivableFindingCodes {
		if strings.EqualFold(c, code) {
			return c, true
		}
	}
	return "", false
}

// ChromeSpec mirrors deckinput.ChromeInput: the deck furniture rendered into
// every slide's footer band.
type ChromeSpec struct {
	// Confidentiality is a classification stamp (e.g. "Strictly confidential").
	Confidentiality string `json:"confidentiality,omitempty" yaml:"confidentiality,omitempty"`
	// ClientName is the client or company name.
	ClientName string `json:"client_name,omitempty" yaml:"client_name,omitempty"`
	// ProjectCode is the project identifier.
	ProjectCode string `json:"project_code,omitempty" yaml:"project_code,omitempty"`
	// FooterDate is the date string shown in the footer; defaults to meta.date.
	FooterDate string `json:"footer_date,omitempty" yaml:"footer_date,omitempty"`
	// PageNumbers controls slide numbering.
	PageNumbers *PageNumbersSpec `json:"page_numbers,omitempty" yaml:"page_numbers,omitempty"`
	// SectionCrumb shows the running section title in the footer.
	SectionCrumb bool `json:"section_crumb,omitempty" yaml:"section_crumb,omitempty"`
	// Tracker sets the current section name above each content slide's title.
	Tracker bool `json:"tracker,omitempty" yaml:"tracker,omitempty"`
}

// PageNumbersSpec mirrors deckinput.PageNumbersInput.
type PageNumbersSpec struct {
	// Enabled turns page numbers on or off (default: on, with or without a
	// chrome block; false suppresses the slide number).
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// Format supports {current} and {total} (e.g. "{current} / {total}").
	Format string `json:"format,omitempty" yaml:"format,omitempty"`
	// Skip lists slide types that show no page number (default: title, closing).
	Skip []string `json:"skip,omitempty" yaml:"skip,omitempty"`
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
