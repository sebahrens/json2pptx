package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// Bring-your-own template resolution (go-slide-creator-ydbk).
//
// A template reached the engine only by NAME, looked up in the server's
// templates dir and the embedded set. An agent holding a client's .pptx had no
// supported way in: an absolute path in `template` failed TEMPLATE_NOT_FOUND
// listing the names it is not, `template_path` was an unknown key, and the one
// tool that did take a path — examine_template — was not in the core profile,
// so a core agent never saw it. What actually worked was copying the file into
// the server's templates dir, which no tool response, instruction or
// get_started task mentioned. BYO onboarding therefore needed the operator.
//
// template_path is the supported way in, on every tool that takes a template.
// It keeps examine_template's containment rule: the path is resolved against
// base_dir (the server CWD when absent) and must stay inside it after ~/$ENV
// expansion and symlink evaluation, so the server's reach never widens past the
// root the caller declared.

// exampleTemplatePath is the example value used across template_path
// diagnostics so agents always see the same concrete shape to mimic.
const exampleTemplatePath = "/Users/you/decks/new-template.pptx"

// resolveGuardedTemplatePath validates an agent-supplied local template path.
// The path must, after ~/$ENV expansion and symlink resolution, be a regular
// .pptx file contained within baseDir (the allowed root). It returns the
// resolved absolute path, or a diagnostic naming the failure mode (forbidden
// traversal/escape, missing file, wrong type/extension).
//
// tool names the calling MCP tool so the diagnostic's next_tool_call is
// executable verbatim; argPath names the argument in that tool's schema
// ("template_path", or "presentation.template_path" for the deck tools) so the
// envelope's path field points at what the caller actually wrote.
func resolveGuardedTemplatePath(tool, argPath, rawPath, baseDir string) (string, *diagnostics.Diagnostic) {
	invalid := func(code, msg string, next bool) (string, *diagnostics.Diagnostic) {
		d := &diagnostics.Diagnostic{
			Code:         code,
			Path:         argPath,
			Message:      msg,
			Severity:     diagnostics.SeverityError,
			ExpectedType: "string",
			ExampleValue: exampleTemplatePath,
		}
		if next {
			d.NextToolCall = nextCallListTemplates()
		}
		return "", d
	}

	// Extension allow-list: a template is a .pptx package.
	if ext := strings.ToLower(filepath.Ext(rawPath)); ext != ".pptx" {
		return invalid(diagnostics.CodeInvalidParameter,
			fmt.Sprintf("%s %q: unsupported extension %q (want .pptx)", argPath, rawPath, ext), false)
	}

	// Pre-clean traversal check on the raw input so "../x.pptx" is rejected
	// before filepath.Clean collapses the "..".
	if err := utils.ValidatePath(filepath.FromSlash(rawPath), nil); err != nil {
		return "", forbiddenTemplatePathDiagnostic(argPath, rawPath, baseDir, err)
	}

	// Expand "~/..." and "$VAR" before joining baseDir so an agent-supplied
	// "~/decks/x.pptx" or "$DECKS/x.pptx" resolves against the home directory
	// or env-pointed root instead of being silently rooted under baseDir.
	expanded, unsetVar := expandAssetPath(rawPath)
	if unsetVar != "" {
		return invalid(diagnostics.CodeInvalidPath,
			fmt.Sprintf("%s %q references unset environment variable %q", argPath, rawPath, unsetVar), false)
	}

	// Resolve relative paths against the allowed root (baseDir).
	p := filepath.FromSlash(expanded)
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	p = filepath.Clean(p)

	// Evaluate symlinks (also catches a missing file) so containment is checked
	// against the real on-disk location, not a symlink that points outside.
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return invalid(diagnostics.CodeFileNotFound, fmt.Sprintf("%s %q: %v", argPath, rawPath, err), true)
	}

	// Containment: the resolved path MUST live within baseDir. This is the
	// forbidden-path guard — an absolute path outside the allowed root, or a
	// symlink escaping it, fails here even though the earlier raw ".." check
	// passed.
	if verr := utils.ValidatePath(resolved, []string{baseDir}); verr != nil {
		return "", forbiddenTemplatePathDiagnostic(argPath, rawPath, baseDir, verr)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return invalid(diagnostics.CodeFileNotFound, fmt.Sprintf("%s %q: %v", argPath, rawPath, err), true)
	}
	if info.IsDir() {
		return invalid(diagnostics.CodeInvalidParameter,
			fmt.Sprintf("%s %q is a directory, not a .pptx file", argPath, rawPath), false)
	}

	return resolved, nil
}

