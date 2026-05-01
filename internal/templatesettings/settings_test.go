package templatesettings

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	f, err := Load(dir, "no-such-template")
	if err != nil {
		t.Fatalf("Load non-existent: %v", err)
	}
	if f.Template != "no-such-template" {
		t.Errorf("Template = %q, want %q", f.Template, "no-such-template")
	}
	if len(f.TableStyles) != 0 || len(f.CellStyles) != 0 {
		t.Errorf("expected empty maps, got table_styles=%d cell_styles=%d", len(f.TableStyles), len(f.CellStyles))
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	banded := true
	f := &File{
		Template: "midnight-blue",
		TableStyles: map[string]TableStyleDef{
			"brand-table": {
				StyleID:       "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}",
				UseTableStyle: true,
				BandedRows:    &banded,
			},
		},
		CellStyles: map[string]CellStyleDef{
			"callout": {
				Fill:      &CellStyleFill{Color: "accent1", Alpha: 0.15},
				TextAlign: "center",
			},
		},
	}

	path, err := Save(dir, f)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if filepath.Ext(path) != ".yaml" {
		t.Errorf("path extension = %q, want .yaml", filepath.Ext(path))
	}

	loaded, err := Load(dir, "midnight-blue")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.TableStyles) != 1 {
		t.Fatalf("table_styles count = %d, want 1", len(loaded.TableStyles))
	}
	ts := loaded.TableStyles["brand-table"]
	if ts.StyleID != "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" {
		t.Errorf("style_id = %q", ts.StyleID)
	}
	if !ts.UseTableStyle {
		t.Error("use_table_style should be true")
	}

	cs := loaded.CellStyles["callout"]
	if cs.Fill == nil || cs.Fill.Color != "accent1" {
		t.Errorf("cell style fill = %+v", cs.Fill)
	}
	if cs.TextAlign != "center" {
		t.Errorf("text_align = %q", cs.TextAlign)
	}
}

func TestDeleteSetting(t *testing.T) {
	dir := t.TempDir()
	f := &File{
		Template: "test",
		TableStyles: map[string]TableStyleDef{
			"a": {StyleID: "1"},
			"b": {StyleID: "2"},
		},
		CellStyles: make(map[string]CellStyleDef),
	}
	if _, err := Save(dir, f); err != nil {
		t.Fatal(err)
	}

	// Delete one — file should remain.
	removed, err := Delete(dir, "test", KindTableStyle, "a")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected removed=true")
	}
	loaded, _ := Load(dir, "test")
	if len(loaded.TableStyles) != 1 {
		t.Errorf("expected 1 table style, got %d", len(loaded.TableStyles))
	}

	// Delete last — file should be removed.
	removed, err = Delete(dir, "test", KindTableStyle, "b")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected removed=true")
	}
	if _, err := os.Stat(filepath.Join(dir, "test.settings.yaml")); !os.IsNotExist(err) {
		t.Error("expected settings file to be removed when empty")
	}

	// Delete non-existent.
	removed, err = Delete(dir, "test", KindTableStyle, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("expected removed=false for non-existent")
	}
}

func TestValidateName(t *testing.T) {
	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{"brand-table", true},
		{"my_style_1", true},
		{"A", true},
		{"", false},
		{"-leading-dash", false},
		{"has spaces", false},
		{"has/slash", false},
	} {
		err := ValidateName(tc.name)
		if (err == nil) != tc.ok {
			t.Errorf("ValidateName(%q) = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

func TestValidateTemplateName(t *testing.T) {
	if err := ValidateTemplateName("midnight-blue"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateTemplateName("../escape"); err == nil {
		t.Error("expected error for path traversal")
	}
	if err := ValidateTemplateName(""); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateKind(t *testing.T) {
	k, err := ValidateKind("table_styles")
	if err != nil || k != KindTableStyle {
		t.Errorf("table_styles: kind=%v err=%v", k, err)
	}
	k, err = ValidateKind("cell_styles")
	if err != nil || k != KindCellStyle {
		t.Errorf("cell_styles: kind=%v err=%v", k, err)
	}
	_, err = ValidateKind("unknown")
	if err == nil {
		t.Error("expected error for unknown kind")
	}
}

func TestConcurrentSave(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			f := &File{
				Template:    "concurrent",
				TableStyles: map[string]TableStyleDef{"s": {StyleID: "{TEST}"}},
				CellStyles:  make(map[string]CellStyleDef),
			}
			if _, err := Save(dir, f); err != nil {
				t.Errorf("concurrent Save %d: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	// File should be valid after concurrent writes.
	loaded, err := Load(dir, "concurrent")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.TableStyles) != 1 {
		t.Errorf("expected 1 table style after concurrent writes, got %d", len(loaded.TableStyles))
	}
}
