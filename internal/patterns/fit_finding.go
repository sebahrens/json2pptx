package patterns

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Extent represents a measured or allowed dimension in EMU.
type Extent struct {
	WidthEMU  int64 `json:"width_emu"`
	HeightEMU int64 `json:"height_emu"`
}

// ToolCallSuggestion is a machine-readable next-step for an agent: the exact
// MCP tool to call and a template for its arguments. Agents can invoke the
// tool directly without inferring the protocol from prose.
type ToolCallSuggestion struct {
	Tool         string         `json:"tool"`
	ArgsTemplate map[string]any `json:"args_template"`
}

// FitFinding is a fit-report finding that embeds ValidationError for
// unification with the existing validation envelope. The embedding flattens
// all ValidationError fields to the top level in JSON output.
type FitFinding struct {
	ValidationError

	// Action is the recommended remediation: "refuse", "shrink_or_split",
	// "review", or "info", ranked from most to least severe.
	Action string `json:"action"`

	// Measured is the actual extent of the content (nil when not applicable).
	Measured *Extent `json:"measured,omitempty"`

	// Allowed is the available extent for the content (nil when not applicable).
	Allowed *Extent `json:"allowed,omitempty"`

	// OverflowRatio is measured/allowed as a fraction (e.g. 1.25 means 25%
	// over). Zero when extents are not available.
	OverflowRatio float64 `json:"overflow_ratio,omitempty"`

	// NextToolCall is a machine-readable suggestion for the next MCP tool call
	// that would resolve this finding. Populated for findings with action
	// "refuse", "shrink_or_split", or "review".
	NextToolCall *ToolCallSuggestion `json:"next_tool_call,omitempty"`

	// SegmentIndex is the 0-based child segment index inside a compose envelope
	// that the finding is attributable to. Nil when the finding is not
	// segment-scoped (i.e. the slide is not a compose slide or the finding is
	// emitted against the merged grid rather than a specific segment).
	SegmentIndex *int `json:"segment_index,omitempty"`
}

// SeverityForAction maps a finding's action to the severity every surface
// reports it at. It lives here, beside the actions themselves, so the
// diagnostics envelope and a raw fit_findings array cannot disagree about one
// finding — which they did: generate_presentation's fit_findings carried no
// severity at all while validate_input reported the same finding as "info"
// (go-slide-creator-dwkkf).
func SeverityForAction(action string) string {
	switch action {
	case "refuse":
		return "error"
	case "shrink_or_split":
		return "warning"
	default: // "review", "info", unknown
		return "info"
	}
}

// Severity is the finding's severity, resolved from its action and falling
// back to the action its CODE declares when the emitter stated none.
func (f FitFinding) Severity() string {
	action := f.Action
	if action == "" {
		if meta, ok := GetFindingMeta(f.Code); ok {
			action = meta.Severity
		}
	}
	return SeverityForAction(action)
}

// fitFindingJSON mirrors FitFinding for marshalling, with the derived severity
// written out. The alias type breaks the recursion into MarshalJSON.
type fitFindingJSON struct {
	fitFindingAlias
	Severity string `json:"severity"`
}

type fitFindingAlias FitFinding

// MarshalJSON emits the derived severity alongside the action, so a client
// filtering fit_findings by severity gets the same answer from every tool.
func (f FitFinding) MarshalJSON() ([]byte, error) {
	return json.Marshal(fitFindingJSON{
		fitFindingAlias: fitFindingAlias(f),
		Severity:        f.Severity(),
	})
}

// ContentDropped builds a CONTENT_DROPPED fit finding for a path where
// author-provided content could not be placed and was dropped. This is the
// single, shared diagnostic that every silent content-drop path (dropped
// slides in partial mode, unplaced content blocks, truncated columns, dropped
// unknown payload fields) should emit so agents see one consistent,
// machine-actionable signal instead of silence.
//
//   - path is the JSON Pointer to the dropped element (e.g. "/slides/3" for a
//     whole slide, "/slides/3/content/2" for a content block).
//   - locator is a short human label for what was dropped (e.g. "slide",
//     "content block 3", "left column").
//   - reason explains why placement failed.
//
// The finding is advisory (action "review") — it never blocks generation; its
// purpose is to turn a silent drop into a visible, repairable signal. Fix kind
// is "review": there is no single deterministic auto-fix, so the agent must
// restructure or split the slide.
func ContentDropped(path, locator, reason string) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Path:    path,
			Code:    ErrCodeContentDropped,
			Message: fmt.Sprintf("author-provided content dropped (%s): %s", locator, reason),
			Fix: &FixSuggestion{
				Kind:   "review",
				Params: map[string]any{"locator": locator, "reason": reason},
			},
		},
		Action: "review",
	}
}

