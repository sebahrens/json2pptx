package semantic

// kindExamples holds one minimal, copy-ready slide per kind. Each validates
// with zero findings under strict (TestKindExamplesValidateClean), so an agent
// can paste it into DeckSpec.slides[] and edit the copy. list_slide_kinds
// publishes these as the per-kind `example`.
var kindExamples = map[SlideKind]map[string]any{
	KindTitle: {
		"kind": "title", "title": "FY26 Growth Plan", "subtitle": "Board update, March 2026",
	},
	KindSection: {
		"kind": "section", "title": "Where we stand",
	},
	KindExecutiveSummary: {
		"kind":  "executive_summary",
		"title": "Growth is on plan; retention needs a fix",
		"points": []any{
			map[string]any{"lead": "Growth is ahead of plan.", "support": "Revenue grew 41% year over year to $48M, against a 30% plan."},
			map[string]any{"lead": "Enterprise is carrying the mix.", "support": "It now drives 55% of new bookings, up from 38% last year."},
			map[string]any{"lead": "SMB retention is the one real risk.", "support": "Monthly churn rose to 3.1%, concentrated in the sub-50-seat tier."},
		},
		"bottom_line": "Fund an SMB retention pod in Q3 and hold the enterprise motion as is.",
		"takeaway":    "Protect SMB retention to keep the growth plan on track.",
	},
	KindOptionMatrix: {
		"kind":  "option_matrix",
		"title": "Hub consolidation scores best on payback and risk",
		"scale": "harvey",
		"criteria": []any{
			"Capex", "Payback", "Execution risk", "Customer impact",
		},
		"options": []any{
			map[string]any{"name": "Parcel automation", "detail": "Automate the three largest hubs", "scores": []any{1, 1, 3, 2}},
			map[string]any{"name": "Hub consolidation", "detail": "Close two hubs, expand one", "scores": []any{2, 4, 3, 3}},
			map[string]any{"name": "Partner network", "detail": "Outsource last mile in tier-2 cities", "scores": []any{4, 4, 1, 1}},
		},
		"recommended":        "Hub consolidation",
		"decisive_criterion": "Payback",
		"highlight_label":    "Recommended",
		"takeaway":           "Hub consolidation pays back in two years at acceptable risk.",
	},
	KindTeam: {
		"kind":  "team",
		"title": "Who you will be working with",
		"members": []any{
			map[string]any{"name": "Amara Okafor", "role": "Engagement partner", "bio": "Led the 2024 settlement migration for two of the three largest clearers."},
			map[string]any{"name": "Jonas Weber", "role": "Delivery lead", "bio": "Ten years in payments platform delivery; runs the cutover rehearsals."},
			map[string]any{"name": "Priya Raman", "role": "Data lead", "bio": "Owns the reconciliation model and the migration waves."},
		},
	},
	KindStat: {
		"kind":     "stat",
		"title":    "The prize",
		"value":    "$2.4B",
		"unit":     "TAM",
		"label":    "Addressable clearing-services market by FY27",
		"context":  "Up from $1.6B in FY24, driven by the T+1 settlement mandate.",
		"source":   "Oliver Wyman market model, 2026",
		"takeaway": "The market is large enough to fund the build twice over.",
	},
	KindTimeline: {
		"kind":  "timeline",
		"title": "How we got here",
		"milestones": []any{
			map[string]any{"label": "Mandate published", "date": "Mar 2024", "body": "The regulator sets the T+1 date."},
			map[string]any{"label": "Programme approved", "date": "Sep 2024", "body": "Board funds the first two waves."},
			map[string]any{"label": "Wave 1 live", "date": "Jun 2025", "body": "Two of the three clearers migrated."},
			map[string]any{"label": "Deadline", "date": "May 2027", "body": "All settlement on the new platform."},
		},
		"takeaway": "Three years of runway, two of them already spent.",
	},
	KindMatrix2x2: {
		"kind":   "matrix_2x2",
		"title":  "Where to spend the next two quarters",
		"x_axis": "Effort to deliver",
		"y_axis": "Impact on settlement risk",
		"x_low":  "Low effort",
		"x_high": "High effort",
		"y_low":  "Low impact",
		"y_high": "High impact",
		"quadrants": []any{
			map[string]any{"header": "Do first", "body": "Reconciliation alerts, cut-off automation."},
			map[string]any{"header": "Plan properly", "body": "Platform migration, wave 2 and 3."},
			map[string]any{"header": "Defer", "body": "Reporting refresh, vendor consolidation."},
			map[string]any{"header": "Fill the gaps", "body": "Runbook tidy-up, dashboard polish."},
		},
		"takeaway": "Two quarters of capacity buys the top half; the bottom half waits.",
	},
	KindFramework: {
		"kind":      "framework",
		"title":     "Where we stand",
		"framework": "swot",
		"sections": map[string]any{
			"strengths":     []any{"Two of three clearers already migrated", "Regulatory relationship is good"},
			"weaknesses":    []any{"Reconciliation is still manual", "One platform team, no bench"},
			"opportunities": []any{"T+1 mandate forces the market to move", "Adjacent custody business"},
			"threats":       []any{"A competitor is already live", "The May 2027 deadline does not move"},
		},
		"takeaway": "The mandate is the opportunity and the threat; the constraint is the team.",
	},
	KindAgenda: {
		"kind":    "agenda",
		"title":   "What we will cover",
		"current": 2,
		"sections": []any{
			map[string]any{"title": "Where we are", "subtitle": "Q3 against the plan, and what moved"},
			map[string]any{"title": "What we found", "subtitle": "Three findings from the operating review"},
			map[string]any{"title": "What we recommend", "subtitle": "The decision we are asking for today"},
			map[string]any{"title": "What happens next", "subtitle": "The first ninety days"},
		},
	},
	KindArchitecture: {
		"kind":  "architecture",
		"title": "Four tiers, two concerns that cut across them",
		"tiers": []any{
			map[string]any{"label": "Experience", "items": []any{"Web console", "Mobile approvals", "Partner portal"}},
			map[string]any{"label": "Services", "items": []any{"Orders", "Pricing", "Fulfilment", "Identity"}},
			map[string]any{"label": "Data", "description": "Event stream, warehouse, feature store"},
			map[string]any{"label": "Platform", "description": "Kubernetes, observability, secrets"},
		},
		"rails":    []any{"Security & compliance", "Cost governance"},
		"takeaway": "Every tier ships independently; the rails are owned centrally.",
	},
	KindTable: {
		"kind":    "table",
		"title":   "Enterprise carried the year; SMB did not",
		"headers": []any{"Segment", "FY25 revenue", "FY26 revenue", "Change"},
		"rows": []any{
			[]any{"Enterprise", "$28.4M", "$41.2M", "+45%"},
			[]any{"Mid-market", "$12.1M", "$14.8M", "+22%"},
			[]any{"SMB", "$9.6M", "$8.9M", "-7%"},
			[]any{"Total", "$50.1M", "$64.9M", "+30%"},
		},
		"column_alignments": []any{"left", "right", "right", "right"},
		"highlight_column":  "FY26 revenue",
		"totals_row":        true,
		"takeaway":          "Enterprise added $12.8M; SMB gave back $0.7M.",
	},
	KindKPISnapshot: {
		"kind":  "kpi_snapshot",
		"title": "Q4 at a glance",
		"kpis": []any{
			map[string]any{"value": "$48M", "label": "Annual revenue", "delta": "+41%"},
			map[string]any{"value": "118%", "label": "Net retention"},
			map[string]any{"value": "41d", "label": "Sales cycle", "delta": "-6d"},
		},
		"takeaway": "Growth and efficiency both improved.",
	},
	KindChartInsight: {
		"kind":  "chart_insight",
		"title": "Revenue accelerated through the year",
		"chart": map[string]any{
			"type":  "bar_chart",
			"title": "Quarterly revenue ($M)",
			"data": map[string]any{
				"categories": []any{"Q1", "Q2", "Q3", "Q4"},
				"series":     []any{map[string]any{"name": "Revenue", "values": []any{34, 40, 44, 48}}},
			},
		},
		"insights": []any{
			"Revenue grew 41% across the year.",
			"The Q4 step-up reflects the EMEA launch.",
		},
		"takeaway": "Momentum supports the H2 targets.",
	},
	KindComparison: {
		"kind":  "comparison",
		"title": "Build versus buy",
		"columns": []any{
			map[string]any{"header": "Build", "items": []any{"Full control of roadmap", "12-month time to market"}},
			map[string]any{"header": "Buy", "items": []any{"Vendor roadmap dependency", "3-month time to market"}},
		},
		"takeaway": "Buying wins on speed; building wins on control.",
	},
	KindProcess: {
		"kind":     "process",
		"title":    "How a deal closes",
		"steps":    []any{"Qualify", "Discover", "Propose", "Close"},
		"takeaway": "Discovery is where most deals stall.",
	},
	KindRoadmap: {
		"kind":  "roadmap",
		"title": "Rollout plan",
		"phases": []any{
			map[string]any{"name": "Pilot", "date_label": "Q1", "description": "Two lighthouse customers"},
			map[string]any{"name": "Expand", "date_label": "Q2", "description": "All EMEA accounts"},
			map[string]any{"name": "Scale", "date_label": "Q3", "description": "Global availability"},
		},
		"takeaway": "Global availability by Q3.",
	},
	KindDecision: {
		"kind":           "decision",
		"title":          "Fund an SMB success pod",
		"recommendation": "Stand up a dedicated SMB customer-success pod in Q3.",
		"options": []any{
			"Hold current coverage and accept rising churn.",
			"Fund a dedicated SMB success pod.",
		},
		"takeaway": "Fund the pod now to protect net retention.",
	},
	KindClosing: {
		"kind": "closing", "title": "Thank you", "subtitle": "Questions and discussion",
	},
	KindRawJSON2pptx: {
		"kind": "raw_json2pptx",
		"slide": map[string]any{
			"slide_type": "content",
			"content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Raw slide"},
				map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": []any{"First point", "Second point"}},
			},
		},
	},
}

// KindExample returns a deep copy of the copy-ready example slide for a kind
// (including its "kind" key), or nil for an unknown kind.
func KindExample(k SlideKind) map[string]any {
	ex, ok := kindExamples[k]
	if !ok {
		return nil
	}
	cp, _ := deepCopyValue(ex).(map[string]any)
	return cp
}

func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = deepCopyValue(e)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = deepCopyValue(e)
		}
		return out
	default:
		return v
	}
}
