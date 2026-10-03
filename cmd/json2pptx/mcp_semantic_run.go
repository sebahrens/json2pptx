package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// The one run behind render_deck_spec and validate_deck_spec
// (go-slide-creator-3rn3s, go-slide-creator-2dit4).
//
// validate_deck_spec used to predict a render's findings from the compiled
// deck. The prediction drifted: findings only generation produces (a wrapped
// title, a contrast fix, a collapsed plot area, the refusal itself) appeared
// at render alone, and predicted shrinkage disagreed with the measured one. It
// now runs the same function render does, into a scratch directory it removes,
// and reports that run's diagnostics. One spec revision on one template has
// one finding set, whichever tool is asked.

// specRunOptions are the per-call knobs of one compiled-spec run.
type specRunOptions struct {
	// ArgTemplate / RawTemplatePath are the call's own template arguments
	// (not the deck handle's), used to resolve a bring-your-own template.
	ArgTemplate     string
	RawTemplatePath string
	// OutputDir / OutputFilename name the artifact.
	OutputDir        string
	OutputFilename   string
	OutputValidation string
	Start            time.Time
}

// specRunOutcome is what one compiled-spec run produced.
type specRunOutcome struct {
	// Result is the render result: success, or a refusal with its findings.
	Result semanticRenderResult
	// Early is a transport-level error result (an unresolvable template, a
	// resolver that could not start) that replaces the tool's own response.
	Early *mcp.CallToolResult
	// EarlyDiagnostics are Early's findings, for a caller that reports them in
	// its own envelope instead.
	EarlyDiagnostics []diagnostics.Diagnostic
	// BaseDir / TemplatePath are the asset root and bring-your-own template
	// file the run resolved, for the deck handle.
	BaseDir      string
	TemplatePath string
}

// runCompiledDeckSpec renders a compiled DeckSpec through the shared runner:
// deck defaults and named settings, the asset root and template file, guarded
// asset resolution, RunPresentation, and the result builders that apply the
// one severity model. It is the whole of a render between "the spec compiled"
// and "here is the result".
func (mc *mcpConfig) runCompiledDeckSpec(ctx context.Context, tool string, request mcp.CallToolRequest, src specSource, spec *semantic.DeckSpec, input *PresentationInput, compileResult *semantic.CompileResult, opts specRunOptions) specRunOutcome {
	var out specRunOutcome

	// Apply the shared pre-render prep a compiled deck still needs (deck defaults
	// and named style references), then render through the shared in-memory
	// runner with the standard native-SVG knobs — the same flow as the CLI.
	applyDefaults(input)
	resolveInputNamedSettingsForDir(mc.templatesDir, input)

	// One root frames every relative path this render reads (asset references
	// and a template_path), and a caller-supplied or deck_id-carried .pptx
	// renders the deck when the spec pins no template of its own
	// (go-slide-creator-ydbk, b7qqg.1, b7qqg.8).
	baseDir, resolvedTemplatePath, d := semanticRenderRoots(request, src, opts.ArgTemplate, opts.RawTemplatePath, spec.Meta.Template)
	out.BaseDir, out.TemplatePath = baseDir, resolvedTemplatePath
	if d != nil {
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck base_dir or template path invalid", map[string]any{"tool": tool})
		out.Result = buildSemanticRenderFailure(compileResult, errors.New(d.Message))
		out.Result.Diagnostics = append(out.Result.Diagnostics, semanticDiagFromCompile(*d))
		return out
	}
	if resolvedTemplatePath == "" {
		path, templateCleanup, err := resolveTemplatePath(input.Template, mc.templatesDir)
		if err != nil {
			out.EarlyDiagnostics = []diagnostics.Diagnostic{
				*semanticTemplateDiagnostic(input.Template, mc.templatesDir, templateResolutionCode(err), err),
			}
			out.Early = api.MCPDiagnosticsError(out.EarlyDiagnostics)
			return out
		}
		resolvedTemplatePath = path
		defer templateCleanup()
	}

	// URL and relative asset references resolve inside the runner, with the
	// guards generate_presentation uses; the download cache lives until the
	// render is done (go-slide-creator-b7qqg.1).
	assets, closeAssets, assetErr := mc.newSemanticAssetPrep(input, baseDir)
	defer closeAssets()
	if assetErr != nil {
		out.EarlyDiagnostics = []diagnostics.Diagnostic{{
			Code: string(diagnostics.CodeURLResolverInit), Message: assetErr.Error(), Severity: diagnostics.SeverityError,
		}}
		out.Early = api.MCPSimpleError("URL_RESOLVER_INIT", assetErr.Error())
		return out
	}

	// SVG knobs come from the server's effective config, as for
	// generate_presentation (go-slide-creator-b7qqg.11).
	runRes, cleanup, renderErr := RunPresentation(ctx, input, mc.withServerSVGConfig(RenderOptions{
		OutputDir:            opts.OutputDir,
		OutputFilename:       opts.OutputFilename,
		TemplatesDir:         mc.templatesDir,
		ResolvedTemplatePath: resolvedTemplatePath,
		OutputValidation:     opts.OutputValidation,
		AccentStrategy:       patterns.AccentStrategy(input.AccentStrategy),
		AllowedImagePaths:    assets.allowed,
		PreConvert:           assets.preConvert(input),
	}))
	defer cleanup()
	if renderErr != nil {
		out.Result = buildSemanticRunFailure(input, compileResult, runRes, renderErr, assets.semanticDiagnostics(compileResult)...)
		return out
	}

	built := buildSemanticRenderSuccess(input, compileResult, runRes, opts.Start)
	built.Warnings = append(built.Warnings, assets.warnings...)
	out.Result = built
	return out
}