// ContentDroppedNoPlaceholder builds a CONTENT_DROPPED fit finding for
// author-provided content that could not be placed because the target
// placeholder_id does not exist in the resolved layout. This is the silent
// killer the raw generate path used to report only as a human-readable warning
// while still answering success:true (go-slide-creator-lhq6): the slide renders
// without the content and nothing machine-readable says so.
//
//   - path is the JSON Pointer to the dropped content block.
//   - locator is a short human label ("content block 2 (image)").
//   - placeholderID / layoutID identify the failed target.
//   - available lists the placeholder IDs the layout does declare.
//
// The Fix carries cause "placeholder_not_found" plus the available ids and a
// did_you_mean suggestion, so callers can both remap the content and
// distinguish this drop from advisory ones (strict output_validation fails the
// render on this cause alone).
func ContentDroppedNoPlaceholder(path, locator, placeholderID, layoutID string, available []string) FitFinding {
	reason := fmt.Sprintf(
		"placeholder_id %q does not exist in layout %q, so the content was not rendered",
		placeholderID, layoutID)
	f := ContentDropped(path, locator, reason)
	f.Fix.Params["cause"] = CausePlaceholderNotFound
	f.Fix.Params["placeholder_id"] = placeholderID
	f.Fix.Params["layout_id"] = layoutID
	if len(available) > 0 {
		f.Fix.Params["available"] = available
	}
	f.Fix.Params["options"] = hardDropOptions
	f.Action = "refuse"
	return f
}

// ContentDroppedPlaceholderOccupied builds the hard CONTENT_DROPPED finding for
// a content block that resolved to a placeholder another block already fills.
// A placeholder renders one text block (or one visual), so the later block is
// not rendered at all. Like a missing placeholder this is actual source loss,
// not advice: the finding carries action "refuse" and cause
// "placeholder_occupied" (go-slide-creator-k3lyz).
//
//   - firstBlock is the 0-based index of the content block that kept the slot.
//   - layoutID is the resolved layout, so the agent can pick a different one.
func ContentDroppedPlaceholderOccupied(path, locator, placeholderID, layoutID string, firstBlock int, what string) FitFinding {
	reason := fmt.Sprintf(
		"placeholder %q already holds content block %d; a placeholder renders one %s — split the slide, choose a layout with a slot for each block, or target a free placeholder (e.g. body_2)",
		placeholderID, firstBlock+1, what)
	f := ContentDropped(path, locator, reason)
	f.Fix.Params["cause"] = CausePlaceholderOccupied
	f.Fix.Params["placeholder_id"] = placeholderID
	f.Fix.Params["layout_id"] = layoutID
	f.Fix.Params["kept_block"] = firstBlock
	f.Fix.Params["options"] = hardDropOptions
	f.Action = "refuse"
	return f
}

// hardDropOptions lists the concrete remedies for a hard content drop, in the
// order an agent should try them.
var hardDropOptions = []string{"split_slide", "choose_layout", "retarget_placeholder", "merge_blocks"}

// CausePlaceholderOccupied is the Fix.Params["cause"] value that marks a
// CONTENT_DROPPED finding as a hard drop caused by two content blocks resolving
// to the same placeholder.
const CausePlaceholderOccupied = "placeholder_occupied"

// CausePlaceholderNotFound is the Fix.Params["cause"] value that marks a
// CONTENT_DROPPED finding as a hard drop caused by a missing placeholder,
// as opposed to the advisory drops (partial-mode slide skips, visual
// collisions) that share the code.
const CausePlaceholderNotFound = "placeholder_not_found"

