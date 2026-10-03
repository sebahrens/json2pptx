package patterns

import "fmt"

// singleHighlightErrors enforces "at most one highlight": the open patterns
// spend their one fill on the single item the slide is about, so a second
// highlighted item is refused at its own path. n is the item count,
// highlighted reports whether item i sets highlight and path names its
// highlight field; noun / plural name the item kind in the message.
func singleHighlightErrors(pattern, noun, plural string, n int, highlighted func(i int) bool, path func(i int) string) []error {
	seen := false
	for i := 0; i < n; i++ {
		if !highlighted(i) {
			continue
		}
		if seen {
			return []error{newValidationError(pattern, path(i), ErrCodeInvalidShape,
				fmt.Sprintf("%s: at most one %s may set highlight; an emphasis shared by several %s is no emphasis", pattern, noun, plural), nil)}
		}
		seen = true
	}
	return nil
}
