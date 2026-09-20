package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/skills"
)

// MCP resources (go-slide-creator-fx52).
//
// The whole workflow's one deliverable — the .pptx — came back as
// pptx_path: /abs/path/on/the/server, and nothing else. resources/list,
// resources/read and resource_link content blocks all returned -32601. On
// stdio against a local server that is fine. On a containerised server, a
// remote deployment, or any sandbox where the model's file tools cannot reach
// the server's output directory, the agent could not hand the user the deck it
// had just made. The PNG side had already solved the same problem with
// ImageContent, so the shape was understood; the deck just never got it.

const (
	// pptxMIMEType is the OOXML presentation media type.
	pptxMIMEType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"

	// deckURIScheme is the prefix of a generated deck's resource URI. The rest
	// is the file name, which every generate/render response already reports as
	// part of pptx_path — so an agent can build the URI from a response it
	// already has, without a lookup table.
	deckURIScheme = "json2pptx://deck/"

	// Static resource URIs. Each answers a question agents currently pay a tool
	// call (and its tokens) for on every session, and each is cacheable by the
	// host because its content only changes when the server does.
	templatesResourceURI = "json2pptx://templates"
	patternsResourceURI  = "json2pptx://patterns"
	deckSpecResourceURI  = "json2pptx://schema/deckspec"
	skillResourceURI     = "json2pptx://skill"
)

// registerMCPResources registers the deck resource template and the static
// catalogue resources on the server.
func registerMCPResources(s *server.MCPServer, mc *mcpConfig) {
	s.AddResourceTemplate(
		mcp.NewResourceTemplate(deckURIScheme+"{filename}", "Generated deck",
			mcp.WithTemplateDescription("A .pptx this server generated, as a base64 blob. The URI is json2pptx://deck/<file name>, the name at the end of the pptx_path a generate / render response returns — so a host can offer the deck as a download without reaching into the server's filesystem."),
			mcp.WithTemplateMIMEType(pptxMIMEType),
		),
		mc.readDeckResource,
	)

	s.AddResource(
		mcp.NewResource(templatesResourceURI, "Templates",
			mcp.WithResourceDescription("The template catalogue: every registered template with its aspect ratio, layout count and table styles. The same payload list_templates returns with fields=compact, free of per-call tokens. Complete template names with completion/complete using this resource URI and argument.name=template."),
			mcp.WithMIMEType("application/json"),
		),
		mc.readTemplatesResource,
	)
	s.AddResource(
		mcp.NewResource(patternsResourceURI, "Patterns",
			mcp.WithResourceDescription("The named-pattern catalogue: every pattern with its description, cell hint and taxonomy. The same payload list_patterns returns with fields=compact. Complete names with completion/complete using this resource URI and argument.name=pattern."),
			mcp.WithMIMEType("application/json"),
		),
		readPatternsResource,
	)
	s.AddResource(
		mcp.NewResource(deckSpecResourceURI, "DeckSpec schema",
			mcp.WithResourceDescription("The DeckSpec JSON Schema: every slide kind's closed payload contract, as validate_deck_spec and render_deck_spec enforce it. Complete kind, archetype and chart_type names with this resource URI."),
			mcp.WithMIMEType("application/schema+json"),
		),
		readDeckSpecResource,
	)
	s.AddResource(
		mcp.NewResource(skillResourceURI, "Authoring skill",
			mcp.WithResourceDescription("SKILL.md: the deck-authoring guide this server ships, covering the workflow, the slide kinds, the pattern catalogue and the finding codes. Complete fix_kind and finding_code with this resource URI."),
			mcp.WithMIMEType("text/markdown"),
		),
		readSkillResource,
	)
}

// deckResourceURI returns the resource URI for a generated deck at outputPath.
func deckResourceURI(outputPath string) string {
	if outputPath == "" {
		return ""
	}
	return deckURIScheme + filepath.Base(outputPath)
}

// deckResourceLink builds the resource_link content block for a generated deck:
// the URI a host reads the blob from, plus the name and media type it needs to
// offer a download without reading the blob at all.
func deckResourceLink(outputPath string) mcp.ResourceLink {
	name := filepath.Base(outputPath)
	return mcp.NewResourceLink(
		deckResourceURI(outputPath),
		name,
		fmt.Sprintf("The generated presentation (%s). Read this resource for the file itself.", name),
		pptxMIMEType,
	)
}

