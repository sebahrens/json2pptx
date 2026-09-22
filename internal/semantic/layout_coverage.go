package semantic

import "strings"

func applyRequiredLayoutCoverage(slides []SlideIR, requested []string) LayoutCoverage {
	coverage := LayoutCoverage{
		Requested: []string{},
		Assigned:  []LayoutAssignment{},
		Missing:   []string{},
	}
	seen := map[string]bool{}
	for _, raw := range requested {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		coverage.Requested = append(coverage.Requested, name)
	}
	// Find a maximum one-to-one matching, rather than taking the first
	// compatible slide greedily. A pattern slide can satisfy both content and
	// blank-title while a table may satisfy only content; first-fit can consume
	// the flexible slide and falsely report blank-title missing.
	requestForSlide := make([]int, len(slides))
	for i := range requestForSlide {
		requestForSlide[i] = -1
	}
	slideForRequest := make([]int, len(coverage.Requested))
	for i := range slideForRequest {
		slideForRequest[i] = -1
	}
	var assign func(int, []bool) bool
	assign = func(requestIndex int, visited []bool) bool {
		for slideIndex := range slides {
			if visited[slideIndex] || !layoutCompatibleWithSlide(coverage.Requested[requestIndex], slides[slideIndex]) {
				continue
			}
			visited[slideIndex] = true
			previous := requestForSlide[slideIndex]
			if previous >= 0 && !assign(previous, visited) {
				continue
			}
			requestForSlide[slideIndex] = requestIndex
			slideForRequest[requestIndex] = slideIndex
			return true
		}
		return false
	}
	for requestIndex := range coverage.Requested {
		_ = assign(requestIndex, make([]bool, len(slides)))
	}
	for requestIndex, name := range coverage.Requested {
		assigned := slideForRequest[requestIndex]
		if assigned < 0 {
			coverage.Missing = append(coverage.Missing, name)
			continue
		}
		slides[assigned].Visual.Layout = name
		coverage.Assigned = append(coverage.Assigned, LayoutAssignment{
			Layout: name, SlideIndex: assigned, SourcePath: slides[assigned].SourcePath,
		})
	}
	return coverage
}

func layoutCompatibleWithSlide(layout string, slide SlideIR) bool {
	switch layout {
	case "title":
		return slide.Kind == KindTitle
	case "section":
		return slide.Kind == KindSection
	case "closing":
		return slide.Kind == KindClosing
	case "agenda":
		return slide.Kind == KindAgenda
	case "quote":
		return slide.Kind == KindQuote
	case "image-left", "image-right":
		return slide.Kind == KindImageCase
	case "two-column", "two-column-wide-narrow", "two-column-narrow-wide":
		return slide.Kind == KindComparison || slide.Kind == KindPillars
	case "blank-title", "blank", "blank+title", "blank-canvas":
		return slide.Visual.Pattern != "" && slide.Kind != KindAgenda
	case "content":
		return slide.Role != RoleOpening && slide.Role != RoleTransition && slide.Role != RoleClosing && slide.Kind != KindRawJSON2pptx
	default:
		return false
	}
}
