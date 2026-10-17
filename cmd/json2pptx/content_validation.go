package main

import (
	"encoding/json"
	"fmt"
	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"slices"
	"sort"
)

func (o *dryRunOutput) addValidationDiagnostics(ds ...diagnostics.Diagnostic) {
	for _, d := range ds {
		if d.Severity == diagnostics.SeverityError {
			o.Valid = false
		}
	}
	o.Diagnostics = append(o.Diagnostics, ds...)
}

func placeholderByID(placeholders []types.PlaceholderInfo, id string) (types.PlaceholderInfo, bool) {
	for _, ph := range placeholders {
		if ph.ID == id {
			return ph, true
		}
	}
	return types.PlaceholderInfo{}, false
}

// measureTitleItem is shared by explicit and auto-selected layouts. Authored
// styling is applied before both the capacity and overflow measurements.
func measureTitleItem(item ContentInput, info *types.PlaceholderInfo, slide SlideInput, layouts []types.LayoutMetadata, i, j int) (bool, int, *diagnostics.Diagnostic) {
	if item.Type != "text" || info.Type != types.PlaceholderTitle {
		return false, 0, nil
	}
	resolved, err := item.ResolveValue()
	if err != nil {
		return false, 0, nil
	}
	text, ok := resolved.(string)
	if !ok {
		return false, 0, nil
	}
	effective := authoredTitlePlaceholder(info, &item)
	m := measureTitleInPlaceholder(text, effective, isSectionSlideInput(slide, layouts))
	return m.OK, generator.TitlePlaceholderCapacityChars(effective), m.diagnostic(i, j)
}

func validateSlideLayout(slideInput SlideInput, i int, analysis *types.TemplateAnalysis, layoutByID map[string]types.LayoutMetadata) (types.LayoutMetadata, bool, []diagnostics.Diagnostic) {
	var ds []diagnostics.Diagnostic
	// Check layout_id reference. layout_id and slide_type are alternatives:
	// the generator auto-selects a layout when only slide_type is provided,
	// so the validator only errors when BOTH are missing.
	lm, layoutFound := layoutByID[slideInput.LayoutID]
	if slideInput.LayoutID == "" {
		if slideInput.SlideType == "" {
			msg := fmt.Sprintf("slide %d: layout_id or slide_type is required", i+1)
			ds = append(ds, diagnostics.Diagnostic{
				Code:     patterns.ErrCodeRequired,
				Path:     slidepath.SlideField(i, "layout_id"),
				Message:  msg,
				Severity: diagnostics.SeverityError,
				Fix: &diagnostics.Fix{
					Kind:   "provide_value",
					Params: map[string]any{"field": "layout_id_or_slide_type"},
				},
			})
		}
	} else if !layoutFound {
		path := slidepath.SlideField(i, "layout_id")
		var msg string
		fix := &patterns.FixSuggestion{Kind: "use_one_of", Params: map[string]any{"path": "layout_id"}}
		if hint := layout.UnsupportedNativeAliasHint(slideInput.LayoutID); hint != "" {
			msg = fmt.Sprintf("slide %d: layout alias %q is unavailable on this template; %s", i+1, slideInput.LayoutID, hint)
			fix = nil
		} else if layout.IsCanonicalName(slideInput.LayoutID) {
			availableMap := layout.ResolveAllCanonicalLayouts(analysis.Layouts)
			available := make([]string, 0, len(availableMap))
			for name := range availableMap {
				available = append(available, name)
			}
			sort.Strings(available)
			msg = fmt.Sprintf("slide %d: %s", i+1, generator.CanonicalLayoutNotFoundError(slideInput.LayoutID, available))
			fix.Params["available"] = available
			if suggestion := generator.SuggestedCanonicalLayout(slideInput.LayoutID, available); suggestion != "" {
				fix.Params["did_you_mean"] = suggestion
				fix.Params["value"] = suggestion
			} else {
				// No safe concrete target: the message still lists options, but
				// repair_slide.use_one_of would reject a missing value.
				fix = nil
			}
		} else {
			available := make([]string, 0, len(layoutByID))
			for id := range layoutByID {
				available = append(available, id)
			}
			msg = fmt.Sprintf("slide %d: %s", i+1, generator.LayoutNotFoundError(slideInput.LayoutID, available))
			fix.Params["available"] = generator.FormatAvailableIDs(available)
			if match, _ := generator.ClosestMatch(slideInput.LayoutID, available, 3); match != "" {
				fix.Params["did_you_mean"] = match
				fix.Params["value"] = match
			} else {
				fix = nil
			}
		}
		ve := &patterns.ValidationError{
			Path:    path,
			Code:    patterns.ErrCodeUnknownLayoutID,
			Message: msg,
			Fix:     fix,
		}
		ds = append(ds, diagnostics.FromValidationError(ve))
	}

	return lm, layoutFound, ds
}

