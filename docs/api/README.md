# Go Slide Creator API Guide

A REST API for converting JSON slide definitions into professional PowerPoint presentations.

## Table of Contents

- [Overview](#overview)
- [Base URL](#base-url)
- [Error Handling](#error-handling)
- [Endpoints](#endpoints)
  - [Health Check](#health-check)
  - [List Templates](#list-templates)
  - [Get Template Details](#get-template-details)
  - [Convert Slides](#convert-slides)
  - [Download File](#download-file)
  - [List Slide Types](#list-slide-types)
- [Common Use Cases](#common-use-cases)
- [SDK Examples](#sdk-examples)

---

## Overview

The Go Slide Creator API enables programmatic generation of PowerPoint presentations from structured JSON slide definitions. The service:

- Accepts JSON input with typed content items (text, bullets, charts, tables, diagrams, images)
- Analyzes PPTX templates to discover available layouts
- Automatically selects optimal layouts based on content type
- Generates production-ready PPTX files

## Base URL

| Environment | URL |
|-------------|-----|
| Local Development | `http://localhost:8080` |
| Docker | `http://localhost:8080` |

## Error Handling

All errors follow a consistent JSON structure:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable description",
    "details": {
      "field": "affected_field"
    }
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Missing or malformed request parameters |
| `INVALID_TEMPLATE` | 400 | Specified template does not exist |
| `INVALID_INPUT` | 400 | JSON input validation failed |
| `TEMPLATE_ERROR` | 500 | Template analysis failed |
| `GENERATION_ERROR` | 500 | PPTX generation failed |
| `FILE_NOT_FOUND` | 404 | Download file not found or expired |
| `FILE_ERROR` | 500 | File system access error |

---

## Endpoints

### Health Check

Check service status and availability.

**Endpoint:** `GET /api/v1/health`

#### Request

```bash
curl http://localhost:8080/api/v1/health
```

#### Response

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 3600
}
```

---

### List Templates

Retrieve all available PPTX templates.

**Endpoint:** `GET /api/v1/templates`


#### Request

```bash
curl http://localhost:8080/api/v1/templates
```

#### Response

```json
{
  "templates": [
    {
      "name": "midnight-blue",
      "display_name": "Midnight Blue",
      "aspect_ratio": "16:9",
      "layout_count": 6
    },
    {
      "name": "forest-green",
      "display_name": "Forest Green",
      "aspect_ratio": "16:9",
      "layout_count": 6
    },
    {
      "name": "warm-coral",
      "display_name": "Warm Coral",
      "aspect_ratio": "16:9",
      "layout_count": 6
    }
  ]
}
```

---

### Get Template Details

Get detailed information about a specific template including layouts, colors, and fonts.

**Endpoint:** `GET /api/v1/templates/{name}`


#### Parameters

| Parameter | Location | Required | Description |
|-----------|----------|----------|-------------|
| `name` | path | Yes | Template name (without `.pptx` extension) |

#### Request

```bash
curl http://localhost:8080/api/v1/templates/midnight-blue
```

#### Response

```json
{
  "name": "midnight-blue",
  "display_name": "Midnight Blue",
  "aspect_ratio": "16:9",
  "layouts": [
    {
      "id": "layout1",
      "name": "Title Slide",
      "tags": ["title", "opening"],
      "placeholders": ["title", "body"]
    },
    {
      "id": "layout2",
      "name": "Title and Content",
      "tags": ["content", "bullets"],
      "placeholders": ["title", "body", "content"]
    }
  ],
  "theme": {
    "colors": ["#1F4E79", "#2E75B6", "#9DC3E6", "#DEEBF7", "#000000", "#FFFFFF"],
    "fonts": {
      "title": "Calibri Light",
      "body": "Calibri"
    }
  }
}
```

---

### Convert Slides

Convert JSON slide definitions to a PowerPoint presentation.

**Endpoint:** `POST /api/v1/convert`


#### Request Body

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `template` | string | Yes | - | Template name |
| `slides` | array | Yes | - | Array of slide definitions |
| `options.output_format` | string | No | `"file"` | `"file"` or `"base64"` |
| `options.svg_scale` | number | No | `2.0` | Scale factor for SVG chart rendering |
| `options.exclude_template_slides` | boolean | No | `true` | Exclude original template slides from output |

#### Slide Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Slide type: `title`, `content`, `section`, `two-column`, `chart`, `diagram`, `image`, `comparison`, `blank` |
| `title` | string | No | Slide title |
| `content` | object | No | Content with `body` (string) and/or `bullets` (string[]) |
| `speaker_notes` | string | No | Speaker notes |
| `source` | string | No | Source attribution |

#### Request (File Output)

```bash
curl -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {
        "type": "title",
        "title": "Q4 Report",
        "content": {"body": "Jane Smith | Strategy Team"}
      },
      {
        "type": "content",
        "title": "Introduction",
        "content": {
          "bullets": [
            "Revenue growth of 15%",
            "New market expansion",
            "Team grew by 20%"
          ]
        }
      }
    ]
  }'
```

#### Response (File Output)

```json
{
  "success": true,
  "file_url": "/api/v1/download/a1b2c3d4e5f6789012345678.pptx",
  "expires_at": "2025-01-17T15:30:00Z",
  "stats": {
    "slide_count": 2,
    "processing_time_ms": 1250,
    "warnings": []
  }
}
```

#### Request (Base64 Output)

```bash
curl -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "warm-coral",
    "slides": [
      {
        "type": "content",
        "title": "Summary",
        "content": {"bullets": ["Point one", "Point two"]}
      }
    ],
    "options": {
      "output_format": "base64"
    }
  }'
```

#### Response (Base64 Output)

```json
{
  "success": true,
  "data": "UEsDBBQAAAAIAG5hdGl2ZS1tYWluLnB...",
  "filename": "Quick Report.pptx",
  "stats": {
    "slide_count": 1,
    "processing_time_ms": 1100,
    "warnings": []
  }
}
```

#### Error Responses

**Missing slides:**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Slides array is required",
    "details": {
      "field": "slides"
    }
  }
}
```

**Template not found:**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_TEMPLATE",
    "message": "Template 'nonexistent' not found"
  }
}
```

---

### Download File

Download a previously generated PPTX file.

**Endpoint:** `GET /api/v1/download/{filename}`


**File Expiration:** 1 hour after generation

#### Parameters

| Parameter | Location | Required | Description |
|-----------|----------|----------|-------------|
| `filename` | path | Yes | Filename from convert response (e.g., `a1b2c3d4e5f6789012345678.pptx`) |

#### Request

```bash
curl -O http://localhost:8080/api/v1/download/a1b2c3d4e5f6789012345678.pptx
```

#### Response

- **Content-Type:** `application/vnd.openxmlformats-officedocument.presentationml.presentation`
- **Content-Disposition:** `attachment; filename="presentation.pptx"`
- **Body:** Binary PPTX file data

#### Error Response

```json
{
  "success": false,
  "error": {
    "code": "FILE_NOT_FOUND",
    "message": "File not found or expired"
  }
}
```

---

### List Slide Types

Retrieve supported slide types.

**Endpoint:** `GET /api/v1/slide-types`


#### Request

```bash
curl http://localhost:8080/api/v1/slide-types
```

#### Response

```json
{
  "slide_types": [
    {"type": "title", "description": "Opening slide with title and optional subtitle"},
    {"type": "content", "description": "Standard bullet slide with title and body"},
    {"type": "two-column", "description": "Side-by-side content layout"},
    {"type": "image", "description": "Image-focused slide with title"},
    {"type": "chart", "description": "Data visualization slide"},
    {"type": "comparison", "description": "Comparison layout for side-by-side elements"},
    {"type": "blank", "description": "Empty slide with no placeholders"}
  ]
}
```

---

## Common Use Cases

### Generate and Download Presentation

```bash
# 1. Convert JSON to PPTX
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {
        "type": "content",
        "title": "Hello World",
        "content": {"bullets": ["First slide content"]}
      }
    ]
  }')

# 2. Extract download URL
FILE_URL=$(echo $RESPONSE | jq -r '.file_url')

# 3. Download the file
curl -o presentation.pptx "http://localhost:8080${FILE_URL}"
```

### Generate Base64 for Direct Use

```bash
# Get base64-encoded PPTX
curl -s -X POST http://localhost:8080/api/v1/convert \
  -H "Content-Type: application/json" \
  -d '{
    "template": "midnight-blue",
    "slides": [
      {
        "type": "content",
        "title": "Summary",
        "content": {"bullets": ["Key point"]}
      }
    ],
    "options": {
      "output_format": "base64"
    }
  }' | jq -r '.data' | base64 -d > report.pptx
```

### Explore Available Templates

```bash
# List all templates
curl -s http://localhost:8080/api/v1/templates | jq '.templates[].name'

# Get details for a specific template
curl -s http://localhost:8080/api/v1/templates/midnight-blue | jq '.layouts[].name'
```

### Check Service Health

```bash
# Basic health check
curl -s http://localhost:8080/api/v1/health | jq '.status'

# Monitor health
watch -n 5 'curl -s http://localhost:8080/api/v1/health | jq'
```

---

## SDK Examples

### JavaScript/TypeScript

```typescript
interface ConvertOptions {
  output_format?: 'file' | 'base64';
  svg_scale?: number;
  exclude_template_slides?: boolean;
}

interface SlideContent {
  body?: string;
  bullets?: string[];
}

interface Slide {
  type: string;
  title?: string;
  content?: SlideContent;
  speaker_notes?: string;
  source?: string;
}

interface ConvertRequest {
  template: string;
  slides: Slide[];
  options?: ConvertOptions;
}

interface ConvertResponse {
  success: boolean;
  file_url?: string;
  expires_at?: string;
  data?: string;
  filename?: string;
  stats: {
    slide_count: number;
    processing_time_ms: number;
    warnings: string[];
  };
}

async function convertSlides(request: ConvertRequest): Promise<ConvertResponse> {
  const response = await fetch('http://localhost:8080/api/v1/convert', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error.message);
  }

  return response.json();
}

// Usage
const result = await convertSlides({
  template: 'midnight-blue',
  slides: [
    {
      type: 'title',
      title: 'Q4 Report',
      content: { body: 'Strategy Team' },
    },
    {
      type: 'content',
      title: 'Introduction',
      content: { bullets: ['Revenue growth of 15%', 'New market expansion'] },
    },
  ],
});

if (result.file_url) {
  console.log(`Download: http://localhost:8080${result.file_url}`);
}
```

### Go

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type SlideCreatorClient struct {
	BaseURL string
	Client  *http.Client
}

type SlideContent struct {
	Body    string   `json:"body,omitempty"`
	Bullets []string `json:"bullets,omitempty"`
}

type Slide struct {
	Type         string       `json:"type"`
	Title        string       `json:"title,omitempty"`
	Content      SlideContent `json:"content,omitempty"`
	SpeakerNotes string       `json:"speaker_notes,omitempty"`
}

type ConvertRequest struct {
	Template string          `json:"template"`
	Slides   []Slide         `json:"slides"`
	Options  *ConvertOptions `json:"options,omitempty"`
}

type ConvertOptions struct {
	OutputFormat          string  `json:"output_format,omitempty"`
	SVGScale              float64 `json:"svg_scale,omitempty"`
	ExcludeTemplateSlides bool    `json:"exclude_template_slides,omitempty"`
}

type ConvertResponse struct {
	Success   bool   `json:"success"`
	FileURL   string `json:"file_url,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Data      string `json:"data,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Stats     struct {
		SlideCount       int      `json:"slide_count"`
		ProcessingTimeMs int64    `json:"processing_time_ms"`
		Warnings         []string `json:"warnings"`
	} `json:"stats"`
}

func NewClient(baseURL string) *SlideCreatorClient {
	return &SlideCreatorClient{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
}

func (c *SlideCreatorClient) Convert(req ConvertRequest) (*ConvertResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Post(
		c.BaseURL+"/api/v1/convert",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ConvertResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *SlideCreatorClient) Download(fileURL, outputPath string) error {
	resp, err := c.Client.Get(c.BaseURL + fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func main() {
	client := NewClient("http://localhost:8080")

	result, err := client.Convert(ConvertRequest{
		Template: "midnight-blue",
		Slides: []Slide{
			{
				Type:  "title",
				Title: "Q4 Report",
				Content: SlideContent{Body: "Strategy Team"},
			},
			{
				Type:  "content",
				Title: "Introduction",
				Content: SlideContent{
					Bullets: []string{"Revenue growth of 15%", "New market expansion"},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	if result.FileURL != "" {
		err = client.Download(result.FileURL, "presentation.pptx")
		if err != nil {
			panic(err)
		}
		fmt.Printf("Downloaded: presentation.pptx (%d slides)\n", result.Stats.SlideCount)
	}
}
```

---

## OpenAPI Specification

The complete OpenAPI 3.0 specification is available at:

- **File:** [`openapi.yaml`](./openapi.yaml)
- **Swagger UI:** Import the spec into [Swagger Editor](https://editor.swagger.io/)
- **Postman:** Import the OpenAPI spec directly into Postman for interactive testing

---

## Support

For issues and feature requests, please open an issue in the repository.
