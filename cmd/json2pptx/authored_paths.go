package main

import (
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Authored paths (go-slide-creator-9564b, -2v8me, -o45pn).
//
// The engine addresses what it checks in the deck it works on: a flat slide
// list (a split_slide entry is one slide per page, a structure block is its
// cover, agenda, dividers and slides), the shape grid a pattern or a compose
// envelope expands to, the chrome a template draws, the shapes a slide is
// written as. None of those is a place in the deck the author sent, and a
// finding's path is a JSON Pointer into exactly that deck.
//
// authoredPaths is the one translation. Every raw-deck surface passes the
// findings it is about to return through it (diagnostics, fitFindings); no
// check knows about it. For each finding it
//
//   - rewrites path to the authored pointer (address);
//   - sets slide_number, the 1-based number of the rendered slide — the index
//     a thumbnail, render_slide_image and repair_slide.slide_index count in;
//   - keeps the engine's own path as debug.locator when the two differ.
//
// A tool that takes a path back (repair_slide's path / cell_path) accepts
// both forms: engine turns an authored pointer into the engine's locator for
// the slide the call names.

// authoredPaths translates engine locators for one decoded deck.
type authoredPaths struct {
	input *PresentationInput
	// expanded caches the grid each pattern object expands to, keyed by the
	// pattern's engine pointer.
	expanded map[string]any
	// dropped are the slides (engine index) partial mode left out of the
	// rendered deck; each leaves a CONTENT_DROPPED finding at its bare
	// /slides/N path (refusedSlideSkipped). They have no rendered number and
	// the slides after them move up.
	dropped map[int]bool
}

func newAuthoredPaths(input *PresentationInput) *authoredPaths {
	return &authoredPaths{input: input, expanded: map[string]any{}}
}

// deckAddress is a finding's place in the authored deck.
type deckAddress struct {
	// Path is a JSON Pointer into the deck the author sent.
	Path string
	// SlideNumber is the 1-based number of the rendered slide, 0 when the
	// path names no slide.
	SlideNumber int
	// Exact says Path names the same element the engine's locator does, so a
	// tool argument may carry it instead. An address that fell back to an
	// enclosing object (a pattern for one of its cells, the slide for its
	// chrome) is not exact.
	Exact bool
}

// address translates one engine path. source is the authored element behind a
// path that locates something written at render time, "" when not known.
func (a *authoredPaths) address(path, source string) deckAddress {
	idx := slidepath.SlideIndex(path)
	if a == nil || a.input == nil || idx < 0 || idx >= len(a.input.Slides) {
		return deckAddress{Path: path, Exact: true}
	}
	prefix := slidepath.Slide(idx)
	if path != prefix && !strings.HasPrefix(path, prefix+"/") {
		return deckAddress{Path: path, Exact: true}
	}
	slide := &a.input.Slides[idx]
	rest, exact := a.slideRest(slide, idx, strings.TrimPrefix(path, prefix), source)
	out := deckAddress{SlideNumber: a.renderedNumber(idx), Exact: exact}
	origin := slide.Origin
	switch {
	case origin == nil || origin.Pointer == "":
		out.Path = prefix + rest
	case origin.Generated:
		var named bool
		out.Path, named = generatedSlideField(origin, rest)
		out.Exact = exact && named
	default:
		out.Path = origin.Pointer + splitPageRest(origin, rest, +1)
	}
	return out
}

// renderedNumber is the 1-based number slide idx renders as, 0 when partial
// mode dropped it.
func (a *authoredPaths) renderedNumber(idx int) int {
	if a.dropped[idx] {
		return 0
	}
	n := idx + 1
	for d := range a.dropped {
		if d < idx {
			n--
		}
	}
	return n
}

// noteDropped records the slide a CONTENT_DROPPED finding at a bare slide
// path says partial mode left out.
func (a *authoredPaths) noteDropped(code, path string) {
	if code != patterns.ErrCodeContentDropped {
		return
	}
	if idx := slidepath.SlideIndex(path); idx >= 0 && path == slidepath.Slide(idx) {
		if a.dropped == nil {
			a.dropped = map[int]bool{}
		}
		a.dropped[idx] = true
	}
}

// generatedSlideField is the authored value behind a part of a slide the
// engine built: the longest Fields key that rest sits under, else the element
// that asks for the slide.
func generatedSlideField(origin *deckinput.SlideOrigin, rest string) (string, bool) {
	best := ""
	for key := range origin.Fields {
		if (rest == key || strings.HasPrefix(rest, key+"/")) && len(key) > len(best) {
			best = key
		}
	}
	if best == "" {
		return origin.Pointer, rest == ""
	}
	return origin.Fields[best], true
}

// splitTableRowRE matches a path into a content block's table rows.
var splitTableRowRE = regexp.MustCompile(`^/content/(\d+)/(table_value|value)(/rows/(\d+))?(/.*)?$`)

// splitPageRest moves a path on a page of a split_slide between the page's
// window of the table and the authored table: the authored key of the table
// and, for a row, its index there. dir is +1 from page to authored deck and -1
// back.
func splitPageRest(origin *deckinput.SlideOrigin, rest string, dir int) string {
	if !origin.SplitPage {
		return rest
	}
	m := splitTableRowRE.FindStringSubmatch(rest)
	if m == nil || m[1] != strconv.Itoa(origin.TableContent) {
		return rest
	}
	field := origin.TableField
	if dir < 0 {
		field = "table_value"
	}
	out := "/content/" + m[1] + "/" + field
	if m[3] == "" {
		return out + m[5]
	}
	row, _ := strconv.Atoi(m[4])
	return out + "/rows/" + strconv.Itoa(row+dir*origin.RowOffset) + m[5]
}

// patternAuthoredKeys are the keys of a pattern object; anything else under
// /pattern addresses the grid it expands to.
var patternAuthoredKeys = jsonKeysOf(reflect.TypeOf(PatternInput{}))

func jsonKeysOf(t reflect.Type) map[string]bool {
	keys := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			keys[name] = true
		}
	}
	return keys
}

