package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// go-slide-creator-n3j96: image_case callouts compile to anchored callout
// overlays on the pattern's picture. The render draws a label and a leader per
// callout whichever side the picture sits on and whether or not a caption
// nests it, and a target the cover crop hides reports OVERLAY_TARGET_CROPPED
// at the callout the author wrote.
func TestSemanticImageCaseCallouts(t *testing.T) {
	screenshot, err := filepath.Abs(screenshotPath)
	if err != nil {
		t.Fatal(err)
	}
	slide := func(extra string) string {
		return fmt.Sprintf(`{"kind":"image_case","title":"Settlement is 47 minutes behind","image":{"path":%q,"alt":"Queue health table"%s,"body":"Retries pile up behind one slow downstream call.","takeaway":"The fix is scoped to the Payments team."}`,
			screenshot, extra)
	}
	spec := `{"meta":{"title":"Callouts","template":"midnight-blue"},"slides":[` + strings.Join([]string{
		// Cover crop: the navigation rail (x 0.03) is trimmed away.
		slide(`},"callouts":[{"label":"Navigation rail","x":0.03,"y":0.5},{"label":"Delayed queue","x":560,"y":484,"units":"px"}]`),
		// Contain, picture on the right, under a caption: nothing is cropped.
		slide(`,"fit":"contain"},"image_side":"right","caption":"OpsConsole, 09:40","callouts":[{"label":"Navigation rail","x":0.03,"y":0.5},{"label":"Delayed queue","x":0.35,"y":0.54}]`),
	}, ",") + `]}`

	dir := t.TempDir()
	specPath := filepath.Join(dir, "deck.json")
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "deck.pptx")
	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"json2pptx", "render", "--spec", specPath, "--output", out, "--templates-dir", testTemplatesDir}
	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() { _ = runSemantic() })
	})

	var res struct {
		Diagnostics []struct {
			Code    string `json:"code"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("render output is not JSON: %v\n%s\n%s", err, stdout, stderr)
	}
	var cropped []string
	for _, d := range res.Diagnostics {
		if d.Code == "OVERLAY_TARGET_CROPPED" {
			cropped = append(cropped, d.Path)
			// go-slide-creator-vg73u: the author wrote a callout on a picture,
			// not "overlay 0" on a "shape_grid image cell".
			if strings.Contains(d.Message, "overlay") || strings.Contains(d.Message, "shape_grid") || !strings.Contains(d.Message, "this callout") {
				t.Errorf("the finding is worded for the compiled deck: %s", d.Message)
			}
		}
	}
	if len(cropped) != 1 || cropped[0] != "/slides/0/callouts/0" {
		t.Errorf("OVERLAY_TARGET_CROPPED at %v, want only /slides/0/callouts/0\n%s", cropped, stdout)
	}

	for slideNum, want := range map[int]struct{ labels, leaders int }{
		1: {labels: 2, leaders: 1}, // the cropped callout keeps its label, no leader
		2: {labels: 2, leaders: 2},
	} {
		xml := readZipEntry(t, out, fmt.Sprintf("ppt/slides/slide%d.xml", slideNum))
		if got := strings.Count(xml, `name="Overlay callout leader `); got != want.leaders {
			t.Errorf("slide %d: %d leaders, want %d", slideNum, got, want.leaders)
		}
		if got := strings.Count(xml, `name="Overlay callout `) - strings.Count(xml, `name="Overlay callout leader `); got != want.labels {
			t.Errorf("slide %d: %d labels, want %d", slideNum, got, want.labels)
		}
		for _, label := range []string{"Navigation rail", "Delayed queue"} {
			if !strings.Contains(xml, label) {
				t.Errorf("slide %d lost the callout label %q", slideNum, label)
			}
		}
	}
}
