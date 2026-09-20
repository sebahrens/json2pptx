package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Quote slides (go-slide-creator-ze1p).
//
// Voice-of-customer is a staple of every research readout and board pack, and
// the DeckSpec had no kind for it: an agent reaching for a quote wrote it into
// a title, or dropped to raw_json2pptx and carried the whole pattern block by
// hand. Both quote visuals already exist — pull-quote for one, quote-cluster
// for several — so this is a compiler over them, not new rendering.
//
// The count decides the visual: one quote is a pull-quote, three to eight are
// a cluster. Two quotes use the content fallback because the cluster pattern
// requires at least three.

const (
	// Pull-quote budgets, from the pattern's own schema.
	pullQuoteTextMax = 500
	pullQuoteNameMax = 60
	pullQuoteRoleMax = 60
	// Quote-cluster budgets and counts, from the pattern's own constants.
	quoteClusterTextMax  = 240
	quoteClusterNameMax  = 60
	quoteClusterRoleMax  = 80
	quoteClusterMaxCount = 8
)

// Quote is one attributed quotation as an author writes it.
type Quote struct {
	Text string
	Name string
	Role string
}

// pullQuoteValues is the pull-quote pattern's values object.
type pullQuoteValues struct {
	Quote       string `json:"quote"`
	Attribution string `json:"attribution"`
	Role        string `json:"role,omitempty"`
}

// quoteClusterItem / quoteClusterValues are the quote-cluster pattern's.
type quoteClusterItem struct {
	Text  string `json:"text"`
	Name  string `json:"name"`
	Title string `json:"title,omitempty"`
}

type quoteClusterValues struct {
	Quotes []quoteClusterItem `json:"quotes"`
}

// CompileQuote compiles a quote payload onto pull-quote (one quote) or
// quote-cluster (several), falling back to a content slide when the payload is
// outside what either pattern can hold.
func CompileQuote(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	quotes := Quotes(in.Body)
	if QuoteOverBudget(in.Body) != "" || len(quotes) == 0 {
		return compileQuoteFallback(in, quotes)
	}

	if len(quotes) == 1 {
		return compilePullQuote(in, quotes[0])
	}
	return compileQuoteCluster(in, quotes)
}

