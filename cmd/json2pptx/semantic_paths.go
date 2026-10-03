package main

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// One address for a DeckSpec finding (go-slide-creator-pilpn).
//
// The agent journey review met five notations for one slide inside a single
// workflow: "slides[2].kpis" on a validate finding, "/slides/2/kpis" in the
// patch it had to write, "slide 3" in a message, a 0-based slide_index beside a
// 1-based slide_number, and pattern-internal pointers such as
// "/slides/7/pattern/rows/2/cells/0/shape/text" that name nothing the author
// wrote.
//
// A finding on a DeckSpec surface now has:
//
//   - path: a JSON Pointer (RFC 6901, 0-based) into the spec the author sent,
//     the notation a patch uses. It always resolves: a finding about a field
//     that is not there yet points at the nearest object that is, and names
//     the field to add in missing_path;
//   - slide_number: the 1-based position of the slide in the rendered deck,
//     the only index meant for a person;
//   - debug: the compiled deck's own pointers (raw_path, cell_path, …), for a
//     maintainer. Nothing outside debug names a compiled object.
//
// Inside the pipeline a path stays in the dotted form the semantic package
// writes ("slides[2].kpis[0].label"); it is converted once, on the way out.

// specPointer renders a dotted DeckSpec path as a JSON Pointer. A path that is
// already a pointer is returned unchanged, and "" stays "" (the document).
func specPointer(path string) string {
	if path == "" || strings.HasPrefix(path, "/") {
		return path
	}
	var b strings.Builder
	for _, tok := range dottedTokens(path) {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(tok, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

// dottedTokens splits "slides[2].kpis[0].label" into its reference tokens.
func dottedTokens(path string) []string {
	var tokens []string
	for _, part := range strings.Split(path, ".") {
		for part != "" {
			open := strings.IndexByte(part, '[')
			if open < 0 {
				tokens = append(tokens, part)
				break
			}
			if open > 0 {
				tokens = append(tokens, part[:open])
			}
			end := strings.IndexByte(part[open:], ']')
			if end < 0 {
				tokens = append(tokens, part[open:])
				break
			}
			tokens = append(tokens, part[open+1:open+end])
			part = part[open+end+1:]
		}
	}
	return tokens
}

// pointerTokens splits a JSON Pointer into unescaped reference tokens.
func pointerTokens(pointer string) []string {
	if pointer == "" {
		return nil
	}
	raw := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for i, tok := range raw {
		raw[i] = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
	}
	return raw
}

// dottedPath renders a JSON Pointer in the pipeline's dotted form. It is the
// inverse of specPointer for the paths the DeckSpec surfaces emit, and lets a
// decoded response be compared with the pipeline's own paths.
func dottedPath(pointer string) string {
	if !strings.HasPrefix(pointer, "/") {
		return pointer
	}
	var b strings.Builder
	for _, tok := range pointerTokens(pointer) {
		if _, err := strconv.Atoi(tok); err == nil {
			b.WriteString("[" + tok + "]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(tok)
	}
	return b.String()
}

// dottedMentionRE finds a dotted DeckSpec path quoted inside a message or a
// blocking reason.
var dottedMentionRE = regexp.MustCompile(`\b(?:slides\[\d+\]|structure\.sections\[\d+\](?:\.slides\[\d+\])?|structure\.(?:cover|closing))(?:\.[A-Za-z_][A-Za-z_0-9]*|\[\d+\])*`)

// pointerNotation rewrites every dotted DeckSpec path in a sentence as a JSON
// Pointer, so prose and path fields name a location the same way.
func pointerNotation(text string) string {
	if !strings.Contains(text, "slides[") && !strings.Contains(text, "structure.") {
		return text
	}
	return dottedMentionRE.ReplaceAllStringFunc(text, specPointer)
}

// specDoc is the authored spec a finding's path is resolved against.
type specDoc struct {
	root any
	// slidePaths are the dotted source paths of the rendered slides in deck
	// order (flat or structured form); nil when the spec does not parse.
	slidePaths []string
}

// newSpecDoc decodes the authored spec (JSON or YAML).
func newSpecDoc(filename string, data []byte) *specDoc {
	canonical, name := canonicalSpec(filename, data)
	doc := &specDoc{}
	if json.Unmarshal(canonical, &doc.root) != nil {
		doc.root = nil
	}
	if spec, diags := semantic.Parse(name, canonical); spec != nil && !diags.HasErrors() {
		for _, s := range semantic.ExpandedSlideSources(spec) {
			doc.slidePaths = append(doc.slidePaths, s.SourcePath)
		}
	}
	return doc
}

// resolve returns the deepest pointer along path that exists in the authored
// spec, and whether path itself exists.
func (d *specDoc) resolve(pointer string) (existing string, exact bool) {
	if d == nil || d.root == nil {
		return pointer, true
	}
	node := d.root
	var b strings.Builder
	for _, tok := range pointerTokens(pointer) {
		var next any
		switch current := node.(type) {
		case map[string]any:
			v, ok := current[tok]
			if !ok {
				return b.String(), false
			}
			next = v
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(current) {
				return b.String(), false
			}
			next = current[i]
		default:
			return b.String(), false
		}
		node = next
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(tok, "~", "~0"), "/", "~1"))
	}
	return b.String(), true
}

// slideContainerRE matches the slide a dotted path belongs to.
var slideContainerRE = regexp.MustCompile(`^(?:slides\[\d+\]|structure\.sections\[\d+\]\.slides\[\d+\]|structure\.(?:cover|closing))`)

// slideContainer returns the dotted path of the slide that path sits in, or "".
func slideContainer(path string) string {
	return slideContainerRE.FindString(dottedPath(path))
}

// slideNumber returns the 1-based deck position of the slide a dotted path
// sits in, or 0 when the path names no slide or the position is not known (a
// structured spec that does not parse).
func (d *specDoc) slideNumber(path string) int {
	container := slideContainer(path)
	if container == "" {
		return 0
	}
	if d != nil && len(d.slidePaths) > 0 {
		for i, p := range d.slidePaths {
			if p == container {
				return i + 1
			}
		}
		return 0
	}
	if m := flatSlidePath.FindStringSubmatch(container); m != nil {
		if i, err := strconv.Atoi(m[1]); err == nil {
			return i + 1
		}
	}
	return 0
}

// slidePathAt returns the dotted source path of the slide at a 0-based deck
// position, or "".
func (d *specDoc) slidePathAt(index int) string {
	if index < 0 {
		return ""
	}
	if d != nil && len(d.slidePaths) > 0 {
		if index < len(d.slidePaths) {
			return d.slidePaths[index]
		}
		return ""
	}
	return "slides[" + strconv.Itoa(index) + "]"
}

// internalPathKeys are evidence and fix-param keys that address the compiled
// deck. They are reported under debug only.
var internalPathKeys = map[string]bool{
	"raw_path": true, "cell_path": true, "target_path": true, "generated_path": true, "shape_path": true, "pointer": true,
}

// authoredAddress is a finding's location in the authored spec.
type authoredAddress struct {
	// Path always resolves in the spec.
	Path string
	// Missing is the pointer of the field the finding is about when the spec
	// does not have it yet (a required field, a takeaway to add).
	Missing string
	// SlideNumber is 1-based, or 0 when the finding is not on a slide.
	SlideNumber int
}

// address resolves a pipeline path (dotted, or "" with a slide index) to the
// authored spec. A raw pointer into the compiled deck is never an address: the
// finding is then placed on its slide.
func (d *specDoc) address(dotted string, slideIndex *int) authoredAddress {
	if strings.HasPrefix(dotted, "/") {
		// A compiled-deck pointer the source map could not trace.
		dotted = ""
	}
	if dotted == "" && slideIndex != nil {
		dotted = d.slidePathAt(*slideIndex)
	}
	pointer := specPointer(dotted)
	existing, exact := d.resolve(pointer)
	out := authoredAddress{Path: existing, SlideNumber: d.slideNumber(dotted)}
	if !exact {
		out.Missing = pointer
	}
	if out.SlideNumber == 0 && slideIndex != nil && *slideIndex >= 0 {
		out.SlideNumber = *slideIndex + 1
	}
	return out
}

// moveInternalPaths moves every compiled-deck locator out of a fact map into
// debug: the keys known to carry one, and any other string that looks like a
// spec pointer but does not resolve in the authored spec.
func (d *specDoc) moveInternalPaths(facts map[string]any, debug map[string]any, keep ...string) map[string]any {
	for key, v := range facts {
		if containsString(keep, key) || !(internalPathKeys[key] && isText(v)) && !d.namesCompiledObject(v) {
			continue
		}
		if debug == nil {
			debug = map[string]any{}
		}
		if _, has := debug[key]; !has {
			debug[key] = v
		}
		delete(facts, key)
	}
	return debug
}

func isText(v any) bool { _, ok := v.(string); return ok }

// namesCompiledObject reports whether a fact is a slide pointer, or a list
// holding one, that does not resolve in the authored spec.
func (d *specDoc) namesCompiledObject(v any) bool {
	switch t := v.(type) {
	case string:
		if !strings.HasPrefix(t, "/slides/") {
			return false
		}
		_, exact := d.resolve(t)
		return !exact
	case []string:
		for _, s := range t {
			if d.namesCompiledObject(s) {
				return true
			}
		}
	case []any:
		for _, e := range t {
			if d.namesCompiledObject(e) {
				return true
			}
		}
	}
	return false
}

// shapeEnvelopeFindings gives every finding of a DeckSpec envelope its
// authored address. diags are the diagnostics the envelope was built from
// (index-aligned), or nil for an envelope built from spec-level diagnostics
// alone.
func shapeEnvelopeFindings(envelope *diagnostics.FindingEnvelope, diags []semanticDiagnostic, doc *specDoc) {
	for i := range envelope.Findings {
		var source *semanticDiagnostic
		if i < len(diags) {
			source = &diags[i]
		}
		shapeEnvelopeFinding(&envelope.Findings[i], source, doc)
	}
}

// shapeEnvelopeFinding shapes one envelope finding; source is the diagnostic
// it was built from, when there is one.
func shapeEnvelopeFinding(f *diagnostics.Finding, source *semanticDiagnostic, doc *specDoc) {
	dotted, _ := f.Evidence["path"].(string)
	var slideIndex *int
	var members []diagMember
	rawPath := ""
	if source != nil {
		slideIndex, members, rawPath = source.SlideIndex, source.members, source.RawPath
		if source.SemanticPath == "" {
			dotted = ""
		}
	}
	addr := doc.address(dotted, slideIndex)
	path := addr.Path
	f.Path = &path
	if addr.SlideNumber > 0 {
		n := addr.SlideNumber
		f.SlideNumber = &n
	}
	f.Where = nil
	delete(f.Evidence, "path")
	f.MissingPath = addr.Missing
	if len(members) > 1 {
		f.Occurrences = len(members)
		f.Paths = memberPointers(members, doc)
		if spansSlides(members) {
			// An entry that stands for several slides has no one slide.
			f.SlideNumber = nil
			delete(f.Evidence, "slide_id")
		}
	}
	if symptoms, ok := f.Evidence[symptomsDetail].([]any); ok {
		for _, s := range symptoms {
			if m, ok := s.(map[string]any); ok {
				shapeSymptom(m, doc)
			}
		}
	}
	pointerFacts(f.Evidence)
	f.Debug = doc.moveInternalPaths(f.Evidence, f.Debug)
	if rawPath != "" {
		f.Debug = withDebug(f.Debug, "raw_path", rawPath)
	}
	if f.Remediation != nil && f.Remediation.Primary != nil {
		f.Debug = doc.moveFixLocators(f.Remediation.Primary.Params, f.Debug)
	}
	if len(f.Evidence) == 0 {
		f.Evidence = nil
	}
	f.Message = pointerNotation(f.Message)
}

// withDebug sets one debug entry, creating the map when needed.
func withDebug(debug map[string]any, key string, value any) map[string]any {
	if debug == nil {
		debug = map[string]any{}
	}
	debug[key] = value
	return debug
}

// moveFixLocators moves a fix's compiled-deck locators into debug. params.path
// is a patch target when the finding was given a DeckSpec patch (it, or the
// object it adds to, exists in the spec); a raw fix's path addresses the
// compiled deck and is kept as debug.fix_path.
func (d *specDoc) moveFixLocators(params, debug map[string]any) map[string]any {
	debug = d.moveInternalPaths(params, debug, "path")
	p, ok := params["path"].(string)
	if !ok || !strings.HasPrefix(p, "/") {
		return debug
	}
	if _, exact := d.resolve(p[:strings.LastIndexByte(p, '/')]); exact {
		return debug
	}
	delete(params, "path")
	return withDebug(debug, "fix_path", p)
}

// shapeSymptom rewrites one symptom's path as an authored pointer; a symptom
// the source map could not trace keeps no path.
func shapeSymptom(m map[string]any, doc *specDoc) {
	p, _ := m["path"].(string)
	if p == "" || strings.HasPrefix(p, "/") {
		delete(m, "path")
	} else {
		m["path"] = doc.address(p, nil).Path
	}
	if msg, ok := m["message"].(string); ok {
		m["message"] = pointerNotation(msg)
	}
}

// memberPointers lists a collapsed finding's affected paths as authored
// pointers.
func memberPointers(members []diagMember, doc *specDoc) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, doc.address(m.Path, m.SlideIndex).Path)
	}
	return out
}

