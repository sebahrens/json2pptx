package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var numberedSlideImageRE = regexp.MustCompile(`(?i)^(.*slide-)([0-9]+)\.(png|jpe?g)$`)

type numberedSlideImage struct {
	path   string
	number int
}

// runInspect implements the "inspect" CLI subcommand — runs visual QA on
// rendered slide images via the same handler as the inspect_slide_images MCP
// tool. Outputs the JSON Report on stdout.
func runInspect() error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)

	imagesDir := fs.String("images", "", "Directory containing slide PNG/JPG images (slide-0.png, slide-1.png, ...)")
	slideInfoPath := fs.String("slide-info", "", "JSON file containing [{index, slide_type?, title?}] metadata (optional)")
	templateName := fs.String("template", "", "Template name to echo back on the report (optional)")
	model := fs.String("model", "", "Claude model override (default: claude-haiku-4-5-20251001)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx inspect --images <dir> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Run vision-based visual QA on rendered slide images.\n\n")
		fmt.Fprintf(os.Stderr, "Requires ANTHROPIC_API_KEY in the environment.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx inspect --images /tmp/slides/\n")
		fmt.Fprintf(os.Stderr, "  json2pptx inspect --images /tmp/slides/ --slide-info /tmp/slide-info.json --template midnight-blue\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *imagesDir == "" {
		fs.Usage()
		return fmt.Errorf("--images is required")
	}

	images, err := collectInspectImages(*imagesDir)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return fmt.Errorf("no slide images found in %s (looked for .png/.jpg/.jpeg)", *imagesDir)
	}

	args := map[string]any{
		"slide_images": images,
	}
	if *slideInfoPath != "" {
		slideInfo, err := loadInspectSlideInfo(*slideInfoPath)
		if err != nil {
			return err
		}
		args["slide_info"] = slideInfo
	}
	if *templateName != "" {
		args["deck_metadata"] = map[string]any{"template": *templateName}
	}
	if *model != "" {
		args["model"] = *model
	}

	mc := cliMCPConfig("./templates", "")
	result, err := mc.handleInspectSlideImages(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}
	return printMCPResultJSON(result)
}

func loadInspectSlideInfo(path string) ([]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read slide info: %w", err)
	}
	var list []any
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse slide info %s: expected a JSON array of {index, slide_type?, title?} objects: %w", path, err)
	}
	for i, item := range list {
		if _, ok := item.(map[string]any); !ok {
			return nil, fmt.Errorf("parse slide info %s: item %d must be an object", path, i)
		}
	}
	return list, nil
}

// collectInspectImages lists image files in dir, sorted by name, and returns
// the slide_images input entries (absolute path form) for the inspect handler.
func collectInspectImages(dir string) ([]map[string]any, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve images dir: %w", err)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read images dir: %w", err)
	}
	var files []string
	groups := make(map[string][]numberedSlideImage)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".png") && !strings.HasSuffix(name, ".jpg") && !strings.HasSuffix(name, ".jpeg") {
			continue
		}
		path := filepath.Join(abs, e.Name())
		files = append(files, path)
		if match := numberedSlideImageRE.FindStringSubmatch(e.Name()); match != nil {
			n, convErr := strconv.Atoi(match[2])
			if convErr != nil {
				return nil, fmt.Errorf("parse slide number from %q: %w", e.Name(), convErr)
			}
			prefix := strings.ToLower(match[1])
			groups[prefix] = append(groups[prefix], numberedSlideImage{path: path, number: n})
		}
	}

	selected, err := selectNumberedSlideImages(abs, groups)
	if err != nil {
		return nil, err
	}
	if selected != nil {
		files = selected
	} else {
		sort.Strings(files)
	}
	out := make([]map[string]any, len(files))
	for i, p := range files {
		out[i] = map[string]any{
			"index": i,
			"path":  p,
		}
	}
	return out, nil
}

// selectNumberedSlideImages chooses one coherent slide-image sequence from a
// directory that may also contain contact sheets, crops, row composites, or
// prior render variants. A renderer-owned directory uses
// <directory-name>-slide-N; the general CLI convention is slide-N. If neither
// preferred group exists, a sole numbered group is unambiguous. Multiple
// unmatched groups are rejected instead of silently inspecting a mixed deck.
// A nil result means there are no numbered slide groups, so callers may retain
// the legacy arbitrary-image-directory behavior.
func selectNumberedSlideImages(dir string, groups map[string][]numberedSlideImage) ([]string, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	dirPrefix := strings.ToLower(filepath.Base(dir) + "-slide-")
	selectedPrefix := ""
	switch {
	case len(groups[dirPrefix]) > 0:
		selectedPrefix = dirPrefix
	case len(groups["slide-"]) > 0:
		selectedPrefix = "slide-"
	case len(groups) == 1:
		for prefix := range groups {
			selectedPrefix = prefix
		}
	default:
		prefixes := make([]string, 0, len(groups))
		for prefix := range groups {
			prefixes = append(prefixes, prefix)
		}
		sort.Strings(prefixes)
		return nil, fmt.Errorf("ambiguous slide image groups in %s: %s; keep one numbered slide sequence or name it slide-N / %sN",
			dir, strings.Join(prefixes, ", "), dirPrefix)
	}

	images := groups[selectedPrefix]
	sort.Slice(images, func(i, j int) bool {
		if images[i].number != images[j].number {
			return images[i].number < images[j].number
		}
		return images[i].path < images[j].path
	})
	files := make([]string, len(images))
	for i, image := range images {
		if i > 0 && image.number == images[i-1].number {
			return nil, fmt.Errorf("duplicate slide number %d in image group %q", image.number, selectedPrefix)
		}
		files[i] = image.path
	}
	return files, nil
}
