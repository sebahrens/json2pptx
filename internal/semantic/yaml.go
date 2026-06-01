package semantic

import (
	"encoding/json"
	"fmt"
	"sort"
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

	// Decode generically first so malformed top-level container shapes (a
	// non-object root, a non-object meta, a non-array slides) fail fast with
	// path-scoped diagnostics instead of leaking the YAML decoder's internal
	// Go type names through a typed-struct decode error.
	var root any
	if err := safeyaml.UnmarshalWithLimits(data, &root, limits); err != nil {
		return nil, parseErrorDiagnostics(err)
	}
	if ds := validateContainerShapes(root); ds.HasErrors() {
		return nil, ds
	}

	var raw rawDeck
	if err := safeyaml.UnmarshalWithLimits(data, &raw, limits); err != nil {
		return nil, parseErrorDiagnostics(err)
	}
	top, _ := root.(map[string]any)
	return buildDeckSpec(raw, top)
}

// ParseJSON decodes a semantic deck document from JSON.
func ParseJSON(data []byte) (*DeckSpec, Diagnostics) {
	if len(data) > maxSpecSize {
		return nil, parseErrorDiagnostics(fmt.Errorf("document exceeds maximum size of %d bytes", maxSpecSize))
	}
	// Decode generically first so malformed top-level container shapes fail
	// fast with path-scoped diagnostics rather than leaking decoder internals
	// (see validateContainerShapes and ParseYAML).
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, parseErrorDiagnostics(err)
	}
	if ds := validateContainerShapes(root); ds.HasErrors() {
		return nil, ds
	}

	var raw rawDeck
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, parseErrorDiagnostics(err)
	}
	top, _ := root.(map[string]any)
	return buildDeckSpec(raw, top)
}

// validateContainerShapes checks the generic decode of a deck document for the
// top-level container shapes the semantic model requires: an object root with an
// optional object "meta" and an optional array "slides". It returns path-scoped
// diagnostics so a malformed container fails fast with an actionable message
// instead of leaking the decoder's internal Go/YAML type names to the agent.
//
// Per-slide element shapes are intentionally left to the slide-element decode
// path; this gate covers only the root/meta/slides containers.
func validateContainerShapes(root any) Diagnostics {
	var ds Diagnostics
	m, ok := root.(map[string]any)
	if !ok {
		ds.add("", CodeInvalidRoot,
			fmt.Sprintf("deck spec root must be an object with %s, got %s",
				strings.Join(knownTopLevelKeys, " and "), jsonShapeName(root)))
		return ds
	}
	if meta, present := m["meta"]; present {
		if _, isMap := meta.(map[string]any); !isMap {
			ds.add("meta", CodeInvalidMeta,
				fmt.Sprintf("meta must be an object, got %s", jsonShapeName(meta)))
		}
	}
	if slides, present := m["slides"]; present {
		if _, isSlice := slides.([]any); !isSlice {
			ds.add("slides", CodeInvalidSlides,
				fmt.Sprintf("slides must be an array, got %s", jsonShapeName(slides)))
		}
	}
	return ds
}

// jsonShapeName returns a friendly name for the JSON/YAML shape of v, used in
// container-shape diagnostics so messages stay free of internal Go type names.
func jsonShapeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case string:
		return "a string"
	case float64, int, int64:
		return "a number"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	default:
		return "a non-object value"
	}
}

// parseErrorDiagnostics wraps a decode error as a root-scoped diagnostic.
func parseErrorDiagnostics(err error) Diagnostics {
	var ds Diagnostics
	ds.add("", CodeParseError, err.Error())
	return ds
}

// knownTopLevelKeys are the only top-level fields a semantic deck spec carries.
var knownTopLevelKeys = []string{"meta", "slides"}

// topLevelMigrations maps a common stale top-level key onto the field it should
// be. These are checked before the generic fuzzy suggestion so well-known
// migration mistakes get an exact, actionable hint.
var topLevelMigrations = map[string]string{
	"deck":         "meta",
	"presentation": "meta",
	"title":        "meta.title",
	"subtitle":     "meta.subtitle",
}

// knownMetaKeys are the only fields the meta object carries. They mirror
// DeckMeta's json tags and the schema's DeckMeta.additionalProperties:false, so
// unknown meta keys are reported rather than silently dropped by the struct
// decode of rawDeck.Meta.
var knownMetaKeys = []string{"title", "subtitle", "archetype", "template", "audience", "author", "date"}

