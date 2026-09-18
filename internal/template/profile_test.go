package template

import (
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestBuildProfileBundledTemplate(t *testing.T) {
	path := filepath.Join("..", "..", "templates", "midnight-blue.pptx")
	r, err := OpenTemplate(path)
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer func() { _ = r.Close() }()

	p, err := BuildProfile(r)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if p.TemplateHash != r.Hash() || p.ParserVersion != ProfileParserVersion {
		t.Fatalf("profile identity = %q/%q", p.TemplateHash, p.ParserVersion)
	}
	if p.SlideWidth <= 0 || p.SlideHeight <= 0 || p.AspectRatio == "unknown" {
		t.Fatalf("invalid dimensions: %dx%d %s", p.SlideWidth, p.SlideHeight, p.AspectRatio)
	}
	for _, role := range []types.CanonicalLayoutType{
		types.CanonicalLayoutTitleSlide, types.CanonicalLayoutOneContent,
		types.CanonicalLayoutSectionDivider, types.CanonicalLayoutBlankTitle,
	} {
		if p.RoleBindings[role] == "" {
			t.Errorf("missing role binding %q", role)
		}
	}
	for _, l := range p.Layouts {
		if l.MasterPath == "" {
			t.Errorf("layout %s missing master path", l.ID)
		}
		if l.ThemePath == "" {
			t.Errorf("layout %s missing theme path", l.ID)
		}
	}

	// Cached profiles are defensive copies: callers cannot corrupt later reads.
	p.RoleBindings[types.CanonicalLayoutTitleSlide] = "corrupt"
	again, err := BuildProfile(r)
	if err != nil {
		t.Fatalf("BuildProfile cached: %v", err)
	}
	if again.RoleBindings[types.CanonicalLayoutTitleSlide] == "corrupt" {
		t.Fatal("cached profile leaked caller mutation")
	}
}

func TestAspectRatioUsesEMUWithoutUnitConversion(t *testing.T) {
	for _, tc := range []struct {
		w, h int64
		want string
	}{
		{12192000, 6858000, "16:9"},
		{9144000, 6858000, "4:3"},
		{20000000, 7000000, "2.857:1"},
	} {
		if got := aspectRatio(tc.w, tc.h); got != tc.want {
			t.Errorf("aspectRatio(%d,%d)=%q want %q", tc.w, tc.h, got, tc.want)
		}
	}
}
