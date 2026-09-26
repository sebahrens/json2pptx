package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

type blindReviewItem struct {
	ID               string `json:"id"`
	Sheet            string `json:"sheet,omitempty"`
	Prompt           string `json:"prompt"`
	GenerationFailed bool   `json:"generation_failed,omitempty"`
}

// Keep the operator report outside packet/. A fresh reviewer sees no original
// IDs, run/configuration names, historical scores, or model identity.
func prepareBlindReviewPacket(reportPath, out string) error {
	data, err := os.ReadFile(reportPath) // #nosec G304 -- operator-selected report
	if err != nil {
		return err
	}
	var report qualitybench.Report
	if err = json.Unmarshal(data, &report); err != nil {
		return err
	}
	if len(report.Evidence) == 0 {
		return fmt.Errorf("report has no evidence")
	}
	if err = os.Mkdir(out, 0o700); err != nil {
		return fmt.Errorf("review directory must be new: %w", err)
	}
	packet := filepath.Join(out, "packet")
	if err = os.Mkdir(packet, 0o700); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(packet, "sheets"), 0o700); err != nil {
		return err
	}
	items := make([]blindReviewItem, 0, len(report.Evidence))
	mapping := map[string]string{}
	for i := range report.Evidence {
		ev := &report.Evidence[i]
		random := make([]byte, 16)
		if _, err = rand.Read(random); err != nil {
			return err
		}
		id := hex.EncodeToString(random)
		if _, exists := mapping[id]; exists {
			return fmt.Errorf("random blind ID collision")
		}
		mapping[id] = ev.BlindID
		ev.BlindID = id
		item := blindReviewItem{ID: id, Prompt: ev.Request.Brief.Prompt, GenerationFailed: ev.Error != ""}
		if ev.Error == "" {
			pixels, err := os.ReadFile(ev.ContactSheet) // #nosec G304 -- report-listed sheet
			if err != nil {
				return err
			}
			item.Sheet = "sheets/" + id + ".png"
			if err = os.WriteFile(filepath.Join(packet, item.Sheet), pixels, 0o600); err != nil {
				return err
			}
			ev.ContactSheet = filepath.Join(packet, item.Sheet)
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if err = writeJSON(filepath.Join(packet, "manifest.json"), items); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(out, "operator-report.json"), report); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(out, "operator-key.json"), mapping); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(packet, "RUBRIC.md"), []byte(blindReviewRubric), 0o600); err != nil {
		return err
	}
	fmt.Printf("Prepared %d newly anonymized items at %s. Give reviewer only packet/. Operator files remain private.\n", len(items), packet)
	return nil
}

const blindReviewRubric = `# Blind deck-quality review

Inspect every available contact sheet visually and use only its brief and
opaque ID. Do not inspect files outside this packet or infer configuration
identity. No original run names, model identities, or configuration key are
provided. Report your actual reviewer/model identity and reviewer_type=llm
for AI review (human for a person). Never label AI review as human review.

Rate each rendered deck on five integer scales, 1 poor / 2 weak / 3 acceptable
/ 4 strong / 5 excellent:
- readability: legibility, clipping, overflow, readable chart/diagram labels;
- hierarchy: clear headline, balanced layout, sensible visual emphasis;
- template_fidelity: consistent typography/colors/chrome within the deck;
- factual_completeness: brief coverage and internal consistency (no external
  truth source is provided; explicitly acknowledge this limitation);
- usability: readiness for real use with minimal editing.

Set lost_critical_fact and critical_template_defect to true/false. Explain
low scores and critical defects in a companion JSON keyed by opaque ID. Do
not assign visual scores to generation_failed items with no sheet; record
that they cannot be visually reviewed. They block release approval separately.

CSV header:
blind_id,reviewer,reviewer_type,readability,hierarchy,template_fidelity,factual_completeness,usability,lost_critical_fact,critical_template_defect

Use one row per visually reviewed item. Keep scores independent; no heuristics
or copied uniform scores in place of actual image inspection. Final output:
ratings.csv plus review.json (per-item reasons, inspected IDs, limitations).
`
