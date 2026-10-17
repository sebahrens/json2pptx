package semantic

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

const (
	KindStakeholderMap   SlideKind = "stakeholder_map"
	KindVenn             SlideKind = "venn"
	KindFishbone         SlideKind = "fishbone"
	KindPESTEL           SlideKind = "pestel"
	KindNineBoxTalent    SlideKind = "nine_box_talent"
	KindHeatmap          SlideKind = "heatmap"
	KindValueChain       SlideKind = "value_chain"
	KindPyramid          SlideKind = "pyramid"
	KindBulletChart      SlideKind = "bullet_chart"
	KindImageHero        SlideKind = "image_hero"
	KindRACI             SlideKind = "raci"
	KindStatusBoard      SlideKind = "status_board"
	KindProse2Col        SlideKind = "prose_2col"
	KindIPOFlow          SlideKind = "ipo_flow"
	KindThreeHorizons    SlideKind = "three_horizons"
	KindSources          SlideKind = "sources"
	KindChecklist        SlideKind = "checklist"
	KindTimelineVertical SlideKind = "timeline_vertical"
	KindSpectrum         SlideKind = "spectrum"
	KindIssueTree        SlideKind = "issue_tree"
	KindDecisionTree     SlideKind = "decision_tree"
)

var consultingVisualKinds = map[SlideKind]string{KindStakeholderMap: "stakeholder-map", KindVenn: "venn-framework", KindFishbone: "fishbone-framework", KindPESTEL: "pestel-framework", KindNineBoxTalent: "talent-nine-box", KindHeatmap: "numeric-heatmap", KindValueChain: "value-chain", KindPyramid: "pyramid", KindBulletChart: "progress-list", KindImageHero: "image-hero", KindRACI: "raci-matrix", KindStatusBoard: "status-board", KindProse2Col: "prose-2col", KindIPOFlow: "ipo-flow", KindThreeHorizons: "three-horizons", KindSources: "sources-list", KindChecklist: "checklist", KindTimelineVertical: "timeline-vertical", KindSpectrum: "spectrum", KindIssueTree: "issue-tree", KindDecisionTree: "decision-tree"}

func init() {
	for kind, name := range consultingVisualKinds {
		registerConsultingVisualKind(kind, name)
	}
}
func registerConsultingVisualKind(kind SlideKind, name string) {
	p, ok := patterns.Default().Get(name)
	if !ok {
		panic("missing consulting pattern " + name)
	}
	values := p.Schema().Property("values")
	fields := universalFields()
	fields["title"] = strField("Slide claim.")
	fields["takeaway"] = strField("One-line conclusion.")
	shapes := map[string]shapeKind{"title": shapeString, "takeaway": shapeString}
	for key, property := range values.Properties() {
		raw, _ := json.Marshal(property)
		var schema map[string]any
		_ = json.Unmarshal(raw, &schema)
		fieldSchema := schema
		fields[key] = payloadField{typ: string(property.TypeName()), schema: func() map[string]any { return fieldSchema }, desc: property.Description()}
		switch property.TypeName() {
		case patterns.TypeArray:
			shapes[key] = shapeArray
		case patterns.TypeObject:
			shapes[key] = shapeObject
		case patterns.TypeBoolean:
			shapes[key] = shapeBoolean
		case patterns.TypeNumber, patterns.TypeInteger:
			shapes[key] = shapeNumber
		default:
			shapes[key] = shapeString
		}
	}
	kindPayloadFields[kind] = fields
	kindFieldShapes[kind] = shapes
	slideKindRegistry[kind] = KindInfo{Kind: kind, Summary: p.Description(), RequiredFields: append([]string{"title"}, values.RequiredNames()...), TypicalFields: []string{"takeaway"}}
	plan := consultingVisualPlan(kind)
	plan.layout = "blank-title"
	plan.pattern = func(map[string]any) string { return name }
	kindPlanRegistry[kind] = plan
	patternReach[name] = kind
	kindNeedsTakeaway[kind] = kind != KindSources && kind != KindImageHero && kind != KindChecklist
	example := map[string]any{"kind": string(kind), "title": consultingVisualTitle(kind), "takeaway": consultingVisualTitle(kind) + "; owners can now act."}
	exemplar, hasExemplar := p.(interface{ ExemplarValues() any })
	if !hasExemplar {
		panic("consulting pattern requires an exemplar: " + name)
	}
	raw, _ := json.Marshal(exemplar.ExemplarValues())
	var vals map[string]any
	_ = json.Unmarshal(raw, &vals)
	for key, value := range vals {
		example[key] = value
	}
	kindExamples[kind] = example
}
func consultingVisualValues(kind SlideKind, body map[string]any) map[string]any {
	p, ok := patterns.Default().Get(consultingVisualKinds[kind])
	if !ok {
		return nil
	}
	values := map[string]any{}
	for key := range p.Schema().Property("values").Properties() {
		if v, set := body[key]; set {
			values[key] = v
		}
	}
	return values
}
func validateConsultingVisual(path string, slide SlideSpec, s *semDiags) bool {
	name, known := consultingVisualKinds[slide.Kind]
	if !known {
		return false
	}
	p, _ := patterns.Default().Get(name)
	raw, _ := json.Marshal(consultingVisualValues(slide.Kind, slide.Body))
	values := p.NewValues()
	if err := json.Unmarshal(raw, values); err != nil {
		s.hard(path, diagnostics.CodeSemanticFieldType, err.Error())
		return true
	}
	if err := p.Validate(values, nil, nil); err != nil {
		s.hard(path, diagnostics.CodeSemanticFieldType, fmt.Sprintf("%s: %v", name, err))
	}
	return true
}

