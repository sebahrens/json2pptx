package deckinput

import (
	"fmt"
	"strings"
)

// MaxHeadingRunes is the shared limit for content and compose headings.
const MaxHeadingRunes = 60

const HeadingLimitMessage = "heading holds at most 60 characters"

// ContentTypes returns the authoring vocabulary in schema order. Callers own
// the returned slice, so diagnostic fixes cannot mutate the schema vocabulary.
func ContentTypes() []string {
	return []string{"text", "bullets", "body_and_bullets", "body_and_lead", "bullet_groups", "table", "chart", "diagram", "image"}
}

func UnknownContentTypeMessage(value string) string {
	allowed := ContentTypes()
	return fmt.Sprintf("unknown type %q (must be %s, or %s)", value, strings.Join(allowed[:len(allowed)-1], ", "), allowed[len(allowed)-1])
}
