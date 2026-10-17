package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

var currencyNumberRE = regexp.MustCompile(`(?i)(USD|EUR|GBP|[$€£])\s*([0-9][0-9.,]*)(?:\s*(million|billion|bn|m)\b)?|([0-9][0-9.,]*)\s*(USD|EUR|GBP|[$€£])(?:\s*(million|billion|bn|m)\b)?`)
var currencyHeaderRE = regexp.MustCompile(`(?i)(USD|EUR|GBP|[$€£])(?:\s*(million|billion|bn|m)\b)?`)
var plainReviewNumberRE = regexp.MustCompile(`^-?[0-9][0-9.,]*$`)
var percentNumberRE = regexp.MustCompile(`(?i)([0-9][0-9.,]*)\s*(percentage points\b|percent\b|pp\b|%)`)

type numberFormatSample struct {
	unit, path, style string
	dimensions        map[string]string
	slide             int
}

func collectNumberFormatFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	groups := map[string][]numberFormatSample{}
	var order []string
	for si, slide := range input.Slides {
		b, err := json.Marshal(slide)
		if err != nil {
			continue
		}
		var value any
		if json.Unmarshal(b, &value) != nil {
			continue
		}
		add := func(sample numberFormatSample) {
			if _, ok := groups[sample.unit]; !ok {
				order = append(order, sample.unit)
			}
			groups[sample.unit] = append(groups[sample.unit], sample)
		}
		walkReviewJSON(value, fmt.Sprintf("/slides/%d", si), func(path, text string) {
			for _, sample := range numberFormatSamples(text, path, si) {
				add(sample)
			}
		})
		walkReviewTables(value, fmt.Sprintf("/slides/%d", si), si, add)
	}
	var out []patterns.FitFinding
	for _, unit := range order {
		samples := groups[unit]
		base, conflict, found := numberFormatConflict(samples)
		if !found {
			continue
		}
		out = append(out, layoutReviewFinding(patterns.ErrCodeNumberFormatInconsistent, conflict.path, "info", fmt.Sprintf("Numbers in the %s context use conflicting conventions: %s and %s; choose one number/unit formatter for this context.", unit, base.style, conflict.style), map[string]any{"slides": []int{base.slide, conflict.slide}, "conflicting_formats": []string{base.style, conflict.style}, "formatter_hint": "Use one decimal separator, grouping convention, currency position and unit spelling per unit/locale."}))
	}
	return out
}

func walkReviewJSON(value any, path string, visit func(string, string)) {
	switch value := value.(type) {
	case string:
		visit(path, value)
	case []any:
		for i, item := range value {
			walkReviewJSON(item, fmt.Sprintf("%s/%d", path, i), visit)
		}
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			switch key {
			case "source", "source_link", "speaker_notes", "url", "path", "svg_data":
				continue
			}
			walkReviewJSON(value[key], path+"/"+key, visit)
		}
	}
}

func numberFormatSamples(text, path string, slide int) []numberFormatSample {
	var out []numberFormatSample
	for _, match := range currencyNumberRE.FindAllStringSubmatch(text, -1) {
		currency, number, magnitude, position := match[1], match[2], match[3], "prefix"
		if currency == "" {
			currency, number, magnitude, position = match[5], match[4], match[6], "suffix"
		}
		unit := strings.ToUpper(currency)
		switch unit {
		case "$":
			unit = "USD"
		case "€":
			unit = "EUR"
		case "£":
			unit = "GBP"
		}
		magnitude = strings.ToLower(magnitude)
		normalized := magnitude
		switch normalized {
		case "million":
			normalized = "m"
		case "billion":
			normalized = "bn"
		}
		dimensions := numericSeparators(number)
		// Currency spelling and placement belong to numeric display labels.
		// Natural prose may spell out a unit while a KPI uses its symbol.
		if strings.TrimSpace(text) == strings.TrimSpace(match[0]) {
			dimensions["currency"] = currency
			dimensions["position"] = position
			dimensions["magnitude"] = magnitude
		}
		out = append(out, numberFormatSample{unit: unit + "/" + normalized, path: path, slide: slide, style: match[0], dimensions: dimensions})
	}
	for _, match := range percentNumberRE.FindAllStringSubmatch(text, -1) {
		unit := "percent"
		label := strings.ToLower(match[2])
		if label == "pp" || label == "percentage points" {
			unit = "percentage_points"
		}
		dimensions := numericSeparators(match[1])
		if strings.TrimSpace(text) == strings.TrimSpace(match[0]) {
			dimensions["unit_label"] = label
		}
		out = append(out, numberFormatSample{unit: unit, path: path, slide: slide, style: match[0], dimensions: dimensions})
	}
	return out
}