func validateContentPlaceholder(item ContentInput, slideInput SlideInput, i, j int, lm types.LayoutMetadata, layoutFound, verdictOnly bool, predicted *types.LayoutMetadata, analysis *types.TemplateAnalysis) (dryRunPlaceholder, []diagnostics.Diagnostic) {
	var ds []diagnostics.Diagnostic
	ph := dryRunPlaceholder{
		PlaceholderID: item.PlaceholderID,
		ContentType:   item.Type,
	}

	if item.PlaceholderID == "" {
		msg := fmt.Sprintf("slide %d, content %d: placeholder_id is required", i+1, j+1)
		ds = append(ds, diagnostics.Diagnostic{
			Code:     patterns.ErrCodeRequired,
			Path:     slidepath.ContentField(i, j, "placeholder_id"),
			Message:  msg,
			Severity: diagnostics.SeverityError,
			Fix: &diagnostics.Fix{
				Kind:   "provide_value",
				Params: map[string]any{"field": "placeholder_id"},
			},
		})
	} else if layoutFound {
		// Check placeholder_id reference against layout
		phInfo, phFound := placeholderByID(lm.Placeholders, item.PlaceholderID)
		if !phFound {
			available := make([]string, 0, len(lm.Placeholders))
			for _, ph := range lm.Placeholders {
				available = append(available, ph.ID)
			}
			path := slidepath.ContentField(i, j, "placeholder_id")
			msg := fmt.Sprintf("slide %d: %s", i+1, generator.PlaceholderNotFoundError(item.PlaceholderID, slideInput.LayoutID, available))
			fix := &patterns.FixSuggestion{
				Kind:   "use_one_of",
				Params: map[string]any{"available": generator.FormatAvailableIDs(available)},
			}
			if match, _ := generator.ClosestMatch(item.PlaceholderID, available, 3); match != "" && item.PlaceholderID != "subtitle" {
				fix.Params["did_you_mean"] = match
			}
			ve := &patterns.ValidationError{
				Path:    path,
				Code:    patterns.ErrCodePlaceholderNotFound,
				Message: msg,
				Fix:     fix,
			}
			ds = append(ds, diagnostics.FromValidationError(ve))
		} else if !verdictOnly {
			effectivePhInfo := authoredTitlePlaceholder(&phInfo, &item)
			ph.MaxChars = generator.ReportedMaxChars(effectivePhInfo)

			// Titles: measured fit against the resolved title box and
			// inherited title style replaces the character estimate
			// (go-slide-creator-vjwn). The capacity reported for the
			// placeholder is measured too: estimateMaxChars claimed 29
			// chars for a Blank+Title box that renders 96 comfortably,
			// so agents shortened titles to a number the renderer never
			// used (go-slide-creator-jcph).
			titleMeasured, _, d := measureTitleItem(item, &phInfo, slideInput, analysis.Layouts, i, j)
			if d != nil {
				ds = append(ds, *d)
			}

			// Check character limits for text content
			if !titleMeasured && item.Type == "text" && phInfo.MaxChars > 0 { //nolint:nestif // mirrors the measured-title branch above
				resolved, _ := item.ResolveValue()
				if text, ok := resolved.(string); ok && len(text) > phInfo.MaxChars {
					ph.Truncated = true
					ph.TruncateAt = phInfo.MaxChars
					msg := fmt.Sprintf("slide %d, content %d: text (%d chars) exceeds placeholder %q limit (%d chars)",
						i+1, j+1, len(text), item.PlaceholderID, phInfo.MaxChars)
					ds = append(ds, diagnostics.Diagnostic{
						Code:     patterns.ErrCodeMaxLength,
						Path:     contentValuePath(&item, i, j),
						Message:  msg,
						Severity: diagnostics.SeverityWarning,
						Fix: &diagnostics.Fix{
							Kind: "shrink_text",
							Params: map[string]any{
								"max_chars":     phInfo.MaxChars,
								"current_chars": len(text),
							},
						},
					})
				}
			}
		}
	}

	// Only title measurement uses a predicted layout: other placeholders are
	// auto-mapped during generation and cannot be checked against that layout.
	if !verdictOnly && !layoutFound && slideInput.LayoutID == "" && item.PlaceholderID != "" {
		if info := titlePlaceholderIn(predicted, item.PlaceholderID); info != nil {
			_, capacity, d := measureTitleItem(item, info, slideInput, analysis.Layouts, i, j)
			if capacity > 0 {
				ph.MaxChars = capacity
			}
			if d != nil {
				ds = append(ds, *d)
			}
		}
	}
	return ph, ds
}

