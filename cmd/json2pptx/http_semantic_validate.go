package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

func newHTTPSemanticValidator(cfg config.Config) api.SemanticValidator {
	return func(ctx context.Context, data []byte, filename string, strict semantic.Strictness, templateName string) (api.SemanticValidationOutcome, error) {
		dir, cleanup, err := trialRenderDir()
		if err != nil {
			return api.SemanticValidationOutcome{}, fmt.Errorf("validation scratch directory: %w", err)
		}
		defer cleanup()
		assetDir := filepath.Join(dir, "assets")
		if err := os.Mkdir(assetDir, 0700); err != nil {
			return api.SemanticValidationOutcome{}, err
		}
		var runErr error
		eval, ds := evaluateSpecFindings(filename, data, strict, func(spec *semantic.DeckSpec, evaluatedData []byte) specEvaluation {
			choice := resolveSpecTemplate(spec.Meta.Template, templateName, "", specSource{})
			evaluated := choice.evaluated(spec)
			e := specEvaluation{Evaluated: true, Choice: choice, TemplateSource: choice.Source, Warnings: choice.Warnings}
			input, cr, err := semantic.Compile(evaluated, semantic.CompileOptions{Strict: strict, DefaultTemplate: templateName})
			if err != nil || input == nil {
				e.CompileFailed = true
				if err != nil {
					e.Diagnostics = buildSemanticRenderFailure(cr, err).Diagnostics
				}
				return e
			}
			e.Template = input.Template
			if input.Template == "" {
				e.Warnings = append(e.Warnings, unpinnedTemplateWarning("", ""))
				e.Diagnostics, e.Waivers = templateFreeDiagnostics(input, cr)
				return e
			}
			res, _, err := renderSemanticForHTTP(ctx, cfg, api.SemanticRenderRequest{Spec: evaluatedData, Filename: filename, Strict: strict, Template: templateName, AssetDir: assetDir, WorkDir: dir, OutputDir: dir, OutputFilename: "validate.pptx", OutputValidation: "strict"})
			if err != nil {
				runErr = err
				return e
			}
			e.Diagnostics = res.Diagnostics
			e.Waivers = res.Waivers
			e.RunFailed = !res.OK
			return e
		})
		if runErr != nil {
			return api.SemanticValidationOutcome{}, runErr
		}
		env := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "semantic_validate", InputSHA256: diagnostics.ComputeInputSHA256(data), Template: eval.Template}, ds)
		stampEnvelopeFindings(&env, eval.Diagnostics)
		cliRemedyContext(filename, data).remedyEnvelope(&env, eval.Diagnostics)
		shapeEnvelopeFindings(&env, eval.Diagnostics, newSpecDoc(filename, data))
		scope := "render"
		if !eval.Evaluated {
			scope = "schema"
		} else if eval.CompileFailed {
			scope = "compile"
		} else if eval.Template == "" {
			scope = "template_free"
		}
		return api.SemanticValidationOutcome{Valid: env.OK, Findings: env, Warnings: eval.Warnings, ValidationScope: scope}, nil
	}
}
