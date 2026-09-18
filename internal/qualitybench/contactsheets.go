package qualitybench

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DryRun renders every brief on every template with the deterministic
// reference decks (no agent, no provider call) and produces blind contact
// sheets plus a results JSON. It is the harness smoke test the real agent
// benchmark reuses for its rating step.
type DryRun struct {
	Renderer   *Renderer
	Briefs     []Brief
	Templates  []Template
	OutputDir  string // decks, renders, contact sheets, blind key
	ResultsDir string // results JSON (tests/quality/results)
}

// Run executes the dry run. Per-run failures are recorded on the evidence
// (Error) instead of aborting, so one broken template does not hide the rest.
func (d DryRun) Run(ctx context.Context) (*Report, string, error) {
	if len(d.Briefs) < 12 {
		return nil, "", fmt.Errorf("at least 12 briefs are required, got %d", len(d.Briefs))
	}
	if len(d.Templates) < 3 {
		return nil, "", fmt.Errorf("at least 3 templates are required, got %d", len(d.Templates))
	}
	if err := d.Renderer.Resolve(); err != nil {
		return nil, "", err
	}
	for _, dir := range []string{d.OutputDir, d.ResultsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, "", err
		}
	}
	report := &Report{GeneratedAt: time.Now().UTC(), Evidence: []Evidence{}}
	for _, b := range d.Briefs {
		for _, t := range d.Templates {
			req := Request{RunID: fmt.Sprintf("%s-%s-%s-1", ReferenceConfiguration, b.ID, t.Name), Configuration: ReferenceConfiguration, Brief: b, Template: t, Repetition: 1}
			report.Evidence = append(report.Evidence, d.one(ctx, req))
		}
	}
	report.Summary.Runs = len(report.Evidence)
	report.Summary.ReleaseDecision = "inconclusive: dry-run reference decks only; agent runs plus two blind reviewer ratings are required"
	if err := AttachBlindContactSheets(ctx, report, d.Renderer, d.OutputDir); err != nil {
		return nil, "", err
	}
	path := filepath.Join(d.ResultsDir, fmt.Sprintf("%s-%s.json", ReferenceConfiguration, report.GeneratedAt.Format("20060102T150405Z")))
	data, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil { // #nosec G306 -- shareable results file
		return nil, "", err
	}
	return report, path, nil
}

func (d DryRun) one(ctx context.Context, req Request) Evidence {
	started := time.Now().UTC()
	ev := Evidence{Request: req, StartedAt: started, Result: AgentResult{Model: "none (deterministic reference deck)", Version: ReferenceConfiguration, Prompt: req.Brief.Prompt, Iterations: 0}}
	defer func() { ev.DurationMS = time.Since(started).Milliseconds() }()
	deck, err := ReferenceDeck(req.Brief, req.Template.Name)
	if err != nil {
		ev.Error = err.Error()
		return ev
	}
	pptx := filepath.Join(d.OutputDir, "decks", req.RunID+".pptx")
	if err := os.MkdirAll(filepath.Dir(pptx), 0o755); err != nil {
		ev.Error = err.Error()
		return ev
	}
	if err := d.Renderer.Generate(ctx, deck, pptx); err != nil {
		ev.Error = err.Error()
		return ev
	}
	ev.Result.ToolCalls = []json.RawMessage{json.RawMessage(`{"tool":"json2pptx generate","reference_deck":true}`)}
	ev.Result.Artifacts = []string{pptx}
	return ev
}

// AttachBlindContactSheets renders every .pptx artifact in the report to a
// contact sheet named by an opaque blind ID (sheets/<blind_id>.png), records
// blind_id / contact_sheet on the evidence, and writes the reviewer bundle:
// blind_key.json (blind_id -> run, for un-blinding after rating — do not give
// to reviewers) and ratings_template.csv (blind IDs only, shuffled).
func AttachBlindContactSheets(ctx context.Context, report *Report, r *Renderer, outDir string) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	sheetsDir := filepath.Join(outDir, "sheets")
	if err := os.MkdirAll(sheetsDir, 0o755); err != nil {
		return err
	}
	key := map[string]string{}
	for i := range report.Evidence {
		ev := &report.Evidence[i]
		pptx := firstPPTX(ev.Result.Artifacts)
		if pptx == "" || ev.Error != "" {
			continue
		}
		sum := sha256.Sum256(append(salt, []byte(ev.Request.RunID)...))
		blind := hex.EncodeToString(sum[:])[:12]
		pngs, err := r.Rasterize(ctx, pptx, filepath.Join(outDir, "renders", ev.Request.RunID))
		if err != nil {
			ev.Error = "render: " + err.Error()
			continue
		}
		sheet := filepath.Join(sheetsDir, blind+".png")
		if err := ContactSheet(pngs, sheet, 3, 480); err != nil {
			ev.Error = "contact sheet: " + err.Error()
			continue
		}
		ev.BlindID, ev.ContactSheet, ev.SlideCount = blind, sheet, len(pngs)
		key[blind] = ev.Request.RunID
	}
	data, _ := json.MarshalIndent(key, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "blind_key.json"), append(data, '\n'), 0o600); err != nil {
		return err
	}
	return writeRatingsTemplate(filepath.Join(outDir, "ratings_template.csv"), key)
}

func firstPPTX(artifacts []string) string {
	for _, a := range artifacts {
		if strings.EqualFold(filepath.Ext(a), ".pptx") {
			return a
		}
	}
	return ""
}

// writeRatingsTemplate writes one row per blind ID (sorted by the opaque ID,
// which is effectively a shuffle) with the Rating columns left blank.
func writeRatingsTemplate(path string, key map[string]string) error {
	ids := make([]string, 0, len(key))
	for id := range key {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	f, err := os.Create(path) // #nosec G304 -- operator-chosen output dir
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"blind_id", "reviewer", "readability", "hierarchy", "template_fidelity", "factual_completeness", "usability", "lost_critical_fact", "critical_template_defect"})
	for _, id := range ids {
		_ = w.Write([]string{id, "", "", "", "", "", "", "", ""})
	}
	w.Flush()
	return w.Error()
}
