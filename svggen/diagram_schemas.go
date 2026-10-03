package svggen

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// Data contracts for the non-chart svggen diagrams (go-slide-creator-x9s5i).
//
// The diagram parsers read their data with type assertions on known keys and
// skip anything else, so a timeline item written {"title", "when"} or a venn
// circle written {"title"} parsed cleanly and rendered without its date or
// label while validate called the deck clean. Every registered type now
// declares a DataSchema beside its parser: the keys each payload level is
// read with. The registry checks a payload against it (at every level) before
// Validate, so validate, generate and every placement — body placeholder,
// shape_grid cell, compose segment — refuse an unread key with the keys that
// would have drawn and a did-you-mean. TestEveryRegisteredTypeHasDataSchema
// keeps the set complete.

// nestedSchemaDepth bounds the recursive node schemas (org chart children,
// treemap children). Deeper levels are not checked; no readable org chart or
// treemap nests this far.
const nestedSchemaDepth = 8

// diagramDataSchema builds the closed top-level schema of a non-chart diagram.
// The diagram-level title / subtitle / footnote the docs list beside data are
// tolerated inside data too, unless the type declares them itself.
func diagramDataSchema(desc string, fields map[string]*DataSchema, required []string) *DataSchema {
	for _, k := range []string{"title", "subtitle", "footnote"} {
		if _, ok := fields[k]; !ok {
			fields[k] = ToleratedDataSchema("Diagram " + k + " (set it on the diagram spec; this renderer does not draw it from data)")
		}
	}
	return ObjectDataSchema(desc, fields, required)
}

// stringListSchema describes a list of strings.
func stringListSchema(desc string) *DataSchema {
	return ArrayDataSchema(desc, StringDataSchema("Item"), 0)
}

// ErrCodeBlankPayload marks a payload whose keys all pass the data contract
// but whose every label parses empty, so the diagram would draw only blank
// shapes.
const ErrCodeBlankPayload = core.ErrCodeConstraint

// diagramWithTexts is implemented by diagrams whose payload can parse to
// nothing but empty labels: drawnTexts returns the label and body text the
// renderer would draw, or nil when nothing parsed.
type diagramWithTexts interface {
	drawnTexts(req *RequestEnvelope) []string
}

// blankPayloadError returns a CONSTRAINT error when d's payload parses to
// texts that are all empty, or nil.
func blankPayloadError(d Diagram, req *RequestEnvelope) error {
	dt, ok := d.(diagramWithTexts)
	if !ok {
		return nil
	}
	texts := dt.drawnTexts(req)
	if len(texts) == 0 {
		return nil
	}
	for _, t := range texts {
		if strings.TrimSpace(t) != "" {
			return nil
		}
	}
	return &ValidationError{
		Field:   "data",
		Code:    ErrCodeBlankPayload,
		Message: fmt.Sprintf("%s: every label in the payload is empty, so the diagram would draw only blank shapes — check the per-type data keys (get_diagram_capabilities / docs/diagrams)", req.Type),
	}
}

// CheckDataContract checks a diagram payload against its type's data
// contract without rendering it: an UNKNOWN_FIELD error per key the renderer
// does not read (at any level, with did-you-mean), or a CONSTRAINT error when
// every label parses empty. It returns nil for a type the registry does not
// know or a payload that passes. data is not modified.
//
// Rendering runs the same checks; callers whose render failures degrade
// rather than abort (a chart in a body placeholder) call this to refuse the
// payload up front (go-slide-creator-x9s5i).
func CheckDataContract(diagramType string, data map[string]any) error {
	d := DefaultRegistry().Get(diagramType)
	if d == nil {
		return nil
	}
	if ds, ok := d.(DiagramWithSchema); ok {
		if err := core.ValidateUnknownFields(data, ds.DataSchema(), diagramType); err != nil {
			return err
		}
	}
	if _, ok := d.(diagramWithTexts); !ok || data == nil {
		return nil
	}
	// Validate normalizes aliases in place (events -> activities, flat
	// nodes -> root); run it on a copy so the caller's data is untouched.
	req := &RequestEnvelope{Type: diagramType, Data: cloneDataTop(data)}
	if err := d.Validate(req); err != nil {
		return nil // reported by the render path with its own message
	}
	return blankPayloadError(d, req)
}

// cloneDataTop copies the top level of data.
func cloneDataTop(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		out[k] = v
	}
	return out
}

func (d *Timeline) drawnTexts(req *RequestEnvelope) []string {
	data, err := parseTimelineData(req)
	if err != nil {
		return nil
	}
	texts := make([]string, 0, 2*len(data.Activities))
	for _, a := range data.Activities {
		texts = append(texts, a.Label, a.Description)
	}
	return texts
}

func (d *VennDiagram) drawnTexts(req *RequestEnvelope) []string {
	data, err := parseVennData(req)
	if err != nil {
		return nil
	}
	var texts []string
	for _, c := range data.Circles {
		texts = append(append(texts, c.Label), c.Items...)
	}
	return texts
}

func (d *OrgChartDiagram) drawnTexts(req *RequestEnvelope) []string {
	data, err := parseOrgChartData(req)
	if err != nil {
		return nil
	}
	var texts []string
	var walk func(n *OrgNode)
	walk = func(n *OrgNode) {
		texts = append(texts, n.Name, n.Title)
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(&data.Root)
	return texts
}

func (d *GanttDiagram) drawnTexts(req *RequestEnvelope) []string {
	data, err := parseGanttData(req)
	if err != nil {
		return nil
	}
	texts := make([]string, 0, len(data.Tasks)+len(data.Milestones))
	for _, t := range data.Tasks {
		texts = append(texts, t.Label)
	}
	for _, m := range data.Milestones {
		texts = append(texts, m.Label)
	}
	return texts
}

func (d *FishboneDiagram) drawnTexts(req *RequestEnvelope) []string {
	data, err := extractFishboneData(req)
	if err != nil {
		return nil
	}
	texts := []string{data.Effect}
	for _, c := range data.Categories {
		texts = append(append(texts, c.Name), c.Causes...)
	}
	return texts
}
