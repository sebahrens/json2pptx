package patterns

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Dates on the gantt time axis (go-slide-creator-o34er).
//
// timeline-horizontal's gantt style drew every stop as one full-width bar
// whatever its dates said: a point milestone, a two-month range and a
// six-month range were the same length. The bars are now placed and sized on
// a shared axis, which needs the date strings read as dates. The grammar is
// what deck authors write: ISO days and months, month names with or without a
// year or a day, quarters, half-years and bare years.

// timelinePeriod is the span a date string names: a day, a month, a quarter,
// a half-year or a year. end is exclusive.
type timelinePeriod struct {
	start, end time.Time
	// yearless is set for a date with no year ("Mar", "Q2", "Apr 30"). It is
	// placed in timelineYearlessYear so yearless dates order among themselves.
	yearless bool
}

// timelineYearlessYear is the reference year of yearless dates. A leap year,
// so "Feb 29" is a date.
const timelineYearlessYear = 2000

var timelineMonths = map[string]time.Month{
	"jan": 1, "january": 1, "feb": 2, "february": 2, "mar": 3, "march": 3,
	"apr": 4, "april": 4, "may": 5, "jun": 6, "june": 6, "jul": 7, "july": 7,
	"aug": 8, "august": 8, "sep": 9, "sept": 9, "september": 9, "oct": 10, "october": 10,
	"nov": 11, "november": 11, "dec": 12, "december": 12,
}

var (
	timelineISODayRE   = regexp.MustCompile(`^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})$`)
	timelineISOMonthRE = regexp.MustCompile(`^(\d{4})[-/](\d{1,2})$`)
	timelineYearRE     = regexp.MustCompile(`^(\d{4})$`)
	timelinePartRE     = regexp.MustCompile(`^([qh])([1-4])(?: ?'?(\d{4}|\d{2}))?$`)
	timelinePartYearRE = regexp.MustCompile(`^(\d{4}) ?([qh])([1-4])$`)
	timelineOrdinalRE  = regexp.MustCompile(`^(\d{1,2})(?:st|nd|rd|th)?$`)
	timelineShortYrRE  = regexp.MustCompile(`^'(\d{2})$`)
)

// parseTimelinePeriod reads a date string. It reports false for text that is
// not a date ("Summer", "TBD", "Phase 1").
func parseTimelinePeriod(raw string) (timelinePeriod, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.NewReplacer(",", " ", "’", "'", "‘", "'").Replace(s)
	if i := strings.IndexByte(s, 't'); i == 10 && timelineISODayRE.MatchString(s[:10]) {
		s = s[:10] // ISO timestamp: keep the day
	}
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return timelinePeriod{}, false
	}

	if m := timelineISODayRE.FindStringSubmatch(s); m != nil {
		return timelineDay(atoi(m[1]), atoi(m[2]), atoi(m[3]), false)
	}
	if m := timelineISOMonthRE.FindStringSubmatch(s); m != nil {
		return timelineMonth(atoi(m[1]), atoi(m[2]), false)
	}
	if m := timelineYearRE.FindStringSubmatch(s); m != nil {
		year := atoi(m[1])
		if year < 1900 || year > 2200 {
			return timelinePeriod{}, false
		}
		return timelinePeriod{start: timelineDate(year, 1, 1), end: timelineDate(year+1, 1, 1)}, true
	}
	if m := timelinePartRE.FindStringSubmatch(s); m != nil {
		year, yearless := timelineYearlessYear, true
		if m[3] != "" {
			year, yearless = timelineFullYear(m[3]), false
		}
		return timelinePart(m[1], atoi(m[2]), year, yearless)
	}
	if m := timelinePartYearRE.FindStringSubmatch(s); m != nil {
		return timelinePart(m[2], atoi(m[3]), atoi(m[1]), false)
	}

	// Month-name forms: "Mar", "Mar 2026", "Mar '26", "Mar 5", "5 Mar",
	// "Mar 5 2026", "5 Mar 2026", "05-Mar-2026".
	month, day, year := 0, 0, 0
	for _, tok := range strings.Fields(strings.ReplaceAll(s, "-", " ")) {
		switch {
		case timelineMonths[tok] != 0 && month == 0:
			month = int(timelineMonths[tok])
		case timelineYearRE.MatchString(tok) && year == 0:
			year = atoi(tok)
		case timelineShortYrRE.MatchString(tok) && year == 0:
			year = timelineFullYear(tok[1:])
		case timelineOrdinalRE.MatchString(tok) && day == 0:
			day = atoi(timelineOrdinalRE.FindStringSubmatch(tok)[1])
		default:
			return timelinePeriod{}, false
		}
	}
	if month == 0 {
		return timelinePeriod{}, false
	}
	yearless := year == 0
	if yearless {
		year = timelineYearlessYear
	}
	if day == 0 {
		return timelineMonth(year, month, yearless)
	}
	return timelineDay(year, month, day, yearless)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func timelineFullYear(s string) int {
	if n := atoi(s); len(s) == 2 {
		return 2000 + n
	}
	return atoi(s)
}

func timelineDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func timelineDay(year, month, day int, yearless bool) (timelinePeriod, bool) {
	start := timelineDate(year, month, day)
	if month < 1 || month > 12 || day < 1 || start.Day() != day {
		return timelinePeriod{}, false // 31 Feb, month 13
	}
	return timelinePeriod{start: start, end: start.AddDate(0, 0, 1), yearless: yearless}, true
}

func timelineMonth(year, month int, yearless bool) (timelinePeriod, bool) {
	if month < 1 || month > 12 {
		return timelinePeriod{}, false
	}
	start := timelineDate(year, month, 1)
	return timelinePeriod{start: start, end: start.AddDate(0, 1, 0), yearless: yearless}, true
}

// timelinePart is a quarter ("q") or a half-year ("h").
func timelinePart(kind string, n, year int, yearless bool) (timelinePeriod, bool) {
	months := 3
	if kind == "h" {
		months = 6
		if n > 2 {
			return timelinePeriod{}, false
		}
	}
	start := timelineDate(year, (n-1)*months+1, 1)
	return timelinePeriod{start: start, end: start.AddDate(0, months, 0), yearless: yearless}, true
}

// timelineGanttSpan is one stop placed in time.
type timelineGanttSpan struct {
	start, end time.Time // end exclusive; equal to start for a point
	point      bool      // no end_date: a milestone marker, not a bar
	ok         bool      // false when the stop's dates could not be placed
}

// timelineGanttDates is every stop of a gantt timeline placed in time.
type timelineGanttDates struct {
	spans    []timelineGanttSpan
	yearless bool     // no stop names a year: the axis prints none
	problems []string // one "values[i].field …" clause per date that could not be placed
}

// placed reports whether any stop could be placed.
func (d timelineGanttDates) placed() bool {
	for _, s := range d.spans {
		if s.ok {
			return true
		}
	}
	return false
}

// resolveTimelineGanttDates reads every stop's date and end_date. A stop with
// an end_date runs from the start of its date to the end of its end_date; a
// stop without one is a point at the middle of the period its date names.
func resolveTimelineGanttDates(stops []TimelineStop) timelineGanttDates {
	out := timelineGanttDates{spans: make([]timelineGanttSpan, len(stops)), yearless: true}
	type parsed struct {
		from, to     timelinePeriod
		fromOK, toOK bool
	}
	all := make([]parsed, len(stops))
	for i, stop := range stops {
		p := &all[i]
		p.from, p.fromOK = parseTimelinePeriod(stop.Date)
		if stop.EndDate != "" {
			p.to, p.toOK = parseTimelinePeriod(stop.EndDate)
		}
		if (p.fromOK && !p.from.yearless) || (p.toOK && !p.to.yearless) {
			out.yearless = false
		}
	}

	var prevStart time.Time
	yearShift := 0
	for i, stop := range stops {
		p := all[i]
		switch {
		case strings.TrimSpace(stop.Date) == "":
			out.problems = append(out.problems, fmt.Sprintf("values[%d].date is empty", i))
			continue
		case !p.fromOK:
			out.problems = append(out.problems, fmt.Sprintf("values[%d].date %q is not a date", i, stop.Date))
			continue
		case stop.EndDate != "" && !p.toOK:
			out.problems = append(out.problems, fmt.Sprintf("values[%d].end_date %q is not a date", i, stop.EndDate))
			continue
		case !out.yearless && p.from.yearless:
			out.problems = append(out.problems, fmt.Sprintf("values[%d].date %q has no year while other stops name one", i, stop.Date))
			continue
		case !out.yearless && stop.EndDate != "" && p.to.yearless:
			out.problems = append(out.problems, fmt.Sprintf("values[%d].end_date %q has no year while other stops name one", i, stop.EndDate))
			continue
		}

		start, end := p.from.start, p.from.end
		if out.yearless {
			// Yearless stops run on in authored order: "Nov", "Dec", "Jan"
			// crosses a year end rather than jumping back eleven months.
			start, end = start.AddDate(yearShift, 0, 0), end.AddDate(yearShift, 0, 0)
			if !prevStart.IsZero() && start.Before(prevStart.AddDate(0, -6, 0)) {
				yearShift++
				start, end = start.AddDate(1, 0, 0), end.AddDate(1, 0, 0)
			}
		}
		span := timelineGanttSpan{ok: true}
		if stop.EndDate == "" {
			mid := start.Add(end.Sub(start) / 2)
			span.start, span.end, span.point = mid, mid, true
		} else {
			to := p.to.end
			if out.yearless {
				to = to.AddDate(yearShift, 0, 0)
				if !to.After(start) {
					to = to.AddDate(1, 0, 0) // "Nov" → "Feb"
				}
			}
			if !to.After(start) {
				out.problems = append(out.problems, fmt.Sprintf("values[%d].end_date %q is before its date %q", i, stop.EndDate, stop.Date))
				continue
			}
			span.start, span.end = start, to
		}
		prevStart = start
		out.spans[i] = span
	}
	return out
}

