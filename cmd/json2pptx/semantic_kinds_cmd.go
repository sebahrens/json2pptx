package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// semanticKindsResult is the list_slide_kinds payload as the CLI reads it. The
// CLI catalogue is not a second catalogue: runSemanticKinds calls the
// list_slide_kinds handler and renders what it returns, so the two cannot
// drift (TestSemanticKindsMatchesListSlideKinds pins it).
type semanticKindsResult struct {
	SlideKinds     []slideKindListEntry  `json:"slide_kinds"`
	TakeawayBudget map[string]any        `json:"takeaway_budget,omitempty"`
	BudgetBasis    *slideKindBudgetBasis `json:"budget_basis,omitempty"`
}

// semanticKindSummary is one row of `semantic kinds --format json`: the
// catalogue without the per-kind examples, which `semantic kinds <kind>` adds.
type semanticKindSummary struct {
	Kind           string   `json:"kind"`
	Summary        string   `json:"summary"`
	RequiredFields []string `json:"required_fields,omitempty"`
}

// runSemanticKinds implements "semantic kinds [<kind>]": the CLI counterpart
// of list_slide_kinds (go-slide-creator-e7jxr).
//
// DeckSpec discovery on the CLI used to be `semantic schema` — 172 KB of JSON
// Schema with no examples — so an agent told to author a DeckSpec filtered it
// with a script before it could write a slide. `semantic kinds` lists every
// kind on one line; `semantic kinds <kind>` prints that kind's fields, budgets
// and a copy-ready example.
func runSemanticKinds() error {
	fs := flag.NewFlagSet("semantic kinds", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	templateName := fs.String("template", "", "Measure the title, subtitle and takeaway budgets on this template (default: the tightest across the shipped templates)")
	templatesDir := fs.String("templates-dir", "", "Directory holding the template (default: the standard search path)")
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage: json2pptx semantic kinds [<kind>] [--template <name>] [--format json]\n\n")
		fmt.Fprintf(out, "List the DeckSpec slide kinds, or describe one.\n\n")
		fmt.Fprintf(out, "  json2pptx semantic kinds                 every kind with a one-line summary (small: ~3 KB)\n")
		fmt.Fprintf(out, "  json2pptx semantic kinds kpi_snapshot    fields, budgets and a copy-ready example (small: 2-5 KB)\n")
		fmt.Fprintf(out, "  json2pptx semantic kinds closing --template midnight-blue   budgets measured on that template\n")
		fmt.Fprintf(out, "  json2pptx semantic kinds kpi_snapshot --format json\n\n")
		fmt.Fprintf(out, "Same catalogue as the list_slide_kinds MCP tool. The full JSON Schema is\n")
		fmt.Fprintf(out, "'json2pptx semantic schema' (large: ~170 KB).\n\n")
		fmt.Fprintf(out, "Options:\n")
		printDoubleDashUsage(fs)
	}
	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("semantic kinds: at most one <kind> is accepted, got %d", fs.NArg())
	}

	args := map[string]any{}
	if fs.NArg() == 1 {
		args["kinds"] = []any{fs.Arg(0)}
		args["fields"] = []any{"item_schema", "compositions", "budgets", "example"}
		if *jsonOutput {
			// A script validating a slide needs every accepted key.
			args["fields"] = []any{"item_schema", "item_schema_full", "compositions", "budgets", "example"}
		}
		if *templateName != "" {
			args["template"] = *templateName
		}
	}
	result, err := (&mcpConfig{templatesDir: *templatesDir, cache: newBudgetTemplateCache()}).handleListSlideKinds(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("semantic kinds: %w", err)
	}
	if result.IsError {
		return printMCPResultJSON(result)
	}
	var catalogue semanticKindsResult
	if err := json.Unmarshal([]byte(cliResultText(result)), &catalogue); err != nil {
		return fmt.Errorf("semantic kinds: %w", err)
	}

	if fs.NArg() == 1 {
		if len(catalogue.SlideKinds) != 1 {
			return fmt.Errorf("semantic kinds: unknown kind %q — run 'json2pptx semantic kinds' for the list", fs.Arg(0))
		}
		if *jsonOutput {
			return printJSONIndent(map[string]any{
				"slide_kind":      catalogue.SlideKinds[0],
				"takeaway_budget": catalogue.TakeawayBudget,
				"budget_basis":    catalogue.BudgetBasis,
			})
		}
		return writeSemanticKindDetail(os.Stdout, catalogue.SlideKinds[0], catalogue.TakeawayBudget, catalogue.BudgetBasis)
	}

	if *jsonOutput {
		rows := make([]semanticKindSummary, 0, len(catalogue.SlideKinds))
		for _, k := range catalogue.SlideKinds {
			rows = append(rows, semanticKindSummary{Kind: k.Kind, Summary: oneLineSummary(k.Summary), RequiredFields: k.RequiredFields})
		}
		return cliPrintJSON(map[string]any{
			"slide_kinds": rows,
			"detail":      "json2pptx semantic kinds <kind>",
		})
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, k := range catalogue.SlideKinds {
		fmt.Fprintf(tw, "%s\t%s\n", k.Kind, oneLineSummary(k.Summary))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, "\nFields, budgets and a copy-ready example: json2pptx semantic kinds <kind>")
	return err
}

