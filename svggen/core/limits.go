package core

import "fmt"

// Capacity limits for chart data. These are generous upper bounds to prevent
// performance degradation and SVG bloat, not strict business constraints.
const (
	MaxSeries     = 50
	MaxCategories = 200
	MaxPoints     = 5000
)

// CheckCapacity validates that series, categories, and points counts are within
// safe rendering limits. Returns a Finding for each limit that is exceeded.
func CheckCapacity(seriesCount, categoryCount, pointCount int) []Finding {
	var findings []Finding

	if seriesCount > MaxSeries {
		findings = append(findings, Finding{
			Code:     FindingCapacityExceeded,
			Message:  fmt.Sprintf("chart has %d series — exceeds safe rendering limit of %d", seriesCount, MaxSeries),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReduceItems,
				Params: map[string]any{"dimension": "series", "count": seriesCount, "limit": MaxSeries},
			},
		})
	}

	if categoryCount > MaxCategories {
		findings = append(findings, Finding{
			Code:     FindingCapacityExceeded,
			Message:  fmt.Sprintf("chart has %d categories — exceeds safe rendering limit of %d", categoryCount, MaxCategories),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReduceItems,
				Params: map[string]any{"dimension": "categories", "count": categoryCount, "limit": MaxCategories},
			},
		})
	}

	if pointCount > MaxPoints {
		findings = append(findings, Finding{
			Code:     FindingCapacityExceeded,
			Message:  fmt.Sprintf("chart has %d data points — exceeds safe rendering limit of %d", pointCount, MaxPoints),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReduceItems,
				Params: map[string]any{"dimension": "points", "count": pointCount, "limit": MaxPoints},
			},
		})
	}

	return findings
}