// forbiddenTemplatePathDiagnostic is the clear forbidden-path diagnostic an
// agent receives when a template path escapes the allowed root: a ".."
// traversal or an absolute/symlinked path resolving outside base_dir. It names
// base_dir explicitly, because the fix is usually to pass the right one.
func forbiddenTemplatePathDiagnostic(argPath, rawPath, baseDir string, cause error) *diagnostics.Diagnostic {
	return &diagnostics.Diagnostic{
		Code:     diagnostics.CodeInvalidPath,
		Path:     argPath,
		Severity: diagnostics.SeverityError,
		Message: fmt.Sprintf("%s %q is outside the allowed base_dir %q: %v — pass base_dir as the directory that contains the template",
			argPath, rawPath, baseDir, cause),
		ExpectedType: "string",
		ExampleValue: exampleTemplatePath,
		Details:      map[string]any{"base_dir": baseDir},
		NextToolCall: nextCallListTemplates(),
	}
}

// resolveTemplateSource resolves the template for one request from exactly one
// of name (a registered/embedded template) or rawPath (a guarded local .pptx).
// It returns the resolved path plus a cleanup function (a no-op except for an
// embedded template extracted to a temp file), or a diagnostic the caller
// reports. The cleanup function is always non-nil and safe to defer.
func (mc *mcpConfig) resolveTemplateSource(request mcp.CallToolRequest, tool, nameArg, pathArg, name, rawPath string) (string, func(), *diagnostics.Diagnostic) {
	noop := func() {}
	hasName := strings.TrimSpace(name) != ""
	hasPath := strings.TrimSpace(rawPath) != ""

	nameField, pathField := lastPathSegment(nameArg), lastPathSegment(pathArg)

	switch {
	case !hasName && !hasPath:
		return "", noop, &diagnostics.Diagnostic{
			Code:         diagnostics.CodeMissingParameter,
			Path:         nameField,
			Severity:     diagnostics.SeverityError,
			Message:      fmt.Sprintf("%s is required: a registered template name, or %s for a local .pptx inside base_dir", nameArg, pathArg),
			ExpectedType: "string",
			ExampleValue: "midnight-blue",
			Fix:          &diagnostics.Fix{Kind: "provide_value", Params: map[string]any{"field": nameField}},
			NextToolCall: nextCallListTemplates(),
		}
	case hasName && hasPath:
		return "", noop, &diagnostics.Diagnostic{
			Code:         diagnostics.CodeAmbiguousInput,
			Path:         pathField,
			Severity:     diagnostics.SeverityError,
			Message:      fmt.Sprintf("set only one of %s (a registered name) or %s (a local .pptx), not both", nameArg, pathArg),
			ExpectedType: "string",
			ExampleValue: exampleTemplatePath,
			Fix:          &diagnostics.Fix{Kind: "remove_field", Params: map[string]any{"path": pathField}},
			NextToolCall: nextCallRetry(tool, nameField),
		}
	case hasName:
		path, cleanup, err := resolveTemplatePath(name, mc.templatesDir)
		if err != nil {
			return "", noop, templateNotFoundDiagnostic(nameArg, pathArg, name, mc.templatesDir)
		}
		return path, cleanup, nil
	default: // hasPath
		baseDir, d := resolveBaseDirDiag(request)
		if d != nil {
			return "", noop, d
		}
		path, d := resolveGuardedTemplatePath(tool, pathField, rawPath, baseDir)
		if d != nil {
			return "", noop, d
		}
		return path, noop, nil
	}
}

// templateNotFoundDiagnostic reports an unregistered template name. When the
// value looks like a filesystem path it says so and names the argument that
// does take one: an agent that pastes a client's .pptx path into `template`
// used to get back the list of names it is not, with nothing about how to use
// the file it actually has.
func templateNotFoundDiagnostic(nameArg, pathArg, name, templatesDir string) *diagnostics.Diagnostic {
	// The envelope's path names the field as the deck JSON spells it
	// ("template"), while the message spells the fully-qualified argument the
	// caller has to change ("presentation.template_path").
	nameArgLabel, pathArgLabel := nameArg, pathArg
	nameArg, pathArg = lastPathSegment(nameArg), lastPathSegment(pathArg)
	available := listAvailableTemplates(templatesDir)
	details := map[string]any{"template_name": name}
	var example any
	if len(available) > 0 {
		details["available_templates"] = available
		example = available[0]
	}
	msg := templateNotFoundError(name, templatesDir)
	var fix *diagnostics.Fix
	if looksLikeTemplatePath(name) {
		msg = fmt.Sprintf("%s takes a registered template name, not a path: %q. Pass it as %s (with base_dir set to a directory containing it), or copy the file into the server's templates dir (%s) and use its file name without .pptx.",
			nameArgLabel, name, pathArgLabel, templatesDirLabel(templatesDir))
		details["templates_dir"] = templatesDirLabel(templatesDir)
		fix = &diagnostics.Fix{Kind: "rename_field", Params: map[string]any{
			"from": nameArg, "to": pathArg, "did_you_mean": pathArg, "value": name,
		}}
		example = exampleTemplatePath
	}
	return &diagnostics.Diagnostic{
		Code:         diagnostics.CodeTemplateNotFound,
		Path:         nameArg,
		Severity:     diagnostics.SeverityError,
		Message:      msg,
		ExpectedType: "string",
		ExampleValue: example,
		Details:      details,
		Fix:          fix,
		NextToolCall: nextCallListTemplates(),
	}
}

