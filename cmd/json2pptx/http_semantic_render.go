package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// newHTTPSemanticRenderer builds the api.SemanticRenderer the HTTP server's
// POST /api/v1/semantic/render drives (go-slide-creator-b7qqg.25). It runs the
// same flow as `json2pptx semantic render` and render_deck_spec: parse →
// compile → design-mode gate → deck defaults / named styles → guarded URL and
// relative-asset resolution → RunPresentation (strict output validation by
// default) → source-mapped diagnostics and the quality / publication verdict.
//
// cfg is the server's own runtime configuration, so templates dir, SVG
// strategy and ALLOWED_IMAGE_PATHS match the process serving the request.
//
// Network-boundary policy: local image paths are ALWAYS restricted to the
// request's uploaded-asset directory, the URL download cache and any
// configured ALLOWED_IMAGE_PATHS roots — an HTTP caller must not be able to
// embed arbitrary server files into a downloadable deck, even when the
// operator configured no allow-list.
func newHTTPSemanticRenderer(cfg config.Config) api.SemanticRenderer {
	return func(ctx context.Context, req api.SemanticRenderRequest) (api.SemanticRenderOutcome, error) {
		res, outputPath, err := renderSemanticForHTTP(ctx, cfg, req)
		if err != nil {
			return api.SemanticRenderOutcome{}, err
		}
		raw, err := json.Marshal(res)
		if err != nil {
			return api.SemanticRenderOutcome{}, fmt.Errorf("marshal semantic render result: %w", err)
		}
		return api.SemanticRenderOutcome{OK: res.OK, OutputPath: outputPath, Result: raw}, nil
	}
}

// renderSemanticForHTTP is the render flow; a returned error is an internal
// failure, a refused render is a result with OK=false.
func renderSemanticForHTTP(ctx context.Context, cfg config.Config, req api.SemanticRenderRequest) (semanticRenderResult, string, error) { //nolint:gocognit // Mirrors the CLI/MCP render orchestration step for step.
	startTime := time.Now()

	spec, parseDiags := semantic.Parse(req.Filename, req.Spec)
	if parseDiags.HasErrors() {
		res := semanticRenderResult{OK: false, Error: "semantic render: spec could not be parsed"}
		for _, d := range semantic.Check(req.Filename, req.Spec, req.Strict) {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
		return res, "", nil
	}

	uploadLabel := ""
	if req.TemplatePath != "" {
		uploadLabel = "uploaded template " + req.TemplateFilename
	}
	precedenceWarning := templatePrecedenceWarning(spec.Meta.Template, req.Template, uploadLabel)

	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          req.Strict,
		DefaultTemplate: req.Template,
	})
	if err != nil {
		return buildSemanticRenderFailure(compileResult, err), "", nil
	}
	if designViolations := compiledDesignModeDiagnostics(input); len(designViolations) > 0 {
		appendCompiledDesignModeDiags(compileResult, input)
		return buildSemanticRenderFailure(compileResult, blockingDesignModeError(designViolations)), "", nil
	}

	applyDefaults(input)
	resolveInputNamedSettingsForDir(cfg.Templates.Dir, input)

	// Template: an uploaded .pptx renders the deck when the spec pins none
	// (meta.template wins, as with render_deck_spec's template_path);
	// otherwise the registered name is resolved up front so a missing
	// template is a meta.template diagnostic, not a bare error string.
	var resolvedTemplatePath, templateLabel string
	if req.TemplatePath != "" && spec.Meta.Template == "" {
		path, d := resolveGuardedTemplatePath("semantic_render", "template", req.TemplatePath, req.WorkDir)
		if d != nil {
			res := semanticRenderResult{OK: false, Error: d.Message}
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(*d))
			return res, "", nil
		}
		resolvedTemplatePath, templateLabel = path, req.TemplateFilename
	} else {
		path, templateCleanup, tplErr := resolveTemplatePath(input.Template, cfg.Templates.Dir)
		if tplErr != nil {
			d := semanticTemplateDiagnostic(input.Template, cfg.Templates.Dir, templateResolutionCode(tplErr), tplErr)
			res := buildSemanticRenderFailure(compileResult, errors.New(d.Message))
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(*d))
			return res, "", nil
		}
		defer templateCleanup()
		resolvedTemplatePath = path
	}

	urlResolver, urlCacheDir, closeResolver, err := newSlideURLResolver(input.Slides)
	if err != nil {
		return semanticRenderResult{}, "", fmt.Errorf("semantic render: %w", err)
	}
	defer closeResolver()
	allowList := httpImageAllowList(cfg.Images.AllowedBasePaths, req.AssetDir, urlCacheDir)

	var assetDiags []diagnostics.Diagnostic
	var preConvertWarnings []string
	preConvert := func() error {
		if urlResolver != nil {
			if urlFindings := resolveURLs(input.Slides, urlResolver); len(urlFindings) > 0 {
				assetDiags = append(assetDiags, urlFindings...)
				return iconFindingsToError(urlFindings)
			}
		}
		assetFindings := resolveLocalAssetPaths(input.Slides, req.AssetDir, allowList...)
		if assetErr := iconFindingsToError(assetFindings); assetErr != nil {
			assetDiags = append(assetDiags, assetFindings...)
			return assetErr
		}
		for _, d := range assetFindings {
			if d.Severity != diagnostics.SeverityError {
				preConvertWarnings = append(preConvertWarnings, fmt.Sprintf("%s at %s: %s", d.Code, d.Path, d.Message))
			}
		}
		return nil
	}

	runRes, cleanup, renderErr := RunPresentation(ctx, input, RenderOptions{
		OutputDir:            req.OutputDir,
		OutputFilename:       req.OutputFilename,
		TemplatesDir:         cfg.Templates.Dir,
		ResolvedTemplatePath: resolvedTemplatePath,
		OutputValidation:     req.OutputValidation,
		AccentStrategy:       patterns.AccentStrategy(input.AccentStrategy),
		SVGStrategy:          string(cfg.SVG.Strategy),
		SVGScale:             cfg.SVG.Scale,
		SVGNativeCompat:      string(cfg.SVG.NativeCompatibility),
		MaxPNGWidth:          cfg.SVG.MaxPNGWidth,
		AllowedImagePaths:    allowList,
		PreConvert:           preConvert,
	})
	defer cleanup()
	if renderErr != nil {
		if ctx.Err() != nil {
			return semanticRenderResult{}, "", ctx.Err()
		}
		res := buildSemanticRunFailure(input, compileResult, runRes, renderErr, semanticAssetDiagnostics(compileResult, assetDiags)...)
		prependWarning(&res, precedenceWarning)
		return res, "", nil
	}

	res := buildSemanticRenderSuccess(input, compileResult, runRes, startTime)
	res.Warnings = append(res.Warnings, preConvertWarnings...)
	prependWarning(&res, precedenceWarning)
	if templateLabel != "" {
		res.Template = templateLabel
	}
	return res, runRes.OutputPath, nil
}

