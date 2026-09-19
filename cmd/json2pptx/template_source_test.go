package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-ydbk. Bring-your-own onboarding was impossible without an
// operator: an absolute .pptx path in `template` came back
// TPL.TEMPLATE_NOT_FOUND listing the registered names it is not, `template_path`
// was an unknown key on every deck tool, and examine_template — the one tool
// that took a path — was outside the core profile. These tests pin the supported
// way in, and the containment rule that keeps it from widening the server's
// reach past the root the caller declared.

// byoTemplate copies a bundled template into a temp dir so the test has a
// template the server has NOT registered, addressable only by path.
func byoTemplate(t *testing.T) (dir, path string) {
	t.Helper()
	src, err := filepath.Abs("../../templates/midnight-blue.pptx")
	if err != nil {
		t.Fatalf("abs source template: %v", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source template: %v", err)
	}
	dir = t.TempDir()
	// EvalSymlinks: on macOS t.TempDir() is under /var, a symlink to /private/var,
	// and containment is checked against the resolved path.
	if resolved, rerr := filepath.EvalSymlinks(dir); rerr == nil {
		dir = resolved
	}
	path = filepath.Join(dir, "client-brand.pptx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write byo template: %v", err)
	}
	return dir, path
}

func byoRequest(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func byoDeck(t *testing.T, templateField, value string) map[string]any {
	t.Helper()
	deck := map[string]any{
		"slides": []any{map[string]any{
			"slide_type": "title",
			"content": []any{map[string]any{
				"placeholder_id": "title", "type": "text", "text_value": "BYO",
			}},
		}},
	}
	if templateField != "" {
		deck[templateField] = value
	}
	return deck
}

func TestGenerateAcceptsTemplatePath(t *testing.T) {
	dir, path := byoTemplate(t)
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}

	result, err := mc.handleGenerate(t.Context(), byoRequest(map[string]any{
		"presentation":    byoDeck(t, "template_path", path),
		"base_dir":        dir,
		"output_filename": "byo.pptx",
	}))
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if result.IsError {
		t.Fatalf("generate with template_path failed: %s", textContent(result))
	}
	var out JSONOutput
	if jerr := json.Unmarshal([]byte(textContent(result)), &out); jerr != nil {
		t.Fatalf("parse result: %v", jerr)
	}
	if out.OutputPath == "" {
		t.Fatal("no output path")
	}
	if _, serr := os.Stat(out.OutputPath); serr != nil {
		t.Errorf("output not written: %v", serr)
	}
}

// The containment rule is the whole reason template_path is safe to expose to a
// model. Each case is a way out of the declared root.
func TestTemplatePathContainment(t *testing.T) {
	dir, path := byoTemplate(t)
	outsideDir := t.TempDir()
	if resolved, rerr := filepath.EvalSymlinks(outsideDir); rerr == nil {
		outsideDir = resolved
	}

	tests := []struct {
		name     string
		rawPath  string
		baseDir  string
		wantCode string
	}{
		{
			name:     "absolute path outside base_dir",
			rawPath:  path,
			baseDir:  outsideDir,
			wantCode: diagnostics.CodeInvalidPath,
		},
		{
			name:     "traversal out of base_dir",
			rawPath:  "../" + filepath.Base(dir) + "/client-brand.pptx",
			baseDir:  dir,
			wantCode: diagnostics.CodeInvalidPath,
		},
		{
			name:     "not a pptx",
			rawPath:  filepath.Join(dir, "notes.txt"),
			baseDir:  dir,
			wantCode: diagnostics.CodeInvalidParameter,
		},
		{
			name:     "missing file",
			rawPath:  filepath.Join(dir, "absent.pptx"),
			baseDir:  dir,
			wantCode: diagnostics.CodeFileNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, d := resolveGuardedTemplatePath("generate_presentation", "template_path", tt.rawPath, tt.baseDir)
			if d == nil {
				t.Fatalf("path %q was accepted inside base_dir %q (resolved %q)", tt.rawPath, tt.baseDir, got)
			}
			if d.Code != tt.wantCode {
				t.Errorf("code = %q, want %q (%s)", d.Code, tt.wantCode, d.Message)
			}
			if d.Path != "template_path" {
				t.Errorf("path = %q, want template_path", d.Path)
			}
		})
	}

	// And the affirmative case: a relative path INSIDE base_dir resolves.
	got, d := resolveGuardedTemplatePath("generate_presentation", "template_path", "client-brand.pptx", dir)
	if d != nil {
		t.Fatalf("relative path inside base_dir was refused: %s", d.Message)
	}
	if got != path {
		t.Errorf("resolved %q, want %q", got, path)
	}
}