func consultingVisualPlan(kind SlideKind) kindPlan {
	p := kindPlan{role: RoleAnalysis, family: FamilyComparison, density: DensityMedium}
	switch kind {
	case KindImageHero:
		p.role = RoleOpening
		p.family = FamilyStructural
		p.density = DensityLight
	case KindRACI, KindStatusBoard:
		p.family = FamilyTable
		p.role = RoleEvidence
	case KindSources:
		p.family = FamilyList
		p.role = RoleEvidence
		p.density = DensityHeavy
	case KindChecklist:
		p.family = FamilyList
		p.role = RolePlan
	case KindProse2Col:
		p.family = FamilyText
	case KindIPOFlow, KindIssueTree, KindDecisionTree, KindValueChain:
		p.family = FamilyProcess
	case KindThreeHorizons, KindTimelineVertical:
		p.family = FamilyTimeline
		p.role = RolePlan
	case KindBulletChart:
		p.family = FamilyChart
		p.role = RoleEvidence
	case KindHeatmap:
		p.family = FamilyChart
		p.role = RoleEvidence
	}
	return p
}

func consultingVisualTitle(kind SlideKind) string {
	titles := map[SlideKind]string{KindRACI: "Every activity has one accountable owner", KindStatusBoard: "Controls need a recovery owner", KindProse2Col: "Capacity must grow before market entry", KindIPOFlow: "Three stages turn evidence into action", KindThreeHorizons: "Growth requires three investment horizons", KindSources: "Four sources support the decision", KindChecklist: "Approval remains the final blocker", KindTimelineVertical: "Three milestones lead to scale", KindSpectrum: "The target shifts decision rights outward", KindIssueTree: "Two hypotheses explain delivery gaps", KindDecisionTree: "Two branches define the next decision", KindImageHero: "A platform supports the next stage", KindBulletChart: "Three KPIs remain below target", KindVenn: "Customer needs overlap our strengths", KindFishbone: "Two causes drive delivery delays", KindPESTEL: "Six forces shape the market outlook", KindNineBoxTalent: "Two teams have high growth potential", KindHeatmap: "The south leads in both quarters", KindValueChain: "Five activities build customer value", KindPyramid: "Three levels connect strategy to delivery", KindStakeholderMap: "Three stakeholders need distinct actions"}
	return titles[kind]
}
