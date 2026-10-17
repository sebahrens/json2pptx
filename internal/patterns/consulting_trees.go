package patterns

import "fmt"

func init() {
	for _, p := range consultingTreePatterns() {
		Default().Register(p)
	}
}
func consultingTreePatterns() []*consultingPattern {
	node := visualObject(map[string]*Schema{"id": visualString(24), "parent": StringSchema(24), "label": visualString(120), "edge": StringSchema(40), "status": EnumSchema("confirmed", "rejected", "open"), "value": StringSchema(40)}, "id", "label")
	schema := visualObject(map[string]*Schema{"nodes": visualArray(node, 3, 15)}, "nodes")
	example := map[string]any{"nodes": []any{map[string]any{"id": "root", "label": "What drives the gap?"}, map[string]any{"id": "demand", "parent": "root", "label": "Demand", "edge": "Market", "status": "open"}, map[string]any{"id": "delivery", "parent": "root", "label": "Delivery", "edge": "Execution", "status": "confirmed"}}}
	return []*consultingPattern{
		{name: "issue-tree", description: "A left-rooted 2–4 level issue tree with explicit parent IDs and leaf hypothesis status", valuesSchema: schema, example: example, check: func(v map[string]any) error { _, err := consultingTreeLevels(v, 4); return err }, draw: drawConsultingTree},
		{name: "decision-tree", description: "A 2–3 level decision tree with labeled branches and optional outcome values", valuesSchema: schema, example: example, check: func(v map[string]any) error { _, err := consultingTreeLevels(v, 3); return err }, draw: drawConsultingTree},
	}
}
func consultingTreeLevels(v map[string]any, maxDepth int) ([]int, error) {
	nodes := visualItems(v, "nodes")
	levels := make([]int, len(nodes))
	seen := map[string]int{}
	roots := 0
	deepest := 0
	for i, raw := range nodes {
		n := visualObj(raw)
		id := visualStr(n, "id")
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate tree node id %q", id)
		}
		parent := visualStr(n, "parent")
		if parent == "" {
			roots++
		} else {
			p, ok := seen[parent]
			if !ok {
				return nil, fmt.Errorf("node %q must reference an earlier parent (cycles and missing parents are invalid)", id)
			}
			levels[i] = levels[p] + 1
		}
		seen[id] = i
		deepest = max(deepest, levels[i]+1)
	}
	if roots != 1 || deepest < 2 || deepest > maxDepth {
		return nil, fmt.Errorf("tree requires one root and 2–%d levels", maxDepth)
	}
	return levels, nil
}
func drawConsultingTree(c *consultingCanvas, v map[string]any) error {
	maxDepth := 4
	if c.name == "decision-tree" {
		maxDepth = 3
	}
	levels, err := consultingTreeLevels(v, maxDepth)
	if err != nil {
		return err
	}
	nodes := visualItems(v, "nodes")
	counts := make([]int, maxDepth)
	depth := 0
	for _, level := range levels {
		counts[level]++
		depth = max(depth, level+1)
	}
	gap := 34.0
	w := (c.w - float64(depth-1)*gap) / float64(depth)
	if w <= 0 {
		return c.refuse("tree columns have no usable width")
	}
	positions := make([][4]float64, len(nodes))
	used := make([]int, depth)
	ids := map[string]int{}
	for i, raw := range nodes {
		n := visualObj(raw)
		level := levels[i]
		slot := c.h / float64(counts[level])
		h := slot - 8
		x := float64(level) * (w + gap)
		y := float64(used[level])*slot + 4
		used[level]++
		positions[i] = [4]float64{x, y, w, h}
		ids[visualStr(n, "id")] = i
	}
	// Connectors are emitted first so they never obscure node text.
	for i, raw := range nodes {
		n := visualObj(raw)
		parent := visualStr(n, "parent")
		if parent == "" {
			continue
		}
		p := positions[ids[parent]]
		q := positions[i]
		x := p[0] + p[2]
		y1 := p[1] + p[3]/2
		y2 := q[1] + q[3]/2
		mid := x + gap/2
		if err := c.rule(x, y1, gap/2, 1, "dk1"); err != nil {
			return err
		}
		if err := c.rule(mid, min(y1, y2), 1, max(1.0, max(y1, y2)-min(y1, y2)), "dk1"); err != nil {
			return err
		}
		if err := c.rule(mid, y2, gap/2, 1, "dk1"); err != nil {
			return err
		}
	}
	for i, raw := range nodes {
		n := visualObj(raw)
		p := positions[i]
		if err := c.shape(p[0], p[1], p[2], p[3], "rect", "lt2", nil); err != nil {
			return err
		}
		lines := []visualParagraph{}
		if edge := visualStr(n, "edge"); edge != "" {
			lines = append(lines, visualP("Branch: "+edge, 12, true))
		}
		lines = append(lines, visualP(visualStr(n, "label"), 12, true))
		if status := visualStr(n, "status"); status != "" {
			lines = append(lines, visualP("Hypothesis: "+status, 12, false))
		}
		if value := visualStr(n, "value"); value != "" {
			lines = append(lines, visualP("Outcome: "+value, 12, false))
		}
		if err := c.text(p[0], p[1], p[2], p[3], lines); err != nil {
			return err
		}
	}
	return nil
}
