package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// slideXMLParts returns every slide part of a generated deck by name.
func slideXMLParts(t *testing.T, pptxPath string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	parts := map[string]string{}
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/slide") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[f.Name] = string(data)
	}
	return parts
}

// go-slide-creator-8jp05: generation expands on a copy. One parsed input
// generated twice (a preview and then a generate, a retry) yields the same
// slide XML both times and is left as it was parsed — the nested pattern is
// still a pattern, not the grid the first pass expanded it to.
func TestGenerateTwiceFromOneInputIsIdentical(t *testing.T) {
	input := parseExpansionParityDeck(t)
	input.Template = "midnight-blue"
	input.OutputFilename = "twice.pptx"
	pristine := parseExpansionParityDeck(t)
	pristine.Template, pristine.OutputFilename = input.Template, input.OutputFilename

	var runs []map[string]string
	for range 2 {
		result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
			OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
			AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
		})
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, slideXMLParts(t, result.OutputPath))
	}
	if len(runs[0]) != len(input.Slides) {
		t.Fatalf("first pass wrote %d slide parts for %d slides", len(runs[0]), len(input.Slides))
	}
	for name, first := range runs[0] {
		if second := runs[1][name]; second != first {
			t.Errorf("%s differs between the first and the second pass over the same input", name)
		}
	}

	nested := input.Slides[4].ShapeGrid.Rows[0].Cells[1]
	if len(nested.Pattern) == 0 || nested.Grid != nil {
		t.Errorf("generation rewrote the authored nested pattern cell: pattern %q, grid %v", nested.Pattern, nested.Grid != nil)
	}
	if !reflect.DeepEqual(input, pristine) {
		got, _ := json.Marshal(input)
		want, _ := json.Marshal(pristine)
		t.Errorf("generation changed its input:\n got %s\nwant %s", got, want)
	}
}

// The copy shares what expansion does not touch and replaces what it does.
func TestNestedPatternExpansionCopyLeavesSourceGrid(t *testing.T) {
	var grid ShapeGridInput
	if err := json.Unmarshal([]byte(`{"columns": 2, "rows": [{"cells": [
		{"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Context"}}},
		{"grid": {"columns": 1, "rows": [{"cells": [
			{"pattern": {"name": "kpi-2up", "values": [{"big": "41%", "small": "Margin"}, {"big": "7", "small": "Markets"}]}}]}]}}
	]}]}`), &grid); err != nil {
		t.Fatal(err)
	}
	grid.BoundsRelativeToContentArea = true // not serialised: a JSON round trip drops it
	before, _ := json.Marshal(&grid)

	copied := nestedPatternExpansionCopy(&grid)
	if copied == &grid || !copied.BoundsRelativeToContentArea {
		t.Fatalf("copy is the source grid (%v) or lost an unserialised field", copied == &grid)
	}
	if copied.Rows[0].Cells[0] != grid.Rows[0].Cells[0] {
		t.Error("a cell with nothing to expand was copied; it should be shared")
	}
	if err := expandNestedCellPatterns(copied, patterns.ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}, patterns.Default()); err != nil {
		t.Fatal(err)
	}
	if inner := copied.Rows[0].Cells[1].Grid.Rows[0].Cells[0]; inner.Grid == nil || len(inner.Pattern) != 0 {
		t.Fatal("the copy's nested pattern was not expanded")
	}
	if after, _ := json.Marshal(&grid); !bytes.Equal(before, after) {
		t.Errorf("expanding the copy rewrote the source grid:\n got %s\nwant %s", after, before)
	}
	plain := &ShapeGridInput{Columns: grid.Columns}
	if nestedPatternExpansionCopy(plain) != plain {
		t.Error("a grid without a nested pattern should be returned as is")
	}
}
