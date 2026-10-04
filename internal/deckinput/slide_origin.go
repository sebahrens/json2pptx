package deckinput

import "strconv"

// SlideOrigin says where one slide of the expanded deck was authored
// (go-slide-creator-9564b, go-slide-creator-2v8me).
//
// The engine works on a flat slide list: a split_slide entry becomes one slide
// per page when the deck is decoded, and a structure block becomes its cover,
// agenda, section dividers, section slides and closing. Every check addresses
// a slide by its index in that list, which is the number of the rendered slide
// but not a place in the deck the author wrote. The origin is the way back: it
// is set on every slide of a deck whose list is not the one the author wrote,
// and is nil on every slide of a deck that is (the slide's index is then its
// address).
type SlideOrigin struct {
	// Pointer is the JSON Pointer of the authored object the slide was built
	// from: "/slides/1/base" for a page of a split_slide, "/slides/2" for a
	// slide that follows one, "/structure/sections/0/slides/3",
	// "/structure/cover". For a slide the engine built (Generated) it is the
	// authored element that asks for it: "/structure/sections/0" for a
	// section divider, "/structure/auto_agenda" for the agenda.
	Pointer string

	// Generated marks a slide with no authored slide object. Fields maps the
	// parts of it that show an authored value (the divider's title) to that
	// value's pointer, keyed by the path inside the slide ("/content/0");
	// anything else on the slide is addressed at Pointer.
	Generated bool
	Fields    map[string]string

	// SplitPage marks a page of a split_slide. TableContent is the index of
	// the content block whose table rows the pages window, TableField the
	// authored key that holds the table ("table_value", or the legacy
	// "value"), and RowOffset the index, in the authored table, of the page's
	// first row.
	SplitPage    bool
	TableContent int
	TableField   string
	RowOffset    int
}

// markSlideOrigins gives every slide of p its origin once the list is known to
// differ from the authored one. pointers is index-aligned with p.Slides; a
// slide that already carries an origin (a split page) keeps its page fields.
func markSlideOrigins(slides []SlideInput, pointers []string) {
	for i := range slides {
		if i >= len(pointers) {
			return
		}
		origin := SlideOrigin{}
		if slides[i].Origin != nil {
			origin = *slides[i].Origin
		}
		origin.Pointer = pointers[i]
		slides[i].Origin = &origin
	}
}

// KeepSlideAddresses records each slide's present index as its authored
// address, for a caller about to remove slides from the list (partial mode):
// the slides that remain are still reported where the author wrote them. A
// slide that already has an origin keeps it.
func KeepSlideAddresses(slides []SlideInput) {
	for i := range slides {
		if slides[i].Origin == nil {
			slides[i].Origin = &SlideOrigin{Pointer: slidesPointer(i)}
		}
	}
}

// ClearSlideOrigins drops every slide's origin: the slide list has become the
// authored one (a tool that returns the expanded deck as its patched deck).
func ClearSlideOrigins(slides []SlideInput) {
	for i := range slides {
		slides[i].Origin = nil
	}
}

func slidesPointer(i int) string { return "/slides/" + strconv.Itoa(i) }