func validateContentType(item ContentInput, i, j int) []diagnostics.Diagnostic {
	path := slidepath.ContentField(i, j, "type")
	if item.Type == "" {
		return []diagnostics.Diagnostic{{Code: patterns.ErrCodeRequired, Path: path, Severity: diagnostics.SeverityError,
			Message: fmt.Sprintf("slide %d, content %d: type is required", i+1, j+1),
			Fix:     &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": "type"}},
		}}
	}
	validTypes := deckinput.ContentTypes()
	if slices.Contains(validTypes, item.Type) {
		return nil
	}
	fix := &diagnostics.Fix{Kind: "use_one_of", Params: map[string]any{"available": validTypes}}
	if match, _ := generator.ClosestMatch(item.Type, validTypes, 3); match != "" {
		fix.Params["did_you_mean"] = match
	}
	return []diagnostics.Diagnostic{{Code: diagnostics.CodeUnknownEnum, Path: path, Severity: diagnostics.SeverityError,
		Message: fmt.Sprintf("slide %d, content %d: %s", i+1, j+1, deckinput.UnknownContentTypeMessage(item.Type)), Fix: fix,
	}}
}

func validateTableContent(item ContentInput, i, j int, densityLayout *types.LayoutMetadata, tableStyleByID map[string]string, availableStyleIDs []string) []diagnostics.Diagnostic {
	var ds []diagnostics.Diagnostic
	// Density check for content-level table.
	table := resolveTableFromContent(&item)
	if table != nil {
		tablePath := slidepath.ContentIndex(i, j)
		bounds := tablePlaceholderBounds(item.PlaceholderID, densityLayout)
		densityWarnings := pipeline.DetectTableDensity(table, tablePath, bounds.Width)
		ds = append(ds, diagnostics.FromValidationWarnings(densityWarnings)...)

		// Warn when both header_background and style_id are explicitly authored.
		if table.Style != nil {
			if w := generator.WarnStyleCollision(i,
				table.Style.HeaderBackground != nil,
				table.Style.StyleID != "",
			); w != "" {
				collisionVE := &patterns.ValidationError{
					Path:    tablePath,
					Code:    "style_collision",
					Message: w,
				}
				ds = append(ds, diagnostics.FromValidationWarning(collisionVE))
			}
		}
		// Reject style_id values that the renderer would drop because
		// they are not GUID-shaped (typos or XML-metacharacter
		// injection attempts). Error, since they can never render.
		tableValuePath := contentTablePath(&item, i, j)
		if d := invalidTableStyleIDDiagnostic(table, tableValuePath, i); d != nil {
			ds = append(ds, *d)
		}
		// A row wider than the headers cannot render (the
		// header count defines the table grid) and generate
		// refuses it; short rows are padded
		// (go-slide-creator-s1uvj.26).
		if err := table.ToTableSpec().CheckRowWidths(); err != nil {
			ds = append(ds, diagnostics.Diagnostic{
				Code:     diagnostics.CodeInvalidParameter,
				Path:     tableValuePath + "/" + tableGridErrorField(table.ToTableSpec()),
				Message:  fmt.Sprintf("slide %d: %v", i+1, err),
				Severity: diagnostics.SeverityError,
			})
		}
		// Validate style_id against template's declared table styles.
		// Advisory only — an unknown (but well-formed) style_id does
		// not invalidate the deck (Valid is left unchanged).
		if vw := validateTableStyleID(table, tableValuePath, i, tableStyleByID, availableStyleIDs); vw != nil {
			ds = append(ds, diagnostics.FromValidationWarning(vw))
		}
		// Reject conditional-format fills that are neither a scheme
		// color nor a 6-digit hex value. resolveConditionalFill drops
		// such values to prevent malformed/injected OOXML, so surface
		// a clear error instead of silently losing the fill.
		ds = append(ds, invalidConditionalFillDiagnostics(table, tableValuePath, i)...)
		// A rule outside the vocabulary, or a threshold the rule
		// cannot use, applies no fill at all. Say which, with the
		// allowed list, instead of leaving the cell quietly plain
		// (go-slide-creator-6hlu).
		ds = append(ds, conditionalRuleDiagnostics(table, tableValuePath, i)...)
	}
	return ds
}