// shapeRenderDiagnostics gives every render diagnostic its authored address.
// The diagnostics keep their pipeline fields; MarshalJSON writes the shaped
// ones.
func shapeRenderDiagnostics(diags []semanticDiagnostic, doc *specDoc) {
	for i := range diags {
		d := &diags[i]
		addr := doc.address(d.SemanticPath, d.SlideIndex)
		d.address = &addr
		// Compiled-deck locators in the evidence and the edit's params move to
		// debug; the maps are shared with the source diagnostic, so copy first.
		d.debug = nil
		if len(d.Evidence) > 0 {
			d.Evidence = copyFacts(d.Evidence)
			d.debug = doc.moveInternalPaths(d.Evidence, d.debug)
			if len(d.Evidence) == 0 {
				d.Evidence = nil
			}
		}
		if d.RecommendedEdit != nil && len(d.RecommendedEdit.Params) > 0 {
			edit := *d.RecommendedEdit
			edit.Params = copyFacts(edit.Params)
			d.debug = doc.moveInternalPaths(edit.Params, d.debug)
			d.RecommendedEdit = &edit
		}
		if len(d.members) > 1 {
			d.memberPaths = memberPointers(d.members, doc)
		}
		for j := range d.Symptoms {
			s := &d.Symptoms[j]
			if s.Path != "" && !strings.HasPrefix(s.Path, "/") {
				s.pointer = doc.address(s.Path, nil).Path
			}
		}
	}
}