// template and template_path are alternatives, not a fallback chain: silently
// preferring one would make the other's value a lie.
func TestTemplateAndTemplatePathAreExclusive(t *testing.T) {
	dir, path := byoTemplate(t)
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}

	deck := byoDeck(t, "template", "midnight-blue")
	deck["template_path"] = path
	result, err := mc.handleGenerate(t.Context(), byoRequest(map[string]any{
		"presentation": deck, "base_dir": dir, "output_filename": "both.pptx",
	}))
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatal("setting both template and template_path was accepted")
	}
	if msg := textContent(result); !strings.Contains(msg, "only one") {
		t.Errorf("message does not say the two are exclusive: %s", msg)
	}
}

// A path in `template` used to come back as the list of names it is not. It has
// to name the argument that does take a path.
func TestTemplateNotFoundRedirectsAPathToTemplatePath(t *testing.T) {
	_, path := byoTemplate(t)
	d := templateNotFoundDiagnostic("presentation.template", "presentation.template_path", path, "../../templates")
	if d.Code != diagnostics.CodeTemplateNotFound {
		t.Errorf("code = %q, want %q", d.Code, diagnostics.CodeTemplateNotFound)
	}
	if d.Path != "template" {
		t.Errorf("path = %q, want the deck field name", d.Path)
	}
	for _, want := range []string{"presentation.template_path", "not a path", "templates dir"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message does not mention %q: %s", want, d.Message)
		}
	}
	if d.Fix == nil || d.Fix.Kind != "rename_field" {
		t.Fatalf("fix = %+v, want kind rename_field", d.Fix)
	}
	// path, from and to all address the deck field, so a caller can apply the
	// rename without re-deriving which object it sits in; the message spells the
	// fully-qualified argument.
	if d.Fix.Params["from"] != "template" || d.Fix.Params["to"] != "template_path" {
		t.Errorf("fix from/to = %v/%v, want template/template_path", d.Fix.Params["from"], d.Fix.Params["to"])
	}
	if d.Details["templates_dir"] == nil {
		t.Error("details do not report templates_dir — the agent cannot find where to install it")
	}

	// A plain unregistered NAME keeps the old shape: the available list.
	d = templateNotFoundDiagnostic("presentation.template", "presentation.template_path", "no-such-template", "../../templates")
	if strings.Contains(d.Message, "template_path") {
		t.Errorf("a bare name should not be redirected to template_path: %s", d.Message)
	}
	if d.Details["available_templates"] == nil {
		t.Error("a bare name must still list the available templates")
	}
}

func TestLooksLikeTemplatePath(t *testing.T) {
	paths := []string{"/abs/x.pptx", "./x.pptx", "x.pptx", "sub/dir/x", "~/decks/x.pptx", "$DECKS/x.pptx", `C:\decks\x`}
	for _, p := range paths {
		if !looksLikeTemplatePath(p) {
			t.Errorf("looksLikeTemplatePath(%q) = false, want true", p)
		}
	}
	for _, n := range []string{"midnight-blue", "warm-coral", "modern-template", ""} {
		if looksLikeTemplatePath(n) {
			t.Errorf("looksLikeTemplatePath(%q) = true, want false", n)
		}
	}
}

// The CLI has no base_dir; a deck's template_path resolves against the deck
// file's own directory, so a deck and its template travel together.
func TestResolveDeckTemplatePathAgainstDeckDir(t *testing.T) {
	dir, path := byoTemplate(t)
	deckPath := filepath.Join(dir, "deck.json")

	got, err := resolveDeckTemplatePath("client-brand.pptx", deckPath)
	if err != nil {
		t.Fatalf("relative path against the deck dir: %v", err)
	}
	if got != path {
		t.Errorf("resolved %q, want %q", got, path)
	}

	if got, err = resolveDeckTemplatePath(path, deckPath); err != nil || got != path {
		t.Errorf("absolute path: got %q, %v", got, err)
	}
	if got, err = resolveDeckTemplatePath("", deckPath); err != nil || got != "" {
		t.Errorf("empty template_path: got %q, %v", got, err)
	}
	if _, err = resolveDeckTemplatePath("client-brand.txt", deckPath); err == nil {
		t.Error("a non-.pptx template_path was accepted")
	}
	if _, err = resolveDeckTemplatePath("absent.pptx", deckPath); err == nil {
		t.Error("a missing template_path was accepted")
	}
}

// examine_template is the entry point of the onboard-template task, so a core
// agent has to be able to see it.
func TestExamineTemplateIsInCoreProfile(t *testing.T) {
	if !coreToolSet()["examine_template"] {
		t.Error("examine_template is not in the core profile — the bring-your-own workflow starts with a tool a core agent cannot call")
	}
}
