package placeholder

import (
	"regexp"
	"strings"
)

// The registry of placeholder copy the product itself emits
// (go-slide-creator-327g6).
//
// plan_deck drafts carry __FILL__, recommend_visual recipes carry "Replace
// with the action title …", and a suggested patch carries "<rewrite this field
// …>". Each is scaffolding an agent is meant to overwrite. The agent journey
// review rendered a recipe verbatim and got deterministic_ready:true: the gate
// knew __FILL__ and nothing else, because every producer worded its own
// placeholder and no detector was told.
//
// Every such string is registered here, and detection reads the registry. A
// producer either builds its text with one of the constructors below or adds a
// Marker; TestProductPlaceholdersAreRegistered renders each recipe verbatim and
// fails when a producer emits copy the registry does not know.

// Marker is one registered placeholder.
type Marker struct {
	// Name is the stable identifier reported with a finding.
	Name string
	// Phrase identifies the placeholder inside a text field; matching ignores
	// ASCII case. It is the fixed part of the producer's wording.
	Phrase string
	// Source names the surface that emits it.
	Source string
}

// Marker names.
const (
	NameFillToken          = "fill_token"
	NameRecipeTitle        = "recipe_action_title"
	NameRecipeAltText      = "recipe_alt_text"
	NameRecipeSampleSource = "recipe_sample_source"
	NameRecipeCopy         = "recipe_copy"
	NameArgumentHint       = "argument_hint"
)

// recipeTitlePhrase / recipeAltPhrase / RecipeSampleSource are the recipe
// wordings; the constructors below are the only way to produce them.
const (
	recipeTitlePhrase = "Replace with the action title"
	recipeAltPhrase   = "Replace with one sentence saying what"
	// RecipeSampleSource is the source line a recipe with illustrative data
	// carries.
	RecipeSampleSource = "Illustrative sample data; replace with the real source"
)

// RecipeActionTitle is the title a recommend_visual recipe slide carries until
// the author writes the real one. label names the visual ("bar chart").
func RecipeActionTitle(label string) string {
	return recipeTitlePhrase + " this " + label + " supports"
}

// RecipeAltText is the alt text a recipe's chart or diagram carries until the
// author describes it.
func RecipeAltText(label string) string {
	return recipeAltPhrase + " this " + label + " shows"
}

// recipeCopyPrefix opens every line of body copy a recipe stands in for.
const recipeCopyPrefix = "Replace with "

// Recipe copy slots: what a line of recipe body copy stands in for. A recipe
// writes the line with RecipeCopy(slot); Detect knows exactly these.
const (
	SlotTakeaway     = "the one-line takeaway."
	SlotChartShows   = "what the chart shows."
	SlotWhyItMatters = "why it matters."
	SlotFirstPoint   = "the first point, one line"
	SlotSecondPoint  = "the second point"
	SlotThirdPoint   = "the third point"
	SlotLeftFirst    = "the first left point"
	SlotLeftSecond   = "the second left point"
	SlotRightFirst   = "the first right point"
	SlotRightSecond  = "the second right point"
	SlotFirstBlock   = "the first block"
	SlotSecondBlock  = "the second block"
	SlotThirdBlock   = "the third block"
	SlotFourthBlock  = "the fourth block"
)

// recipeCopySlots is the set Detect matches, lower-cased.
var recipeCopySlots = map[string]bool{
	SlotTakeaway: true, SlotChartShows: true, SlotWhyItMatters: true,
	SlotFirstPoint: true, SlotSecondPoint: true, SlotThirdPoint: true,
	SlotLeftFirst: true, SlotLeftSecond: true, SlotRightFirst: true, SlotRightSecond: true,
	SlotFirstBlock: true, SlotSecondBlock: true, SlotThirdBlock: true, SlotFourthBlock: true,
}

// RecipeCopy is a line of body copy a recipe carries until the author writes
// it: an insight, a bullet, a takeaway, a block's text. slot is one of the
// Slot constants; a line built from any other text is not detected, which
// TestRecommendVisualRecipeCopyIsRegistered fails on.
func RecipeCopy(slot string) string {
	return recipeCopyPrefix + slot
}

// registered lists every phrase-identified placeholder. The __FILL__ token and
// the "<…>" argument-hint shape are matched structurally; see Detect.
var registered = []Marker{
	{Name: NameFillToken, Phrase: Token, Source: "plan_deck drafts and pattern skeletons"},
	{Name: NameRecipeTitle, Phrase: recipeTitlePhrase, Source: "recommend_visual recipes"},
	{Name: NameRecipeAltText, Phrase: recipeAltPhrase, Source: "recommend_visual recipes"},
	{Name: NameRecipeSampleSource, Phrase: RecipeSampleSource, Source: "recommend_visual recipes"},
}

// argumentHint is the last registered marker: a text field that is nothing but
// an angle-bracketed instruction, the shape of every args_template hint and
// suggested-patch value ("<rewrite this field to resolve … >", "<the point
// this slide makes>").
var argumentHint = Marker{Name: NameArgumentHint, Phrase: "<…>", Source: "next_tool_call args_template and suggested patch values"}

// recipeCopy is matched as a whole field too: "Replace with " and one of the
// registered slots. Authored copy may well open with those two words
// ("Replace with usage-based pricing"), so the prefix alone is not a marker.
var recipeCopy = Marker{Name: NameRecipeCopy, Phrase: recipeCopyPrefix + "…", Source: "recommend_visual recipes"}

// argumentHintRE matches a whole field that is one "<instruction>": it opens
// with a letter, so "<5%" and "< 10 days" are not hints, and holds no further
// angle bracket, so markup is not one either.
var argumentHintRE = regexp.MustCompile(`^<[A-Za-z][^<>]{2,}>$`)

// Registered returns every registered marker, in a stable order.
func Registered() []Marker {
	out := make([]Marker, 0, len(registered)+1)
	out = append(out, registered...)
	return append(out, recipeCopy, argumentHint)
}

// Detect reports the registered placeholder a text field still carries. A
// phrase matches anywhere in the field, so a partly edited string ("Q3
// __FILL__ results") is caught; a line of recipe copy and an argument hint
// match only as the whole field.
func Detect(s string) (Marker, bool) {
	if s == "" {
		return Marker{}, false
	}
	lower := strings.ToLower(s)
	for _, m := range registered {
		if strings.Contains(lower, strings.ToLower(m.Phrase)) {
			return m, true
		}
	}
	field := strings.TrimSpace(lower)
	if slot, ok := strings.CutPrefix(field, strings.ToLower(recipeCopyPrefix)); ok && recipeCopySlots[slot] {
		return recipeCopy, true
	}
	if argumentHintRE.MatchString(strings.TrimSpace(s)) {
		return argumentHint, true
	}
	return Marker{}, false
}