// semanticDiagnosticWire is the JSON form of a render diagnostic.
type semanticDiagnosticWire struct {
	Code        string `json:"code"`
	Severity    string `json:"severity,omitempty"`
	Blocking    bool   `json:"blocking"`
	Message     string `json:"message"`
	Path        string `json:"path"`
	MissingPath string `json:"missing_path,omitempty"`
	SlideNumber int    `json:"slide_number,omitempty"`
	SlideID     string `json:"slide_id,omitempty"`
	Occurrences int    `json:"occurrences,omitempty"`
	// Paths lists every affected path of a finding that stands for several.
	Paths           []string               `json:"paths,omitempty"`
	Action          string                 `json:"action,omitempty"`
	RecommendedEdit *semantic.SemanticEdit `json:"recommended_edit,omitempty"`
	NextToolCall    json.RawMessage        `json:"next_tool_call,omitempty"`
	// PatchVerified: next_tool_call's patch was applied and validated.
	PatchVerified bool                 `json:"patch_verified,omitempty"`
	Evidence      map[string]any       `json:"evidence,omitempty"`
	Waived        string               `json:"waived,omitempty"`
	Symptoms      []findingSymptomWire `json:"symptoms,omitempty"`
	Debug         map[string]any       `json:"debug,omitempty"`
}