// specTemplateChoice is the template a DeckSpec call evaluates and where the
// choice came from (go-slide-creator-2dit4).
type specTemplateChoice struct {
	// Default is the template to compile with when the spec pins none: the
	// call's argument, else the deck handle's bound template.
	Default string
	// Source names what decided the evaluated template: "meta.template",
	// "template argument", "template_path argument", "deck_id", or
	// "archetype default".
	Source string
	// Warnings explain a choice the caller may not expect.
	Warnings []string
	// Bind is the template to bind to the deck handle on this call ("" keeps
	// what the handle has).
	Bind string
	// OneOff reports that the call's template argument applies to this call
	// only: the handle is already bound to another template.
	OneOff bool
	// Override is the template argument when it replaces the spec's own
	// meta.template for this call (go-slide-creator-ifkxs).
	Override string
}

// evaluated returns the spec as this call evaluates it: the spec itself, or a
// copy whose meta.template is the call's template argument.
func (c specTemplateChoice) evaluated(spec *semantic.DeckSpec) *semantic.DeckSpec {
	if c.Override == "" || spec == nil {
		return spec
	}
	out := *spec
	out.Meta.Template = c.Override
	return &out
}

// resolveSpecTemplate applies the DeckSpec template precedence and the deck
// handle's binding rule. The call's template argument wins for that call;
// then meta.template; then a template_path argument; then the template the
// deck_id is bound to; then the archetype default.
//
// A template argument that differs from meta.template used to be ignored with
// a warning, so trying a finished spec on a second template meant editing the
// spec (go-slide-creator-ifkxs). It now replaces meta.template for the call,
// and the response says so; the spec and the deck's binding keep the pin.
//
// A deck_id is bound by the first call that names a template for it, and the
// binding then changes only through a patch to /meta/template: a later call
// naming a different template is evaluated on that template for that call
// alone and says so. The handle used to adopt whichever template the last
// render used, so a one-off render on another template silently changed what
// every later validate measured against.
func resolveSpecTemplate(metaTemplate, argTemplate, argTemplatePath string, src specSource) specTemplateChoice {
	c := specTemplateChoice{Default: argTemplate}
	bound := firstNonEmpty(src.Template, src.TemplatePath)
	switch {
	case metaTemplate != "" && argTemplate != "" && argTemplate != metaTemplate:
		c.Source = "template argument"
		c.Override = argTemplate
		c.Bind = metaTemplate
		c.Warnings = append(c.Warnings, templateOverrideWarning(metaTemplate, argTemplate))
		return c
	case metaTemplate != "":
		// The spec's own pin is the deck's template, and the handle follows
		// it: patching /meta/template is how a deck changes template.
		c.Source = "meta.template"
		c.Bind = metaTemplate
		if w := templatePrecedenceWarning(metaTemplate, argTemplate, argTemplatePath); w != "" {
			c.Warnings = append(c.Warnings, w)
		}
	case argTemplatePath != "":
		c.Source = "template_path argument"
		c.OneOff = bound != "" && src.TemplatePath != argTemplatePath
	case argTemplate != "":
		c.Source = "template argument"
		if bound == "" {
			c.Bind = argTemplate
		} else if src.Template != argTemplate {
			c.OneOff = true
		}
	case bound != "":
		c.Source = "deck_id"
		c.Default = src.Template
	default:
		c.Source = "archetype default"
	}
	if c.OneOff {
		c.Warnings = append(c.Warnings, fmt.Sprintf(
			"this call was evaluated on %s, but the deck_id stays bound to %q: a deck's template changes only by patch; to switch it, patch [{\"op\":\"add\",\"path\":\"/meta/template\",\"value\":<name>}]",
			firstNonEmpty(argTemplate, argTemplatePath), bound))
	}
	return c
}

// templateOverrideWarning says that the call's template replaced the spec's.
func templateOverrideWarning(metaTemplate, argTemplate string) string {
	return fmt.Sprintf("template argument %q overrides meta.template %q for this call only: the spec still pins %q; to keep %q, set meta.template to it (patch [{\"op\":\"replace\",\"path\":\"/meta/template\",\"value\":%q}])",
		argTemplate, metaTemplate, metaTemplate, argTemplate, argTemplate)
}

// unpinnedTemplateWarning tells the caller that nothing in the spec fixes the
// template its findings were measured against.
func unpinnedTemplateWarning(evaluated, source string) string {
	if evaluated == "" {
		return "the spec pins no template and none was given, so template-dependent checks (fit, readability, contrast) did not run; set meta.template or pass template"
	}
	return fmt.Sprintf("the spec pins no template: findings were measured on %q (%s). Fit and readability differ by template, so render on the same one — set meta.template, or pass the same template when you render", evaluated, source)
}

// trialRenderDir creates the scratch directory a validating run writes its
// throwaway deck into.
func trialRenderDir() (string, func(), error) {
	dir, err := os.MkdirTemp("", "json2pptx-validate-*")
	if err != nil {
		return "", func() {}, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}