func prependWarning(res *semanticRenderResult, w string) {
	if w != "" {
		res.Warnings = append([]string{w}, res.Warnings...)
	}
}

// httpImageAllowList is the image-root allow-list for an HTTP render: the
// configured roots plus the request's own asset and URL-cache directories.
// Unlike imageAllowList it never returns nil — an HTTP caller gets no
// unrestricted access to the server filesystem.
func httpImageAllowList(configured []string, extra ...string) []string {
	out := append([]string(nil), configured...)
	for _, dir := range extra {
		if dir != "" {
			out = append(out, dir)
		}
	}
	return out
}

// semanticAssetDiagnostics maps blocking asset-resolution diagnostics (raw
// PresentationInput paths) back to the semantic source the author wrote.
func semanticAssetDiagnostics(cr *semantic.CompileResult, ds []diagnostics.Diagnostic) []semanticDiagnostic {
	var sm *semantic.SourceMap
	if cr != nil {
		sm = cr.SourceMap
	}
	var out []semanticDiagnostic
	for _, d := range ds {
		if d.Severity != diagnostics.SeverityError {
			continue
		}
		mapped := semantic.MapFinding(sm, semantic.RawFinding{
			Code: d.Code, Message: d.Message, Severity: d.Severity, RawPath: d.Path,
		})
		sd := semanticDiagnostic{
			Code:         mapped.Code,
			Severity:     string(mapped.Severity),
			Message:      mapped.Message,
			SemanticPath: mapped.SemanticPath,
			RawPath:      mapped.RawPath,
		}
		if mapped.SlideIndex >= 0 {
			idx := mapped.SlideIndex
			sd.SlideIndex = &idx
		}
		out = append(out, sd)
	}
	return out
}