// timelineGanttTick is one labelled division of the time axis.
type timelineGanttTick struct {
	at    time.Time
	label string
}

// timelineGanttAxis is the time axis: whole units covering every placed stop.
type timelineGanttAxis struct {
	start, end time.Time
	ticks      []timelineGanttTick // each tick labels the unit that starts at it
}

// frac is where t sits on the axis, 0 at its start and 1 at its end.
func (a timelineGanttAxis) frac(t time.Time) float64 {
	total := a.end.Sub(a.start)
	if total <= 0 {
		return 0
	}
	f := float64(t.Sub(a.start)) / float64(total)
	return min(max(f, 0), 1)
}

// timelineGanttMaxTicks is the most divisions the axis is cut into; a longer
// span takes the next coarser unit.
const timelineGanttMaxTicks = 8

// timelineGanttUnit is one candidate axis unit: floor snaps a time to the
// start of its unit, next steps to the following one, label names it.
type timelineGanttUnit struct {
	floor func(time.Time) time.Time
	next  func(time.Time) time.Time
	label func(t time.Time, first, yearless bool) string
}

func timelineYearSuffix(t time.Time, show, yearless bool) string {
	if !show || yearless {
		return ""
	}
	return " " + t.Format("'06")
}

func timelineGanttUnits() []timelineGanttUnit {
	day := func(t time.Time) time.Time { return timelineDate(t.Year(), int(t.Month()), t.Day()) }
	month := func(t time.Time) time.Time { return timelineDate(t.Year(), int(t.Month()), 1) }
	monthsFloor := func(n int) func(time.Time) time.Time {
		return func(t time.Time) time.Time { return timelineDate(t.Year(), (int(t.Month())-1)/n*n+1, 1) }
	}
	yearsFloor := func(n int) func(time.Time) time.Time {
		return func(t time.Time) time.Time { return timelineDate(t.Year()/n*n, 1, 1) }
	}
	dayLabel := func(t time.Time, _, _ bool) string { return t.Format("2 Jan") }
	part := func(prefix string, n int) func(time.Time, bool, bool) string {
		return func(t time.Time, first, yearless bool) string {
			idx := (int(t.Month())-1)/n + 1
			return fmt.Sprintf("%s%d%s", prefix, idx, timelineYearSuffix(t, first || idx == 1, yearless))
		}
	}
	units := []timelineGanttUnit{
		{day, func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }, dayLabel},
		{day, func(t time.Time) time.Time { return t.AddDate(0, 0, 7) }, dayLabel},
		{month, func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }, func(t time.Time, first, yearless bool) string {
			return t.Format("Jan") + timelineYearSuffix(t, first || t.Month() == time.January, yearless)
		}},
		{monthsFloor(3), func(t time.Time) time.Time { return t.AddDate(0, 3, 0) }, part("Q", 3)},
		{monthsFloor(6), func(t time.Time) time.Time { return t.AddDate(0, 6, 0) }, part("H", 6)},
	}
	for _, n := range []int{1, 2, 5, 10, 25, 50} {
		units = append(units, timelineGanttUnit{yearsFloor(n), func(t time.Time) time.Time { return t.AddDate(n, 0, 0) },
			func(t time.Time, _, _ bool) string { return t.Format("2006") }})
	}
	return units
}

// newTimelineGanttAxis builds the axis for the placed stops: the finest unit
// (days, weeks, months, quarters, half-years, years) that covers them in at
// most maxTicks divisions.
func newTimelineGanttAxis(dates timelineGanttDates, maxTicks int) timelineGanttAxis {
	var lo, hi time.Time
	for _, s := range dates.spans {
		if !s.ok {
			continue
		}
		if lo.IsZero() || s.start.Before(lo) {
			lo = s.start
		}
		if hi.IsZero() || s.end.After(hi) {
			hi = s.end
		}
	}
	maxTicks = min(max(maxTicks, 1), timelineGanttMaxTicks)
	units := timelineGanttUnits()
	for u, unit := range units {
		axis := timelineGanttAxis{start: unit.floor(lo)}
		for at := axis.start; at.Before(hi) || len(axis.ticks) == 0; at = unit.next(at) {
			axis.ticks = append(axis.ticks, timelineGanttTick{at: at, label: unit.label(at, len(axis.ticks) == 0, dates.yearless)})
			axis.end = unit.next(at)
			if len(axis.ticks) > maxTicks {
				break
			}
		}
		if len(axis.ticks) <= maxTicks || u == len(units)-1 {
			return axis
		}
	}
	return timelineGanttAxis{start: lo, end: hi}
}
