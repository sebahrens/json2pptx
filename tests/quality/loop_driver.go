// Package quality provides the repair loop driver for agent-driven
// author→validate→fix cycles. The driver wraps json2pptx validate
// --fit-report and enforces an anti-thrash cap of MaxRepairAttempts
// per slide before forcing a split_slide envelope or failing loudly.
package quality

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// fitFinding mirrors the NDJSON structure from json2pptx validate --fit-report.
type fitFinding struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

// MaxRepairAttempts is the hard cap on repair attempts per slide before
// the driver forces a split_slide injection or fails loudly. Models will
// shrink-font→shrink-font forever if not externally capped.
const MaxRepairAttempts = 2

// RepairAction is what the loop driver tells the agent to do.
type RepairAction string

const (
	// ActionRepair means the agent should fix the indicated cells.
	ActionRepair RepairAction = "repair"
	// ActionForceSplit means the agent must inject a split_slide envelope.
	ActionForceSplit RepairAction = "force_split"
	// ActionPass means all cells fit — no repair needed.
	ActionPass RepairAction = "pass"
	// ActionVisualQA means repair attempts are exhausted for some slides;
	// the agent should escalate to visual QA inspection before forcing split.
	ActionVisualQA RepairAction = "visual_qa"
)

// LoopConfig controls loop driver behavior.
type LoopConfig struct {
	// Binary is the path to the json2pptx binary.
	Binary string
	// ForceSplitOnCap controls whether the driver suggests force_split
	// (true) or returns an error (false) when the repair cap is reached.
	ForceSplitOnCap bool
	// EnableVisualQA controls whether the driver escalates to visual QA
	// inspection before forcing split. When true and repair cap is reached,
	// the driver returns ActionVisualQA instead of ActionForceSplit on the
	// first cap hit, giving the agent a chance to run autofix_visual.
	EnableVisualQA bool
}

// SlideFinding groups fit findings for a single slide.
type SlideFinding struct {
	SlideIndex int          `json:"slide_index"`
	Findings   []fitFinding `json:"findings"`
	Summary    string       `json:"summary"`
}

// LoopResult is the structured output of a single validate pass.
type LoopResult struct {
	Action   RepairAction   `json:"action"`
	Slides   []SlideFinding `json:"slides,omitempty"`
	CappedAt []int          `json:"capped_at,omitempty"` // slide indices that hit the cap
	Message  string         `json:"message"`
}

// LoopState tracks per-slide repair attempt counts across iterations.
type LoopState struct {
	Attempts     map[int]int  // slide index → attempt count
	VisualQADone map[int]bool // slide index → true if visual QA was already attempted
}

// NewLoopState creates a fresh loop state.
func NewLoopState() *LoopState {
	return &LoopState{
		Attempts:     make(map[int]int),
		VisualQADone: make(map[int]bool),
	}
}

// MarkVisualQADone records that visual QA has been attempted for the given
// slide indices. After this, the next cap hit will escalate to force_split.
func (s *LoopState) MarkVisualQADone(slideIndices []int) {
	for _, idx := range slideIndices {
		s.VisualQADone[idx] = true
	}
}

// RunValidatePass executes json2pptx validate --fit-report --format=ndjson on
// the given JSON file and returns structured fit findings.
//
// The CLI emits the MCP validate_input dryRunOutput shape on stdout (a single
// JSON object per file). Diagnostics — including fit findings — are folded into
// the single findings envelope (replacing the legacy fit_findings[] array); fit
// findings carry category "FIT" and stash their action and JSON path in the
// finding's evidence map. For invalid inputs the CLI emits the diagnostics error
// envelope, which has no findings envelope; this function then returns an empty
// slice (the loop driver only cares about fit findings).
func RunValidatePass(cfg LoopConfig, jsonPath string) ([]fitFinding, error) {
	cmd := exec.Command(cfg.Binary, "validate", "--fit-report", "--format=ndjson", jsonPath) //nolint:gosec // controlled inputs in test/agent context
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// validate may return non-zero for invalid inputs — that's expected.
	_ = cmd.Run()

	// NDJSON: one envelope per file, but the CLI is invoked with a single file
	// here so we parse the single object on the first non-empty line.
	type wireFinding struct {
		Code     string         `json:"code"`
		Category string         `json:"category"`
		Message  string         `json:"message"`
		Evidence map[string]any `json:"evidence"`
	}
	type dryRunEnvelope struct {
		Findings struct {
			Findings []wireFinding `json:"findings"`
		} `json:"findings"`
	}

	var findings []fitFinding
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var env dryRunEnvelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			continue
		}
		for _, f := range env.Findings.Findings {
			if f.Category != "FIT" {
				continue
			}
			ff := fitFinding{Code: f.Code, Message: f.Message}
			if p, ok := f.Evidence["path"].(string); ok {
				ff.Path = p
			}
			if a, ok := f.Evidence["action"].(string); ok {
				ff.Action = a
			}
			findings = append(findings, ff)
		}
	}

	return findings, nil
}