// validateSlideContent preserves authored item order for diagnostics and reports.
func validateSlideContent(output *dryRunOutput, slideInput SlideInput, i int, lm types.LayoutMetadata, layoutFound bool, predictedLayouts []*types.LayoutMetadata, densityLayout *types.LayoutMetadata, analysis *types.TemplateAnalysis, tableStyleByID map[string]string, availableStyleIDs []string) []dryRunPlaceholder {
	var placeholders []dryRunPlaceholder
	// Validate content items using ContentInput.ResolveValue() for typed + legacy support
	for j, item := range slideInput.Content {
		if len([]rune(item.Heading)) > deckinput.MaxHeadingRunes {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{Code: diagnostics.CodeInvalidParameter, Path: slidepath.ContentField(i, j, "heading"), Severity: diagnostics.SeverityError, Message: deckinput.HeadingLimitMessage})
		}
		var predicted *types.LayoutMetadata
		if !output.verdictOnly {
			predicted = predictedLayouts[i]
		}
		ph, placeholderDiags := validateContentPlaceholder(item, slideInput, i, j, lm, layoutFound, output.verdictOnly, predicted, analysis)
		output.addValidationDiagnostics(placeholderDiags...)

		output.addValidationDiagnostics(validateContentType(item, i, j)...)
		if item.Type != "" {
			// Count content types
			switch item.Type {
			case "chart", "diagram":
				if item.Type == "chart" {
					output.ChartCount++
				} else {
					output.DiagramCount++
				}
				if dd := contentDiagramValidationDiagnostics(item, i, j); len(dd) > 0 {
					output.Valid = false
					output.Diagnostics = append(output.Diagnostics, dd...)
				}
			case "table":
				output.TableCount++
				output.addValidationDiagnostics(validateTableContent(item, i, j, densityLayout, tableStyleByID, availableStyleIDs)...)

			}
		}

		output.addValidationDiagnostics(validateContentValue(item, i, j)...)

		placeholders = append(placeholders, ph)
	}

	return placeholders
}

func validateContentValue(item ContentInput, i, j int) []diagnostics.Diagnostic {
	var ds []diagnostics.Diagnostic
	// Validate content value is parseable using ResolveValue
	if item.Type != "" {
		_, valueErr := item.ResolveValue()
		// ResolveValue deliberately leaves legacy chart/diagram "value"
		// decoding to its callers. Dry-run must still reject malformed
		// values before optional svggen warnings are collected.
		if valueErr == nil && len(item.Value) > 0 {
			switch item.Type {
			case "chart":
				if item.ChartValue == nil {
					var chart types.ChartSpec //nolint:staticcheck // legacy input
					valueErr = json.Unmarshal(item.Value, &chart)
				}
			case "diagram":
				if item.DiagramValue == nil {
					var diagram types.DiagramSpec
					valueErr = json.Unmarshal(item.Value, &diagram)
				}
			}
		}
		if valueErr != nil {
			msg := fmt.Sprintf("slide %d, content %d: %v", i+1, j+1, valueErr)
			ds = append(ds, diagnostics.Diagnostic{
				Code:     diagnostics.CodeInvalidParameter,
				Path:     contentValuePath(&item, i, j),
				Message:  msg,
				Severity: diagnostics.SeverityError,
				Details:  map[string]any{"cause": valueErr.Error()},
			})
		}
	}

	// Detect legacy authoring form: "value" field instead of typed fields
	if item.UsesLegacyValue() {
		typedField := item.Type + "_value"
		path := slidepath.ContentIndex(i, j)
		legacyVE := &patterns.ValidationError{
			Path:    path,
			Code:    "legacy_authoring_form",
			Message: fmt.Sprintf("slide %d, content %d: uses legacy \"value\" field; prefer \"%s\" for new decks", i+1, j+1, typedField),
			Fix: &patterns.FixSuggestion{
				Kind:   "rewrite_field",
				Params: map[string]any{"from": "value", "to": typedField},
			},
		}
		ds = append(ds, diagnostics.FromValidationWarning(legacyVE))
	}

	// Validate chart/diagram data structure via svggen
	if item.Type == "chart" || item.Type == "diagram" {
		chartWarnings := validateContentDiagramData(item, i+1, j+1)
		for _, w := range chartWarnings {
			ds = append(ds, diagnostics.Diagnostic{
				Code:     patterns.ErrCodeChartValueCoerced,
				Path:     slidepath.ContentIndex(i, j),
				Message:  w,
				Severity: diagnostics.SeverityWarning,
			})
		}
	}

	return ds
}

