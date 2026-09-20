package slides

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/types"
)

type orgEntry struct{ id, name, title, parent string }
type orgResolution struct {
	entries       []orgEntry
	children      map[string][]string
	root          string
	path, problem string
	hard          bool
}

// A small tree is the only tree this fast path draws. svggen can prune a
// larger one even when it is under max_nodes, which would hide named people.
const (
	orgMaxNodes    = 7
	orgMaxDepth    = 3
	orgMaxSiblings = 4
	orgLabelMax    = 40
)

func resolveOrg(body map[string]any) orgResolution {
	raw, ok := body["nodes"].([]any)
	if !ok {
		return orgResolution{path: "nodes", problem: "nodes must be an array", hard: true}
	}
	if len(raw) == 0 {
		return orgResolution{path: "nodes", problem: "provide at least one named node", hard: true}
	}
	r := orgResolution{children: map[string][]string{}}
	byID := map[string]int{}
	for i, item := range raw {
		e, path, problem := parseOrgEntry(item, i)
		if problem != "" {
			return orgResolution{path: path, problem: problem, hard: true}
		}
		if _, exists := byID[e.id]; exists {
			return orgResolution{path: fmt.Sprintf("nodes[%d].id", i), problem: "id must be unique", hard: true}
		}
		byID[e.id] = i
		r.entries = append(r.entries, e)
	}
	for i, e := range r.entries {
		if e.parent == "" {
			if r.root != "" {
				return orgResolution{path: fmt.Sprintf("nodes[%d].parent", i), problem: "exactly one node may omit parent", hard: true}
			}
			r.root = e.id
			continue
		}
		if _, exists := byID[e.parent]; !exists {
			return orgResolution{path: fmt.Sprintf("nodes[%d].parent", i), problem: "parent must name an existing node id", hard: true}
		}
		r.children[e.parent] = append(r.children[e.parent], e.id)
	}
	if r.root == "" {
		return orgResolution{path: "nodes", problem: "exactly one root node must omit parent", hard: true}
	}
	visited := map[string]bool{}
	var walk func(string, int)
	maxDepth := 0
	walk = func(id string, depth int) {
		if visited[id] {
			return
		}
		visited[id] = true
		if depth > maxDepth {
			maxDepth = depth
		}
		for _, child := range r.children[id] {
			walk(child, depth+1)
		}
	}
	walk(r.root, 1)
	if len(visited) != len(r.entries) {
		return orgResolution{path: "nodes", problem: "nodes must form one connected reporting tree without cycles", hard: true}
	}
	if len(r.entries) > orgMaxNodes {
		r.path = "nodes"
		r.problem = fmt.Sprintf("%d nodes exceed the seven-node visual budget", len(r.entries))
		return r
	}
	if maxDepth > orgMaxDepth {
		r.path = "nodes"
		r.problem = "tree exceeds three reporting levels"
		return r
	}
	for i, e := range r.entries {
		if utf8.RuneCountInString(e.name) > orgLabelMax {
			r.path = fmt.Sprintf("nodes[%d].name", i)
			r.problem = "name exceeds 40 characters"
			return r
		}
		if utf8.RuneCountInString(e.title) > orgLabelMax {
			r.path = fmt.Sprintf("nodes[%d].title", i)
			r.problem = "title exceeds 40 characters"
			return r
		}
		if len(r.children[e.id]) > orgMaxSiblings {
			r.path = fmt.Sprintf("nodes[%d]", i)
			r.problem = "node has more than four direct reports"
			return r
		}
	}
	return r
}

func parseOrgEntry(item any, i int) (orgEntry, string, string) {
	path := fmt.Sprintf("nodes[%d]", i)
	obj, ok := item.(map[string]any)
	if !ok {
		return orgEntry{}, path, "node must be an object"
	}
	id, ok := obj["id"].(string)
	if !ok || strings.TrimSpace(id) == "" {
		return orgEntry{}, path + ".id", "id must be a non-empty string"
	}
	name, ok := obj["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return orgEntry{}, path + ".name", "name must be a non-empty string"
	}
	e := orgEntry{id: strings.TrimSpace(id), name: strings.TrimSpace(name)}
	if v, present := obj["title"]; present {
		s, ok := v.(string)
		if !ok {
			return orgEntry{}, path + ".title", "title must be a string"
		}
		e.title = strings.TrimSpace(s)
	}
	if v, present := obj["parent"]; present {
		s, ok := v.(string)
		if !ok {
			return orgEntry{}, path + ".parent", "parent must be a string"
		}
		e.parent = strings.TrimSpace(s)
	}
	return e, "", ""
}

func OrgFits(body map[string]any) bool { return resolveOrg(body).problem == "" }
func OrgIssue(body map[string]any) (string, string, bool) {
	r := resolveOrg(body)
	return r.path, r.problem, r.hard
}

func CompileOrg(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	r := resolveOrg(in.Body)
	if r.problem != "" || in.Layout == "content" {
		return compileOrgFallback(in)
	}
	byID := map[string]orgEntry{}
	for _, e := range r.entries {
		byID[e.id] = e
	}
	var build func(string) map[string]any
	build = func(id string) map[string]any {
		e := byID[id]
		node := map[string]any{"name": e.name}
		if e.title != "" {
			node["title"] = e.title
		}
		if children := r.children[id]; len(children) > 0 {
			nested := make([]any, 0, len(children))
			for _, child := range children {
				nested = append(nested, build(child))
			}
			node["children"] = nested
		}
		return node
	}
	slide := &deckinput.SlideInput{SlideType: "diagram"}
	links := titleLink(slide, in)
	idx := appendContent(slide, diagramContent("body", &types.DiagramSpec{Type: "org_chart", Data: map[string]any{"root": build(r.root)}, Alt: visualAltText(in)}))
	links = append(links, SourceLink{RawPath: fmt.Sprintf("%s.content[%d].diagram_value.data.root", in.rawSlide(), idx), SemanticPath: in.semSlide() + ".nodes"})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

func compileOrgFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	r := resolveOrg(in.Body)
	depth := map[string]int{}
	var markDepth func(string, int)
	markDepth = func(id string, level int) {
		if _, seen := depth[id]; seen {
			return
		}
		depth[id] = level
		for _, child := range r.children[id] {
			markDepth(child, level+1)
		}
	}
	if r.root != "" {
		markDepth(r.root, 0)
	}
	names := map[string]string{}
	for _, e := range r.entries {
		names[e.id] = e.name
	}
	raw, _ := in.Body["nodes"].([]any)
	bullets := make([]string, 0, len(raw))
	for _, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			bullets = append(bullets, fmt.Sprint(item))
			continue
		}
		id, _ := obj["id"].(string)
		line := strings.Repeat("↳ ", depth[id]) + fmt.Sprint(obj["name"])
		if v, present := obj["title"]; present {
			line += " — " + fmt.Sprint(v)
		}
		if v, present := obj["parent"]; present && fmt.Sprint(v) != "" {
			parent := fmt.Sprint(v)
			if name := names[parent]; name != "" {
				parent = name
			}
			line += " (reports to " + parent + ")"
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{RawPath: fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx), SemanticPath: in.semSlide() + ".nodes"})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}
