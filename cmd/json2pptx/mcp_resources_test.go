package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// go-slide-creator-fx52. The workflow's one deliverable — the .pptx — came back
// as pptx_path: /abs/path/on/the/server and nothing else: resources/list,
// resources/read and resource_link all returned -32601. On a containerised or
// remote server, or any sandbox where the model's file tools cannot reach the
// server's output directory, the agent could not hand the user the deck it had
// just made.

func resourceRequest(uri string) mcp.ReadResourceRequest {
	var req mcp.ReadResourceRequest
	req.Params.URI = uri
	return req
}

func TestDeckResource_ServesTheFileItself(t *testing.T) {
	dir := t.TempDir()
	want := []byte("PK\x03\x04 pretend this is a deck")
	if err := os.WriteFile(filepath.Join(dir, "deck.pptx"), want, 0o600); err != nil {
		t.Fatalf("write deck: %v", err)
	}
	mc := &mcpConfig{outputDir: dir}

	contents, err := mc.readDeckResource(context.Background(), resourceRequest("json2pptx://deck/deck.pptx"))
	if err != nil {
		t.Fatalf("readDeckResource: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("want one content, got %d", len(contents))
	}
	blob, ok := contents[0].(mcp.BlobResourceContents)
	if !ok {
		t.Fatalf("content is %T, want a blob", contents[0])
	}
	if blob.MIMEType != pptxMIMEType {
		t.Errorf("mimeType = %q, want the OOXML presentation type", blob.MIMEType)
	}
	got, err := base64.StdEncoding.DecodeString(blob.Blob)
	if err != nil {
		t.Fatalf("blob is not base64: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("blob = %q, want the file bytes", got)
	}
	// The blob's digest is what the response's content_hash reports, so an
	// agent can verify it got the deck it was told about.
	sum := sha256.Sum256(got)
	if hex.EncodeToString(sum[:]) != fileSHA256(t, filepath.Join(dir, "deck.pptx")) {
		t.Error("blob digest does not match the file on disk")
	}
}

// The URI names a file inside the output directory and nothing else.
func TestDeckResource_RefusesEverythingElse(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deck.pptx"), []byte("deck"), 0o600); err != nil {
		t.Fatalf("write deck: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "secret.pptx")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	mc := &mcpConfig{outputDir: dir}

	for _, uri := range []string{
		"json2pptx://deck/",
		"json2pptx://deck/notes.txt",
		"json2pptx://deck/" + outside,
		"json2pptx://deck/../" + filepath.Base(outside),
		"json2pptx://deck/../../etc/passwd",
		"json2pptx://deck/absent.pptx",
	} {
		if _, err := mc.readDeckResource(context.Background(), resourceRequest(uri)); err == nil {
			t.Errorf("%s was served; it must not be", uri)
		}
	}
}

func TestDeckResourceLink(t *testing.T) {
	link := deckResourceLink("/srv/output/board-deck.pptx")
	if link.URI != "json2pptx://deck/board-deck.pptx" {
		t.Errorf("uri = %q", link.URI)
	}
	if link.Name != "board-deck.pptx" || link.MIMEType != pptxMIMEType {
		t.Errorf("link = %+v", link)
	}

	// The link rides alongside the JSON payload; it never replaces it, and a
	// failed call gets none.
	result := &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("{}")}}
	withDeckResourceLink(result, "/srv/output/board-deck.pptx")
	if len(result.Content) != 2 {
		t.Fatalf("content blocks = %d, want the text plus the link", len(result.Content))
	}
	if _, ok := result.Content[1].(mcp.ResourceLink); !ok {
		t.Errorf("second block is %T, want a resource link", result.Content[1])
	}

	failed := &mcp.CallToolResult{IsError: true, Content: []mcp.Content{mcp.NewTextContent("{}")}}
	withDeckResourceLink(failed, "/srv/output/board-deck.pptx")
	if len(failed.Content) != 1 {
		t.Error("a failed call must not advertise a deck resource")
	}
	empty := &mcp.CallToolResult{}
	withDeckResourceLink(empty, "")
	if len(empty.Content) != 0 {
		t.Error("no output path means no link")
	}
}

// The static resources answer what agents currently spend tool calls on, so
// each has to serve real content rather than an empty envelope.
func TestStaticResourcesServeContent(t *testing.T) {
	mc := profileTestConfig(t)
	ctx := context.Background()

	tests := []struct {
		uri      string
		read     func() ([]mcp.ResourceContents, error)
		mime     string
		contains string
	}{
		{
			uri: templatesResourceURI,
			read: func() ([]mcp.ResourceContents, error) {
				return mc.readTemplatesResource(ctx, resourceRequest(templatesResourceURI))
			},
			mime: "application/json", contains: "midnight-blue",
		},
		{
			uri: patternsResourceURI,
			read: func() ([]mcp.ResourceContents, error) {
				return readPatternsResource(ctx, resourceRequest(patternsResourceURI))
			},
			mime: "application/json", contains: "kpi-3up",
		},
		{
			uri: deckSpecResourceURI,
			read: func() ([]mcp.ResourceContents, error) {
				return readDeckSpecResource(ctx, resourceRequest(deckSpecResourceURI))
			},
			mime: "application/schema+json", contains: "option_matrix",
		},
		{
			uri: skillResourceURI,
			read: func() ([]mcp.ResourceContents, error) {
				return readSkillResource(ctx, resourceRequest(skillResourceURI))
			},
			mime: "text/markdown", contains: "render_deck_spec",
		},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			contents, err := tt.read()
			if err != nil {
				t.Fatalf("read %s: %v", tt.uri, err)
			}
			if len(contents) != 1 {
				t.Fatalf("want one content, got %d", len(contents))
			}
			text, ok := contents[0].(mcp.TextResourceContents)
			if !ok {
				t.Fatalf("content is %T, want text", contents[0])
			}
			if text.URI != tt.uri || text.MIMEType != tt.mime {
				t.Errorf("uri/mime = %q/%q, want %q/%q", text.URI, text.MIMEType, tt.uri, tt.mime)
			}
			if !strings.Contains(text.Text, tt.contains) {
				t.Errorf("%s does not carry %q (len %d)", tt.uri, tt.contains, len(text.Text))
			}
		})
	}
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
