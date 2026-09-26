package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

func reviewFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sheets"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{"ratings_template.csv": "blind_id,reviewer\na,\nb,\n", "sheets/a.png": "png-a", "sheets/b.png": "png-b", "blind_key.json": "secret configuration"} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReviewHandlerBlindAllowlist(t *testing.T) {
	h, err := newReviewHandler(reviewFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, host, method string
		status             int
	}{{"/", "127.0.0.1:8765", "GET", 200}, {"/manifest", "localhost:8765", "GET", 200}, {"/sheet/a", "127.0.0.1", "GET", 200}, {"/model.js", "[::1]:8765", "GET", 200}, {"/blind_key.json", "127.0.0.1", "GET", 404}, {"/sheet/../blind_key.json", "127.0.0.1", "GET", 404}, {"/sheet/unknown", "127.0.0.1", "GET", 404}, {"/", "evil.example", "GET", 403}, {"/", "127.0.0.1", "POST", 405}} {
		t.Run(tc.path+tc.host+tc.method, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d want %d", w.Code, tc.status)
			}
			if strings.Contains(w.Body.String(), "secret configuration") {
				t.Fatal("unblinding leak")
			}
		})
	}
}

func TestReviewManifestChangesWithPixels(t *testing.T) {
	dir := reviewFixture(t)
	manifest := func() map[string]any {
		h, err := newReviewHandler(dir)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/manifest", nil))
		var data map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		return data
	}
	a := manifest()
	if len(a["ids"].([]any)) != 2 {
		t.Fatal("wrong sheets")
	}
	if err := os.WriteFile(filepath.Join(dir, "sheets/a.png"), []byte("changed pixels"), 0o600); err != nil {
		t.Fatal(err)
	}
	if a["dataset"] == manifest()["dataset"] {
		t.Fatal("new pixels must invalidate old browser progress")
	}
}

func TestReviewBundleRejectsBrokenOrEscapingSheets(t *testing.T) {
	for _, kind := range []string{"duplicate", "invalid", "missing", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := reviewFixture(t)
			switch kind {
			case "duplicate":
				if err := os.WriteFile(filepath.Join(dir, "ratings_template.csv"), []byte("blind_id,reviewer\na,\na,\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "invalid":
				if err := os.WriteFile(filepath.Join(dir, "ratings_template.csv"), []byte("blind_id,reviewer\n../x,\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(filepath.Join(dir, "sheets/a.png")); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(filepath.Join(dir, "sheets/a.png")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "blind_key.json"), filepath.Join(dir, "sheets/a.png")); err != nil {
					t.Skip(err)
				}
			}
			if _, err := newReviewHandler(dir); err == nil {
				t.Fatal("invalid bundle accepted")
			}
		})
	}
}

func TestReviewBrowserModel(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node required for browser-model tests")
	}
	cmd := exec.Command(node, "--test", "review-ui/model.test.js") // #nosec G204 -- fixed local test script
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("browser model: %v\n%s", err, out)
	}
}

func TestReviewCSVCompatibleWithRatingIngestion(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node required for browser CSV contract")
	}
	cmd := exec.Command(node, "-e", `const m=require('./review-ui/model.js');const r=Object.fromEntries([...m.scores.map(k=>[k,4]),...m.defects.map(k=>[k,false])]);process.stdout.write(m.csv(['a'],'Seb, "one"',{a:r}));`) // #nosec G204 -- fixed local test
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ratings.csv")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	ratings, err := readRatingsCSV(path, map[string]string{"a": "run"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ratings) != 1 || ratings[0].Reviewer != `Seb, "one"` || ratings[0].ReviewerType != "human" || ratings[0].Usability != 4 || ratings[0].LostCriticalFact {
		t.Fatalf("CSV contract mismatch: %+v", ratings)
	}
}

