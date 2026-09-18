package textfit

import "github.com/sebahrens/json2pptx/internal/tokens"

type ReadabilityResult struct {
	Readable     bool
	EffectiveHPt int
	MinimumHPt   int
	Mode         tokens.ViewingMode
	Role         tokens.TextRole
}

func CheckReadability(fontSizeHPt, fontScale int, mode tokens.ViewingMode, role tokens.TextRole) ReadabilityResult {
	if fontScale <= 0 {
		fontScale = 100000
	}
	effective := fontSizeHPt * fontScale / 100000
	minimum := tokens.MinReadableHPt(mode, role)
	return ReadabilityResult{Readable: effective >= minimum, EffectiveHPt: effective, MinimumHPt: minimum, Mode: mode, Role: role}
}
