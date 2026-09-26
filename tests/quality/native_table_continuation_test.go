package quality

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// authorNativeTableContinuations is a fixture-authoring alternative, not an
// automatic renderer repair or public pagination API. All native tables use
// the same row window so related columns stay together without Cartesian pages.
func authorNativeTableContinuations(source nativeProbe, budget int) ([]nativeProbe, error) {
	if source.Profile != "table-stress" || budget <= 0 {
		return nil, fmt.Errorf("continuations require a table-stress probe and positive row budget")
	}
	rowCount := -1
	markers := map[string]bool{}
	for _, item := range source.Slide.Content {
		if item.Type != generator.ContentTable {
			continue
		}
		table, ok := item.Value.(*types.TableSpec)
		if !ok || table == nil || len(table.Rows) == 0 || len(table.Merges) != 0 {
			return nil, fmt.Errorf("fixture table must be populated, typed and unmerged")
		}
		if rowCount >= 0 && rowCount != len(table.Rows) {
			return nil, fmt.Errorf("fixture tables must have aligned row counts")
		}
		rowCount = len(table.Rows)
		for _, row := range table.Rows {
			if len(row) == 0 || row[0].Content == "" || markers[row[0].Content] {
				return nil, fmt.Errorf("fixture row identifiers must be nonempty and unique")
			}
			markers[row[0].Content] = true
			for _, cell := range row {
				if cell.RowSpan > 1 || cell.ColSpan > 1 || cell.IsMerged {
					return nil, fmt.Errorf("merged fixture cells need explicit continuation authoring")
				}
			}
		}
	}
	if rowCount <= 0 {
		return nil, fmt.Errorf("fixture has no tables")
	}
	var pages []nativeProbe
	for start := 0; start < rowCount; start += budget {
		end := min(start+budget, rowCount)
		page := source
		page.Profile = "table-continuation"
		page.ID = fmt.Sprintf("%s-table-continuation-%02d", source.LayoutID, len(pages)+1)
		page.ExpectedText = nil
		for _, text := range source.ExpectedText {
			if !markers[text] {
				page.ExpectedText = append(page.ExpectedText, text)
			}
		}
		page.Slide.Content = append([]generator.ContentItem(nil), source.Slide.Content...)
		for i, item := range source.Slide.Content {
			if item.Type != generator.ContentTable {
				continue
			}
			table := *item.Value.(*types.TableSpec)
			table.Rows = make([][]types.TableCell, end-start)
			for r, row := range item.Value.(*types.TableSpec).Rows[start:end] {
				table.Rows[r] = append([]types.TableCell(nil), row...)
				for _, cell := range row {
					if cell.Content != "" {
						page.ExpectedText = append(page.ExpectedText, cell.Content)
					}
				}
			}
			page.Slide.Content[i].Value = &table
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func TestNativeTableContinuationsPreserveEverySourceCellAndLayout(t *testing.T) {
	count := 0
	for _, path := range nativeCorpusPaths(t) {
		r, err := template.OpenTemplate(path)
		if err != nil {
			t.Fatal(err)
		}
		layouts, err := template.ParseLayouts(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, layout := range layouts {
			for _, source := range makeNativeProbes(layout, nativeReferenceImage()) {
				if source.Profile != "table-stress" {
					continue
				}
				count++
				before, err := json.Marshal(source)
				if err != nil {
					t.Fatal(err)
				}
				pages, err := authorNativeTableContinuations(source, 6)
				if err != nil || len(pages) != 4 {
					t.Fatalf("%s/%s: pages=%d err=%v", path, layout.ID, len(pages), err)
				}
				for i, item := range source.Slide.Content {
					if item.Type != generator.ContentTable {
						for _, page := range pages {
							if !reflect.DeepEqual(page.Slide.Content[i], item) {
								t.Fatal("non-table source changed")
							}
						}
						continue
					}
					original := item.Value.(*types.TableSpec)
					var joined [][]types.TableCell
					for _, page := range pages {
						if page.Slide.LayoutID != source.Slide.LayoutID || page.ExpectedTables != source.ExpectedTables {
							t.Fatal("layout/table inventory changed")
						}
						table := page.Slide.Content[i].Value.(*types.TableSpec)
						want := *original
						want.Rows = table.Rows
						if !reflect.DeepEqual(*table, want) {
							t.Fatal("table header/style/metadata changed")
						}
						joined = append(joined, table.Rows...)
					}
					if !reflect.DeepEqual(joined, original.Rows) {
						t.Fatal("source rows/cells omitted, repeated or reordered")
					}
				}
				after, err := json.Marshal(source)
				if err != nil || string(before) != string(after) {
					t.Fatal("source fixture mutated")
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("no native stress layouts covered")
	}
	t.Logf("verified coordinated continuation authoring for %d native stress layouts", count)
}

func TestNativeTableContinuationsRejectUnsupportedFixturesAtomically(t *testing.T) {
	for _, kind := range []string{"wrong-profile", "zero-budget", "no-tables", "untyped", "nil-table", "empty", "unequal", "duplicate", "empty-row", "empty-id", "merged", "column-span", "merge-region", "merge-continuation"} {
		t.Run(kind, func(t *testing.T) {
			source := nativeProbe{Profile: "table-stress", LayoutID: "native", Slide: generator.SlideSpec{LayoutID: "native"}}
			left := &types.TableSpec{Headers: []string{"Case"}, Rows: [][]types.TableCell{{{Content: "L0"}}, {{Content: "L1"}}, {{Content: "L2"}}}}
			source.Slide.Content = []generator.ContentItem{{Type: generator.ContentTable, Value: left}}
			budget := 2
			switch kind {
			case "wrong-profile":
				source.Profile = "representative"
			case "zero-budget":
				budget = 0
			case "no-tables":
				source.Slide.Content = nil
			case "untyped":
				source.Slide.Content[0].Value = "not a table"
			case "nil-table":
				source.Slide.Content[0].Value = (*types.TableSpec)(nil)
			case "empty":
				left.Rows = nil
			case "unequal":
				source.Slide.Content = append(source.Slide.Content, generator.ContentItem{Type: generator.ContentTable, Value: &types.TableSpec{Rows: [][]types.TableCell{{{Content: "R0"}}}}})
			case "duplicate":
				left.Rows[1][0].Content = "L0"
			case "empty-row":
				left.Rows[1] = nil
			case "empty-id":
				left.Rows[1][0].Content = ""
			case "merged":
				left.Rows[1][0].RowSpan = 2
			case "column-span":
				left.Rows[1][0].ColSpan = 2
			case "merge-region":
				left.Merges = []types.CellMerge{{StartRow: 0, EndRow: 1}}
			case "merge-continuation":
				left.Rows[1][0].IsMerged = true
			}
			before, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			pages, err := authorNativeTableContinuations(source, budget)
			if err == nil || pages != nil {
				t.Fatal("unsupported fixture accepted")
			}
			after, err := json.Marshal(source)
			if err != nil || string(before) != string(after) {
				t.Fatal("rejected authoring mutated source")
			}
		})
	}
}

func TestNativeTableContinuationsPartialFinalPage(t *testing.T) {
	table := &types.TableSpec{Headers: []string{"Case"}, Rows: [][]types.TableCell{{{Content: "R0"}}, {{Content: "R1"}}, {{Content: "R2"}}}}
	source := nativeProbe{Profile: "table-stress", LayoutID: "native", ExpectedText: []string{"Title", "R0", "R1", "R2"}, Slide: generator.SlideSpec{LayoutID: "native", Content: []generator.ContentItem{{Type: generator.ContentTable, Value: table}}}}
	pages, err := authorNativeTableContinuations(source, 2)
	if err != nil || len(pages) != 2 {
		t.Fatalf("partial page lost: pages=%d err=%v", len(pages), err)
	}
	if !reflect.DeepEqual(pages[0].ExpectedText, []string{"Title", "R0", "R1"}) || !reflect.DeepEqual(pages[1].ExpectedText, []string{"Title", "R2"}) {
		t.Fatal("page-specific expected source content is wrong")
	}
	pages[1].Slide.Content[0].Value.(*types.TableSpec).Rows[0][0].Content = "Edited"
	if table.Rows[2][0].Content != "R2" {
		t.Fatal("continuation row edit mutated source")
	}
}