// IsHardContentDrop reports whether a finding is a CONTENT_DROPPED caused by a
// placeholder that does not exist in the resolved layout or that another
// content block already fills. Strict output_validation treats these as render
// failures: the artifact is missing content the author asked for.
func IsHardContentDrop(f FitFinding) bool {
	if f.Code != ErrCodeContentDropped || f.Fix == nil {
		return false
	}
	cause, _ := f.Fix.Params["cause"].(string)
	return cause == CausePlaceholderNotFound || cause == CausePlaceholderOccupied
}

// SparseSingleRowFlow builds a SPARSE_SINGLE_ROW_FLOW fit finding for a
// slide-level single-row sequence pattern (process-flow or the single-row
// "dots" style of timeline-horizontal) that carries sparse per-cell text and
// is the slide's only content. Both patterns size the row to its text
// (process-flow since go-slide-creator-xb06p/pfyeg; the finding used to say
// the boxes stretch to fill the slide), so the fault is the other one: a
// short band of a few words and an otherwise empty slide
// (go-slide-creator-tm46l).
//
//   - patternName is the offending pattern ("process-flow" / "timeline-horizontal").
//   - path is the JSON Pointer to the slide's pattern field (e.g. "/slides/3/pattern").
//   - slideIdx is the 0-based slide index (used only to humanise the message).
//   - itemCount is the number of steps / stops in the single row.
//   - avgChars is the average per-cell text length that tripped the sparse gate.
//
// The finding is advisory (action "review"): it never blocks generation. The
// fix is a swap_pattern suggestion ranked toward numbered-step-strip (which adds
// a per-step detail zone), with process-grid-2row and phase-roadmap as
// alternatives. A pattern the author placed with bounds / max_height_pct is
// not reported, but the cap does not change the row and is not advice.
func SparseSingleRowFlow(patternName, path string, slideIdx, itemCount int, avgChars float64) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Pattern: patternName,
			Path:    path,
			Code:    ErrCodeSparseSingleRowFlow,
			Message: fmt.Sprintf(
				"slide %d: %s is the slide's only content: one row of %d short cells (avg %.0f chars), sized to its text, so most of the slide stays empty — give each step a detail line (numbered-step-strip), use process-grid-2row / phase-roadmap, or pair the row with a second zone (compose)",
				slideIdx+1, patternName, itemCount, avgChars),
			Fix: &FixSuggestion{
				Kind: "swap_pattern",
				Params: map[string]any{
					"from":       patternName,
					"item_count": itemCount,
					"avg_chars":  avgChars,
					"reason":     "single_row_sparse",
					"suggested": []any{
						map[string]any{"to": "numbered-step-strip", "rationale": "ordered steps with a per-step detail zone fill the vertical space"},
						map[string]any{"to": "process-grid-2row", "rationale": "use two parallel tracks when the steps split into two lanes"},
						map[string]any{"to": "phase-roadmap", "rationale": "milestones with dates/descriptions belong on a phased roadmap"},
					},
				},
			},
		},
		Action: "review",
	}
}

// FlowDiamondNoContent builds a FLOW_DIAMOND_NO_CONTENT fit finding for a
// standalone process-flow that carries at least one decision diamond
// (step.type == "decision") but has no supporting content zone — the flow
// draws one path through its steps (one row, or two rows from seven steps)
// and no branch, so nothing says what the outcomes are. Compose envelopes
// and nested cell patterns are exempt (a second zone already carries the
// explanation), enforced by the caller reading slide.Pattern directly.
//
//   - path is the JSON Pointer to the slide's pattern field.
//   - slideIdx is the 0-based slide index (humanises the message only).
//   - diamondCount is the number of decision steps in the flow.
func FlowDiamondNoContent(path string, slideIdx, diamondCount int) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Pattern: "process-flow",
			Path:    path,
			Code:    ErrCodeFlowDiamondNoContent,
			Message: fmt.Sprintf(
				"slide %d: process-flow has %d decision diamond(s) but no supporting content zone to explain the branch outcomes — process-flow draws one path through its steps and no yes/no branches; add an explanatory zone via compose, or switch to numbered-step-strip with per-step detail",
				slideIdx+1, diamondCount),
			Fix: &FixSuggestion{
				Kind: "swap_pattern",
				Params: map[string]any{
					"from":          "process-flow",
					"diamond_count": diamondCount,
					"reason":        "decision_without_branch_zone",
					"suggested": []any{
						map[string]any{"to": "numbered-step-strip", "rationale": "per-step detail zone explains each decision outcome"},
						map[string]any{"to": "compose", "rationale": "pair the flow with an explanatory panel so the branch outcomes are visible"},
					},
				},
			},
		},
		Action: "review",
	}
}