func TestReviewPairingDoesNotRevealConfiguration(t *testing.T) {
	dir := reviewFixture(t)
	reportPath := filepath.Join(dir, "report.json")
	report := qualitybench.Report{Evidence: []qualitybench.Evidence{{BlindID: "a", Request: qualitybench.Request{RunID: "secret-baseline", Configuration: "baseline", Brief: qualitybench.Brief{ID: "brief", Prompt: "Make a KPI deck"}, Template: qualitybench.Template{Name: "private"}, Repetition: 1}}, {BlindID: "b", Request: qualitybench.Request{RunID: "secret-redesigned", Configuration: "redesigned", Brief: qualitybench.Brief{ID: "brief", Prompt: "Make a KPI deck"}, Template: qualitybench.Template{Name: "private"}, Repetition: 1}}}}
	if err := writeJSON(reportPath, report); err != nil {
		t.Fatal(err)
	}
	h, err := newReviewHandlerWithReport(dir, reportPath)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/manifest", nil))
	for _, secret := range []string{"baseline", "redesigned", "private", "secret"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("metadata leaked %s", secret)
		}
	}
	var manifest struct {
		Pairs   map[string]string
		Prompts map[string]string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Pairs["a"] != "b" || manifest.Pairs["b"] != "a" || manifest.Prompts["a"] != "Make a KPI deck" {
		t.Fatalf("wrong paired context: %+v", manifest)
	}
	report.Evidence[1].Request.Repetition = 2
	if err := writeJSON(reportPath, report); err != nil {
		t.Fatal(err)
	}
	h, err = newReviewHandlerWithReport(dir, reportPath)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/manifest", nil))
	if strings.Contains(w.Body.String(), `"a":"b"`) {
		t.Fatal("paired different repetitions")
	}
}

func TestSheetPixelsTrackActualRenderedContent(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, c color.Color) {
		t.Helper()
		img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		img.Set(0, 0, c)
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	write("a.png", color.Black)
	write("b.png", color.White)
	a, err := sheetPixels(filepath.Join(dir, "a.png"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := sheetPixels(filepath.Join(dir, "b.png"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("changed pixels were missed")
	}
	write("c.png", color.Black)
	c, err := sheetPixels(filepath.Join(dir, "c.png"))
	if err != nil {
		t.Fatal(err)
	}
	if a != c {
		t.Fatal("identical pixels changed identity")
	}
}

func TestRefreshRejectsIncompleteInputsBeforeCreatingOutput(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	out := filepath.Join(dir, "new-bundle")
	if err := writeJSON(report, qualitybench.Report{}); err != nil {
		t.Fatal(err)
	}
	if err := refreshContactSheets(report, out, "templates", "unused"); err == nil {
		t.Fatal("empty report accepted")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("failed preflight created output")
	}
}

func TestBlindPacketRekeysIDsAndHidesOperatorMetadata(t *testing.T) {
	dir := reviewFixture(t)
	source := filepath.Join(dir, "source.json")
	out := filepath.Join(dir, "blind")
	report := qualitybench.Report{Evidence: []qualitybench.Evidence{{BlindID: "a", ContactSheet: filepath.Join(dir, "sheets/a.png"), Request: qualitybench.Request{RunID: "secret-run", Configuration: "secret-configuration", Brief: qualitybench.Brief{Prompt: "Make a deck"}}, Result: qualitybench.AgentResult{Model: "secret-model"}}, {BlindID: "b", Error: "secret generation diagnostic", Request: qualitybench.Request{Brief: qualitybench.Brief{Prompt: "Make another deck"}}}}}
	if err := writeJSON(source, report); err != nil {
		t.Fatal(err)
	}
	if err := prepareBlindReviewPacket(source, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "packet/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret") {
		t.Fatal("operator metadata leaked")
	}
	var items []blindReviewItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatal("missing blind items")
	}
	for _, item := range items {
		if len(item.ID) != 32 || item.ID == "a" || item.ID == "b" {
			t.Fatal("ID not reblinded")
		}
		if item.GenerationFailed {
			if item.Sheet != "" {
				t.Fatal("failed run pretends to have pixels")
			}
		} else {
			if _, err := os.Stat(filepath.Join(out, "packet", item.Sheet)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(out, "packet/operator-key.json")); !os.IsNotExist(err) {
		t.Fatal("key included in reviewer packet")
	}
}
