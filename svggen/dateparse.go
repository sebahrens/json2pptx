package svggen

import (
	"fmt"
	"strings"
	"time"
)

// parseDate parses a date string in various formats.
func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"Jan 2, 2006",
		"January 2, 2006",
		"2006/01/02",
		"02-Jan-2006",
		"2006-01",      // Year-month (YYYY-MM) — common in Gantt/timeline data
		"Jan 2006",     // Month-year (abbreviated)
		"January 2006", // Month-year (full)
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	// Try half-year format (returns start of half as single date)
	if start, _, err := parseHalfDate(s); err == nil {
		return start, nil
	}

	// Try quarter format (returns start of quarter as single date)
	if start, _, err := parseQuarterDate(s); err == nil {
		return start, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

// parseDateRange parses a date string that may represent a range (like a quarter).
// Returns start and end dates. For point-in-time dates, start == end.
func parseDateRange(s string) (start, end time.Time, err error) {
	// Try half-year format first (e.g., "2026 H1", "H2 2026")
	if start, end, err = parseHalfDate(s); err == nil {
		return start, end, nil
	}

	// Try quarter format (e.g., "2026 Q1", "Q1 2026")
	if start, end, err = parseQuarterDate(s); err == nil {
		return start, end, nil
	}

	// Try month-year format (e.g., "Mar 2026", "March 2026")
	if start, end, err = parseMonthDate(s); err == nil {
		return start, end, nil
	}

	// Fall back to regular date parsing (point in time)
	if t, err := parseDate(s); err == nil {
		return t, t, nil
	}

	return time.Time{}, time.Time{}, fmt.Errorf("unable to parse date range: %s", s)
}

// parseQuarterDate parses quarter formats like "2026 Q1", "Q1 2026", "2026Q1", or bare "Q1".
// Bare quarter formats (no year) use the current year.
// Returns start and end dates of the quarter.
func parseQuarterDate(s string) (start, end time.Time, err error) {
	s = strings.TrimSpace(s)

	// Try various quarter patterns
	var year, quarter int

	// Pattern: "2026 Q1" or "2026Q1"
	if _, scanErr := fmt.Sscanf(s, "%d Q%d", &year, &quarter); scanErr == nil && quarter >= 1 && quarter <= 4 {
		return quarterToDateRange(year, quarter)
	}
	if _, scanErr := fmt.Sscanf(s, "%dQ%d", &year, &quarter); scanErr == nil && quarter >= 1 && quarter <= 4 {
		return quarterToDateRange(year, quarter)
	}

	// Pattern: "Q1 2026"
	if _, scanErr := fmt.Sscanf(s, "Q%d %d", &quarter, &year); scanErr == nil && quarter >= 1 && quarter <= 4 {
		return quarterToDateRange(year, quarter)
	}

	// Pattern: bare "Q1" through "Q4" (no year — use current year)
	if _, scanErr := fmt.Sscanf(s, "Q%d", &quarter); scanErr == nil && quarter >= 1 && quarter <= 4 && len(s) == 2 {
		return quarterToDateRange(time.Now().Year(), quarter)
	}

	return time.Time{}, time.Time{}, fmt.Errorf("not a quarter format: %s", s)
}

// parseHalfDate parses half-year formats like "2026 H1", "H1 2026", "2026H1".
// H1 = January–June, H2 = July–December.
// Returns start and end dates of the half-year.
func parseHalfDate(s string) (start, end time.Time, err error) {
	s = strings.TrimSpace(s)

	var year, half int

	// Pattern: "2026 H1" or "2026H1"
	if _, scanErr := fmt.Sscanf(s, "%d H%d", &year, &half); scanErr == nil && half >= 1 && half <= 2 {
		return halfToDateRange(year, half)
	}
	if _, scanErr := fmt.Sscanf(s, "%dH%d", &year, &half); scanErr == nil && half >= 1 && half <= 2 {
		return halfToDateRange(year, half)
	}

	// Pattern: "H1 2026"
	if _, scanErr := fmt.Sscanf(s, "H%d %d", &half, &year); scanErr == nil && half >= 1 && half <= 2 {
		return halfToDateRange(year, half)
	}

	return time.Time{}, time.Time{}, fmt.Errorf("not a half-year format: %s", s)
}

// halfToDateRange converts a year and half to start and end dates.
func halfToDateRange(year, half int) (start, end time.Time, err error) {
	if half == 1 {
		start = time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(year, time.June, 30, 0, 0, 0, 0, time.UTC)
	} else {
		start = time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC)
		end = time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC)
	}
	return start, end, nil
}

// parseMonthDate parses month-year formats like "Mar 2026" or "March 2026".
// Returns start and end dates of the month.
func parseMonthDate(s string) (start, end time.Time, err error) {
	s = strings.TrimSpace(s)

	// Try abbreviated month: "Jan 2006"
	if t, parseErr := time.Parse("Jan 2006", s); parseErr == nil {
		start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, -1) // Last day of the month
		return start, end, nil
	}

	// Try full month: "January 2006"
	if t, parseErr := time.Parse("January 2006", s); parseErr == nil {
		start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, -1)
		return start, end, nil
	}

	return time.Time{}, time.Time{}, fmt.Errorf("not a month-year format: %s", s)
}

// quarterToDateRange converts a year and quarter to start and end dates.
func quarterToDateRange(year, quarter int) (start, end time.Time, err error) {
	// Quarter start months: Q1=Jan, Q2=Apr, Q3=Jul, Q4=Oct
	startMonth := time.Month((quarter-1)*3 + 1)
	start = time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)

	// End is the last day of the quarter
	endMonth := startMonth + 3
	endYear := year
	if endMonth > 12 {
		endMonth = 1
		endYear++
	}
	end = time.Date(endYear, endMonth, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)

	return start, end, nil
}