// slideRest translates the part of an engine path below its slide.
func (a *authoredPaths) slideRest(slide *SlideInput, idx int, rest, source string) (string, bool) {
	switch {
	case rest == "/chrome" || strings.HasPrefix(rest, "/chrome/"):
		// Footer and page-number chrome is drawn by the template on that
		// slide; the deck's chrome block configures it for every slide.
		return "", false
	case strings.HasPrefix(rest, "/rendered_shapes"):
		prefix := slidepath.Slide(idx)
		if source != "" && source != prefix && strings.HasPrefix(source, prefix+"/") && !strings.Contains(source, "/rendered_shapes") {
			sourceRest, _ := a.slideRest(slide, idx, strings.TrimPrefix(source, prefix), "")
			return sourceRest, false
		}
		return "", false
	}
	if tail, ok := cutPrefixPath(rest, "/shape_grid"); ok && slide.ShapeGrid == nil {
		switch {
		case slide.Compose != nil:
			sub, exact := a.composeRest(slide.Compose, slidepath.Slide(idx)+"/compose", tail)
			return "/compose" + sub, exact
		case slide.Pattern != nil:
			rest = "/pattern" + tail
		default:
			return "", false
		}
	}
	// A cell of the grid a pattern expands to, at the slide or nested in a
	// grid cell: the first /pattern whose next key is not one of its own.
	for from := 0; ; {
		at := strings.Index(rest[from:], "/pattern/")
		if at < 0 {
			break
		}
		at += from
		object := rest[:at+len("/pattern")]
		tail := rest[len(object):]
		key, _, _ := strings.Cut(tail[1:], "/")
		if patternAuthoredKeys[key] {
			from = at + len("/pattern/")
			continue
		}
		value, exact := a.patternValue(slide, idx, object, tail)
		return object + value, exact
	}
	return rest, true
}

// cutPrefixPath reports whether path is prefix or sits under it, and returns
// what follows.
func cutPrefixPath(path, prefix string) (string, bool) {
	if path == prefix {
		return "", true
	}
	if strings.HasPrefix(path, prefix+"/") {
		return path[len(prefix):], true
	}
	return "", false
}