type findingSymptomWire struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

// copyFacts returns a shallow copy of a fact map.
func copyFacts(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// MarshalJSON writes the diagnostic with its authored address
// (go-slide-creator-pilpn): path as a JSON Pointer, slide_number, and the
// compiled deck's pointer under debug. A diagnostic that was not resolved
// against a spec is converted by notation alone.
func (d semanticDiagnostic) MarshalJSON() ([]byte, error) {
	addr := d.address
	if addr == nil {
		a := (*specDoc)(nil).address(d.SemanticPath, d.SlideIndex)
		addr = &a
	}
	w := semanticDiagnosticWire{
		Code: d.Code, Severity: d.Severity, Blocking: d.Blocking, Message: pointerNotation(d.Message),
		Path: addr.Path, MissingPath: addr.Missing, SlideNumber: addr.SlideNumber, SlideID: d.SlideID,
		Action: d.Action, RecommendedEdit: d.RecommendedEdit, Evidence: d.Evidence, Waived: d.Waived,
		PatchVerified: d.patchVerified,
	}
	if len(d.members) > 1 {
		w.Occurrences = len(d.members)
		w.Paths = d.memberPaths
		if w.Paths == nil {
			w.Paths = memberPointers(d.members, nil)
		}
		if spansSlides(d.members) {
			w.SlideNumber, w.SlideID = 0, ""
		}
	}
	if d.NextToolCall != nil {
		raw, err := json.Marshal(d.NextToolCall)
		if err != nil {
			return nil, err
		}
		w.NextToolCall = raw
	}
	for _, s := range d.Symptoms {
		sw := findingSymptomWire{Code: s.Code, Path: s.pointer, Message: pointerNotation(s.Message)}
		if sw.Path == "" && s.Path != "" && !strings.HasPrefix(s.Path, "/") {
			sw.Path = specPointer(s.Path)
		}
		w.Symptoms = append(w.Symptoms, sw)
	}
	if len(d.debug) > 0 {
		w.Debug = copyFacts(d.debug)
	}
	if d.RawPath != "" {
		w.Debug = withDebug(w.Debug, "raw_path", d.RawPath)
	}
	return json.Marshal(w)
}

// UnmarshalJSON reads a diagnostic back into the pipeline's fields: path
// becomes the dotted SemanticPath, slide_number the 0-based SlideIndex and
// debug.raw_path RawPath. It exists so a response can be compared with the
// pipeline's own diagnostics.
func (d *semanticDiagnostic) UnmarshalJSON(data []byte) error {
	var w struct {
		semanticDiagnosticWire
		NextToolCall *json.RawMessage `json:"next_tool_call"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*d = semanticDiagnostic{
		Code: w.Code, Severity: w.Severity, Blocking: w.Blocking, Message: w.Message,
		SemanticPath: dottedPath(firstNonEmpty(w.MissingPath, w.Path)), SlideID: w.SlideID,
		Action: w.Action, RecommendedEdit: w.RecommendedEdit, Evidence: w.Evidence, Waived: w.Waived,
		patchVerified: w.PatchVerified,
	}
	if w.SlideNumber > 0 {
		idx := w.SlideNumber - 1
		d.SlideIndex = &idx
	}
	if w.Debug != nil {
		d.RawPath, _ = w.Debug["raw_path"].(string)
	}
	if w.NextToolCall != nil {
		if err := json.Unmarshal(*w.NextToolCall, &d.NextToolCall); err != nil {
			return err
		}
	}
	for _, s := range w.Symptoms {
		d.Symptoms = append(d.Symptoms, findingSymptom{Code: s.Code, Path: dottedPath(s.Path), Message: s.Message})
	}
	for _, p := range w.Paths {
		d.members = append(d.members, diagMember{Path: dottedPath(p)})
	}
	addr := authoredAddress{Path: w.Path, Missing: w.MissingPath, SlideNumber: w.SlideNumber}
	d.address = &addr
	d.memberPaths = w.Paths
	return nil
}

// pointerFacts rewrites evidence values that are dotted DeckSpec paths (a
// rhythm run's slides) as JSON Pointers.
func pointerFacts(facts map[string]any) {
	isPath := func(s string) bool { return s != "" && dottedMentionRE.FindString(s) == s }
	for key, v := range facts {
		switch t := v.(type) {
		case string:
			if isPath(t) {
				facts[key] = specPointer(t)
			}
		case []string:
			out := make([]any, len(t))
			for i, s := range t {
				out[i] = s
				if isPath(s) {
					out[i] = specPointer(s)
				}
			}
			facts[key] = out
		case []any:
			for i, e := range t {
				if s, ok := e.(string); ok && isPath(s) {
					t[i] = specPointer(s)
				}
			}
		}
	}
}
