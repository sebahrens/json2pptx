package generator

import (
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// SourcePreservingParagraphRepair derives an executable split from authored
// bullet structure, not fitted paragraphs (which may include headings, children
// and an ellipsis). Unsupported sources get manual source-preserving guidance.
func SourcePreservingParagraphRepair(loss patterns.ValidationError, slides []SlideSpec) patterns.ValidationError {
	if loss.Code != patterns.ErrCodeTextTrimmed && loss.Code != patterns.ErrCodeReadabilityTrimmed && loss.Code != patterns.ErrCodeTextBelowReadableMin {
		return loss
	}
	loss.Fix = nil
	parts := strings.Split(loss.Path, "/")
	if len(parts) != 5 || parts[0] != "" || parts[1] != "slides" || parts[3] != "content" {
		return manualParagraphContinuation(loss)
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil || strconv.Itoa(index) != parts[2] || index < 0 || index >= len(slides) {
		return manualParagraphContinuation(loss)
	}
	contentIndex, err := strconv.Atoi(parts[4])
	if err != nil || strconv.Itoa(contentIndex) != parts[4] || contentIndex < 0 || contentIndex >= len(slides[index].Content) || slides[index].Content[contentIndex].Type != ContentBullets {
		return manualParagraphContinuation(loss)
	}
	base := deckinput.SlideInput{}
	count := 0
	var columns [][]string
	for _, item := range slides[index].Content {
		content := deckinput.ContentInput{Type: string(item.Type)}
		if item.Type == ContentBullets {
			bullets, ok := item.Value.([]string)
			if !ok {
				return manualParagraphContinuation(loss)
			}
			content.BulletsValue = &bullets
			columns = append(columns, bullets)
			if count == 0 {
				count = len(bullets)
			}
		}
		base.Content = append(base.Content, content)
	}
	budget := alignedBulletGroupBudget(columns, count)
	if budget > 0 && budget < count {
		pages, err := deckinput.SplitBulletSlide(base, budget)
		if err == nil && len(pages) > 1 {
			loss.Fix = &patterns.FixSuggestion{Kind: "split_bullets", Params: map[string]any{"max_items": budget}}
			loss.Message += "; preserve all bullet columns with the suggested grouped split, then render every resulting page; do not exceed a fixed slide count without authorization"
			return loss
		}
	}
	return manualParagraphContinuation(loss)
}

func alignedBulletGroupBudget(columns [][]string, count int) int {
	for _, column := range columns {
		if len(column) != count {
			return 0
		}
	}
	// Compute the largest indivisible aligned group in one pass. Trying every
	// possible page budget would make a large nested group quadratic on refusal.
	budget, previous := 0, 0
	for i := 1; i <= count; i++ {
		boundary := true
		if i < count {
			for _, column := range columns {
				if depth, _ := pptx.BulletIndentDepth(column[i]); depth > 0 {
					boundary = false
					break
				}
			}
		}
		if boundary {
			budget = max(budget, i-previous)
			previous = i
		}
	}
	return budget
}

func manualParagraphContinuation(loss patterns.ValidationError) patterns.ValidationError {
	loss.Message += "; this source cannot use automatic plain-bullet splitting; author source-complete continuation slides or choose a suitable layout, preserving groups and any fixed slide count"
	return loss
}