// metaMigrations maps a common stale/alias meta key onto the field it should be,
// checked before the generic fuzzy suggestion (mirrors topLevelMigrations).
var metaMigrations = map[string]string{
	"name":      "title",
	"heading":   "title",
	"type":      "archetype",
	"category":  "archetype",
	"theme":     "template",
	"presenter": "author",
}

// buildDeckSpec converts the lenient rawDeck into a DeckSpec, validating the
// archetype and each slide's kind discriminator and emitting path-scoped
// diagnostics for anything unrecognized. top is the same document decoded into a
// generic map (nil if the root is not a mapping) and is used to flag unknown
// top-level keys with targeted suggestions.
func buildDeckSpec(raw rawDeck, top map[string]any) (*DeckSpec, Diagnostics) {
	var ds Diagnostics

	checkUnknownTopLevel(top, &ds)
	checkUnknownMeta(top, &ds)

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

// checkUnknownTopLevel emits a direct diagnostic for every top-level key that is
// not part of the semantic deck-spec vocabulary, attaching a targeted "did you
// mean" suggestion for common migration mistakes and near-miss typos. top is nil
// when the root is not a mapping, in which case there is nothing to check.
func checkUnknownTopLevel(top map[string]any, ds *Diagnostics) {
	if top == nil {
		return
	}
	known := make(map[string]bool, len(knownTopLevelKeys))
	for _, k := range knownTopLevelKeys {
		known[k] = true
	}
	keys := make([]string, 0, len(top))
	for k := range top {
		if !known[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		msg := fmt.Sprintf("unknown top-level field %q; expected one of %s",
			k, strings.Join(knownTopLevelKeys, ", "))
		if sug := suggestTopLevel(k); sug != "" {
			msg = fmt.Sprintf("unknown top-level field %q; did you mean %q?", k, sug)
		}
		ds.add(k, CodeUnknownField, msg)
	}
}

// suggestTopLevel returns the field an unknown top-level key was most likely
// meant to be: a known migration target, then the closest valid key within a
// small edit distance, or "" when nothing is close enough.
func suggestTopLevel(key string) string {
	lower := strings.ToLower(key)
	if hint, ok := topLevelMigrations[lower]; ok {
		return hint
	}
	best, bestDist := "", -1
	for _, k := range knownTopLevelKeys {
		d := editDistance(lower, k)
		if bestDist < 0 || d < bestDist {
			best, bestDist = k, d
		}
	}
	if bestDist >= 0 && bestDist <= 2 {
		return best
	}
	return ""
}

// checkUnknownMeta emits a diagnostic for every key inside the meta object that
// is not a recognized DeckMeta field, mirroring the schema's
// DeckMeta.additionalProperties:false. Without this, unknown meta keys vanish in
// the struct decode of rawDeck.Meta, so a typo like "tilte" produces no hint.
//
// top is the generic decode of the document root (nil when the root is not a
// mapping). A present-but-non-object meta is reported separately as
// CodeInvalidMeta before this runs, so a failed type assertion here is simply a
// no-op.
func checkUnknownMeta(top map[string]any, ds *Diagnostics) {
	if top == nil {
		return
	}
	meta, ok := top["meta"].(map[string]any)
	if !ok {
		return
	}
	known := make(map[string]bool, len(knownMetaKeys))
	for _, k := range knownMetaKeys {
		known[k] = true
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		if !known[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		msg := fmt.Sprintf("unknown meta field %q; expected one of %s",
			k, strings.Join(knownMetaKeys, ", "))
		if sug := suggestMeta(k); sug != "" {
			msg = fmt.Sprintf("unknown meta field %q; did you mean %q?", k, sug)
		}
		ds.add("meta."+k, CodeUnknownField, msg)
	}
}

// suggestMeta returns the meta field an unknown key was most likely meant to be:
// a known migration target, then the closest valid key within a small edit
// distance, or "" when nothing is close enough (mirrors suggestTopLevel).
func suggestMeta(key string) string {
	lower := strings.ToLower(key)
	if hint, ok := metaMigrations[lower]; ok {
		return hint
	}
	best, bestDist := "", -1
	for _, k := range knownMetaKeys {
		d := editDistance(lower, k)
		if bestDist < 0 || d < bestDist {
			best, bestDist = k, d
		}
	}
	if bestDist >= 0 && bestDist <= 2 {
		return best
	}
	return ""
}

// editDistance returns the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur := make([]int, len(br)+1)
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(br)]
}

// min3 returns the smallest of three ints.
func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
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
