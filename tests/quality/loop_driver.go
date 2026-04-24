// Package quality provides the repair loop driver for agent-driven
// author→validate→fix cycles. The driver wraps json2pptx validate
// --fit-report and enforces an anti-thrash cap of MaxRepairAttempts
// per slide before forcing a split_slide envelope or failing loudly.
package quality

import (
	"bufio"
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
)

// LoopConfig controls loop driver behavior.
type LoopConfig struct {
	// Binary is the path to the json2pptx binary.
	Binary string
	// ForceSplitOnCap controls whether the driver suggests force_split
	// (true) or returns an error (false) when the repair cap is reached.
	ForceSplitOnCap bool
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
	Attempts map[int]int // slide index → attempt count
}

// NewLoopState creates a fresh loop state.
func NewLoopState() *LoopState {
	return &LoopState{Attempts: make(map[int]int)}
}

// RunValidatePass executes json2pptx validate --fit-report on the given
// JSON file and returns structured findings grouped by slide.
func RunValidatePass(cfg LoopConfig, jsonPath string) ([]fitFinding, error) {
	cmd := exec.Command(cfg.Binary, "validate", "-fit-report", jsonPath) //nolint:gosec // controlled inputs in test/agent context
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// validate may return non-zero for invalid inputs — that's expected.
	_ = cmd.Run()

	var findings []fitFinding
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var f fitFinding
		if err := json.Unmarshal([]byte(line), &f); err != nil {
			continue // skip malformed lines
		}
		findings = append(findings, f)
	}

	return findings, scanner.Err()
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

	// All failing slides are capped → force split or fail.
	if len(repairSlides) == 0 && len(cappedSlides) > 0 {
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

// parseSlideIndex extracts the slide index from a path like "slides[3].content[0]".
func parseSlideIndex(path string) int {
	// Find "slides[" and extract the number.
	const prefix = "slides["
	idx := strings.Index(path, prefix)
	if idx < 0 {
		return -1
	}
	rest := path[idx+len(prefix):]
	end := strings.Index(rest, "]")
	if end < 0 {
		return -1
	}
	var n int
	if _, err := fmt.Sscanf(rest[:end], "%d", &n); err != nil {
		return -1
	}
	return n
}
