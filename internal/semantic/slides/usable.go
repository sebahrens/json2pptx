package slides

// This file exposes the per-kind compilers' content-extraction counts to the
// semantic validation layer. Validation must agree with compile about whether a
// content-bearing field actually yields renderable content: a required list of
// blank or labelless entries passes a raw presence/length check but compiles to
// an empty (title-only) slide because the compiler's extractors trim and drop
// those entries. These helpers count entries that survive the SAME extraction
// the per-kind compilers apply, so a validation gate can fail fast on content
// that would otherwise be silently dropped, and so density-range advisories
// reflect the count compile will actually render.

// UsableStepCount returns the number of process-flow steps that survive
// extraction (CompileProcess via processSteps); blank or labelless entries are
// dropped.
func UsableStepCount(body map[string]any) int { return len(processSteps(body)) }

// UsablePhaseCount returns the number of roadmap phases that survive extraction
// (CompileRoadmap via roadmapPhases); blank or nameless entries are dropped.
func UsablePhaseCount(body map[string]any) int { return len(roadmapPhases(body)) }

// UsableKPICount returns the number of KPI cells that survive extraction
// (CompileKPISnapshot via kpiCells); cells carrying neither a number nor a
// caption are dropped.
func UsableKPICount(body map[string]any) int {
	cells, _ := kpiCells(body)
	return len(cells)
}

// UsableComparisonColumnCount returns the number of comparison columns that
// render at least one readable line, mirroring compileComparisonFallback (the
// safe degradation): a column contributes when it carries a header or any
// usable items. Columns that are entirely blank are dropped.
func UsableComparisonColumnCount(body map[string]any) int {
	n := 0
	for _, col := range mapList(body, "columns") {
		if columnHeader(col) != "" || len(columnItems(col)) > 0 {
			n++
		}
	}
	return n
}