// TocFlowchartVocab builds a TOC_FLOWCHART_VOCAB fit finding for an agenda /
// table-of-contents slide rendered with sequential flowchart vocabulary
// (process-flow / swimlane / timeline-horizontal). A contents list is not a
// sequence with arrows; the flowchart vocabulary implies a causal/temporal
// order the agenda does not have.
//
//   - patternName is the offending sequential pattern.
//   - path is the JSON Pointer to the slide's pattern field.
//   - slideIdx is the 0-based slide index (humanises the message only).
//   - title is the slide's title text that matched the agenda/ToC vocabulary.
func TocFlowchartVocab(patternName, path string, slideIdx int, title string) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Pattern: patternName,
			Path:    path,
			Code:    ErrCodeTocFlowchartVocab,
			Message: fmt.Sprintf(
				"slide %d: agenda / table-of-contents slide (%q) is drawn with %s flowchart vocabulary — a contents list is not a sequence with arrows; use the agenda pattern or numbered-step-strip in 'toc' style",
				slideIdx+1, title, patternName),
			Fix: &FixSuggestion{
				Kind: "swap_pattern",
				Params: map[string]any{
					"from":   patternName,
					"reason": "toc_as_flowchart",
					"suggested": []any{
						map[string]any{"to": "agenda", "rationale": "numbered section list is the canonical agenda / table-of-contents layout"},
						map[string]any{"to": "numbered-step-strip", "rationale": "use the 'toc' style for a contents list without flowchart arrows"},
					},
				},
			},
		},
		Action: "review",
	}
}

// MatrixAxisImbalance builds a MATRIX_AXIS_IMBALANCE fit finding for a rotated
// text band (an axis label rotated ~90°/270°) that spans rows or columns.
// Rotating the whole band flips its width/height about its center, so a
// narrow-tall axis band renders wide-short (or vice versa) and intrudes into
// the adjacent quadrants/cells (the J2P-MATRIX-005 anti-pattern). This is a
// rendering-geometry smell, not a pattern-choice one (see FindingClass).
//
//   - path is the JSON Pointer to the offending grid cell's shape.
//   - slideIdx is the 0-based slide index (humanises the message only).
//   - rotationDeg is the shape's rotation in degrees.
func MatrixAxisImbalance(path string, slideIdx int, rotationDeg float64) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Pattern: "shape_grid",
			Path:    path,
			Code:    ErrCodeMatrixAxisImbalance,
			Message: fmt.Sprintf(
				"slide %d: a spanning text band is rotated %.0f° — rotating the band flips its width/height about its center, so it renders wide-short (or tall-narrow) and intrudes into the adjacent cells; render the label with vert text direction (vert270) in an unrotated band instead of rotating the shape",
				slideIdx+1, rotationDeg),
			Fix: &FixSuggestion{
				Kind: "autofix_visual",
				Params: map[string]any{
					"reason":       "rotated_band_aspect_flip",
					"rotation_deg": rotationDeg,
					"guidance":     "set the band shape rotation to 0 and rotate only the text via vert=\"vert270\" (vertical text direction) so the fill geometry is never transformed",
				},
			},
		},
		Action: "review",
	}
}

// actionRanks maps action strings to severity ranks. Higher rank = more severe.
var actionRanks = map[string]int{
	"info":            0,
	"review":          1,
	"shrink_or_split": 2,
	"refuse":          3,
}

// ActionRank returns the severity rank for the given action string.
// Unknown actions return -1.
func ActionRank(action string) int {
	rank, ok := actionRanks[action]
	if !ok {
		return -1
	}
	return rank
}