func validateSlideVisuals(output *dryRunOutput, slideInput SlideInput, i int, analysis *types.TemplateAnalysis, rhythm *resolvedGrid, tableStyleByID map[string]string, availableStyleIDs []string) int {
	shapeCount := 0
	// Validate shape_grid if present
	if slideInput.ShapeGrid != nil {
		geom, _ := patternExpansionGeometry(slideInput, analysis.Layouts, analysis.SlideWidth, analysis.SlideHeight, rhythm)
		resolved := resolveGridForStructural(slideInput.ShapeGrid, geom.OverrideBounds, geom.Zone, analysis.SlideWidth, analysis.SlideHeight)
		gridCounts, gridWarnings, gridErrors, gridValWarnings := validateShapeGrid(slideInput.ShapeGrid, i+1, resolved)
		shapeCount = gridCounts.Shapes
		output.ShapeCount += gridCounts.Shapes
		output.TableCount += gridCounts.Tables
		output.DiagramCount += gridCounts.Diagrams
		output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarnings(gridValWarnings)...)
		for _, w := range gridWarnings {
			output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
				Code:     diagnostics.CodeInvalidGrid,
				Path:     slidepath.ShapeGrid(i),
				Message:  w,
				Severity: diagnostics.SeverityWarning,
			})
		}
		if len(gridErrors) > 0 {
			output.Valid = false
			for _, e := range gridErrors {
				output.Diagnostics = append(output.Diagnostics, diagnostics.Diagnostic{
					Code:     diagnostics.CodeInvalidGrid,
					Path:     slidepath.ShapeGrid(i),
					Message:  e,
					Severity: diagnostics.SeverityError,
				})
			}
		}

		// A table in a shape_grid cell gets the style and
		// conditional-format checks a placeholder table gets, nested
		// sub-grids included (go-slide-creator-dln5i).
		tableErrs, tableWarns := gridTableValueDiagnostics(slideInput.ShapeGrid, slidepath.ShapeGrid(i), i, tableStyleByID, availableStyleIDs)
		if len(tableErrs) > 0 {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, tableErrs...)
		}
		output.Diagnostics = append(output.Diagnostics, tableWarns...)

		// A grid-cell table row wider than its headers cannot render and
		// generate refuses it, exactly as for placeholder tables; check
		// every grid-cell table, nested sub-grids included
		// (go-slide-creator-csclk.5).
		if rd := gridTableRowWidthDiagnostics(slideInput.ShapeGrid, slidepath.ShapeGrid(i), i); len(rd) > 0 {
			output.Valid = false
			output.Diagnostics = append(output.Diagnostics, rd...)
		}
	}

	// Pattern values: generate validates every named pattern against the
	// pattern's own value schema and refuses the deck on failure, so
	// validate must run the same check rather than silently accepting a
	// deck generate will reject (go-slide-creator-xvek).
	patternCtx := patterns.ExpandContext{
		Theme:       analysis.Theme,
		Metadata:    analysis.Metadata,
		SlideWidth:  analysis.SlideWidth,
		SlideHeight: analysis.SlideHeight,
		SlideIndex:  i,
	}
	if pd := slidePatternDiagnostics(&slideInput, i, patternCtx, patterns.Default()); len(pd) > 0 {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, pd...)
	}

	// Grid diagram data: generate aborts on svggen validation failures in
	// shape_grid diagrams, including those produced by pattern expansion
	// (e.g. chart-insights-split), so report them as errors here.
	gridDiags := gridDiagramValidationDiagnostics(slideInput.ShapeGrid, i, "shape_grid", slidepath.ShapeGrid(i))
	if pg := expandSlidePatternGrid(&slideInput, i, analysis.SlideWidth, analysis.SlideHeight, &analysis.Theme); pg != nil {
		gridDiags = append(gridDiags, gridDiagramValidationDiagnostics(pg, i, "pattern "+slideInput.Pattern.Name, slidepath.SlideField(i, "pattern"))...)
	}
	gridDiags = append(gridDiags, composeDiagramValidationDiagnostics(slideInput.Compose, i, slidepath.SlideField(i, "compose"))...)
	if len(gridDiags) > 0 {
		output.Valid = false
		output.Diagnostics = append(output.Diagnostics, gridDiags...)
	}

	// Takeaway warning: a slide that argues from data and says nothing
	// about what the data means loses most of its narrative value — the
	// audience cannot tell what the chart is supposed to argue. Warn (do
	// not error) when missing, and stay quiet when the title already
	// carries the argument.
	if slideMissesTakeaway(slideInput) {
		// Advisory only — a missing takeaway does not invalidate the deck
		// (Valid is left unchanged), so it is a warning. The fit report
		// carries the same finding (same path and message) for score_deck's
		// gate; the envelope dedupes the pair.
		output.Diagnostics = append(output.Diagnostics, diagnostics.FromValidationWarning(takeawayMissingError(i)))
	}

	return shapeCount
}
