package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoRepairFinalRefusesDroppedContent(t *testing.T) {
	mc := repairMC(t)
	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(`{"template":"midnight-blue","slides":[{"slide_type":"content","content":[{"placeholder_id":"title","type":"text","text_value":"One accountable owner per case"},{"placeholder_id":"body","type":"bullets","bullets_value":["Assign the owner"]},{"placeholder_id":"body","type":"bullets","bullets_value":["Retain the reviewer"]}]}]}`), &input); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "repaired.pptx")
	_, err := mc.renderAutoRepairFinal(context.Background(), &input, filepath.Join("..", "..", "templates", "midnight-blue.pptx"), out)
	if err == nil || !strings.Contains(err.Error(), "CONTENT_DROPPED") || !strings.Contains(err.Error(), "/slides/0/content/2") {
		t.Fatalf("repair did not retain normal generation's content-loss refusal: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("refused repair left a publishable-looking artifact: %v", err)
	}
}
