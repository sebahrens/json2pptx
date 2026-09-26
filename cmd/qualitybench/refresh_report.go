package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

// This is an engine replay, not a new agent run: authoring inputs, model,
// configuration, and opaque IDs remain unchanged. Original evidence is never
// overwritten. The audit records both input and rendered-pixel fingerprints.
type refreshItem struct {
	RunID       string `json:"run_id"`
	BlindID     string `json:"blind_id"`
	InputSHA256 string `json:"input_sha256"`
	OldPixels   string `json:"old_pixels_sha256"`
	NewPixels   string `json:"new_pixels_sha256"`
	Changed     bool   `json:"changed"`
	Error       string `json:"error,omitempty"`
}

func sheetPixels(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- operator report/sheet path
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	b := img.Bounds()
	_, _ = fmt.Fprintf(h, "%d,%d\x00", b.Dx(), b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			_, _ = h.Write([]byte{byte(r >> 8), byte(g >> 8), byte(bl >> 8), byte(a >> 8)})
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type frozenInput struct {
	body                       []byte
	source, path, templatesDir string
	templateOverride           string
}

func prepareFrozenInputs(report *qualitybench.Report, templatesDir string) ([]frozenInput, error) {
	inputs := make([]frozenInput, len(report.Evidence))
	seen := map[string]bool{}
	validID := regexpBlindID()
	for i, ev := range report.Evidence {
		if ev.Error != "" || !validID.MatchString(ev.BlindID) || !validID.MatchString(ev.Request.Template.Name) || seen[ev.BlindID] || filepath.Base(ev.Request.RunID) != ev.Request.RunID || ev.Request.RunID == "." || ev.Request.RunID == "" {
			return nil, fmt.Errorf("run %d lacks safe, unique, renderable evidence", i)
		}
		seen[ev.BlindID] = true
		for _, p := range ev.Result.Artifacts {
			if strings.EqualFold(filepath.Ext(p), ".pptx") {
				inputs[i].source = p
			}
			if strings.EqualFold(filepath.Ext(p), ".json") {
				inputs[i].path = p
			}
		}
		if inputs[i].source == "" {
			return nil, fmt.Errorf("run %s has no PPTX", ev.Request.RunID)
		}
		if inputs[i].path == "" {
			inputs[i].path = strings.TrimSuffix(inputs[i].source, filepath.Ext(inputs[i].source)) + ".json"
		}
		body, err := os.ReadFile(inputs[i].path) // #nosec G304 -- source paired with operator report artifact
		if err != nil {
			return nil, fmt.Errorf("frozen input %s: %w", inputs[i].path, err)
		}
		inputs[i].body = body
		var selector struct {
			Template     string `json:"template"`
			TemplatePath string `json:"template_path"`
		}
		if err := json.Unmarshal(body, &selector); err != nil {
			return nil, fmt.Errorf("frozen input %s: %w", inputs[i].path, err)
		}
		if selector.Template == "" && selector.TemplatePath == "" {
			inputs[i].templateOverride = ev.Request.Template.Name
		}
		if _, err := sheetPixels(ev.ContactSheet); err != nil {
			return nil, fmt.Errorf("original sheet %s: %w", ev.BlindID, err)
		}
		inputs[i].templatesDir = templatesDir
		bundleTemplates := filepath.Join(filepath.Dir(filepath.Dir(inputs[i].source)), "templates")
		if _, statErr := os.Stat(filepath.Join(bundleTemplates, ev.Request.Template.Name+".pptx")); statErr == nil {
			inputs[i].templatesDir = bundleTemplates
		}
	}
	return inputs, nil
}

func refreshContactSheets(reportPath, out, templatesDir, bin string) error {
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
	inputs, err := prepareFrozenInputs(&report, templatesDir)
	if err != nil {
		return err
	}
	if err = os.Mkdir(out, 0o755); err != nil {
		return fmt.Errorf("-out must be a new directory (originals are preserved): %w", err)
	}
	for _, sub := range []string{"decks", "sheets"} {
		if err = os.Mkdir(filepath.Join(out, sub), 0o755); err != nil {
			return err
		}
	}
	var items []refreshItem
	changed := 0
	failed := 0
	for i := range report.Evidence {
		ev := &report.Evidence[i]
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		r := &qualitybench.Renderer{JSON2PPTX: bin, TemplatesDir: inputs[i].templatesDir, TemplateName: inputs[i].templateOverride}
		if err = r.Resolve(); err != nil {
			cancel()
			return err
		}
		deck := filepath.Join(out, "decks", ev.Request.RunID+".pptx")
		inputPath := inputs[i].path
		err = r.GenerateFromFile(ctx, inputPath, deck)
		var slides []string
		if err == nil {
			slides, err = r.Rasterize(ctx, deck, filepath.Join(out, "renders", ev.Request.RunID))
		}
		cancel()
		if err != nil {
			sum := sha256.Sum256(inputs[i].body)
			items = append(items, refreshItem{RunID: ev.Request.RunID, BlindID: ev.BlindID, InputSHA256: hex.EncodeToString(sum[:]), Error: err.Error()})
			ev.Error = "current-engine replay: " + err.Error()
			ev.ContactSheet = ""
			failed++
			fmt.Printf("refresh %d/%d: failed: %s\n", i+1, len(report.Evidence), err)
			continue
		}
		sheet := filepath.Join(out, "sheets", ev.BlindID+".png")
		if err = qualitybench.ContactSheet(slides, sheet, 3, 480); err != nil {
			return err
		}
		oldPixels, err := sheetPixels(ev.ContactSheet)
		if err != nil {
			return err
		}
		newPixels, err := sheetPixels(sheet)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(inputs[i].body)
		item := refreshItem{RunID: ev.Request.RunID, BlindID: ev.BlindID, InputSHA256: hex.EncodeToString(sum[:]), OldPixels: oldPixels, NewPixels: newPixels, Changed: oldPixels != newPixels}
		items = append(items, item)
		if item.Changed {
			changed++
		}
		ev.ContactSheet, ev.SlideCount, ev.Result.Artifacts = sheet, len(slides), []string{deck, inputPath}
		fmt.Printf("refresh %d/%d: changed=%t\n", i+1, len(report.Evidence), item.Changed)
	}
	report.Ratings = nil
	report.Summary = qualitybench.Summary{Runs: len(report.Evidence), FailedRuns: failed, ReleaseDecision: "inconclusive: frozen agent inputs replayed on current engine; one blind review required"}
	if err = writeJSON(filepath.Join(out, "report.json"), &report); err != nil {
		return err
	}
	sha, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("record engine commit: %w", err)
	}
	audit := struct {
		SourceReport string        `json:"source_report"`
		EngineCommit string        `json:"engine_commit"`
		RefreshedAt  time.Time     `json:"refreshed_at"`
		Changed      int           `json:"changed_sheets"`
		Failed       int           `json:"failed_runs"`
		Items        []refreshItem `json:"items"`
	}{reportPath, strings.TrimSpace(string(sha)), time.Now().UTC(), changed, failed, items}
	if err = writeJSON(filepath.Join(out, "refresh_audit.json"), audit); err != nil {
		return err
	}
	if err = writeRefreshRatings(out, items); err != nil {
		return err
	}
	fmt.Printf("Current-engine replay: %d/%d sheets changed; %d failures. Historical bundle preserved. Review: %s\n", changed, len(items)-failed, failed, out)
	if failed > 0 {
		return fmt.Errorf("%d frozen inputs could not replay; see %s (do not approve a release with incomplete evidence)", failed, filepath.Join(out, "refresh_audit.json"))
	}
	return nil
}

func writeRefreshRatings(out string, items []refreshItem) error {
	f, err := os.Create(filepath.Join(out, "ratings_template.csv")) // #nosec G304 -- new operator output bundle
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	// Opaque ID order avoids grouping all baseline decks before redesigned decks.
	sortedItems := append([]refreshItem(nil), items...)
	sort.Slice(sortedItems, func(i, j int) bool { return sortedItems[i].BlindID < sortedItems[j].BlindID })
	if err = w.Write([]string{"blind_id", "reviewer", "reviewer_type", "readability", "hierarchy", "template_fidelity", "factual_completeness", "usability", "lost_critical_fact", "critical_template_defect"}); err == nil {
		for _, item := range sortedItems {
			if item.Error != "" {
				continue
			}
			if err = w.Write([]string{item.BlindID, "", "human", "", "", "", "", "", "", ""}); err != nil {
				break
			}
		}
	}
	w.Flush()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if w.Error() != nil {
		return w.Error()
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}