// oneLineSummary cuts a kind's summary to its first sentence (at most ~110
// characters) for the listing; `semantic kinds <kind>` prints it in full.
func oneLineSummary(summary string) string {
	s := strings.Join(strings.Fields(summary), " ")
	for _, stop := range []string{". ", ": ", " — "} {
		if i := strings.Index(s, stop); i > 0 && i < len(s)-len(stop) {
			s = s[:i]
			if stop == ". " {
				s += "."
			}
			break
		}
	}
	const limit = 110
	if r := []rune(s); len(r) > limit {
		s = strings.TrimRight(string(r[:limit-1]), " ,;") + "…"
	}
	return s
}

// writeSemanticKindDetail prints one kind for a human or an agent reading
// text: what it is for, each field with its type and budget, the compositions
// its pattern/layout override accepts, and an example slide to paste under
// `slides:`.
func writeSemanticKindDetail(w io.Writer, k slideKindListEntry, takeawayBudget map[string]any, basis *slideKindBudgetBasis) error {
	fmt.Fprintf(w, "%s — %s\n\n", k.Kind, k.Summary)
	if len(k.RequiredFields) > 0 {
		fmt.Fprintf(w, "Required: %s\n", strings.Join(k.RequiredFields, ", "))
	}
	if len(k.TypicalFields) > 0 {
		fmt.Fprintf(w, "Typical:  %s\n", strings.Join(k.TypicalFields, ", "))
	}

	required := map[string]bool{}
	for _, f := range k.RequiredFields {
		required[f] = true
	}
	props, _ := k.ItemSchema["properties"].(map[string]any)
	if len(props) > 0 {
		names := make([]string, 0, len(props))
		for name := range props {
			names = append(names, name)
		}
		// Required fields first, then the rest alphabetically.
		sort.Slice(names, func(i, j int) bool {
			if required[names[i]] != required[names[j]] {
				return required[names[i]]
			}
			return names[i] < names[j]
		})
		fmt.Fprintf(w, "\nFields (type; budget; meaning):\n")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, name := range names {
			if name == "kind" {
				continue // the discriminator, not a payload field
			}
			prop, _ := props[name].(map[string]any)
			label := name
			if required[name] {
				label += " *"
			}
			meaning := schemaDescription(prop)
			if aliases := schemaAliases(prop); aliases != "" {
				meaning = strings.TrimSpace(meaning + " (also written: " + aliases + ")")
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", label, schemaTypeLabel(prop), schemaBudgetLabel(prop), meaning)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(w, "  (* required)\n")
	}
	if note, _ := takeawayBudget["note"].(string); note != "" {
		fmt.Fprintf(w, "\nTakeaway budget: %s\n", strings.ReplaceAll(note, "validate_deck_spec", "'json2pptx semantic validate'"))
	}

	if err := writeSemanticKindBudgets(w, k.Budgets, basis); err != nil {
		return err
	}

	if len(k.Compositions) > 0 {
		fmt.Fprintf(w, "\nCompositions (optional pattern / layout override):\n")
		for _, c := range k.Compositions {
			name := c.Pattern
			if name == "" {
				name = "layout: " + c.Layout
			} else {
				name = "pattern: " + name
			}
			fmt.Fprintf(w, "  %s — %s\n", name, c.Reason)
		}
	}

	example, err := semanticKindExampleYAML(k.Example)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "\nExample (paste under `slides:` in a DeckSpec; validates with zero findings):\n\n%s", example)
	return nil
}

// schemaAliases joins a compact schema property's "aliases" annotation.
func schemaAliases(prop map[string]any) string {
	list, _ := prop["aliases"].([]any)
	names := make([]string, 0, len(list))
	for _, a := range list {
		if s, ok := a.(string); ok {
			names = append(names, s)
		}
	}
	return strings.Join(names, ", ")
}

// writeSemanticKindBudgets prints a kind's per-field budgets and what the
// measured ones were measured on.
func writeSemanticKindBudgets(w io.Writer, budgets []slideKindBudget, basis *slideKindBudgetBasis) error {
	if len(budgets) == 0 {
		return nil
	}
	on := "the tightest across the shipped templates"
	if basis != nil && basis.Template != "" {
		on = "template " + basis.Template
	}
	fmt.Fprintf(w, "\nBudgets (measured on %s; fixed ones hold on every template):\n", on)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, b := range budgets {
		var parts []string
		if b.MaxChars > 0 {
			parts = append(parts, fmt.Sprintf("≤%d chars", b.MaxChars))
		}
		if b.MaxCharsPerLine > 0 {
			parts = append(parts, fmt.Sprintf("≤%d per line", b.MaxCharsPerLine))
		}
		if b.MaxLines > 0 {
			parts = append(parts, fmt.Sprintf("%d line(s)", b.MaxLines))
		}
		switch {
		case b.MinItems > 0 && b.MaxItems > 0:
			parts = append(parts, fmt.Sprintf("%d–%d items", b.MinItems, b.MaxItems))
		case b.MaxItems > 0:
			parts = append(parts, fmt.Sprintf("≤%d items", b.MaxItems))
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", b.Field, strings.Join(parts, ", "), b.Basis, b.Note)
	}
	return tw.Flush()
}

// schemaTypeLabel names a property's type the way an author thinks of it.
func schemaTypeLabel(prop map[string]any) string {
	if ref, _ := prop["$ref"].(string); ref != "" {
		return "alias of " + ref[strings.LastIndexByte(ref, '/')+1:]
	}
	typ := fmt.Sprint(prop["type"])
	if prop["type"] == nil {
		if _, ok := prop["oneOf"]; ok {
			return "one of several shapes"
		}
		if _, ok := prop["anyOf"]; ok {
			return "one of several shapes"
		}
		return "any"
	}
	if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
		vals := make([]string, 0, len(enum))
		for _, v := range enum {
			vals = append(vals, fmt.Sprint(v))
		}
		return strings.Join(vals, "|")
	}
	if typ == "array" {
		if items, ok := prop["items"].(map[string]any); ok {
			if it := fmt.Sprint(items["type"]); items["type"] != nil {
				return "list of " + it
			}
		}
		return "list"
	}
	return typ
}

// schemaBudgetLabel renders the limits a property declares (text length, item
// count, numeric range), or "-" when it declares none.
func schemaBudgetLabel(prop map[string]any) string {
	var parts []string
	num := func(key string) (string, bool) {
		v, ok := prop[key]
		if !ok {
			return "", false
		}
		return fmt.Sprint(v), true
	}
	if v, ok := num("maxLength"); ok {
		parts = append(parts, "max "+v+" chars")
	}
	minItems, hasMin := num("minItems")
	maxItems, hasMax := num("maxItems")
	switch {
	case hasMin && hasMax:
		parts = append(parts, minItems+"-"+maxItems+" items")
	case hasMax:
		parts = append(parts, "max "+maxItems+" items")
	case hasMin:
		parts = append(parts, "min "+minItems+" items")
	}
	if v, ok := num("minimum"); ok {
		parts = append(parts, "min "+v)
	}
	if v, ok := num("maximum"); ok {
		parts = append(parts, "max "+v)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// schemaDescription returns a property's description on one line.
func schemaDescription(prop map[string]any) string {
	desc, _ := prop["description"].(string)
	return strings.Join(strings.Fields(desc), " ")
}

// semanticKindExampleYAML renders a kind's example slide as a YAML list item
// with the fields in authoring order (kind, id, title, takeaway, then the
// rest alphabetically), ready to paste under `slides:`.
func semanticKindExampleYAML(example map[string]any) (string, error) {
	lead := []string{"kind", "id", "title", "takeaway"}
	isLead := map[string]bool{}
	for _, name := range lead {
		isLead[name] = true
	}
	keys := make([]string, 0, len(example))
	for _, name := range lead {
		if _, ok := example[name]; ok {
			keys = append(keys, name)
		}
	}
	rest := make([]string, 0, len(example))
	for name := range example {
		if !isLead[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	keys = append(keys, rest...)

	slide := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range keys {
		var value yaml.Node
		if err := value.Encode(example[name]); err != nil {
			return "", fmt.Errorf("semantic kinds: encode example field %s: %w", name, err)
		}
		slide.Content = append(slide.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, &value)
	}
	doc := &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{slide}}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", fmt.Errorf("semantic kinds: encode example: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