func compilePullQuote(in Input, q Quote) (*deckinput.SlideInput, []SourceLink, error) {
	encoded, err := json.Marshal(pullQuoteValues{
		Quote:       q.Text,
		Attribution: q.Name,
		Role:        q.Role,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal pull-quote values: %w", err)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "pull-quote", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.quote",
		SemanticPath: in.semSlide() + "." + quoteFieldName(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

func compileQuoteCluster(in Input, quotes []Quote) (*deckinput.SlideInput, []SourceLink, error) {
	items := make([]quoteClusterItem, 0, len(quotes))
	for _, q := range quotes {
		items = append(items, quoteClusterItem{Text: q.Text, Name: q.Name, Title: q.Role})
	}
	encoded, err := json.Marshal(quoteClusterValues{Quotes: items})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal quote-cluster values: %w", err)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "quote-cluster", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.quotes",
		SemanticPath: in.semSlide() + "." + quoteFieldName(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileQuoteFallback renders the quotes as an attributed bullet list. The
// content is never lost — only the visual — and validateQuote reports the
// degrade so the author is told rather than left to notice.
func compileQuoteFallback(in Input, quotes []Quote) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := make([]string, 0, len(quotes))
	for _, q := range quotes {
		line := "“" + q.Text + "”"
		if q.Name != "" {
			line += " — " + q.Name
			if q.Role != "" {
				line += ", " + q.Role
			}
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{
		RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + "." + quoteFieldName(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// Quotes extracts the quote list from a payload. It accepts the canonical
// `quotes` array, its aliases, and the single-quote shorthand where `quote` is
// a bare string with `attribution` / `role` beside it — which is how anyone
// writes ONE quote.
func Quotes(body map[string]any) []Quote {
	field := quoteFieldName(body)
	switch field {
	case "quotes", "testimonials", "voices":
		return quotesFromList(body[field])
	}
	// Single-quote shorthand.
	text := strField(body, field)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	q := Quote{Text: strings.TrimSpace(text)}
	q.Name = firstNonEmptyField(body, "attribution", "name", "speaker", "author")
	q.Role = firstNonEmptyField(body, "role")
	return []Quote{q}
}

// quotesFromList reads a quotes array. Each entry is an object, or a bare
// string when the author has no attribution to give.
func quotesFromList(raw any) []Quote {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]Quote, 0, len(list))
	for _, item := range list {
		switch v := item.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				out = append(out, Quote{Text: s})
			}
		case map[string]any:
			q := Quote{
				Text: strings.TrimSpace(firstNonEmptyField(v, "text", "quote")),
				Name: firstNonEmptyField(v, "name", "attribution", "speaker", "author"),
				Role: firstNonEmptyField(v, "role", "title"),
			}
			if q.Text != "" {
				out = append(out, q)
			}
		}
	}
	return out
}

// firstNonEmptyField returns the first of names that holds a non-empty string.
func firstNonEmptyField(body map[string]any, names ...string) string {
	for _, n := range names {
		if s := strField(body, n); s != "" {
			return s
		}
	}
	return ""
}

// quoteFieldName reports which key the author actually used, so a finding
// points at what they wrote rather than the canonical name.
func quoteFieldName(body map[string]any) string {
	for _, n := range []string{"quotes", "testimonials", "voices", "quote", "text"} {
		switch v := body[n].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return n
			}
		case []any:
			if len(v) > 0 {
				return n
			}
		}
	}
	for _, n := range []string{"quotes", "testimonials", "voices", "quote", "text"} {
		if _, ok := body[n]; ok {
			return n
		}
	}
	return "quotes"
}

// QuoteFieldName names the authored quote field for semantic findings.
func QuoteFieldName(body map[string]any) string { return quoteFieldName(body) }

// UsableQuoteCount is how many quotes survive extraction.
func UsableQuoteCount(body map[string]any) int { return len(Quotes(body)) }

// QuoteOverBudget reports why the quotes cannot take a quote visual, or "" when
// they can. The budgets are the patterns' own: past them the pattern refuses
// the slide, so the compiler degrades rather than handing it a payload it will
// reject.
func QuoteOverBudget(body map[string]any) string {
	quotes := Quotes(body)
	switch {
	case len(quotes) == 0:
		return ""
	case len(quotes) == 2:
		return "has 2 quotes; quote-cluster needs at least 3"
	case len(quotes) > quoteClusterMaxCount:
		return fmt.Sprintf("has %d quotes; quote-cluster draws at most %d", len(quotes), quoteClusterMaxCount)
	}

	textMax, nameMax, roleMax := quoteClusterTextMax, quoteClusterNameMax, quoteClusterRoleMax
	visual := "quote-cluster"
	if len(quotes) == 1 {
		textMax, nameMax, roleMax = pullQuoteTextMax, pullQuoteNameMax, pullQuoteRoleMax
		visual = "pull-quote"
	}
	for i, q := range quotes {
		switch {
		case q.Name == "":
			return fmt.Sprintf("quote %d has no attribution; %s requires a speaker name", i+1, visual)
		case runeLen(q.Text) > textMax:
			return fmt.Sprintf("quote %d is %d characters; %s holds %d", i+1, runeLen(q.Text), visual, textMax)
		case runeLen(q.Name) > nameMax:
			return fmt.Sprintf("quote %d's attribution is %d characters; %s holds %d", i+1, runeLen(q.Name), visual, nameMax)
		case runeLen(q.Role) > roleMax:
			return fmt.Sprintf("quote %d's role is %d characters; %s holds %d", i+1, runeLen(q.Role), visual, roleMax)
		}
	}
	return ""
}
