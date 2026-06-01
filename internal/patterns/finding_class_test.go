package patterns

import "testing"

func TestFindingClass_PatternChoiceCodes(t *testing.T) {
	for _, code := range []string{
		ErrCodeOvertallFlowLane,
		ErrCodeFlowDiamondNoContent,
		ErrCodeTocFlowchartVocab,
		ErrCodeSparseSingleRowFlow,
		ErrCodeWrongPattern,
		ErrCodePatternOvercrowded,
		ErrCodePatternUnderfilled,
		ErrCodeSparseLayout,
		ErrCodeCellUnderfilled,
	} {
		if got := FindingClass(code); got != FindingClassPatternChoice {
			t.Errorf("FindingClass(%q) = %q, want %q", code, got, FindingClassPatternChoice)
		}
	}
}

func TestFindingClass_RenderingCodes(t *testing.T) {
	// The matrix axis band is a rendering-geometry smell, NOT a pattern-choice
	// mismatch — that distinction is the whole point of the class field.
	for _, code := range []string{
		ErrCodeMatrixAxisImbalance,
		ErrCodePlaceholderOverflow,
		ErrCodeFitOverflow,
		ErrCodeContrastPredicted,
		ErrCodeDiagramClamped,
		"contrast_autofixed",
		"some_unmapped_future_code",
	} {
		if got := FindingClass(code); got != FindingClassRendering {
			t.Errorf("FindingClass(%q) = %q, want %q", code, got, FindingClassRendering)
		}
	}
}

func TestFindingClass_ContentCodes(t *testing.T) {
	for _, code := range []string{
		ErrCodeHeadlineTooLong,
		ErrCodeBodyTooLong,
		ErrCodeBulletNestingDeep,
		ErrCodeMissingAltText,
		ErrCodeDuplicateTitle,
		ErrCodeTakeawayMissing,
	} {
		if got := FindingClass(code); got != FindingClassContent {
			t.Errorf("FindingClass(%q) = %q, want %q", code, got, FindingClassContent)
		}
	}
}
