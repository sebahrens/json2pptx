package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/pptxread"
	"github.com/sebahrens/json2pptx/internal/render"
)

type exportDeckResult struct {
	Format     string `json:"format"`
	OutputPath string `json:"output_path"`
	Bytes      int64  `json:"bytes"`
	SlideCount int    `json:"slide_count,omitempty"`
}

func mcpExportDeckTool() mcp.Tool {
	return mcp.NewTool("export_deck",
		mcp.WithDescription("Export an existing PPTX as a retained PDF or Markdown speaker-notes handout. PDF uses LibreOffice; notes use the PPTX reader and need no rendering tools. Returns the absolute output_path and file size. Output filenames include the PPTX content hash so different deck versions do not overwrite one another."),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaExportDeck)),
		mcp.WithString("pptx_path", mcp.Required(), mcp.Description("Path to the existing PPTX file.")),
		mcp.WithString("format", mcp.Required(), mcp.Enum("pdf", "notes"), mcp.Description("pdf for a review copy, notes for a Markdown speaker handout.")),
	)
}

func (mc *mcpConfig) handleExportDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pptxPath, err := request.RequireString("pptx_path")
	if err != nil {
		return argRequired(request, "export_deck", "pptx_path", "string", "/tmp/deck.pptx", nil), nil
	}
	if err := api.ValidatePptxPath(pptxPath); err != nil {
		return argInvalidValue("export_deck", diagnostics.CodeInvalidPath, "pptx_path", err.Error(), "string", "/tmp/deck.pptx", nil), nil
	}
	if _, err := os.Stat(pptxPath); errors.Is(err, os.ErrNotExist) {
		return mcpFileNotFoundError("export_deck", "pptx_path", pptxPath), nil
	}
	format, err := request.RequireString("format")
	if err != nil {
		return argRequired(request, "export_deck", "format", "string", "pdf", nil), nil
	}
	if format != "pdf" && format != "notes" {
		return argInvalidValue("export_deck", diagnostics.CodeInvalidParameter, "format", "format must be pdf or notes", "string", "pdf", nil), nil
	}
	if err := ctx.Err(); err != nil {
		return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error()), nil
	}
	hash, err := render.HashFile(pptxPath)
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeReadFailed, fmt.Sprintf("hash PPTX: %v", err)), nil
	}
	outDir := mc.outputDir
	if outDir == "" {
		outDir = os.TempDir()
	}
	outDir, err = filepath.Abs(outDir)
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeOutputDir, err.Error()), nil
	}
	base := strings.TrimSuffix(filepath.Base(pptxPath), filepath.Ext(pptxPath))
	ext := ".pdf"
	if format == "notes" {
		ext = ".notes.md"
	}
	outPath := filepath.Join(outDir, base+"-"+hash[:12]+ext)
	result := exportDeckResult{Format: format, OutputPath: outPath}
	if format == "pdf" {
		if err := render.ExportPDFContext(ctx, pptxPath, outPath); err != nil {
			if errors.Is(err, context.Canceled) {
				return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error()), nil
			}
			return api.MCPSimpleError(diagnostics.CodeRenderFailed, err.Error()), nil
		}
	} else {
		pres, err := pptxread.ReadFile(pptxPath)
		if err != nil {
			return api.MCPSimpleError(diagnostics.CodeReadFailed, err.Error()), nil
		}
		result.SlideCount = pres.SlideCount
		if err := writeNotesHandout(ctx, outPath, pres); err != nil {
			if errors.Is(err, context.Canceled) {
				return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error()), nil
			}
			return api.MCPSimpleError(diagnostics.CodeOutputDir, err.Error()), nil
		}
	}
	if err := ctx.Err(); err != nil {
		return api.MCPSimpleError(diagnostics.CodeCancelled, err.Error()), nil
	}
	info, err := os.Stat(outPath)
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeReadFailed, fmt.Sprintf("stat export: %v", err)), nil
	}
	result.Bytes = info.Size()
	response, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError(diagnostics.CodeInternal, err.Error()), nil
	}
	return response, nil
}

func writeNotesHandout(ctx context.Context, outPath string, pres *pptxread.Presentation) error {
	var b strings.Builder
	for _, slide := range pres.Slides {
		if err := ctx.Err(); err != nil {
			return err
		}
		title := slideTitle(slide)
		fmt.Fprintf(&b, "## Slide %d — %s\n\n", slide.Index+1, title)
		if notes := strings.TrimSpace(slide.SpeakerNotes); notes != "" {
			b.WriteString(notes)
		} else {
			b.WriteString("(No speaker notes)")
		}
		b.WriteString("\n\n")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("create notes export directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(outPath), ".export-notes-*")
	if err != nil {
		return fmt.Errorf("create notes export file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return fmt.Errorf("write notes handout: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close notes handout: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), outPath); err != nil {
		return fmt.Errorf("retain notes handout: %w", err)
	}
	return nil
}

func slideTitle(slide pptxread.Slide) string {
	for _, ph := range slide.Placeholders {
		if strings.Contains(strings.ToLower(ph.ID), "title") || strings.Contains(strings.ToLower(ph.Type), "title") {
			if title := strings.Join(strings.Fields(ph.Text), " "); title != "" {
				return title
			}
		}
	}
	for _, shape := range slide.Shapes {
		if strings.Contains(strings.ToLower(shape.Name), "title") {
			if title := strings.Join(strings.Fields(shape.Text), " "); title != "" {
				return title
			}
		}
	}
	return "Untitled slide"
}
