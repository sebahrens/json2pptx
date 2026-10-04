package patterns

// Text budgets the DeckSpec kind catalogue states (list_slide_kinds budgets).
// Each is the pattern's own limit, exported so the catalogue, the schema
// descriptions and the findings quote one number (go-slide-creator-iubjb).
const (
	ExecSummaryLeadMax       = execSummaryLeadMax
	ExecSummarySupportMax    = execSummarySupportMax
	ExecSummaryBottomLineMax = execSummaryBottomLineMax

	NextStepsActionMax   = nextStepsActionMax
	NextStepsOwnerMax    = nextStepsOwnerMax
	NextStepsDateMax     = nextStepsDateMax
	NextStepsDecisionMax = nextStepsDecisionMax

	KPIValueMax = kpiNupBigMaxChars
	// KPIValueFiveMax / KPIValueSixMax are the digits a value holds on one
	// line with five and six KPIs on the narrowest shipped template
	// (KPIValueLineBudget measures a named template).
	KPIValueFiveMax  = 11
	KPIValueSixMax   = 9
	KPIDeltaMax      = kpiSubMaxChars
	KPIComparatorMax = kpiComparatorMaxChars

	// StatHeroStackBudget is the combined label + context + source length the
	// stack beneath a stat-hero number holds at readable sizes.
	StatHeroStackBudget = statHeroStackBudget
)