func numericSeparators(number string) map[string]string {
	out := map[string]string{}
	comma, dot := strings.LastIndex(number, ","), strings.LastIndex(number, ".")
	switch {
	case comma >= 0 && dot >= 0:
		if comma > dot {
			out["decimal"], out["group"] = "comma", "dot"
		} else {
			out["decimal"], out["group"] = "dot", "comma"
		}
	case comma >= 0:
		if len(number)-comma-1 != 3 {
			out["decimal"] = "comma"
		}
	case dot >= 0:
		if len(number)-dot-1 != 3 {
			out["decimal"] = "dot"
		}
	}
	return out
}

func numberFormatConflict(samples []numberFormatSample) (numberFormatSample, numberFormatSample, bool) {
	first := map[string]numberFormatSample{}
	for _, sample := range samples {
		for _, key := range []string{"currency", "position", "decimal", "group", "magnitude", "unit_label"} {
			value := sample.dimensions[key]
			if value == "" {
				continue
			}
			if previous, ok := first[key]; ok {
				if previous.dimensions[key] != value {
					return previous, sample, true
				}
			} else {
				first[key] = sample
			}
		}
	}
	return numberFormatSample{}, numberFormatSample{}, false
}

// A unit in a table header applies to bare numbers in that column. Do not
// compare arbitrary numeric prose or infer units from unrelated nearby text.
func walkReviewTables(value any, path string, slide int, add func(numberFormatSample)) {
	switch v := value.(type) {
	case []any:
		for i, item := range v {
			walkReviewTables(item, fmt.Sprintf("%s/%d", path, i), slide, add)
		}
	case map[string]any:
		headers, hOK := v["headers"].([]any)
		rows, rOK := v["rows"].([]any)
		if hOK && rOK {
			tableUnitSamples(headers, rows, path, slide, add)
		}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			walkReviewTables(v[key], path+"/"+key, slide, add)
		}
	}
}
func tableUnitSamples(headers, rows []any, path string, slide int, add func(numberFormatSample)) {
	// Header-to-cell positions are ambiguous in authored merged rows. Explicit
	// unit-bearing strings still pass through the ordinary text scanner.
	if mergedReviewTable(rows) {
		return
	}
	for ci, header := range headers {
		label, ok := header.(string)
		if !ok {
			continue
		}
		unit := currencyHeaderRE.FindStringSubmatch(label)
		percent := strings.Contains(label, "%") || strings.Contains(strings.ToLower(label), "percent")
		if len(unit) == 0 && !percent {
			continue
		}
		for ri, raw := range rows {
			cells, ok := raw.([]any)
			if !ok || ci >= len(cells) {
				continue
			}
			text, ok := cells[ci].(string)
			if cell, yes := cells[ci].(map[string]any); yes {
				text, ok = cell["content"].(string)
			}
			text = strings.TrimSpace(text)
			if !ok || !plainReviewNumberRE.MatchString(text) {
				continue
			}
			sampleText := text + "%"
			if len(unit) > 0 {
				sampleText = unit[1] + " " + text + " " + unit[2]
			}
			for _, sample := range numberFormatSamples(sampleText, fmt.Sprintf("%s/rows/%d/%d", path, ri, ci), slide) {
				add(sample)
			}
		}
	}
}

func mergedReviewTable(rows []any) bool {
	for _, raw := range rows {
		cells, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, raw := range cells {
			cell, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range []string{"col_span", "row_span"} {
				if span, ok := cell[key].(float64); ok && span > 1 {
					return true
				}
			}
		}
	}
	return false
}
