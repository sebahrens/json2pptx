package patterns

// ---------------------------------------------------------------------------
// kpi-Nup variant registrations (2..6)
//
// Each variant is a thin config passed to the parametric kpiNup adapter.
// Adding a new variant (e.g. kpi-7up) requires ~10 lines here.
// ---------------------------------------------------------------------------

func init() {
	for _, cfg := range kpiVariants {
		Default().Register(NewKPINup(cfg))
	}
}

// kpiVariants defines all registered KPI variants.
var kpiVariants = []KPINupConfig{
	{
		Count:        2,
		DensityClass: "low",
		Exemplars: []KPICell{
			{Big: "$4.2M", Small: "ARR", Icon: "currency-dollar"},
			{Big: "127%", Small: "NRR", Icon: "trending-up"},
		},
	},
	{
		Count:        3,
		DensityClass: "low",
		Exemplars: []KPICell{
			{Big: "$4.2M", Small: "ARR", Icon: "currency-dollar"},
			{Big: "127%", Small: "NRR", Icon: "trending-up"},
			{Big: "12d", Small: "Sales cycle", Icon: "clock"},
		},
	},
	{
		Count:        4,
		DensityClass: "medium",
		Exemplars: []KPICell{
			{Big: "$4.2M", Small: "ARR", Icon: "currency-dollar"},
			{Big: "127%", Small: "NRR", Icon: "trending-up"},
			{Big: "12d", Small: "Sales cycle", Icon: "clock"},
			{Big: "98%", Small: "CSAT", Icon: "star"},
		},
	},
	{
		Count:        5,
		DensityClass: "medium",
		Exemplars: []KPICell{
			{Big: "$4.2M", Small: "ARR", Icon: "currency-dollar"},
			{Big: "127%", Small: "NRR", Icon: "trending-up"},
			{Big: "12d", Small: "Sales cycle", Icon: "clock"},
			{Big: "98%", Small: "CSAT", Icon: "star"},
			{Big: "42", Small: "NPS", Icon: "chart-bar"},
		},
	},
	{
		Count:        6,
		DensityClass: "high",
		Exemplars: []KPICell{
			{Big: "$4.2M", Small: "ARR", Icon: "currency-dollar"},
			{Big: "127%", Small: "NRR", Icon: "trending-up"},
			{Big: "12d", Small: "Sales cycle", Icon: "clock"},
			{Big: "98%", Small: "CSAT", Icon: "star"},
			{Big: "42", Small: "NPS", Icon: "chart-bar"},
			{Big: "3.2x", Small: "LTV/CAC", Icon: "scale"},
		},
	},
}

// ---------------------------------------------------------------------------
// Backward-compatible type aliases
// ---------------------------------------------------------------------------

// Kpi3upCell is a single KPI cell: a big number and a short caption.
type Kpi3upCell = KPICell

// Kpi3upValues is the values type: exactly 3 KPI cells.
type Kpi3upValues = []KPICell

// Kpi3upOverrides contains pattern-level overrides for kpi-3up.
type Kpi3upOverrides = KPIOverrides

// Kpi3upCellOverride contains per-cell overrides for kpi-3up.
type Kpi3upCellOverride = KPICellOverride

// Kpi4upValues is the values type: exactly 4 KPI cells.
type Kpi4upValues = []KPICell

// Kpi4upOverrides contains pattern-level overrides for kpi-4up.
type Kpi4upOverrides = KPIOverrides

// Kpi4upCellOverride contains per-cell overrides for kpi-4up.
type Kpi4upCellOverride = KPICellOverride
