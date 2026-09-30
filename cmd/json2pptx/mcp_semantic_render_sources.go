package main

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// render_deck_spec inputs that come from outside the spec: the asset root,
// the URL resolver, a bring-your-own template carried by a deck_id, and the
// server's SVG configuration. Kept apart from the handler so the handler reads
// as the same compile → prepare → run flow the CLI uses.

// semanticAssetBaseDir is the directory relative asset paths (and a new
// template_path) resolve against for a render_deck_spec call: an explicit
// base_dir wins, then the root the deck_id's last render used, then the server
// CWD — the same fallback generate_presentation applies
// (go-slide-creator-b7qqg.1, .8).
func semanticAssetBaseDir(request mcp.CallToolRequest, src specSource) (string, *diagnostics.Diagnostic) {
	if raw, _ := request.GetArguments()["base_dir"].(string); strings.TrimSpace(raw) == "" && src.BaseDir != "" {
		return src.BaseDir, nil
	}
	return resolveBaseDirDiag(request)
}

// semanticAssetPrep resolves a compiled deck's URL and relative local asset
// references with the guards generate_presentation uses: the SSRF-protected
// resource resolver (mc.resolverOpts) and resolveLocalAssetPaths against
// baseDir under the configured image allow-list. RunPresentation expects both
// done (see presentation_runner.go); without them a relative image_case photo
// was silently dropped (go-slide-creator-b7qqg.1).
//
// The resolver's download cache must outlive generation because the resolved
// local paths are embedded in the slides, so the caller defers close until the
// render is finished.
type semanticAssetPrep struct {
	resolver *resource.Resolver
	cacheDir string
	baseDir  string
	allowed  []string

	// errors are the error-severity asset findings the pre-convert hook
	// refused on; warnings are the non-blocking ones, surfaced like generate.
	errors   []diagnostics.Diagnostic
	warnings []string
}

func (mc *mcpConfig) newSemanticAssetPrep(input *PresentationInput, baseDir string) (*semanticAssetPrep, func(), error) {
	p := &semanticAssetPrep{baseDir: baseDir}
	closeFn := func() {}
	if hasURLReferences(input.Slides) {
		r, err := resource.NewResolver(mc.resolverOpts)
		if err != nil {
			return nil, closeFn, fmt.Errorf("resource resolver: %w", err)
		}
		p.resolver, p.cacheDir, closeFn = r, r.Dir(), r.Close
	}
	p.allowed = imageAllowList(mc.cfg.Images.AllowedBasePaths, p.cacheDir)
	return p, closeFn, nil
}

// preConvert is the RunPresentation PreConvert hook: it runs at the point the
// raw pipeline resolves assets, so error precedence matches the CLI.
func (p *semanticAssetPrep) preConvert(input *PresentationInput) func() error {
	return func() error {
		if p.resolver != nil {
			if urlFindings := resolveURLs(input.Slides, p.resolver); len(urlFindings) > 0 {
				p.errors = append(p.errors, diagnostics.FilterBySeverity(urlFindings, diagnostics.SeverityError)...)
				return iconFindingsToError(urlFindings)
			}
		}
		findings := resolveLocalAssetPaths(input.Slides, p.baseDir, p.allowed...)
		if err := iconFindingsToError(findings); err != nil {
			p.errors = append(p.errors, diagnostics.FilterBySeverity(findings, diagnostics.SeverityError)...)
			return err
		}
		for _, d := range findings {
			if d.Severity != diagnostics.SeverityError {
				p.warnings = append(p.warnings, fmt.Sprintf("%s at %s: %s", d.Code, d.Path, d.Message))
			}
		}
		return nil
	}
}

// semanticDiagnostics maps the refused asset findings back to the DeckSpec
// paths the author wrote, keeping the raw path as evidence.
func (p *semanticAssetPrep) semanticDiagnostics(cr *semantic.CompileResult) []semanticDiagnostic {
	var sm *semantic.SourceMap
	if cr != nil {
		sm = cr.SourceMap
	}
	out := make([]semanticDiagnostic, 0, len(p.errors))
	for _, d := range p.errors {
		m := semantic.MapFinding(sm, semantic.RawFinding{Code: d.Code, Message: d.Message, Severity: d.Severity, RawPath: d.Path})
		sd := semanticDiagnostic{
			Code:         m.Code,
			Severity:     string(m.Severity),
			Message:      m.Message,
			SemanticPath: m.SemanticPath,
			RawPath:      m.RawPath,
		}
		if m.SlideIndex >= 0 {
			idx := m.SlideIndex
			sd.SlideIndex = &idx
			if sd.SemanticPath == "" {
				// Pattern-expanded asset fields have no field-level source
				// link; point at the authored slide rather than nowhere.
				sd.SemanticPath = fmt.Sprintf("slides[%d]", idx)
			}
		}
		out = append(out, sd)
	}
	return out
}

// semanticRenderRoots resolves a render_deck_spec call's asset root and its
// bring-your-own template file: a template_path argument (when the spec pins
// no meta.template) vetted against the asset root, else the deck_id's stored
// file. The returned baseDir is set even when the template fails, so the
// handle keeps it.
func semanticRenderRoots(request mcp.CallToolRequest, src specSource, argTemplate, rawTemplatePath, metaTemplate string) (baseDir, templatePath string, d *diagnostics.Diagnostic) {
	baseDir, d = semanticAssetBaseDir(request, src)
	if d != nil {
		return "", "", d
	}
	if rawTemplatePath != "" && metaTemplate == "" {
		templatePath, d = resolveGuardedTemplatePath("render_deck_spec", "template_path", rawTemplatePath, baseDir)
	} else {
		templatePath, d = storedByoTemplatePath("render_deck_spec", src, argTemplate, rawTemplatePath, metaTemplate)
	}
	return baseDir, templatePath, d
}

// storedByoTemplatePath returns the bring-your-own template a deck_id carries,
// re-vetted against the root it was first vetted in, when this call names no
// template of its own. "" means the handle has none or the call overrides it.
func storedByoTemplatePath(tool string, src specSource, argTemplate, rawTemplatePath, metaTemplate string) (string, *diagnostics.Diagnostic) {
	if src.TemplatePath == "" || argTemplate != "" || rawTemplatePath != "" || metaTemplate != "" {
		return "", nil
	}
	return resolveGuardedTemplatePath(tool, "template_path", src.TemplatePath, src.BaseDir)
}

// specTemplateFile opens the template a compiled spec validates against: the
// handle's vetted bring-your-own file when there is one, else the registered
// name.
func (mc *mcpConfig) specTemplateFile(name, byoTemplatePath string) (string, func(), error) {
	if byoTemplatePath != "" {
		return byoTemplatePath, func() {}, nil
	}
	return resolveTemplatePath(name, mc.templatesDir)
}

// withServerSVGConfig fills the SVG knobs from the server's effective config
// (defaults + config file + SVG_* env overrides), the same values
// generate_presentation hands the generator. render_deck_spec used to build a
// fresh config.DefaultConfig() and silently ignored an operator's PNG / EMF
// strategy, scale, compatibility mode and PNG width (go-slide-creator-b7qqg.11).
func (mc *mcpConfig) withServerSVGConfig(opts RenderOptions) RenderOptions {
	opts.SVGStrategy = string(mc.cfg.SVG.Strategy)
	opts.SVGScale = mc.cfg.SVG.Scale
	opts.SVGNativeCompat = string(mc.cfg.SVG.NativeCompatibility)
	opts.MaxPNGWidth = mc.cfg.SVG.MaxPNGWidth
	return opts
}
