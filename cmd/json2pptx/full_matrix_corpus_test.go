//go:build integration

package main

import "testing"

// shortReducedMatrixTests are the tests whose -short run takes a
// representative part of their matrix (two templates instead of all, one
// template per pattern, a few example decks), because the whole matrix is
// minutes under -race and CI's sharded race step is held to a time budget
// (go-slide-creator-q7cpq, scripts/ci_test_cmd_shards.sh). Without -short each
// runs its whole matrix.
//
// A test that cuts its matrix under -short and has neither "Corpus" nor
// "AcrossTemplates" in its name belongs in this list: the integration corpus
// job selects tests by those two words.
var shortReducedMatrixTests = map[string]func(*testing.T){
	"TestSchemaMaximaStayReadable":                       TestSchemaMaximaStayReadable,
	"TestSchemaMaximaMeasurementsMatchWrittenRuns":       TestSchemaMaximaMeasurementsMatchWrittenRuns,
	"TestCLIExtremeSchemaPatternsCannotPublishTinyText":  TestCLIExtremeSchemaPatternsCannotPublishTinyText,
	"TestPatternPreviewIsTheGeneratedSlide":              TestPatternPreviewIsTheGeneratedSlide,
	"TestPlanDeckRegionsDraftsValidateOnEveryTemplate":   TestPlanDeckRegionsDraftsValidateOnEveryTemplate,
	"TestNativeDiagramReadabilityValidateGenerateParity": TestNativeDiagramReadabilityValidateGenerateParity,
}

// TestShortReducedMatricesCorpus runs the whole matrix of every test in
// shortReducedMatrixTests. It carries "Corpus" in its name so the integration
// corpus job (`go test -tags=integration -run 'Corpus|AcrossTemplates'`,
// without -short and without -race) runs it on every push: what the short
// race run no longer covers is covered there.
func TestShortReducedMatricesCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("the whole matrices; -short runs each test's own subset")
	}
	for name, test := range shortReducedMatrixTests {
		t.Run(name, test)
	}
}
