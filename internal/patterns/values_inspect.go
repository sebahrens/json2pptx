package patterns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Pattern input diagnostics (go-slide-creator-20jm).
//
// Every pattern failure used to collapse into one INPUT.INVALID_SLIDE finding
// whose message was whatever encoding/json or Pattern.Validate happened to
// say — including Go type names ("cannot unmarshal string into Go struct field
// ProcessFlowValues.steps of type patterns.ProcessFlowStep"), no JSON path, and
// no remediation. Worse, the keys a pattern's decoder does not read were
// dropped in silence: {"columns": [...]} on comparison-2col reported "rows must
// contain at least 1 row" and never mentioned columns; {"items": [...]} on
// kpi-3up decoded as ONE empty cell and produced a swap suggestion for a
// pattern the author never asked about.
//
// InspectPatternInput answers both with per-field findings. It makes no guesses
// about what a pattern should accept: it runs the pattern's own decoder and
// reports only what that decoder demonstrably rejected (a shape it cannot
// unmarshal) or demonstrably discarded (authored text absent from the decoded
// value). Tolerated aliases — KPICell's {value, label} for {big, small}, the
// "$4.2M | ARR" string shorthand — survive the round trip, so they are never
// flagged.

// InspectPatternInput reports, per field, what a pattern's decoder rejects or
// discards in the raw values / overrides / cell_overrides payloads. It returns
// nil when everything the caller wrote is read.
//
// Findings are ordered by JSON path so two runs over one deck agree.
func InspectPatternInput(pat Pattern, values, overrides json.RawMessage, cellOverrides map[string]json.RawMessage) []*ValidationError {
	if pat == nil {
		return nil
	}
	root := pat.Schema()
	var out []*ValidationError
	out = append(out, inspectSection(pat.Name(), "values", values, pat.NewValues(), sectionSchema(root, "values"), root)...)
	out = append(out, inspectSection(pat.Name(), "overrides", overrides, pat.NewOverrides(), sectionSchema(root, "overrides"), root)...)
	for _, key := range sortedRawKeys(cellOverrides) {
		path := fmt.Sprintf("cell_overrides[%s]", key)
		out = append(out, inspectSection(pat.Name(), path, cellOverrides[key], pat.NewCellOverride(), cellOverrideSchema(root), root)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// sectionSchema returns the sub-schema for one top-level pattern input section.
func sectionSchema(root *Schema, name string) *Schema {
	return root.Property(name).Deref(root)
}

// cellOverrideSchema returns the schema one cell_overrides entry must match.
// The cell_overrides object declares it via patternProperties["^[0-9]+$"].
func cellOverrideSchema(root *Schema) *Schema {
	co := root.Property("cell_overrides").Deref(root)
	for _, sch := range co.PatternPropertySchemas() {
		return sch.Deref(root)
	}
	return nil
}

// inspectSection runs one section of a pattern's input through the pattern's own
// decoder. target is the pointer the decoder fills (nil when the pattern has no
// such section, in which case a non-empty payload is reported as unsupported).
func inspectSection(pattern, base string, raw json.RawMessage, target any, sch, root *Schema) []*ValidationError {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	rawTree, err := decodeTree(raw)
	if err != nil {
		return nil // malformed JSON is rejected at the request boundary
	}
	if target == nil {
		return []*ValidationError{{
			Pattern: pattern,
			Path:    base,
			Code:    ErrCodeUnknownKey,
			Message: fmt.Sprintf("%s: %s is not supported by this pattern and is ignored; remove it", pattern, base),
			Fix:     &FixSuggestion{Kind: "remove_field", Params: map[string]any{"path": base}},
		}}
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return []*ValidationError{shapeFinding(pattern, base, rawTree, sch, root)}
	}

	canonBytes, mErr := json.Marshal(target)
	if mErr != nil {
		return nil
	}
	canonTree, cErr := decodeTree(canonBytes)
	if cErr != nil {
		return nil
	}
	drops := confirmedDrops(base, rawTree, canonTree, sch.Deref(root), root)
	if len(drops) == 0 {
		return nil
	}

	// A tolerant decoder can accept a shape the schema rejects and keep none of
	// the content: KPINupValues wraps a lone object into a one-cell slice, so
	// {"items": [...]} decodes to one empty cell that Validate then reports as a
	// count mismatch. When the container holding lost content is itself the wrong
	// JSON kind, saying so beats listing the keys of a wrapper that should not be
	// there at all.
	//
	// The schema is consulted only about a node that demonstrably lost content,
	// never about the payload at large: a schema that lags its decoder (a
	// tolerated scalar shorthand the schema still describes as an object) must
	// not be able to refuse a working deck.
	for _, d := range drops {
		if d.container != nil && !kindAllowed(d.container, d.containerNode) {
			return []*ValidationError{shapeFindingAt(pattern, d.containerPath, d.container, d.containerNode, root)}
		}
	}

	var out []*ValidationError
	for _, d := range drops {
		// Without a declared property list there is no basis for calling a key
		// unknown: an object whose schema declares no properties is free-form
		// content (a chart's map-form data, keyed by series name), and patterns
		// that keep such a payload as raw JSON normalize it after decode — so it
		// does not survive a decode round trip and is not dropped either.
		if !d.element && len(d.container.PropertyNames()) == 0 {
			continue
		}
		out = append(out, unknownFieldFinding(pattern, d, root))
	}
	return out
}

// decodeTree decodes JSON into a generic tree, keeping numbers as json.Number so
// a round-tripped value compares equal to the one the caller wrote.
func decodeTree(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Shape findings — the decoder could not read the payload at all
// ---------------------------------------------------------------------------

// shapeFinding describes, in schema terms, the first place the payload's JSON
// shape contradicts the pattern's declared schema. Go type names never appear:
// the expected shape is read off the schema, and when the schema is silent the
// message names the section and points at show_pattern.
func shapeFinding(pattern, base string, rawTree any, sch, root *Schema) *ValidationError {
	path, expected, got := firstShapeMismatch(base, sch, root, rawTree)
	if expected == nil {
		return &ValidationError{
			Pattern: pattern,
			Path:    base,
			Code:    ErrCodeInvalidShape,
			Message: fmt.Sprintf("%s: %s has a shape this pattern cannot read; compare it against show_pattern %q example_values", pattern, base, pattern),
			Fix:     &FixSuggestion{Kind: "reshape_value", Params: map[string]any{"path": base}},
		}
	}
	return shapeFindingAt(pattern, path, expected, got, root)
}

// shapeFindingAt renders one located shape mismatch.
func shapeFindingAt(pattern, path string, expected *Schema, got any, root *Schema) *ValidationError {
	// A wrapper object around the array the pattern wants is the single most
	// common shape mistake: {"steps": [...]} where values IS the list. Name the
	// wrapper key so the fix is one unwrap, not a re-read of the schema.
	if key, inner, ok := singleArrayWrapper(expected, got); ok {
		return &ValidationError{
			Pattern: pattern,
			Path:    path,
			Code:    ErrCodeInvalidShape,
			Message: fmt.Sprintf("%s: %s must be %s, not an object wrapping one; the key %q is never read — send its value as %s directly",
				pattern, path, describeArrayOf(expected, root), key, path),
			Fix: &FixSuggestion{Kind: "reshape_value", Params: map[string]any{
				"path":       path,
				"unwrap_key": key,
				"expected":   "array",
				"got":        "object",
				"items":      inner,
			}},
		}
	}

	params := map[string]any{
		"path":     path,
		"expected": string(expected.TypeName()),
		"got":      jsonKind(got),
	}
	msg := fmt.Sprintf("%s: %s must be %s; got %s", pattern, path, describeExpected(expected, root), withValue(got))
	if ex := exampleFor(expected, root, got); ex != nil {
		encoded, err := json.Marshal(ex)
		if err == nil {
			params["example"] = ex
			msg += fmt.Sprintf(". Example: %s", encoded)
		}
	}
	return &ValidationError{
		Pattern: pattern,
		Path:    path,
		Code:    ErrCodeInvalidShape,
		Message: msg,
		Fix:     &FixSuggestion{Kind: "reshape_value", Params: params},
	}
}

// firstShapeMismatch walks node against sch and returns the path, schema, and
// value of the first node whose JSON kind the schema disallows. A oneOf is a
// mismatch only when no branch accepts the value.
func firstShapeMismatch(path string, sch, root *Schema, node any) (string, *Schema, any) {
	sch = sch.Deref(root)
	if sch == nil || node == nil {
		return "", nil, nil
	}
	if branches := sch.OneOfBranches(); len(branches) > 0 {
		for _, b := range branches {
			if kindAllowed(b.Deref(root), node) {
				// The branch fits at this level; a deeper mismatch inside it is
				// reported against that branch.
				return firstShapeMismatch(path, b, root, node)
			}
		}
		return path, sch, node
	}
	if !kindAllowed(sch, node) {
		return path, sch, node
	}
	switch v := node.(type) {
	case map[string]any:
		for _, key := range sortedObjectKeys(v) {
			prop := sch.Property(key)
			if prop == nil {
				continue // unknown keys are the dropped-content check's business
			}
			if p, s, g := firstShapeMismatch(path+"."+key, prop, root, v[key]); s != nil {
				return p, s, g
			}
		}
	case []any:
		items := sch.ItemSchema()
		if items == nil {
			return "", nil, nil
		}
		for i, elem := range v {
			if p, s, g := firstShapeMismatch(fmt.Sprintf("%s[%d]", path, i), items, root, elem); s != nil {
				return p, s, g
			}
		}
	}
	return "", nil, nil
}

// kindAllowed reports whether node's JSON kind satisfies sch's type keyword. A
// schema with no type keyword allows anything.
func kindAllowed(sch *Schema, node any) bool {
	if sch == nil {
		return true
	}
	want := sch.TypeName()
	if want == "" {
		return true
	}
	got := jsonKind(node)
	switch want {
	case TypeNumber, TypeInteger:
		return got == "number"
	case TypeObject:
		return got == "object"
	case TypeArray:
		return got == "array"
	case TypeString:
		return got == "string"
	case TypeBoolean:
		return got == "boolean"
	}
	return true
}

// branchFor resolves a oneOf/anyOf schema to the branch that describes a node of
// this JSON kind, so the caller sees the properties that actually apply.
//
// Every pattern whose values are a list of cells declares the element as
// oneOf{string shorthand, object}, and the object branch is where the property
// names live. Asking the oneOf itself for PropertyNames returns nothing, which
// read as "free-form content, nothing can be unknown here" — so a typo inside a
// kpi-3up cell or a card-grid card was dropped in silence, which is exactly the
// case an agent hits when it misspells a field (go-slide-creator-4cqh).
func branchFor(sch *Schema, node any, root *Schema) *Schema {
	if sch == nil {
		return nil
	}
	branches := sch.OneOfBranches()
	if len(branches) == 0 {
		return sch
	}
	kind := jsonKind(node)
	for _, b := range branches {
		b = b.Deref(root)
		if b == nil {
			continue
		}
		switch {
		case b.TypeName() == TypeObject && kind == "object":
			return b
		case b.TypeName() == TypeArray && kind == "array":
			return b
		case b.TypeName() == TypeString && kind == "string":
			return b
		case b.TypeName() == TypeNumber && kind == "number",
			b.TypeName() == TypeInteger && kind == "number":
			return b
		case b.TypeName() == TypeBoolean && kind == "boolean":
			return b
		}
	}
	return sch
}

// jsonKind names a decoded value's JSON kind in schema vocabulary.
func jsonKind(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number, float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	}
	return "value"
}

// withValue renders the offending value for a message: kind plus the literal
// when it is a short scalar, so the agent can find it without counting indices.
func withValue(v any) string {
	switch t := v.(type) {
	case string:
		if len(t) <= 40 {
			return fmt.Sprintf("the string %q", t)
		}
		return "a string"
	case json.Number:
		return fmt.Sprintf("the number %s", t.String())
	case bool:
		return fmt.Sprintf("the boolean %t", t)
	case map[string]any:
		if keys := sortedObjectKeys(t); len(keys) > 0 && len(keys) <= 6 {
			return fmt.Sprintf("an object with keys %s", strings.Join(quoteAll(keys), ", "))
		}
		return "an object"
	case []any:
		return fmt.Sprintf("an array of %d", len(t))
	case nil:
		return "null"
	}
	return "a value"
}

// describeExpected renders a schema as an agent-readable shape: the type, and
// for an object its key names with optional ones marked "?".
func describeExpected(sch *Schema, root *Schema) string {
	sch = sch.Deref(root)
	if sch == nil {
		return "a different shape"
	}
	if branches := sch.OneOfBranches(); len(branches) > 0 {
		parts := make([]string, 0, len(branches))
		for _, b := range branches {
			parts = append(parts, describeExpected(b, root))
		}
		return strings.Join(parts, " or ")
	}
	switch sch.TypeName() {
	case TypeObject:
		if keys := objectKeySummary(sch); keys != "" {
			return "an object " + keys
		}
		return "an object"
	case TypeArray:
		return describeArrayOf(sch, root)
	case TypeString:
		if enum := sch.EnumValues(); len(enum) > 0 {
			return "one of " + strings.Join(quoteAll(enum), ", ")
		}
		return "a string"
	case TypeNumber:
		return "a number"
	case TypeInteger:
		return "an integer"
	case TypeBoolean:
		return "a boolean"
	}
	return "a different shape"
}

// describeArrayOf renders an array schema as "an array of <item shape>".
func describeArrayOf(sch *Schema, root *Schema) string {
	items := sch.ItemSchema().Deref(root)
	if items == nil {
		return "an array"
	}
	if items.TypeName() == TypeObject {
		if keys := objectKeySummary(items); keys != "" {
			return "an array of objects " + keys
		}
		return "an array of objects"
	}
	if len(items.OneOfBranches()) > 0 {
		// "an array of a string or an object {…}" does not read; name the item.
		return "an array where each item is " + describeExpected(items, root)
	}
	switch items.TypeName() {
	case TypeString:
		return "an array of strings"
	case TypeNumber, TypeInteger:
		return "an array of numbers"
	}
	return "an array"
}

// objectKeySummary renders an object schema's keys as "{a, b, c?}", required
// keys first and optional keys suffixed "?".
func objectKeySummary(sch *Schema) string {
	names := sch.PropertyNames()
	if len(names) == 0 {
		return ""
	}
	var req, opt []string
	for _, n := range names {
		if sch.IsRequired(n) {
			req = append(req, n)
		} else {
			opt = append(opt, n+"?")
		}
	}
	const maxKeys = 8
	all := append(req, opt...)
	if len(all) > maxKeys {
		all = append(all[:maxKeys:maxKeys], "…")
	}
	return "{" + strings.Join(all, ", ") + "}"
}

// exampleFor builds a copy-ready value of the expected shape, reusing the
// caller's own content where it fits. A string supplied where a one-required-
// string-key object belongs becomes {"<key>": "<their string>"} — the exact
// edit, not a generic sample.
func exampleFor(sch *Schema, root *Schema, got any) any {
	sch = sch.Deref(root)
	if sch == nil {
		return nil
	}
	if sch.TypeName() == TypeObject {
		if s, ok := got.(string); ok {
			if key, ok := soleRequiredString(sch); ok {
				return map[string]any{key: s}
			}
		}
		return nil
	}
	if sch.TypeName() == TypeArray {
		if _, ok := got.(map[string]any); ok {
			return nil // handled by the wrapper case
		}
		if items := sch.ItemSchema().Deref(root); items != nil && items.TypeName() == TypeString {
			if s, ok := got.(string); ok {
				return []any{s}
			}
		}
	}
	return nil
}

// soleRequiredString returns the name of sch's only required property when that
// property is a string, so a bare string can be lifted into it.
func soleRequiredString(sch *Schema) (string, bool) {
	req := sch.RequiredNames()
	if len(req) != 1 {
		return "", false
	}
	if p := sch.Property(req[0]); p != nil && p.TypeName() == TypeString {
		return req[0], true
	}
	return "", false
}

// singleArrayWrapper recognizes an object that wraps the array the schema wants:
// exactly one key, whose value is an array. Returns the key and its element
// count.
func singleArrayWrapper(expected *Schema, got any) (string, int, bool) {
	if expected.TypeName() != TypeArray {
		return "", 0, false
	}
	obj, ok := got.(map[string]any)
	if !ok || len(obj) != 1 {
		return "", 0, false
	}
	for k, v := range obj {
		if list, ok := v.([]any); ok {
			return k, len(list), true
		}
	}
	return "", 0, false
}

// ---------------------------------------------------------------------------
// Dropped-content findings — the decoder read the payload but kept less of it
// ---------------------------------------------------------------------------

// confirmedDrops returns the keys the caller wrote whose content is absent from
// the decoded value. A key the decoder merely renamed (a KPI cell's "label" into
// "small") keeps its content and is not a drop.
func confirmedDrops(base string, rawTree, canonTree any, sch, root *Schema) []dropCandidate {
	survived := make(map[string]int)
	collectLeaves(canonTree, survived)

	var out []dropCandidate
	for _, d := range candidateDrops(base, rawTree, canonTree, sch, root) {
		if contentSurvives(d.value, survived) {
			continue // a tolerated alias: renamed on decode, content intact
		}
		out = append(out, d)
	}
	return out
}

// dropCandidate is one key (or array element) present in the caller's payload
// but absent from the decoded value.
type dropCandidate struct {
	path      string  // dotted path of the key itself, e.g. "values.members[0].title"
	key       string  // the key name, or "[i]" for a dropped array element
	value     any     // the raw value the caller wrote under it
	container *Schema // schema of the node the key sits in (nil for an element)
	// containerPath and containerNode describe that node, so a container of the
	// wrong JSON kind can be reported in place of its keys.
	containerPath string
	containerNode any
	present       []string
	element       bool // true when an array element was dropped, not a named key
}

// candidateDrops walks the caller's payload and the decoded payload in parallel,
// collecting keys the decode did not carry over. It does not recurse into a key
// it has already reported: the outermost dropped key is the one to fix.
func candidateDrops(path string, rawNode, canonNode any, sch, root *Schema) []dropCandidate {
	var out []dropCandidate
	sch = branchFor(sch, rawNode, root)
	switch raw := rawNode.(type) {
	case map[string]any:
		canon, _ := canonNode.(map[string]any)
		keys := sortedObjectKeys(raw)
		var present []string
		for _, k := range keys {
			if canon != nil {
				if _, ok := canon[k]; ok {
					present = append(present, k)
				}
			}
		}
		for _, k := range keys {
			var canonChild any
			_, inCanon := canon[k]
			if inCanon {
				canonChild = canon[k]
			}
			if !inCanon {
				if !hasContent(raw[k]) {
					continue
				}
				out = append(out, dropCandidate{
					path:          joinField(path, k),
					key:           k,
					value:         raw[k],
					container:     sch,
					containerPath: path,
					containerNode: rawNode,
					present:       present,
				})
				continue
			}
			out = append(out, candidateDrops(joinField(path, k), raw[k], canonChild,
				sch.Property(k).Deref(root), root)...)
		}
	case []any:
		canon, _ := canonNode.([]any)
		items := sch.ItemSchema().Deref(root)
		for i, elem := range raw {
			if i >= len(canon) {
				if !hasContent(elem) {
					continue
				}
				out = append(out, dropCandidate{
					path:    fmt.Sprintf("%s[%d]", path, i),
					key:     fmt.Sprintf("[%d]", i),
					value:   elem,
					element: true,
				})
				continue
			}
			out = append(out, candidateDrops(fmt.Sprintf("%s[%d]", path, i), elem, canon[i], items, root)...)
		}
	}
	return out
}

// unknownFieldFinding renders one dropped key, naming the likely intended field
// when the schema has a plausible target.
func unknownFieldFinding(pattern string, d dropCandidate, root *Schema) *ValidationError {
	ve := &ValidationError{
		Pattern: pattern,
		Path:    d.path,
		Code:    ErrCodePatternUnknownField,
		Fix:     &FixSuggestion{Kind: "remove_field", Params: map[string]any{"path": d.path}},
	}
	if d.element || len(d.container.PropertyNames()) == 0 {
		ve.Message = fmt.Sprintf("%s: %s is dropped — this pattern does not read it, so its content never reaches the slide", pattern, d.path)
		return ve
	}
	known := d.container.PropertyNames()
	if sug := suggestFieldName(d.key, d.container, root, d.present); sug != "" {
		ve.Message = fmt.Sprintf("%s: unknown field %q at %s is dropped — its content never reaches the slide; did you mean %q?",
			pattern, d.key, d.path, sug)
		ve.Fix = &FixSuggestion{Kind: "rename_field", Params: map[string]any{
			"path": d.path, "from": d.key, "to": sug, "did_you_mean": sug,
		}}
		return ve
	}
	ve.Message = fmt.Sprintf("%s: unknown field %q at %s is dropped — its content never reaches the slide; this pattern reads %s",
		pattern, d.key, d.path, strings.Join(quoteAll(known), ", "))
	ve.Fix.Params["allowed"] = known
	return ve
}

// ---------------------------------------------------------------------------
// Content survival
// ---------------------------------------------------------------------------

// collectLeaves accumulates the scalar text of every leaf in node, keyed by its
// normalized form, so the check is about content rather than position: a decoder
// that renames a key ({label} → {small}) keeps the content and is not a drop.
func collectLeaves(node any, out map[string]int) {
	switch v := node.(type) {
	case map[string]any:
		for _, child := range v {
			collectLeaves(child, out)
		}
	case []any:
		for _, child := range v {
			collectLeaves(child, out)
		}
	default:
		if s, ok := leafText(node); ok {
			out[s]++
		}
	}
}

// leafText renders a scalar leaf in its comparison form, reporting false for
// leaves that carry no content (empty string, zero, false, null) — a decoder
// that omits those on re-marshal has dropped nothing.
func leafText(node any) (string, bool) {
	switch v := node.(type) {
	case string:
		s := normalizeLeaf(v)
		return s, s != ""
	case json.Number:
		s := v.String()
		if s == "0" || s == "0.0" || s == "" {
			return "", false
		}
		return s, true
	case bool:
		if !v {
			return "", false
		}
		return "true", true
	}
	return "", false
}

// normalizeLeaf folds case and strips non-alphanumerics so a decoder that
// canonicalizes a value ("#2C3932" → "2C3932") still counts as keeping it.
func normalizeLeaf(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hasContent reports whether a raw node carries any authored content worth
// reporting as lost.
func hasContent(node any) bool {
	switch v := node.(type) {
	case map[string]any:
		for _, child := range v {
			if hasContent(child) {
				return true
			}
		}
		return false
	case []any:
		for _, child := range v {
			if hasContent(child) {
				return true
			}
		}
		return false
	default:
		_, ok := leafText(node)
		return ok
	}
}

// contentSurvives reports whether every leaf of the caller's value is present
// in the decoded payload. A partial survival still counts as a drop: the fields
// that vanished are the ones the author cares about.
func contentSurvives(node any, survived map[string]int) bool {
	leaves := make(map[string]int)
	collectLeaves(node, leaves)
	if len(leaves) == 0 {
		return true
	}
	for leaf, n := range leaves {
		if survived[leaf] < n && !containedIn(leaf, survived) {
			return false
		}
	}
	return true
}

// containedIn reports whether some surviving leaf contains leaf, covering a
// decoder that merges fields into one string.
func containedIn(leaf string, survived map[string]int) bool {
	for s := range survived {
		if strings.Contains(s, leaf) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Field-name suggestion
// ---------------------------------------------------------------------------

// fieldSynonymClasses group the words agents reach for when a schema uses a
// different one. Membership in a class is the strongest signal there is: an
// unknown "title" next to an unfilled "role" is a rename, not a new field.
// Keep each class to genuinely interchangeable slide vocabulary.
var fieldSynonymClasses = [][]string{
	{"title", "name", "label", "heading", "header", "role", "caption", "subtitle", "eyebrow", "position"},
	{"rows", "columns", "cols", "items", "entries", "cells", "cards", "list", "points", "lines"},
	{"body", "text", "content", "description", "detail", "details", "summary", "blurb", "bio", "note"},
	{"date", "date_label", "when", "timeframe", "period", "timing", "duration"},
	{"value", "number", "metric", "stat", "figure", "amount", "big", "total"},
	{"steps", "stages", "phases", "milestones", "stops"},
	{"image", "photo", "picture", "img", "image_path", "headshot", "avatar"},
	{"color", "accent", "fill", "tint", "shade"},
	{"icon", "glyph", "symbol"},
	{"quote", "statement", "testimonial"},
	{"attribution", "author", "source", "speaker", "who"},
}

// suggestFieldName names the schema property the caller most likely meant by
// unknown. Candidates are the properties the caller has NOT already filled —
// a key whose content did land cannot be the one they meant — scored by
// synonym class, containment, and edit distance, with a bonus for a required
// property still missing.
func suggestFieldName(unknown string, container *Schema, root *Schema, present []string) string {
	_ = root
	names := container.PropertyNames()
	if len(names) == 0 {
		return ""
	}
	filled := make(map[string]bool, len(present))
	for _, p := range present {
		filled[p] = true
	}
	missingRequired := 0
	for _, n := range names {
		if container.IsRequired(n) && !filled[n] {
			missingRequired++
		}
	}

	best, bestScore := "", 0
	for _, cand := range names {
		if filled[cand] || cand == unknown {
			continue
		}
		score := 0
		switch {
		case sameSynonymClass(unknown, cand):
			score = 100
		case wordContains(unknown, cand):
			score = 90
		default:
			if d := damerauLevenshtein(unknown, cand); d <= distanceLimit(unknown) {
				score = 80 - d
			}
		}
		if container.IsRequired(cand) && !filled[cand] {
			if score > 0 {
				score += 10
			} else if missingRequired == 1 {
				// Nothing links the words, but exactly one required property is
				// still missing and this content has to be it.
				score = 50
			}
		}
		if score > bestScore || (score == bestScore && score > 0 && cand < best) {
			best, bestScore = cand, score
		}
	}
	if bestScore == 0 {
		return ""
	}
	return best
}

// sameSynonymClass reports whether a and b are interchangeable slide vocabulary.
func sameSynonymClass(a, b string) bool {
	if a == b {
		return false
	}
	for _, class := range fieldSynonymClasses {
		var hasA, hasB bool
		for _, w := range class {
			if w == a {
				hasA = true
			}
			if w == b {
				hasB = true
			}
		}
		if hasA && hasB {
			return true
		}
	}
	return false
}

// wordContains reports whether one name is a whole-word part of the other
// ("date" in "date_label"), which is a rename far more often than a coincidence.
func wordContains(a, b string) bool {
	long, short := a, b
	if len(b) > len(a) {
		long, short = b, a
	}
	if len(short) < 3 || long == short {
		return false
	}
	for _, part := range strings.Split(long, "_") {
		if part == short {
			return true
		}
	}
	return false
}

// distanceLimit scales the edit-distance tolerance with name length, matching
// internal/semantic's unknown-field suggester.
func distanceLimit(name string) int {
	if len(name) >= 8 {
		return 3
	}
	return 2
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

func sortedObjectKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedRawKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func quoteAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

// joinField appends a key to a dotted path.
func joinField(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