// composeRest translates a path into the grid a compose envelope expands to
// (tail, below the grid root) into the envelope: the segment that owns the
// cell and, inside it, its pattern's value, its diagram or its own nested
// envelope; the banner or callout band; the envelope itself when the cell
// belongs to no one of them. pointer is the envelope's engine pointer.
//
// The merged grid is one nested grid per segment (mergeVertical: a row each;
// mergeHorizontal: a cell each of one row), with the banner row prepended and
// the callout row appended.
func (a *authoredPaths) composeRest(c *ComposeInput, pointer, tail string) (string, bool) {
	tokens := pointerTokens(tail)
	if len(tokens) < 2 || tokens[0] != "rows" {
		return "", tail == ""
	}
	band, seg, tokens := composeCell(c, tokens)
	if band != "" || seg < 0 {
		return band, false
	}
	segment := "/segments/" + strconv.Itoa(seg)
	if len(tokens) == 0 || tokens[0] != "grid" {
		return segment, len(tokens) == 0
	}
	inner := joinPointer(tokens[1:])
	s := &c.Segments[seg]
	switch {
	case s.Compose != nil:
		sub, exact := a.composeRest(s.Compose, pointer+segment+"/compose", inner)
		return segment + "/compose" + sub, exact
	case s.HasDiagram():
		// diagramSegmentGrid: one row, one cell, the diagram.
		if sub, ok := cutPrefixPath(inner, "/rows/0/cells/0/diagram"); ok {
			return segment + "/diagram" + sub, true
		}
		return segment + "/diagram", false
	case s.HasPattern():
		if inner == "" {
			return segment + "/pattern", true
		}
		value, exact := a.patternCellValue(&s.Pattern, pointer+segment+"/pattern", inner)
		return segment + "/pattern" + value, exact
	}
	return segment, false
}

// composeCell says which part of envelope c owns a place in its merged grid
// (tokens, from "rows"): a band ("/banner", "/callout"), or segment seg with
// the tokens that follow the segment's cell. seg is -1 with no band when the
// place belongs to neither.
func composeCell(c *ComposeInput, tokens []string) (band string, seg int, rest []string) {
	row, err := strconv.Atoi(tokens[1])
	if err != nil {
		return "", -1, nil
	}
	tokens = tokens[2:]
	if c.Banner != nil {
		if row == 0 {
			return "/banner", -1, nil
		}
		row--
	}
	if c.Direction == "horizontal" {
		// One row of segment cells, then the callout.
		if row == 1 && c.Callout != nil {
			return "/callout", -1, nil
		}
		if row != 0 || len(tokens) < 2 || tokens[0] != "cells" {
			return "", -1, nil
		}
		if seg, err = strconv.Atoi(tokens[1]); err != nil {
			return "", -1, nil
		}
		tokens = tokens[2:]
	} else {
		// A row per segment, then the callout.
		if row == len(c.Segments) && c.Callout != nil {
			return "/callout", -1, nil
		}
		seg = row
		if len(tokens) >= 2 && tokens[0] == "cells" {
			tokens = tokens[2:]
		}
	}
	if seg < 0 || seg >= len(c.Segments) {
		return "", -1, nil
	}
	return "", seg, tokens
}