// EvaluateFindings takes raw findings and the current loop state, and
// produces a LoopResult with the appropriate action for the agent.
func EvaluateFindings(findings []fitFinding, state *LoopState, cfg LoopConfig) LoopResult {
	if len(findings) == 0 {
		return LoopResult{
			Action:  ActionPass,
			Message: "All cells fit — no repair needed.",
		}
	}

	// Group findings by slide index (extracted from path).
	grouped := groupBySlide(findings)

	var repairSlides []SlideFinding
	var cappedSlides []int

	for slideIdx, slideFinding := range grouped {
		// Only count slides with refuse-action findings.
		hasUnfittable := false
		for _, f := range slideFinding.Findings {
			if f.Action == "refuse" {
				hasUnfittable = true
				break
			}
		}
		if !hasUnfittable {
			continue
		}

		attempts := state.Attempts[slideIdx]
		if attempts >= MaxRepairAttempts {
			cappedSlides = append(cappedSlides, slideIdx)
		} else {
			state.Attempts[slideIdx] = attempts + 1
			repairSlides = append(repairSlides, slideFinding)
		}
	}

	// No refuse-action findings at all → pass.
	if len(repairSlides) == 0 && len(cappedSlides) == 0 {
		return LoopResult{
			Action:  ActionPass,
			Message: "All cells fit — no repair needed.",
		}
	}

	// All failing slides are capped → try visual QA, then force split or fail.
	if len(repairSlides) == 0 && len(cappedSlides) > 0 {
		// If visual QA is enabled and any capped slide hasn't been through
		// visual QA yet, escalate to visual QA first.
		if cfg.EnableVisualQA {
			var needsVQA []int
			for _, idx := range cappedSlides {
				if !state.VisualQADone[idx] {
					needsVQA = append(needsVQA, idx)
				}
			}
			if len(needsVQA) > 0 {
				return LoopResult{
					Action:   ActionVisualQA,
					CappedAt: needsVQA,
					Message: fmt.Sprintf(
						"Repair cap (%d attempts) reached for slide(s) %v. "+
							"Escalating to visual QA inspection before forcing split.",
						MaxRepairAttempts, needsVQA,
					),
				}
			}
		}

		action := ActionForceSplit
		msg := fmt.Sprintf(
			"Repair cap (%d attempts) reached for slide(s) %v. "+
				"Inject split_slide envelope to split the table across pages.",
			MaxRepairAttempts, cappedSlides,
		)
		if !cfg.ForceSplitOnCap {
			msg = fmt.Sprintf(
				"Repair cap (%d attempts) reached for slide(s) %v. "+
					"Table content does not fit and cannot be repaired further.",
				MaxRepairAttempts, cappedSlides,
			)
		}
		return LoopResult{
			Action:   action,
			CappedAt: cappedSlides,
			Message:  msg,
		}
	}

	// Some slides are repairable.
	msg := fmt.Sprintf("%d slide(s) have refuse-action cells — repair needed.", len(repairSlides))
	if len(cappedSlides) > 0 {
		msg += fmt.Sprintf(" Slide(s) %v hit repair cap — force split.", cappedSlides)
	}

	return LoopResult{
		Action:   ActionRepair,
		Slides:   repairSlides,
		CappedAt: cappedSlides,
		Message:  msg,
	}
}

// groupBySlide groups findings by slide index, extracted from the path field.
// Path format: "slides[N].content[M]..." or "slides[N].shape_grid..."
func groupBySlide(findings []fitFinding) map[int]SlideFinding {
	grouped := make(map[int]SlideFinding)

	for _, f := range findings {
		idx := parseSlideIndex(f.Path)
		sf := grouped[idx]
		sf.SlideIndex = idx
		sf.Findings = append(sf.Findings, f)
		grouped[idx] = sf
	}

	// Generate summaries.
	for idx, sf := range grouped {
		refused := 0
		for _, f := range sf.Findings {
			if f.Action == "refuse" {
				refused++
			}
		}
		sf.Summary = fmt.Sprintf(
			"Slide %d: %d finding(s), %d refused",
			idx, len(sf.Findings), refused,
		)
		grouped[idx] = sf
	}

	return grouped
}

// parseSlideIndex extracts the slide index from a JSON Pointer path like "/slides/3/content/0".
func parseSlideIndex(path string) int {
	const prefix = "/slides/"
	if !strings.HasPrefix(path, prefix) {
		return -1
	}
	rest := path[len(prefix):]
	end := strings.IndexByte(rest, '/')
	numStr := rest
	if end >= 0 {
		numStr = rest[:end]
	}
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
		return -1
	}
	return n
}
