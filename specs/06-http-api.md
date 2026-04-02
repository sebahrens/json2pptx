# HTTP API

REST API for presentation conversion.

## Scope

This specification covers ONLY the HTTP interface. It does NOT cover:
- Internal processing (see other specs)

## Endpoints

### GET /api/v1/health

Health check for load balancers.

### GET /api/v1/templates

List available templates.

### GET /api/v1/templates/{name}

Get template details.

### GET /api/v1/slide-types

List supported slide types.

### GET /api/v1/download/{id}

Download generated PPTX file.

### POST /api/v1/convert

Convert JSON slide definitions to PPTX.

**Request:**
```json
{
  "template": "midnight-blue",
  "slides": [
    {
      "type": "content",
      "title": "Key Points",
      "content": {"bullets": ["First point", "Second point"]}
    }
  ],
  "options": {
    "output_format": "file"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| template | string | Yes | Template name (without .pptx). Only alphanumeric, hyphens, and underscores allowed. |
| slides | array | Yes | Array of slide definitions |
| options.output_format | string | No | "file" or "base64" (default: "file") |
| options.svg_scale | number | No | Scale factor for SVG chart rendering (default: 1.0) |
| options.exclude_template_slides | boolean | No | Exclude original template slides from output (default: false) |

**Request Size Limit:** 10 MB maximum body size.

**Response (Success):**
```json
{
  "success": true,
  "file_url": "/api/v1/download/abc123.pptx",
  "expires_at": "2025-01-17T12:00:00Z",
  "stats": {
    "slide_count": 10,
    "processing_time_ms": 1500,
    "warnings": []
  }
}
```

**Response (output_format=base64):**
```json
{
  "success": true,
  "data": "UEsDBBQAAAAI...",
  "filename": "presentation.pptx",
  "stats": { ... }
}
```

**Response (Error):**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_MARKDOWN",
    "message": "Missing required frontmatter field: title",
    "details": {
      "line": 1
    }
  }
}
```

### GET /api/v1/download/{id}

Download generated PPTX file.

**Response:** Binary PPTX file with headers:
- `Content-Type: application/vnd.openxmlformats-officedocument.presentationml.presentation`
- `Content-Disposition: attachment; filename="presentation.pptx"`

Supports HTTP range requests for partial downloads.

**Error (expired/not found):**
- Status: 404
- Body: `{"success": false, "error": {"code": "FILE_NOT_FOUND", "message": "File not found or expired"}}`

### GET /api/v1/templates

List available templates.

**Response:**
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
    }
  ]
}
```

### GET /api/v1/templates/{name}

Get template details.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| aspect | string | No | Override aspect ratio ("16:9" or "4:3"). If not provided, uses auto-detection. |

**Response:**
```json
{
  "name": "midnight-blue",
  "display_name": "Midnight Blue",
  "aspect_ratio": "16:9",
  "layouts": [
    {
      "id": "layout1",
      "name": "Title Slide",
      "tags": ["title-slide"],
      "placeholders": ["title", "subtitle"]
    }
  ],
  "theme": {
    "colors": ["#1F4E79", "#2E75B6", ...],
    "fonts": {
      "title": "Calibri Light",
      "body": "Calibri"
    }
  }
}
```

### GET /api/v1/slide-types

List supported slide types.

**Response:**
```json
{
  "slide_types": [
    {
      "type": "title",
      "description": "Opening slide with title and optional subtitle"
    },
    {
      "type": "content",
      "description": "Standard bullet slide with title and body"
    },
    {
      "type": "two-column",
      "description": "Side-by-side content layout"
    },
    {
      "type": "image",
      "description": "Image-focused slide with title"
    },
    {
      "type": "chart",
      "description": "Data visualization slide"
    },
    {
      "type": "comparison",
      "description": "Comparison layout for side-by-side elements"
    },
    {
      "type": "blank",
      "description": "Empty slide with no placeholders"
    }
  ]
}
```

### GET /api/v1/health

Health check for load balancers.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime_seconds": 3600
}
```

Note: Health check is not rate limited.

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| INVALID_REQUEST | 400 | General request validation failed |
| INVALID_CONTENT_TYPE | 415 | Missing or invalid Content-Type header (must be application/json) |
| INVALID_INPUT | 400 | JSON input validation failed |
| INVALID_SLIDE_TYPE | 400 | Unrecognized slide type in input |
| INVALID_TEMPLATE | 400 | Template name invalid or not found |
| REQUEST_TOO_LARGE | 413 | Request body exceeds 10 MB limit |
| TEMPLATE_ERROR | 500 | Template analysis failed |
| GENERATION_ERROR | 500 | PPTX generation failed |
| FILE_NOT_FOUND | 404 | Download file expired/missing |
| FILE_ERROR | 500 | Failed to access file |
| RATE_LIMITED | 429 | Too many requests |

