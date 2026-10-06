//go:build integration

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// shortSensitiveTests are the tests that do less under -short: a
// representative part of their matrix (two templates instead of all, one
// template per pattern, a few example decks), or nothing at all (a skip),
// because the whole of it is minutes under -race and CI's sharded race step is
// held to a time budget (go-slide-creator-q7cpq, scripts/ci_test_cmd_shards.sh).
// Without -short each runs in full.
//
// CI's test job runs -short only, so what a test does without -short runs
// nowhere unless it is listed here: TestDeckSpecAdviceNamesOnlyFieldsOfTheKind
// failed on main for a day in its long form with every job green
// (go-slide-creator-c9w86). A test that reads testing.Short() and has neither
// "Corpus" nor "AcrossTemplates" in its name belongs in this list — the
// integration corpus job selects tests by those two words — and
// TestShortSensitiveTestsRunInCorpus fails until it is.
var shortSensitiveTests = map[string]func(*testing.T){
	// A part of the matrix under -short.
	"TestSchemaMaximaStayReadable":                             TestSchemaMaximaStayReadable,
	"TestSchemaMaximaMeasurementsMatchWrittenRuns":             TestSchemaMaximaMeasurementsMatchWrittenRuns,
	"TestCLIExtremeSchemaPatternsCannotPublishTinyText":        TestCLIExtremeSchemaPatternsCannotPublishTinyText,
	"TestPatternPreviewIsTheGeneratedSlide":                    TestPatternPreviewIsTheGeneratedSlide,
	"TestPlanDeckRegionsDraftsValidateOnEveryTemplate":         TestPlanDeckRegionsDraftsValidateOnEveryTemplate,
	"TestNativeDiagramReadabilityValidateGenerateParity":       TestNativeDiagramReadabilityValidateGenerateParity,
	"TestDeckSpecAdviceNamesOnlyFieldsOfTheKind":               TestDeckSpecAdviceNamesOnlyFieldsOfTheKind,
	"TestEveryDeckSpecFindingPathResolves":                     TestEveryDeckSpecFindingPathResolves,
	"TestLongListFindingsNameTheirField":                       TestLongListFindingsNameTheirField,
	"TestEveryEmittedPatchClearsItsFinding":                    TestEveryEmittedPatchClearsItsFinding,
	"TestDeckSpecReadabilityVerdictParityAllKindsAllTemplates": TestDeckSpecReadabilityVerdictParityAllKindsAllTemplates,
	"TestEveryKindRendersAtItsDocumentedCounts":                TestEveryKindRendersAtItsDocumentedCounts,
	"TestTwelveFlawDraftCleanInThreeRoundTrips":                TestTwelveFlawDraftCleanInThreeRoundTrips,
	"TestRecommendVisualEvalTopCandidateRenders":               TestRecommendVisualEvalTopCandidateRenders,
	"TestRecommendVisualEveryPatternIsAuthorable":              TestRecommendVisualEveryPatternIsAuthorable,
	"TestRecommendVisualLayoutCandidatesAreAuthorable":         TestRecommendVisualLayoutCandidatesAreAuthorable,
	"TestRecommendVisualSameSlideIntentReturnsRegions":         TestRecommendVisualSameSlideIntentReturnsRegions,
	"TestRecommendVisualComposeRecipes":                        TestRecommendVisualComposeRecipes,
	"TestRecipesVerbatimAreBlockedAsExemplarContent":           TestRecipesVerbatimAreBlockedAsExemplarContent,
	"TestEightStepProcessValidatesCleanOnOneSlide":             TestEightStepProcessValidatesCleanOnOneSlide,
	"TestValidateExpandsPatternsLikeGeneration":                TestValidateExpandsPatternsLikeGeneration,
	"TestSchemaMaximaRunTemplateNames":                         TestSchemaMaximaRunTemplateNames,
	"TestGetStartedSequences_Executable":                       TestGetStartedSequences_Executable,
	// go-slide-creator-efhg2: two templates of nine, a third of the example
	// decks, one field per marker, a named template for a kind's detail.
	"TestStatusBoardWithDetailsIsReadableOnEveryTemplate": TestStatusBoardWithDetailsIsReadableOnEveryTemplate,
	"TestSlideKindBudgetsAgreeWithRenderFindings":         TestSlideKindBudgetsAgreeWithRenderFindings,
	"TestProductPlaceholdersAreDetectedInAnyField":        TestProductPlaceholdersAreDetectedInAnyField,
	"TestShippedExamplesValidate":                         TestShippedExamplesValidate,
	"TestSemanticKindsMatchesListSlideKinds":              TestSemanticKindsMatchesListSlideKinds,
	"TestSemanticKindExamplesValidate":                    TestSemanticKindExamplesValidate,

	// Skipped under -short.
	"TestAppendixPageLabelsRawStructure":                              TestAppendixPageLabelsRawStructure,
	"TestAppendixPageLabelsRawFlat":                                   TestAppendixPageLabelsRawFlat,
	"TestAppendixPageLabelsDeckSpec":                                  TestAppendixPageLabelsDeckSpec,
	"TestExemplarsClearTheUnderusedThreshold":                         TestExemplarsClearTheUnderusedThreshold,
	"TestCLIRenderThumbnailsWritesFiles":                              TestCLIRenderThumbnailsWritesFiles,
	"TestPatternBlockStaysAboveChrome":                                TestPatternBlockStaysAboveChrome,
	"TestRepairGateAgreesWithScoreDeck":                               TestRepairGateAgreesWithScoreDeck,
	"TestAutoRepairDoesNotConvergeOnLowComposition":                   TestAutoRepairDoesNotConvergeOnLowComposition,
	"TestPatternIconsRenderWithHexPaint":                              TestPatternIconsRenderWithHexPaint,
	"TestJourneyMaturityModelTitleNoOverlap":                          TestJourneyMaturityModelTitleNoOverlap,
	"TestKPICaptionNotReportedAsBodyText":                             TestKPICaptionNotReportedAsBodyText,
	"TestExportDeckPDFIntegration":                                    TestExportDeckPDFIntegration,
	"TestPatternPreviewPixelsMatchGeneratedSlide":                     TestPatternPreviewPixelsMatchGeneratedSlide,
	"TestListSlideKindsPreviewMatchesRenderDeckSpec":                  TestListSlideKindsPreviewMatchesRenderDeckSpec,
	"TestRecommendVisualPreviewReturnsImages":                         TestRecommendVisualPreviewReturnsImages,
	"TestRenderThumbnailsBySlideIDIntegration":                        TestRenderThumbnailsBySlideIDIntegration,
	"TestCLIRenderBySlideID":                                          TestCLIRenderBySlideID,
	"TestRenderDeckThumbnails_ImageContentIntegration":                TestRenderDeckThumbnails_ImageContentIntegration,
	"TestRenderDeckThumbnails_SlideIndicesIntegration":                TestRenderDeckThumbnails_SlideIndicesIntegration,
	"TestNotesOnlyRevisionKeepsThumbnailHashes":                       TestNotesOnlyRevisionKeepsThumbnailHashes,
	"TestSpecOutlineRenderRejectsUnknownFields":                       TestSpecOutlineRenderRejectsUnknownFields,
	"TestSubmitVisualReview_SurvivesSelectedForceRefresh":             TestSubmitVisualReview_SurvivesSelectedForceRefresh,
	"TestVisualQA_EnabledWithRenderTools":                             TestVisualQA_EnabledWithRenderTools,
	"TestVisualQA_AuditPaletteRequested":                              TestVisualQA_AuditPaletteRequested,
	"TestPatternExemplarsStoreNoAutofitShrink":                        TestPatternExemplarsStoreNoAutofitShrink,
	"TestOverlaysPaintAboveMedia":                                     TestOverlaysPaintAboveMedia,
	"TestOverlaysStayAboveGridCells":                                  TestOverlaysStayAboveGridCells,
	"TestImageTextSplit_GeneratesCoverCroppedPicture":                 TestImageTextSplit_GeneratesCoverCroppedPicture,
	"TestCalloutCrossTemplate":                                        TestCalloutCrossTemplate,
	"TestProcessContentOverrideRendersWhatItReports":                  TestProcessContentOverrideRendersWhatItReports,
	"TestExpandCrossTemplate":                                         TestExpandCrossTemplate,
	"TestRecommendVisualRecipesRenderForEveryType":                    TestRecommendVisualRecipesRenderForEveryType,
	"TestRecommendVisualRankingIntentGetsHorizontalBars":              TestRecommendVisualRankingIntentGetsHorizontalBars,
	"TestHandleRenderSlideImageFromJSON_CachedAnswerHasTheFreshShape": TestHandleRenderSlideImageFromJSON_CachedAnswerHasTheFreshShape,
	"TestScoreDeckStillGradesARawDeck":                                TestScoreDeckStillGradesARawDeck,
	"TestRenderDeckSpecDistinctOutputNames":                           TestRenderDeckSpecDistinctOutputNames,
	"TestRenderDeckSpecHonoursOutputFilename":                         TestRenderDeckSpecHonoursOutputFilename,
	"TestRenderDeckSpecReportsOverwrote":                              TestRenderDeckSpecReportsOverwrote,
	"TestConcurrentRenderDeckSpecHashMatchesFile":                     TestConcurrentRenderDeckSpecHashMatchesFile,
	"TestSectionCrumbFlatDeckRenders":                                 TestSectionCrumbFlatDeckRenders,
	"TestRenderDeckSpecFailureIsError":                                TestRenderDeckSpecFailureIsError,
	"TestRenderDeckSpecSuccessIsNotError":                             TestRenderDeckSpecSuccessIsNotError,
	"TestSemanticToolsIsErrorParity":                                  TestSemanticToolsIsErrorParity,
	"TestShapeGridCrossTemplate":                                      TestShapeGridCrossTemplate,
	"TestValidateDeckSpecReportsMeasuredTitle":                        TestValidateDeckSpecReportsMeasuredTitle,
	// Wave 3 (go-slide-creator-ux1fl, -ptazs): a recipe render and a draft
	// thumbnail review under -short.
	"TestRecommendVisualBridgeBesideTextRecipe": TestRecommendVisualBridgeBesideTextRecipe,
	"TestImageCaseDraftThumbnailReview":         TestImageCaseDraftThumbnailReview,
}