func joinPointer(tokens []string) string {
	var b strings.Builder
	for _, tok := range tokens {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(strings.ReplaceAll(tok, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

// patternValue is patternCellValue for the pattern object at an engine pointer
// below a slide (object, e.g. "/pattern" or
// "/shape_grid/rows/0/cells/1/pattern").
func (a *authoredPaths) patternValue(slide *SlideInput, idx int, object, tail string) (string, bool) {
	var p *PatternInput
	if object == "/pattern" {
		p = slide.Pattern
	} else {
		p = patternAt(slide, object)
	}
	if p == nil {
		return "", false
	}
	return a.patternCellValue(p, slidepath.Slide(idx)+object, tail)
}

// patternAt decodes the pattern object at a pointer below slide, nil when
// there is none.
func patternAt(slide *SlideInput, object string) *PatternInput {
	raw, err := json.Marshal(slide)
	if err != nil {
		return nil
	}
	var doc any
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	node, _, whole := descend(doc, pointerTokens(object))
	if !whole {
		return nil
	}
	raw, err = json.Marshal(node)
	if err != nil {
		return nil
	}
	var p PatternInput
	if json.Unmarshal(raw, &p) != nil || p.Name == "" {
		return nil
	}
	return &p
}

// descend walks tokens from node as far as they resolve. It returns the
// deepest node reached, the innermost grid cell passed on the way (nil when
// none), and whether every token resolved.
func descend(node any, tokens []string) (reached, cell any, whole bool) {
	for i, tok := range tokens {
		var next any
		switch cur := node.(type) {
		case map[string]any:
			v, ok := cur[tok]
			if !ok {
				return node, cell, false
			}
			next = v
		case []any:
			n, err := strconv.Atoi(tok)
			if err != nil || n < 0 || n >= len(cur) {
				return node, cell, false
			}
			next = cur[n]
		default:
			return node, cell, false
		}
		node = next
		if i > 0 && tokens[i-1] == "cells" {
			cell = node
		}
	}
	return node, cell, true
}

// patternCellValue is the authored value behind a place in the grid pattern p
// expands to (tail, below the grid root): "/values/2/small" for the cell text
// that value is written as, "/values/2" for a cell that shows several fields
// of one value, "" when the expansion does not say (a row, a connector, text
// the pattern composes) — the finding is then on the pattern itself.
//
// The expansion is deterministic, so expanding the pattern, reading the
// cell's text and finding that string in pattern.values is the provenance map
// reduce_cell_text has always used (reduceCellTextOnPattern). A string that
// appears twice in the values names no one of them.
func (a *authoredPaths) patternCellValue(p *PatternInput, pointer, tail string) (string, bool) {
	if p == nil || len(p.Values) == 0 {
		return "", false
	}
	grid, ok := a.expanded[pointer]
	if !ok {
		grid = expandedPatternDoc(p)
		a.expanded[pointer] = grid
	}
	if grid == nil {
		return "", false
	}
	tokens := pointerTokens(tail)
	node, cell, whole := descend(grid, tokens)
	if cell == nil {
		return "", false
	}
	// A path that names the cell's text (or one paragraph of it) is about
	// that text; any other place in the cell is about the cell.
	textual := whole && len(tokens) > 0 && (tokens[len(tokens)-1] == "text" ||
		(len(tokens) > 1 && tokens[len(tokens)-2] == "paragraphs"))
	var texts []string
	if textual {
		texts = textParts(node)
	} else {
		texts = cellTexts(cell)
	}
	var values any
	if json.Unmarshal(p.Values, &values) != nil {
		return "", false
	}
	var found [][]string
	seen := map[string]bool{}
	for _, text := range texts {
		matches := findValueStrings(values, "", text)
		if len(matches) != 1 || seen[matches[0].path] {
			continue
		}
		seen[matches[0].path] = true
		found = append(found, dottedTokens(matches[0].path))
	}
	if len(found) == 0 {
		return "", false
	}
	common := found[0]
	for _, tokens := range found[1:] {
		n := 0
		for n < len(common) && n < len(tokens) && common[n] == tokens[n] {
			n++
		}
		common = common[:n]
	}
	return "/values" + joinPointer(common), textual && len(found) == 1
}

// expandedPatternDoc is the grid p expands to, as decoded JSON; nil when it
// does not expand.
func expandedPatternDoc(p *PatternInput) any {
	grid := expandSlidePatternGrid(&SlideInput{Pattern: p}, 0, 0, 0, nil)
	if grid == nil {
		return nil
	}
	raw, err := json.Marshal(grid)
	if err != nil {
		return nil
	}
	var doc any
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	return doc
}

// textParts are the strings a text node (a string, or {content, paragraphs})
// or one paragraph of it holds.
func textParts(node any) []string {
	raw, err := json.Marshal(node)
	if err != nil {
		return nil
	}
	return cellTextParts(raw)
}

// cellTexts are the strings a grid cell shows: its shape's text and a
// composite's text half.
func cellTexts(cell any) []string {
	m, _ := cell.(map[string]any)
	if m == nil {
		return nil
	}
	var out []string
	if shape, ok := m["shape"].(map[string]any); ok {
		out = append(out, textParts(shape["text"])...)
	}
	if composite, ok := m["composite"].(map[string]any); ok {
		if text, ok := composite["text"].(map[string]any); ok {
			out = append(out, textParts(text["text"])...)
		}
	}
	return out
}

// engine turns an authored pointer a caller hands back into the engine's
// locator on slide slideIdx of the expanded deck (the slide the call names).
// A path that already is one, or that names nothing on that slide, is
// returned unchanged.
func (a *authoredPaths) engine(path string, slideIdx int) string {
	if a == nil || a.input == nil || !strings.HasPrefix(path, "/") {
		return path
	}
	if out, ok := a.engineOnSlide(path, slideIdx); ok {
		return out
	}
	// A path on the slide the call names is the engine's locator already.
	if _, ok := cutPrefixPath(path, slidepath.Slide(slideIdx)); ok && slideIdx >= 0 {
		return path
	}
	// The longest authored prefix wins: "/slides/1/base" before "/slides/1".
	best, bestLen := path, -1
	for i := range a.input.Slides {
		origin := a.input.Slides[i].Origin
		if origin == nil || len(origin.Pointer) <= bestLen {
			continue
		}
		if out, ok := a.engineOnSlide(path, i); ok {
			best, bestLen = out, len(origin.Pointer)
		}
	}
	return best
}

// engineOnSlide is the engine's locator for an authored pointer on slide i of
// the expanded deck; ok is false when the pointer does not name that slide.
func (a *authoredPaths) engineOnSlide(path string, i int) (string, bool) {
	slides := a.input.Slides
	if i < 0 || i >= len(slides) || slides[i].Origin == nil || slides[i].Origin.Pointer == "" {
		return "", false
	}
	origin := slides[i].Origin
	if origin.Generated {
		for key, authored := range origin.Fields {
			if rest, ok := cutPrefixPath(path, authored); ok {
				return slidepath.Slide(i) + key + rest, true
			}
		}
		return slidepath.Slide(i), path == origin.Pointer
	}
	rest, ok := cutPrefixPath(path, origin.Pointer)
	if !ok || !a.splitPageHolds(i, rest) {
		return "", false
	}
	return slidepath.Slide(i) + splitPageRest(origin, rest, -1), true
}

// splitPageHolds reports whether a path below a split_slide's base is on
// page i: any path that is not a row of the table, and the rows the page
// carries.
func (a *authoredPaths) splitPageHolds(i int, rest string) bool {
	slides := a.input.Slides
	origin := slides[i].Origin
	if !origin.SplitPage {
		return true
	}
	m := splitTableRowRE.FindStringSubmatch(rest)
	if m == nil || m[3] == "" || m[1] != strconv.Itoa(origin.TableContent) {
		return true
	}
	row, _ := strconv.Atoi(m[4])
	if row < origin.RowOffset {
		return false
	}
	if i+1 < len(slides) {
		if next := slides[i+1].Origin; next != nil && next.SplitPage && next.Pointer == origin.Pointer {
			return row < next.RowOffset
		}
	}
	return true
}

// engineSlideIndex is the index, in the expanded deck, of the slide an
// authored pointer is on; -1 when it names no slide.
func (a *authoredPaths) engineSlideIndex(path string) int {
	if a == nil || a.input == nil {
		return -1
	}
	idx := slidepath.SlideIndex(a.engine(path, -1))
	if idx >= len(a.input.Slides) {
		return -1
	}
	return idx
}

// hasOrigins reports a deck whose slide list is not the one the author wrote.
func (a *authoredPaths) hasOrigins() bool {
	if a == nil || a.input == nil {
		return false
	}
	for i := range a.input.Slides {
		if a.input.Slides[i].Origin != nil {
			return true
		}
	}
	return false
}

// sentences returns lines that quote a finding's place ("CODE at path", an
// error line) with each engine locator replaced by the authored path of the
// finding that carries it. findings have been through fitFindings.
func (a *authoredPaths) sentences(lines []string, findings []patterns.FitFinding) []string {
	if len(lines) == 0 {
		return lines
	}
	type pair struct{ locator, path string }
	var found []pair
	for _, f := range findings {
		if locator, _ := f.Debug[locatorDebugKey].(string); locator != "" {
			found = append(found, pair{locator, f.Path})
		}
	}
	if len(found) == 0 {
		return lines
	}
	// One pass, the longer locator first: an authored path is never taken
	// for another finding's locator, nor /slides/2/content/1 for the start of
	// /slides/2/content/10.
	sort.SliceStable(found, func(i, j int) bool { return len(found[i].locator) > len(found[j].locator) })
	out := make([]string, len(lines))
	for i, line := range lines {
		var b strings.Builder
		for at := 0; at < len(line); {
			matched := false
			for _, p := range found {
				end := at + len(p.locator)
				if strings.HasPrefix(line[at:], p.locator) && (end == len(line) || !isPointerByte(line[end])) {
					b.WriteString(p.path)
					at, matched = end, true
					break
				}
			}
			if !matched {
				b.WriteByte(line[at])
				at++
			}
		}
		out[i] = b.String()
	}
	return out
}

// mcpError is api.MCPDiagnosticsError for findings about the deck.
func (a *authoredPaths) mcpError(ds []diagnostics.Diagnostic) *mcp.CallToolResult {
	return api.MCPDiagnosticsError(a.diagnostics(ds))
}

// replacePathMentions rewrites each mention of the pointer old in text as
// replacement. A mention ends where the pointer does: "/slides/1" inside
// "/slides/10" or "/slides/1/content/0" is another pointer and is left alone.
func replacePathMentions(text, old, replacement string) string {
	if old == "" || !strings.Contains(text, old) {
		return text
	}
	var b strings.Builder
	for {
		at := strings.Index(text, old)
		if at < 0 {
			b.WriteString(text)
			return b.String()
		}
		end := at + len(old)
		b.WriteString(text[:at])
		if end < len(text) && isPointerByte(text[end]) {
			b.WriteString(old)
		} else {
			b.WriteString(replacement)
		}
		text = text[end:]
	}
}

// isPointerByte reports a byte that continues a JSON Pointer in prose.
func isPointerByte(c byte) bool {
	return c == '/' || c == '_' || c == '~' || c == '-' ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// locatorDebugKey is the debug entry that keeps the engine's own path.
const locatorDebugKey = "locator"

// diagnostics returns ds addressed to the authored deck. ds is not modified.
func (a *authoredPaths) diagnostics(ds []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	if a == nil || a.input == nil || len(ds) == 0 {
		return ds
	}
	for _, d := range ds {
		if !d.Authored {
			a.noteDropped(d.Code, d.Path)
		}
	}
	out := make([]diagnostics.Diagnostic, 0, len(ds))
	seen := map[string]bool{}
	for _, d := range ds {
		page := a.onSplitPage(d.Path, d.Authored)
		d = a.diagnostic(d)
		if a.repeatsSplitPage(seen, d.Code, d.Path, page) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// onSplitPage reports whether an engine path is on a page of a split_slide.
func (a *authoredPaths) onSplitPage(path string, authored bool) bool {
	idx := slidepath.SlideIndex(path)
	if authored || idx < 0 || idx >= len(a.input.Slides) {
		return false
	}
	o := a.input.Slides[idx].Origin
	return o != nil && o.SplitPage
}

// repeatsSplitPage reports a finding on a page of a split_slide that an
// earlier page of the same slide already carries: the pages are one authored
// slide, so its title, its layout and its notes are one element each, and a
// finding about one of them is reported once (the rows of the table are
// distinct elements and never repeat).
func (a *authoredPaths) repeatsSplitPage(seen map[string]bool, code, path string, onSplitPage bool) bool {
	if !onSplitPage {
		return false
	}
	key := code + "\x00" + path
	if seen[key] {
		return true
	}
	seen[key] = true
	return false
}

func (a *authoredPaths) diagnostic(d diagnostics.Diagnostic) diagnostics.Diagnostic {
	if d.Authored {
		if d.SlideNumber == 0 {
			d.SlideNumber = a.engineSlideIndex(d.Path) + 1
		}
		return d
	}
	d.Authored = true
	addr := a.address(d.Path, d.Source)
	if addr.SlideNumber > 0 {
		d.SlideNumber = addr.SlideNumber
	}
	if addr.Path != d.Path {
		d.Debug = withDebug(copyFacts(d.Debug), locatorDebugKey, d.Path)
		d.Message = replacePathMentions(d.Message, d.Path, addr.Path)
		d.Path = addr.Path
	}
	if d.Fix != nil && len(d.Fix.Params) > 0 {
		fix := *d.Fix
		fix.Params, _ = a.facts(fix.Params, true).(map[string]any)
		d.Fix = &fix
	}
	if d.NextToolCall != nil && len(d.NextToolCall.ArgsTemplate) > 0 {
		call := *d.NextToolCall
		call.ArgsTemplate, _ = a.facts(call.ArgsTemplate, true).(map[string]any)
		d.NextToolCall = &call
	}
	if len(d.Details) > 0 {
		d.Details, _ = a.facts(d.Details, false).(map[string]any)
	}
	return d
}

// fitFindings returns fs addressed to the authored deck. fs is not modified.
func (a *authoredPaths) fitFindings(fs []patterns.FitFinding) []patterns.FitFinding {
	if a == nil || a.input == nil || len(fs) == 0 {
		return fs
	}
	for _, f := range fs {
		if !f.Authored {
			a.noteDropped(f.Code, f.Path)
		}
	}
	out := make([]patterns.FitFinding, 0, len(fs))
	seen := map[string]bool{}
	for _, f := range fs {
		page := a.onSplitPage(f.Path, f.Authored)
		f = a.fitFinding(f)
		if a.repeatsSplitPage(seen, f.Code, f.Path, page) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (a *authoredPaths) fitFinding(f patterns.FitFinding) patterns.FitFinding {
	if f.Authored {
		if f.SlideNumber == 0 {
			f.SlideNumber = a.engineSlideIndex(f.Path) + 1
		}
		return f
	}
	f.Authored = true
	addr := a.address(f.Path, f.Source)
	if addr.SlideNumber > 0 {
		f.SlideNumber = addr.SlideNumber
	}
	if addr.Path != f.Path {
		f.Debug = withDebug(copyFacts(f.Debug), locatorDebugKey, f.Path)
		f.Message = replacePathMentions(f.Message, f.Path, addr.Path)
		f.Path = addr.Path
	}
	if f.Fix != nil && len(f.Fix.Params) > 0 {
		fix := *f.Fix
		fix.Params, _ = a.facts(fix.Params, true).(map[string]any)
		f.Fix = &fix
	}
	if f.NextToolCall != nil && len(f.NextToolCall.ArgsTemplate) > 0 {
		call := *f.NextToolCall
		call.ArgsTemplate, _ = a.facts(call.ArgsTemplate, true).(map[string]any)
		f.NextToolCall = &call
	}
	return f
}

// facts returns a copy of a fact tree (fix params, a tool-call template,
// details) with every engine path in it addressed to the authored deck.
// toolArgs says the tree is one a tool is called with: a path / cell_path
// whose authored address is only an enclosing object (a pattern for one of
// its cells) is then left as the engine's locator, which the tool accepts as
// well and can act on.
func (a *authoredPaths) facts(v any, toolArgs bool) any {
	return a.fact(v, false, toolArgs)
}

func (a *authoredPaths) fact(v any, exactOnly, toolArgs bool) any {
	switch t := v.(type) {
	case string:
		if !strings.HasPrefix(t, "/slides/") {
			return t
		}
		if addr := a.address(t, ""); addr.Exact || !exactOnly {
			return addr.Path
		}
		return t
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = a.fact(e, toolArgs && containsString(pathParamKeys, k), toolArgs)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = a.fact(e, exactOnly, toolArgs)
		}
		return out
	case []string:
		out := make([]string, len(t))
		for i, e := range t {
			out[i], _ = a.fact(e, exactOnly, toolArgs).(string)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(t))
		for i, e := range t {
			out[i], _ = a.fact(e, exactOnly, toolArgs).(map[string]any)
		}
		return out
	}
	return v
}
