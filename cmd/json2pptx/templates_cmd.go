package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/sebahrens/json2pptx/internal/template"
)

// templateNameEntry is one row of `json2pptx templates`: the name an author
// puts in the deck's "template" field and just enough to choose between them.
type templateNameEntry struct {
	Name        string `json:"name"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	LayoutCount int    `json:"layout_count,omitempty"`
	TitleFont   string `json:"title_font,omitempty"`
	BodyFont    string `json:"body_font,omitempty"`
	Error       string `json:"error,omitempty"`
}

// runTemplates implements "templates": the names-only template listing.
//
// The only way to learn the template names used to be `skill-info --mode=list`
// — 84 KB, of which the names were a few hundred bytes — so an agent that
// needed nothing but a name read all of it (go-slide-creator-l7tg3). This
// prints one line per template and stays under 2 KB; `skill-info --template
// <name>` remains the detailed view of one template.
func runTemplates() error {
	fs := flag.NewFlagSet("templates", flag.ContinueOnError)
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Usage = cliDefaultUsage(fs, "templates [--templates-dir DIR] [--format json]",
		"List template names, one line each. Output size: small (under 2 KB).\nFor one template's layouts, colors and fonts: json2pptx skill-info --template <name>  (large).")
	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	entries := listTemplateNameEntries(*templatesDir)
	if *jsonOutput {
		return cliPrintJSON(map[string]any{"templates": entries})
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		if e.Error != "" {
			fmt.Fprintf(tw, "%s\t(unreadable: %s)\n", e.Name, e.Error)
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%d layouts\t%s / %s\n", e.Name, e.AspectRatio, e.LayoutCount, e.TitleFont, e.BodyFont)
	}
	return tw.Flush()
}

// listTemplateNameEntries describes every template on the search path (disk
// directory first, then the embedded set), sorted by name. No preview PNGs are
// generated and nothing is written to disk.
func listTemplateNameEntries(templatesDir string) []templateNameEntry {
	names := listAvailableTemplates(templatesDir)
	sort.Strings(names)
	cache := template.NewMemoryCache(time.Hour)
	entries := make([]templateNameEntry, 0, len(names))
	for _, name := range names {
		entry := templateNameEntry{Name: name}
		path, cleanup, err := resolveTemplatePath(name, templatesDir)
		if err != nil {
			entry.Error = err.Error()
			entries = append(entries, entry)
			continue
		}
		info, err := analyzeTemplateForSkillInfoOpts(path, cache, "list", skillInfoOptions{NoPreview: true})
		cleanup()
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.AspectRatio = info.AspectRatio
			entry.LayoutCount = info.LayoutCount
			entry.TitleFont = info.TitleFont
			entry.BodyFont = info.BodyFont
		}
		entries = append(entries, entry)
	}
	return entries
}