// shortSensitiveElsewhere are the short-sensitive tests another CI job runs
// without -short, with the job.
var shortSensitiveElsewhere = map[string]string{
	"TestTemplatePatternMatrix": "template-pattern-matrix",
}

// corpusJobRE is the corpus job's own selection (.github/workflows/ci.yml,
// corpus-headless).
var corpusJobRE = regexp.MustCompile(`Corpus|AcrossTemplates`)

// TestShortReducedMatricesCorpus runs every test in shortSensitiveTests in
// full. It carries "Corpus" in its name so the integration corpus job
// (`go test -tags=integration -run 'Corpus|AcrossTemplates'`, without -short
// and without -race) runs it on every push: what the short race run does not
// cover is covered there.
func TestShortReducedMatricesCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("the whole matrices; -short runs each test's own subset")
	}
	names := make([]string, 0, len(shortSensitiveTests))
	for name := range shortSensitiveTests {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, shortSensitiveTests[name])
	}
}

// TestShortSensitiveTestsRunInCorpus reads the package's test sources: every
// test that reads testing.Short() — itself, or through a helper of the
// package that does — runs without -short somewhere in CI. That is the corpus
// job when its name carries one of the job's two words or it is listed in
// shortSensitiveTests, and the job named in shortSensitiveElsewhere otherwise.
func TestShortSensitiveTestsRunInCorpus(t *testing.T) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", nil, 0) //nolint:staticcheck // one directory of files, no type information needed
	if err != nil {
		t.Fatal(err)
	}
	readsShort := func(body *ast.BlockStmt, helpers map[string]bool) bool {
		found := false
		ast.Inspect(body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return !found
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fun.X.(*ast.Ident); ok && x.Name == "testing" && fun.Sel.Name == "Short" {
					found = true
				}
			case *ast.Ident:
				found = found || helpers[fun.Name]
			}
			return !found
		})
		return found
	}
	funcs := map[string]*ast.BlockStmt{}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if !strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
					funcs[fn.Name.Name] = fn.Body
				}
			}
		}
	}
	isTest := func(name string) bool { return strings.HasPrefix(name, "Test") }
	// Helpers that read testing.Short(), to a fixed point: a helper that calls
	// one is one.
	helpers := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for name, body := range funcs {
			if !isTest(name) && !helpers[name] && readsShort(body, helpers) {
				helpers[name], grew = true, true
			}
		}
	}
	sensitive := 0
	for name, body := range funcs {
		if !isTest(name) || !readsShort(body, helpers) {
			continue
		}
		sensitive++
		_, listed := shortSensitiveTests[name]
		_, elsewhere := shortSensitiveElsewhere[name]
		if !listed && !elsewhere && !corpusJobRE.MatchString(name) {
			t.Errorf("%s reads testing.Short() and runs without -short nowhere in CI: add it to shortSensitiveTests (cmd/json2pptx/full_matrix_corpus_test.go)", name)
		}
	}
	if sensitive < len(shortSensitiveTests) {
		t.Errorf("found %d short-sensitive tests, fewer than the %d listed: the scan no longer sees them", sensitive, len(shortSensitiveTests))
	}
	for name := range shortSensitiveTests {
		if body, ok := funcs[name]; ok && !readsShort(body, helpers) {
			t.Errorf("%s is listed in shortSensitiveTests and no longer reads testing.Short(): the short race run covers it, remove it from the list", name)
		}
	}
}