// RepairToolCall builds a ToolCallSuggestion for the repair_slide tool from a
// fix suggestion and slide index. Returns nil if the fix kind is not a
// repair_slide kind.
func RepairToolCall(slideIdx int, fix *FixSuggestion) *ToolCallSuggestion {
	if fix == nil {
		return nil
	}
	// Only an EXECUTABLE kind can be handed to repair_slide. This used to be a
	// second hand-maintained list that had drifted: reduce_cell_text was absent,
	// so every grid-cell overflow finding shipped next_tool_call: null and the
	// agent had to derive the repair itself (go-slide-creator-9zof).
	if !FixKindIsExecutable(fix.Kind) {
		return nil
	}

	fixDirective := map[string]any{"kind": fix.Kind}
	if len(fix.Params) > 0 {
		fixDirective["params"] = fix.Params
	}

	return &ToolCallSuggestion{
		Tool: "repair_slide",
		ArgsTemplate: map[string]any{
			"slide_index": slideIdx,
			"fixes":       []any{fixDirective},
		},
	}
}

// RecommendToolCall builds a ToolCallSuggestion for the recommend_visual tool.
// Used when a finding suggests adopting or switching patterns.
//
// recommend_pattern requires intent and has no item_count parameter, so the old
// {tool: recommend_pattern, args: {item_count}} suggestion failed with
// INPUT.UNKNOWN_PARAMETER in every profile that did not rewrite it
// (go-slide-creator-csclk.124). recommend_visual is in every profile and takes
// the count as a content hint.
// A non-positive itemCount (unknown) is left out of content_hints rather than
// sent as a misleading 0.
func RecommendToolCall(itemCount int) *ToolCallSuggestion {
	hints := map[string]any{}
	if itemCount > 0 {
		hints["item_count"] = itemCount
	}
	return &ToolCallSuggestion{
		Tool: "recommend_visual",
		ArgsTemplate: map[string]any{
			"intent":        "<one sentence: what this slide should show>",
			"content_hints": hints,
		},
	}
}

// FixItemCount returns the item count a pattern-swap fix carries in its params
// (item_count, else filled_slots), or 0 when it carries none.
func FixItemCount(fix *FixSuggestion) int {
	if fix == nil {
		return 0
	}
	for _, key := range []string{"item_count", "filled_slots"} {
		if n, ok := fix.Params[key].(int); ok {
			return n
		}
	}
	return 0
}

// SortCanonical sorts findings in place into the canonical serialization order:
//
//  1. severity descending (highest ActionRank first — refuse > shrink_or_split > review > info)
//  2. slide index ascending (path-derived; -1 for deck-level findings sorts before slide 0)
//  3. code ascending (lexicographic, stable tiebreaker)
//
// This is the invariant every fit_report / findings array must satisfy before
// it crosses a serialization boundary so agents can rely on findings[0] being
// the most important fix, with deterministic ordering across runs and tools.
//
// slideIndexFn extracts the 0-based slide index from a finding's path; pass
// slidepath.SlideIndex (or an equivalent extractor) from the caller.
func SortCanonical(findings []FitFinding, slideIndexFn func(string) int) {
	if len(findings) <= 1 {
		return
	}
	sort.Slice(findings, func(i, j int) bool {
		ri := ActionRank(findings[i].Action)
		rj := ActionRank(findings[j].Action)
		if ri != rj {
			return ri > rj
		}
		si := slideIndexFn(findings[i].Path)
		sj := slideIndexFn(findings[j].Path)
		if si != sj {
			return si < sj
		}
		return findings[i].Code < findings[j].Code
	})
}

// AttachNextToolCalls populates NextToolCall on each finding whose action is
// not "info", using the fix kind and path to determine the appropriate tool.
// slideIndexFn extracts the 0-based slide index from a finding's path.
func AttachNextToolCalls(findings []FitFinding, slideIndexFn func(string) int) {
	for i := range findings {
		f := &findings[i]
		if f.Action == "info" || f.NextToolCall != nil {
			continue
		}
		if f.Fix == nil {
			continue
		}

		slideIdx := slideIndexFn(f.Path)

		switch f.Fix.Kind {
		case "adopt_pattern", "swap_pattern":
			// Adopting or switching patterns — point to recommend_visual for
			// the agent to pick the right one.
			f.NextToolCall = RecommendToolCall(FixItemCount(f.Fix))
		default:
			f.NextToolCall = RepairToolCall(slideIdx, f.Fix)
		}
	}
}