// lastPathSegment returns the final dotted segment of an argument label, so
// "presentation.template" addresses the deck field "template".
func lastPathSegment(arg string) string {
	if i := strings.LastIndexByte(arg, '.'); i >= 0 {
		return arg[i+1:]
	}
	return arg
}

// looksLikeTemplatePath reports whether a template value is a filesystem path
// rather than a registered name: it carries a separator, a ".pptx" suffix, or a
// "~"/"$" expansion prefix. Registered names are bare slugs.
func looksLikeTemplatePath(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasSuffix(strings.ToLower(name), ".pptx") {
		return true
	}
	if strings.ContainsAny(name, `/\`) {
		return true
	}
	return strings.HasPrefix(name, "~") || strings.HasPrefix(name, "$")
}

// templatesDirLabel renders the server's templates dir for a message, naming
// the embedded set when no directory is configured.
func templatesDirLabel(templatesDir string) string {
	dir, embedded := resolveTemplatesDir(templatesDir)
	if embedded || dir == "" {
		return "embedded templates only — start the server with --templates-dir to add your own"
	}
	return dir
}

// resolveRequestTemplatePath resolves a tool-level template_path argument
// against the call's base_dir. It is the thin form for tools that take a
// template path but no deck struct (render_deck_spec, list_templates).
func resolveRequestTemplatePath(request mcp.CallToolRequest, tool, rawPath string) (string, *diagnostics.Diagnostic) {
	baseDir, d := resolveBaseDirDiag(request)
	if d != nil {
		return "", d
	}
	return resolveGuardedTemplatePath(tool, "template_path", rawPath, baseDir)
}

// resolveDeckTemplatePath resolves a deck's template_path for the CLI. Relative
// values resolve against the deck file's own directory — the same frame the CLI
// already uses for relative asset paths — so a deck and the template it names
// travel together. Returns "" when the deck names no path.
//
// The CLI has no base_dir containment: the caller is the user running the
// binary, who can already read any file the process can. The MCP surface, where
// the caller is a model, keeps the base_dir guard.
func resolveDeckTemplatePath(rawPath, jsonPath string) (string, error) {
	if rawPath == "" {
		return "", nil
	}
	if ext := strings.ToLower(filepath.Ext(rawPath)); ext != ".pptx" {
		return "", fmt.Errorf("template_path %q: unsupported extension %q (want .pptx)", rawPath, ext)
	}
	expanded, unsetVar := expandAssetPath(rawPath)
	if unsetVar != "" {
		return "", fmt.Errorf("template_path %q references unset environment variable %q", rawPath, unsetVar)
	}
	p := filepath.FromSlash(expanded)
	if !filepath.IsAbs(p) && jsonPath != "" && jsonPath != "-" {
		p = filepath.Join(filepath.Dir(jsonPath), p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("template_path %q: %w", rawPath, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("template_path %q: %w", rawPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("template_path %q is a directory, not a .pptx file", rawPath)
	}
	return abs, nil
}

// listTemplatesSources decides what list_templates should analyze: the named
// template, every registered one, or a single bring-your-own .pptx addressed by
// template_path (go-slide-creator-ydbk). Exactly one of names / byoPath is
// populated; a structured error result means the caller returns it unchanged.
func listTemplatesSources(request mcp.CallToolRequest, templatesDir, templateName string) (names []string, byoPath string, errResult *mcp.CallToolResult) {
	rawTemplatePath, _ := request.GetArguments()["template_path"].(string)
	if rawTemplatePath == "" {
		if templateName != "" {
			return []string{templateName}, "", nil
		}
		names = listAvailableTemplates(templatesDir)
		sort.Strings(names)
		return names, "", nil
	}
	if templateName != "" {
		return nil, "", argInvalidValue("list_templates", diagnostics.CodeAmbiguousInput, "template_path",
			"set only one of template (a registered name) or template_path (a local .pptx), not both",
			"string", exampleTemplatePath, nil)
	}
	path, d := resolveRequestTemplatePath(request, "list_templates", rawTemplatePath)
	if d != nil {
		return nil, "", api.MCPDiagnosticsError([]diagnostics.Diagnostic{*d})
	}
	return nil, path, nil
}