## Content-Type Validation

All POST endpoints strictly enforce `Content-Type: application/json` header. Requests without this header or with incorrect content types will receive a `415 Unsupported Media Type` response with error code `INVALID_CONTENT_TYPE`.

## Rate Limiting

- `/api/v1/convert`: 10 requests/minute per IP (configurable)
- `/api/v1/health`: No rate limiting
- Other endpoints: 100 requests/minute per IP (configurable)

Rate limit headers (on all rate-limited responses):
- `X-RateLimit-Limit`: Max requests per window
- `X-RateLimit-Remaining`: Remaining requests in window
- `X-RateLimit-Reset`: Unix timestamp when limit resets

## Security Headers

All responses include security headers:
- `X-Content-Type-Options: nosniff` - Prevents MIME type sniffing
- `X-Frame-Options: DENY` - Prevents clickjacking
- `X-XSS-Protection: 1; mode=block` - Legacy XSS protection
- `Content-Security-Policy: default-src 'none'` - Strict CSP for API
- `Strict-Transport-Security: max-age=31536000; includeSubDomains` - Enforces HTTPS

## CORS Configuration

CORS is controlled via `AllowedOrigins` configuration. Only explicitly allowed origins receive CORS headers. If no origins are configured, all cross-origin requests are blocked.

When allowed:
- `Access-Control-Allow-Origin`: Set to the requesting origin
- `Access-Control-Allow-Methods`: GET, POST, PUT, DELETE, OPTIONS
- `Access-Control-Allow-Headers`: Content-Type
- `Access-Control-Max-Age`: 3600

Preflight (OPTIONS) requests return 204 No Content.

## File Retention

- Generated PPTX files: 1 hour (configurable via `FileRetention`)
- Files are checked for expiration on download request and deleted if expired

## Acceptance Criteria

### AC1: Convert Success
- Given valid JSON input and template
- When POST /api/v1/convert
- Then returns 200 with file_url

### AC2: Convert with Base64
- Given options.output_format="base64"
- When POST /api/v1/convert
- Then returns base64-encoded PPTX in data field

### AC3: Convert Invalid Input
- Given JSON input with missing required fields
- When POST /api/v1/convert
- Then returns 400 with INVALID_INPUT

### AC4: Convert Invalid Template
- Given template="nonexistent"
- When POST /api/v1/convert
- Then returns 400 with INVALID_TEMPLATE

### AC5: Download Valid
- Given valid file ID from convert
- When GET /api/v1/download/{id}
- Then returns PPTX binary

### AC6: Download Expired
- Given expired file ID
- When GET /api/v1/download/{id}
- Then returns 404 with FILE_NOT_FOUND

### AC7: List Templates
- When GET /api/v1/templates
- Then returns array of available templates

### AC8: Template Details
- Given existing template name
- When GET /api/v1/templates/{name}
- Then returns layout and theme info

### AC9: Health Check
- When GET /api/v1/health
- Then returns status and version

### AC10: Rate Limiting
- Given 11 convert requests in 1 minute
- When 11th request sent
- Then returns 429 with rate limit headers

### AC11: CORS Headers
- When request from allowed browser origin
- Then response includes appropriate CORS headers
- When request from disallowed origin
- Then no CORS headers are set

### AC12: Request Validation
- Given request missing required field
- When POST to any endpoint
- Then returns 400 with INVALID_REQUEST and field-specific details

### AC13: Request Size Limit
- Given request body exceeding 10 MB
- When POST /api/v1/convert
- Then returns 413 with REQUEST_TOO_LARGE

### AC14: Security Headers
- When any API request is made
- Then response includes all security headers (X-Content-Type-Options, X-Frame-Options, etc.)

### AC15: Template Name Validation
- Given template name with path traversal (e.g., "../secret")
- When POST /api/v1/convert
- Then returns 400 with INVALID_TEMPLATE

## Testing Requirements

Integration tests:
- Full convert flow (JSON input -> download)
- Error scenarios for each error code
- Rate limiting behavior
- File expiration
- Security header presence
- CORS behavior with allowed/disallowed origins

Load tests:
- 10 concurrent conversions
- Template caching effectiveness
- Memory usage under load