// ImageHeavyCrop builds the IMAGE_HEAVY_CROP review finding for a cover-fit
// picture that discards more than 30% of an axis (go-slide-creator-dk5sk).
// defaulted reports that cover was the placeholder default, not authored.
func ImageHeavyCrop(path, placeholderID string, discarded float64, defaulted bool) FitFinding {
	pct := int(discarded*100 + 0.5)
	how := "fit \"cover\""
	if defaulted {
		how = "the picture placeholder's default cover fit"
	}
	return FitFinding{
		ValidationError: ValidationError{
			Path:    path,
			Code:    ErrCodeImageHeavyCrop,
			Message: fmt.Sprintf("image in placeholder %q: %s crops away %d%% of the picture to fill the frame", placeholderID, how, pct),
			Fix: &FixSuggestion{Kind: "provide_value", Params: map[string]any{
				"path":          path + "/image_value/fit",
				"value":         "contain",
				"discarded_pct": pct,
				"hint":          "use fit contain to keep the whole picture, or supply a crop whose aspect matches the frame",
			}},
		},
		Action: "review",
	}
}

// OverlayTargetCropped builds the OVERLAY_TARGET_CROPPED review finding for
// an overlay endpoint whose anchor_image target (source fractions uv) lies
// outside the visible source interval vis = [x0, x1, y0, y1] of image cell
// [row, col]. consequence says what the renderer did instead of pointing
// elsewhere (go-slide-creator-kkc5t).
func OverlayTargetCropped(path string, overlayIdx, row, col int, uv [2]float64, vis [4]float64, consequence string) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Path: path,
			Code: ErrCodeOverlayTargetCropped,
			Message: fmt.Sprintf("overlay %d: image target (%.3f, %.3f) on shape_grid image cell [%d,%d] is cropped away — the frame shows source x %.3f–%.3f, y %.3f–%.3f; %s",
				overlayIdx, uv[0], uv[1], row, col, vis[0], vis[1], vis[2], vis[3], consequence),
			Fix: &FixSuggestion{Kind: "review", Params: map[string]any{
				"path":      path,
				"visible_x": []float64{vis[0], vis[1]},
				"visible_y": []float64{vis[2], vis[3]},
				"hint":      "set the image cell's image.fit to \"contain\" to keep the whole picture, give the cell a frame closer to the picture's aspect, or move the target inside the visible interval",
			}},
		},
		Action: "review",
	}
}

// ChromeOverImage builds the CHROME_OVER_IMAGE info finding for a slide whose
// footer chrome was omitted because a picture covers the footer band
// (go-slide-creator-3bph8).
func ChromeOverImage(path string, slideNum int) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Path:    path,
			Code:    ErrCodeChromeOverImage,
			Message: fmt.Sprintf("slide %d: footer text / page number omitted because a picture covers the footer band and their contrast against the photo cannot be verified", slideNum),
			Fix: &FixSuggestion{Kind: "review", Params: map[string]any{
				"hint": "accept the omission on a full-bleed photo slide, or use a layout whose picture frame stops above the footer band",
			}},
		},
		Action: "info",
	}
}

// SubtitleWraps builds the SUBTITLE_WRAPS info finding (go-slide-creator-9bmaz).
func SubtitleWraps(path string, slideNum, lines int) FitFinding {
	return FitFinding{
		ValidationError: ValidationError{
			Path:    path,
			Code:    ErrCodeSubtitleWraps,
			Message: fmt.Sprintf("slide %d: subtitle wraps onto %d lines even at the title's width", slideNum, lines),
			Fix: &FixSuggestion{Kind: "review", Params: map[string]any{
				"path": path,
				"hint": "shorten the subtitle to a one-line dateline",
			}},
		},
		Action: "info",
	}
}