// withDeckResourceLink appends the deck's resource_link to a tool result, so a
// host can surface the deliverable without a filesystem path. It is additive:
// the JSON payload the agent reads is untouched.
func withDeckResourceLink(result *mcp.CallToolResult, outputPath string) *mcp.CallToolResult {
	if result == nil || outputPath == "" || result.IsError {
		return result
	}
	result.Content = append(result.Content, deckResourceLink(outputPath))
	return result
}

// readDeckResource serves a generated deck as a base64 blob. The URI names a
// file INSIDE the server's output directory and nothing else: the name is taken
// as a base name, so a traversal in the URI cannot reach another directory.
func (mc *mcpConfig) readDeckResource(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	name := strings.TrimPrefix(request.Params.URI, deckURIScheme)
	if name == "" {
		return nil, fmt.Errorf("resource %q names no deck", request.Params.URI)
	}
	// filepath.Base defeats "../" and any absolute path in the URI; the result
	// can only ever be a file directly inside the output directory.
	name = filepath.Base(filepath.Clean("/" + name))
	if filepath.Ext(name) != ".pptx" {
		return nil, fmt.Errorf("resource %q is not a .pptx", request.Params.URI)
	}
	path := filepath.Join(mc.outputDir, name)
	data, err := os.ReadFile(path) //nolint:gosec // path is outputDir joined with a base name
	if err != nil {
		return nil, fmt.Errorf("deck %q not found in the output directory: %w", name, err)
	}
	return []mcp.ResourceContents{mcp.BlobResourceContents{
		URI:      request.Params.URI,
		MIMEType: pptxMIMEType,
		Blob:     base64.StdEncoding.EncodeToString(data),
	}}, nil
}

// readTemplatesResource serves the template catalogue as JSON.
func (mc *mcpConfig) readTemplatesResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	result, err := mc.handleListTemplates(ctx, mcpRequestWithArgs(map[string]any{
		"fields":    listFieldsCompact,
		"read_only": true,
	}))
	if err != nil {
		return nil, err
	}
	return jsonResourceContents(request.Params.URI, result)
}

// readPatternsResource serves the pattern catalogue as JSON.
func readPatternsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	result, err := handleListPatterns(ctx, mcpRequestWithArgs(map[string]any{"fields": listFieldsCompact}))
	if err != nil {
		return nil, err
	}
	return jsonResourceContents(request.Params.URI, result)
}

// readDeckSpecResource serves the DeckSpec JSON Schema.
func readDeckSpecResource(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	schema, err := semantic.SchemaJSON()
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{mcp.TextResourceContents{
		URI:      request.Params.URI,
		MIMEType: "application/schema+json",
		Text:     string(schema),
	}}, nil
}

// readSkillResource serves the authoring skill document.
func readSkillResource(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{mcp.TextResourceContents{
		URI:      request.Params.URI,
		MIMEType: "text/markdown",
		Text:     skills.SkillMarkdown(),
	}}, nil
}

// jsonResourceContents turns a tool result's structured payload into a JSON
// resource, so a resource and its tool return the same bytes.
func jsonResourceContents(uri string, result *mcp.CallToolResult) ([]mcp.ResourceContents, error) {
	if result == nil {
		return nil, fmt.Errorf("resource %q produced no content", uri)
	}
	if result.StructuredContent != nil {
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return nil, fmt.Errorf("resource %q: %w", uri, err)
		}
		return []mcp.ResourceContents{mcp.TextResourceContents{
			URI: uri, MIMEType: "application/json", Text: string(encoded),
		}}, nil
	}
	for _, c := range result.Content {
		if text, ok := c.(mcp.TextContent); ok {
			return []mcp.ResourceContents{mcp.TextResourceContents{
				URI: uri, MIMEType: "application/json", Text: text.Text,
			}}, nil
		}
	}
	return nil, fmt.Errorf("resource %q produced no readable content", uri)
}
