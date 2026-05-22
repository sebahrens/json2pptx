// mcp_output_schemas.go defines JSON Schema output schemas for every MCP tool.
// These allow MCP clients to validate tool responses deterministically
// without prose-parsing the description field.
package main

import "encoding/json"

// Each schema variable is a json.RawMessage containing a valid JSON Schema
// that describes the tool's success response shape.

// --- generate_presentation ---
var outputSchemaGenerate = json.RawMessage(`{
  "type": "object",
  "properties": {
    "success":           {"type": "boolean"},
    "output_path":       {"type": "string"},
    "slide_count":       {"type": "integer"},
    "duration_ms":       {"type": "integer"},
    "warnings":          {"type": "array", "items": {"type": "string"}},
    "quality":           {"$ref": "#/$defs/quality_score"},
    "validation_errors": {"type": "array", "items": {"$ref": "#/$defs/validation_error"}},
    "fit_findings":      {"type": "array", "items": {"$ref": "#/$defs/fit_finding"}},
    "idempotent_replay": {"type": "boolean", "description": "True when this response was served from the idempotency cache (the caller passed an idempotency_key that matched a prior successful call)."},
    "output_validation_findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "code":        {"type": "string"},
          "severity":    {"type": "string", "enum": ["blocking", "warning"]},
          "path":        {"type": "string"},
          "message":     {"type": "string"},
          "phase":       {"type": "string"},
          "validator":   {"type": "string"},
          "slide_index": {"type": "integer"},
          "source_path": {"type": "string"},
          "scope":       {"type": "string", "enum": ["source", "template", "generator"]}
        },
        "required": ["code", "severity", "message", "phase", "validator", "slide_index", "scope"]
      }
    }
  },
  "required": ["success"],
  "$defs": {
    "quality_score": {
      "type": "object",
      "properties": {
        "overall":    {"type": "integer"},
        "variety":    {"type": "integer"},
        "coverage":   {"type": "integer"},
        "structure":  {"type": "integer"}
      }
    },
    "validation_error": {
      "type": "object",
      "properties": {
        "path":    {"type": "string"},
        "code":    {"type": "string"},
        "message": {"type": "string"}
      }
    },
    "fit_finding": {
      "type": "object",
      "properties": {
        "code":        {"type": "string"},
        "slide_index": {"type": "integer"},
        "path":        {"type": "string"},
        "message":     {"type": "string"},
        "action":      {"type": "string"},
        "fix":         {"type": "object"},
        "next_tool_call": {
          "type": "object",
          "properties": {
            "tool":          {"type": "string"},
            "args_template": {"type": "object"}
          },
          "required": ["tool", "args_template"]
        }
      },
      "required": ["code", "slide_index", "message", "action"]
    }
  }
}`)

// --- validate_input ---
// The diagnostic-bearing fields (warnings/errors/validation_warnings/
// diagnostics/fit_findings) are collapsed into the single findings envelope.
var outputSchemaValidate = json.RawMessage(`{
  "type": "object",
  "properties": {
    "valid":               {"type": "boolean"},
    "slide_count":         {"type": "integer"},
    "chart_count":         {"type": "integer"},
    "diagram_count":       {"type": "integer"},
    "table_count":         {"type": "integer"},
    "shape_count":         {"type": "integer"},
    "slides":              {"type": "array", "items": {"type": "object"}},
    "findings":            ` + findingEnvelopeSchema + `,
    "response_fingerprint": {"type": "string", "description": "Lowercase sha256 hex (64 chars) over the canonical JSON of this response (with the field zeroed). Use as cache key / drift detector."}
  },
  "required": ["valid", "findings"]
}`)

// --- list_templates ---
var outputSchemaListTemplates = json.RawMessage(`{
  "type": "object",
  "properties": {
    "tool":            {"type": "object", "properties": {"name": {"type": "string"}, "version": {"type": "string"}}},
    "templates":       {"type": "array", "items": {"type": "object"}},
    "supported_types": {"type": "object"},
    "input_formats":   {"type": "array", "items": {"type": "string"}},
    "output_formats":  {"type": "array", "items": {"type": "string"}},
    "total_count":     {"type": "integer", "description": "Total number of templates discovered (after filter), irrespective of the current page."},
    "page_size":       {"type": "integer", "description": "Maximum number of template entries in this page."},
    "next_cursor":     {"type": "string", "description": "Opaque continuation token. Pass back as cursor on the next call. Absent on the last page."},
    "warnings":        {"type": "array", "items": {"type": "string"}, "description": "Advisory hints (e.g. deprecation notice when fields is omitted)."},
    "side_effects":    {"type": "object", "description": "Disk side effects of this call: whether layout-preview PNG cache files were (or could be) written, the cache directory, whether read-only mode was active, and the opt-out (read_only=true).", "properties": {"preview_cache_writes": {"type": "boolean"}, "read_only": {"type": "boolean"}, "preview_cache_dir": {"type": "string"}, "disable_with": {"type": "string"}}}
  },
  "required": ["tool", "templates", "supported_types", "input_formats", "output_formats"]
}`)

// --- get_data_format_hints ---
var outputSchemaGetDataFormatHints = json.RawMessage(`{
  "type": "object",
  "properties": {
    "digest":           {"type": "string"},
    "not_modified":     {"type": "boolean"},
    "data_format_hints": {"type": "object"}
  },
  "required": ["digest"]
}`)

// --- get_chart_capabilities ---
var outputSchemaGetChartCapabilities = json.RawMessage(`{
  "type": "object",
  "properties": {
    "chart_capabilities": {"type": "array", "items": {"type": "object"}}
  },
  "required": ["chart_capabilities"]
}`)

// --- get_diagram_capabilities ---
var outputSchemaGetDiagramCapabilities = json.RawMessage(`{
  "type": "object",
  "properties": {
    "diagram_capabilities": {"type": "array", "items": {"type": "object"}}
  },
  "required": ["diagram_capabilities"]
}`)

// --- list_patterns ---
var outputSchemaListPatterns = json.RawMessage(`{
  "type": "object",
  "properties": {
    "groups": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "category": {"type": "string"},
          "patterns": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "name":             {"type": "string"},
                "cells":            {"type": "string"},
                "use_when":         {"type": "string"},
                "not_when":         {"type": "string"},
                "category":         {"type": "string"},
                "narrative_role":   {"type": "array", "items": {"type": "string"}},
                "pairs_with":       {"type": "array", "items": {"type": "string"}},
                "composes_with":    {"type": "array", "items": {"type": "string"}},
                "role_on_slide":    {"type": "array", "items": {"type": "string"}},
                "density_class":    {"type": "string"},
                "accent_weight":    {"type": "string"},
                "supports_callout": {"type": "boolean"}
              },
              "required": ["name", "category"]
            }
          }
        },
        "required": ["category", "patterns"]
      }
    },
    "total_count": {"type": "integer", "description": "Total number of patterns across all categories (after filter)."},
    "page_size":   {"type": "integer", "description": "Maximum number of pattern entries per page."},
    "next_cursor": {"type": "string", "description": "Opaque continuation token; absent on the last page."},
    "warnings":    {"type": "array", "items": {"type": "string"}, "description": "Advisory hints (e.g. deprecation notice when fields is omitted)."}
  },
  "required": ["groups", "total_count", "page_size"]
}`)

// --- show_pattern ---
var outputSchemaShowPattern = json.RawMessage(`{
  "type": "object",
  "properties": {
    "name":        {"type": "string"},
    "description": {"type": "string"},
    "cells":       {"type": "string"},
    "use_when":    {"type": "string"},
    "not_when":    {"type": "string"},
    "supports_callout": {"type": "boolean"},
    "callout_schema": {
      "type": "object",
      "description": "JSON Schema fragment for the envelope-level callout DTO. Present only when supports_callout=true."
    },
    "version":     {"type": "integer"},
    "schema":      {"type": "object"},
    "composes_with": {"type": "array", "items": {"type": "string"}},
    "role_on_slide": {"type": "array", "items": {"type": "string"}},
    "text_budget_guide": {
      "type": "object",
      "properties": {
        "target_density": {
          "type": "object",
          "properties": {
            "min_pct":   {"type": "integer"},
            "ideal_pct": {"type": "integer"},
            "max_pct":   {"type": "integer"}
          },
          "required": ["min_pct", "ideal_pct", "max_pct"]
        },
        "configurations": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "columns":         {"type": "integer"},
              "rows":            {"type": "integer"},
              "body_max_chars":  {"type": "integer"},
              "header_max_chars": {"type": "integer"}
            },
            "required": ["columns", "rows", "body_max_chars", "header_max_chars"]
          }
        }
      },
      "required": ["target_density", "configurations"]
    },
    "example_values": {
      "type": "object",
      "description": "Canonical example values for this pattern. Shows the expected shape and realistic content for the values parameter."
    },
    "rendering_capabilities": {
      "type": "object",
      "description": "Describes how this pattern renders icons and visual elements.",
      "properties": {
        "icon_support": {"type": "string", "enum": ["none", "text_only", "svg_only", "svg_and_text"], "description": "What icon rendering modes the pattern supports."}
      },
      "required": ["icon_support"]
    }
  },
  "required": ["name", "description", "use_when", "not_when", "version", "schema"]
}`)

// --- describe_finding ---
var outputSchemaDescribeFinding = json.RawMessage(`{
  "type": "object",
  "properties": {
    "code":         {"type": "string", "description": "Echo of the requested finding code."},
    "summary":      {"type": "string", "description": "One-line description of what the finding means."},
    "severity":     {"type": "string", "enum": ["refuse", "shrink_or_split", "review", "info"], "description": "Action rank — matches fit_findings[].action."},
    "when_emitted": {"type": "string", "description": "The condition under which the engine emits this code."},
    "remediation_steps": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Ordered remediation steps, most direct first. Steps that map to repair_slide fix kinds name the kind explicitly."
    },
    "example_before": {"type": "string", "description": "Illustrative snippet showing the input/finding shape that triggers this code (may be omitted)."},
    "example_after":  {"type": "string", "description": "Illustrative snippet showing the same input after applying remediation (may be omitted)."},
    "related_codes": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Codes that often co-occur or that share a root cause."
    }
  },
  "required": ["code", "summary", "severity", "when_emitted", "remediation_steps"]
}`)

// --- validate_pattern ---
var outputSchemaValidatePattern = json.RawMessage(`{
  "type": "object",
  "properties": {
    "ok":     {"type": "boolean"},
    "errors": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "field":   {"type": "string"},
          "code":    {"type": "string"},
          "message": {"type": "string"},
          "fix":     {
            "type": "object",
            "properties": {
              "kind":   {"type": "string", "description": "Machine-readable fix category (e.g. rename_field, reshape_value, use_one_of)"},
              "params": {"type": "object", "description": "Kind-specific parameters for the fix"}
            },
            "required": ["kind"]
          }
        },
        "required": ["field", "message"]
      }
    }
  },
  "required": ["ok"]
}`)

// --- expand_pattern ---
var outputSchemaExpandPattern = json.RawMessage(`{
  "type": "object",
  "properties": {
    "pattern":           {"type": "string"},
    "version":           {"type": "integer"},
    "bounds_source":     {"type": "string", "enum": ["template", "default_fallback"]},
    "shape_grid":        {"type": "object"},
    "occupancy": {
      "type": "object",
      "properties": {
        "filled_pct":       {"type": "number"},
        "rows_used":        {"type": "integer"},
        "rows_empty":       {"type": "integer"},
        "bounds_height_pct": {"type": "number"}
      },
      "required": ["filled_pct", "rows_used", "rows_empty", "bounds_height_pct"]
    },
    "density_warnings":  {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "field":   {"type": "string"},
          "code":    {"type": "string"},
          "message": {"type": "string"},
          "fix":     {"type": "object"}
        }
      }
    },
    "layout_suggestions": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "pattern":   {"type": "string"},
          "overrides": {"type": "object"},
          "reason":    {"type": "string"}
        },
        "required": ["pattern", "reason"]
      }
    }
  },
  "required": ["pattern", "version", "bounds_source", "shape_grid", "occupancy"]
}`)

// --- expand_patterns (batch) ---
var outputSchemaExpandPatterns = json.RawMessage(`{
  "type": "object",
  "properties": {
    "bounds_source": {"type": "string", "enum": ["template", "default_fallback"]},
    "results": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "pattern":       {"type": "string"},
          "used_exemplar": {"type": "boolean"},
          "error": {
            "type": "object",
            "properties": {
              "field":   {"type": "string"},
              "code":    {"type": "string"},
              "message": {"type": "string"},
              "fix":     {"type": "object"},
              "next_tool_call": {"type": "object"}
            }
          },
          "result": {"type": "object"}
        },
        "required": ["pattern", "used_exemplar"]
      }
    }
  },
  "required": ["bounds_source", "results"]
}`)

// --- recommend_pattern ---
var outputSchemaRecommendPattern = json.RawMessage(`{
  "type": "object",
  "properties": {
    "candidates": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "pattern_name":      {"type": "string"},
          "score":             {"type": "number"},
          "rationale":         {"type": "string"},
          "confidence_band":   {"type": "string", "enum": ["high", "medium", "low"]},
          "diversity_bonus":   {"type": "boolean"},
          "expansion_preview": {"type": "object"}
        },
        "required": ["pattern_name", "score", "rationale", "confidence_band"]
      }
    },
    "query_understood_as": {"type": "string"},
    "suggestion":          {"type": "string"},
    "near_misses": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "pattern_name": {"type": "string"},
          "score":        {"type": "number"},
          "would_tip_if": {"type": "string"}
        },
        "required": ["pattern_name", "score", "would_tip_if"]
      }
    },
    "disambiguating_questions": {
      "type": "array",
      "items": {"type": "string"}
    }
  },
  "required": ["candidates", "query_understood_as"]
}`)

// --- recommend_visual ---
var outputSchemaRecommendVisual = json.RawMessage(`{
  "type": "object",
  "properties": {
    "candidates": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "category":         {"type": "string", "enum": ["placeholder_layout", "named_pattern", "chart", "diagram", "raw_shape_grid", "compose"]},
          "name":             {"type": "string"},
          "score":            {"type": "number"},
          "rationale":        {"type": "string"},
          "confidence_band":  {"type": "string", "enum": ["high", "medium", "low"]},
          "diversity_bonus":  {"type": "boolean"},
          "placement":        {"$ref": "#/$defs/placement_guidance"},
          "template_support": {"$ref": "#/$defs/template_support"}
        },
        "required": ["category", "name", "score", "rationale", "confidence_band"]
      }
    },
    "query_understood_as": {"type": "string"},
    "disambiguating_questions": {
      "type": "array",
      "items": {"type": "string"}
    },
    "response_fingerprint": {"type": "string", "description": "Lowercase sha256 hex (64 chars) over the canonical JSON of this response (with the field zeroed). Use as cache key / drift detector."}
  },
  "required": ["candidates", "query_understood_as"],
  "$defs": {
    "placement_guidance": {
      "type": "object",
      "description": "Authoring guidance: how and where to place this candidate on a slide.",
      "properties": {
        "preferred_placement": {"type": "string", "enum": ["placeholder", "shape_grid", "either"], "description": "Recommended placement context for best fidelity."},
        "host_strategy":       {"type": "string", "enum": ["placeholder_content", "grid_cell", "pattern_expansion", "standalone_slide"], "description": "How to host this visual in the JSON input."},
        "grid_embeddable":     {"type": "boolean", "description": "Whether this candidate can be placed inside a shape_grid cell."},
        "render_pipeline":     {"type": "string", "enum": ["native_ooxml", "svg", "template_driven"], "description": "Render strategy in the preferred placement."},
        "composable_with":     {"type": "array", "items": {"type": "string"}, "description": "Categories or patterns this composes with on a single slide."}
      },
      "required": ["preferred_placement", "host_strategy", "grid_embeddable", "render_pipeline"]
    },
    "template_support": {
      "type": "object",
      "description": "Per-candidate feasibility for the template passed in the 'template' argument. Present only when template context was supplied. Candidates that are unsupported (or risky) are demoted in the ranking so they no longer appear first.",
      "properties": {
        "status":          {"type": "string", "enum": ["supported", "risky", "unsupported"], "description": "supported = the template natively covers the needed layout/capability; risky = producible only via a synthesised/derived layout or close to a capacity/content-zone limit; unsupported = requires an absent canonical/derivable layout."},
        "reasons":         {"type": "array", "items": {"type": "string"}, "description": "Why the status applies: which layouts cover the candidate, what is synthesised, which capacity/content-zone constraint bites, or what is missing."},
        "required_layout": {"type": "string", "description": "The canonical layout or derivable capability the candidate needs (e.g. \"Title Slide\", \"Two Content\", \"full-image\", \"grid base\"). Omitted when the candidate has no specific layout requirement."}
      },
      "required": ["status"]
    }
  }
}`)

// --- list_icons ---
var outputSchemaListIcons = json.RawMessage(`{
  "type": "object",
  "properties": {
    "sets": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "set":   {"type": "string"},
          "count": {"type": "integer", "description": "Number of icons from this set included on the current page."},
          "names": {"type": "array", "items": {"type": "string"}, "description": "Bare-name list (no set prefix). Always present; in fields=compact this is the only payload (qualified_name can be synthesized as set + ':' + name)."},
          "icons": {
            "type": "array",
            "description": "Per-icon entries with the canonical authoring identifier. Omitted when fields=compact.",
            "items": {
              "type": "object",
              "properties": {
                "name":           {"type": "string", "description": "Bare icon name without the set prefix."},
                "qualified_name": {"type": "string", "description": "Canonical authoring identifier in '<set>:<name>' form (e.g. 'filled:chart-pie', 'outline:chart-pie'). Drop directly into icon.name."}
              },
              "required": ["name", "qualified_name"]
            }
          }
        },
        "required": ["set", "count", "names"]
      }
    },
    "total_count": {"type": "integer", "description": "Total number of icons across all requested sets (after the search filter)."},
    "page_size":   {"type": "integer", "description": "Maximum number of icons per page."},
    "next_cursor": {"type": "string", "description": "Opaque continuation token; absent on the last page."},
    "warnings":    {"type": "array", "items": {"type": "string"}, "description": "Advisory hints (e.g. deprecation notice when fields is omitted)."}
  },
  "required": ["sets", "total_count", "page_size"]
}`)

// --- get_shape_catalog ---
var outputSchemaGetShapeCatalog = json.RawMessage(`{
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "category":    {"type": "string"},
      "description": {"type": "string"},
      "shapes": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "name":           {"type": "string"},
            "adjust_handles": {"type": "array", "items": {"type": "string"}}
          },
          "required": ["name"]
        }
      }
    },
    "required": ["category", "description", "shapes"]
  }
}`)

// --- preview_icon ---
var outputSchemaPreviewIcon = json.RawMessage(`{
  "type": "object",
  "properties": {
    "svg_data":       {"type": "string", "description": "SVG markup for the resolved icon (Fill applied for non-inline sources)."},
    "png_base64":     {"type": "string", "description": "Base64-encoded PNG rasterization of the SVG. Absent when rasterization fails."},
    "alt":            {"type": "string", "description": "Alt text — IconInput.alt when set, otherwise derived from name/path/url."},
    "source_kind":    {"type": "string", "enum": ["bundled", "path", "url", "inline"], "description": "Which IconInput field was set."},
    "qualified_name": {"type": "string", "description": "Canonical '<set>:<name>' identifier for bundled icons. Drop directly into icon.name in deck JSON."},
    "width":          {"type": "integer", "description": "PNG width in pixels."},
    "height":         {"type": "integer", "description": "PNG height in pixels."},
    "warnings":       {"type": "array", "items": {"type": "string"}}
  },
  "required": ["svg_data", "alt", "source_kind"]
}`)

// --- render_slide_image ---
var outputSchemaRenderSlideImage = json.RawMessage(`{
  "type": "object",
  "properties": {
    "index":        {"type": "integer"},
    "png_base64":   {"type": "string"},
    "path":         {"type": "string", "description": "Content-addressed artifact path, returned instead of png_base64 when the image exceeds the inline cap (~200KB). The filename embeds the PNG content hash, so the path is collision-free across decks and never overwritten with different content."},
    "width":        {"type": "integer"},
    "height":       {"type": "integer"},
    "size_error":   {"type": "string"},
    "content_hash": {"type": "string", "description": "SHA-256 of the rendered PNG bytes. Stable identity of this image regardless of delivery (inline or path)."},
    "source_hash":  {"type": "string", "description": "Identity of the upstream artifact this image was rendered from: the PPTX file content hash, or the caller-supplied cache key for keyed renders."},
    "cleanup":      {"type": "string", "description": "Lifetime/cleanup semantics of the on-disk path artifact. Set only when path is returned; empty for inline png_base64."}
  },
  "required": ["index"]
}`)

// --- preview_slide_wireframe ---
var outputSchemaPreviewSlideWireframe = json.RawMessage(`{
  "type": "object",
  "properties": {
    "index":             {"type": "integer"},
    "svg":               {"type": "string", "description": "SVG document (omitted when format=\"png\")."},
    "png_base64":        {"type": "string", "description": "Base64-encoded PNG (omitted when format=\"svg\")."},
    "width":             {"type": "integer", "description": "Canvas width in pixels."},
    "height":            {"type": "integer", "description": "Canvas height in pixels."},
    "cell_count":        {"type": "integer", "description": "Number of resolved grid cells drawn."},
    "placeholder_count": {"type": "integer", "description": "Number of layout placeholders drawn."},
    "finding_count":     {"type": "integer", "description": "Number of fit findings overlaid (cell-attached + footer)."},
    "layout_id":         {"type": "string"},
    "layout_name":       {"type": "string"},
    "slide_type":        {"type": "string"},
    "warnings":          {"type": "array", "items": {"type": "string"}},
    "errors":            {"type": "array", "items": {"type": "string"}}
  },
  "required": ["index", "cell_count", "placeholder_count", "finding_count"]
}`)

// --- render_deck_thumbnails ---
var outputSchemaRenderDeckThumbnails = json.RawMessage(`{
  "type": "object",
  "properties": {
    "slides": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "index":        {"type": "integer"},
          "png_base64":   {"type": "string"},
          "path":         {"type": "string", "description": "Content-addressed artifact path, returned instead of png_base64 when a thumbnail exceeds the inline cap (~200KB). The filename embeds the PNG content hash, so the path is collision-free across decks and never overwritten with different content."},
          "width":        {"type": "integer"},
          "height":       {"type": "integer"},
          "size_error":   {"type": "string"},
          "content_hash": {"type": "string", "description": "SHA-256 of the rendered thumbnail PNG bytes."},
          "source_hash":  {"type": "string", "description": "PPTX file content hash this thumbnail was rendered from."},
          "cleanup":      {"type": "string", "description": "Lifetime/cleanup semantics of the on-disk path artifact. Set only when path is returned."}
        },
        "required": ["index"]
      }
    },
    "truncated": {"type": "boolean"}
  },
  "required": ["slides", "truncated"]
}`)

// --- inspect_slide_images ---
var outputSchemaInspectSlideImages = json.RawMessage(`{
  "type": "object",
  "properties": {
    "template":    {"type": "string"},
    "mode":        {"type": "string", "enum": ["vision", "heuristic"], "description": "Which backend produced the report. 'vision' = Claude vision API; 'heuristic' = pure-Go fallback when ANTHROPIC_API_KEY is unset."},
    "slide_count": {"type": "integer"},
    "results": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index": {"type": "integer"},
          "slide_type":  {"type": "string"},
          "raw_output":  {"type": "string"},
          "error":       {"type": "string"},
          "findings": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "slide_index": {"type": "integer"},
                "slide_type":  {"type": "string"},
                "severity":    {"type": "string", "description": "P0 (catastrophic) | P1 (major) | P2 (minor) | P3 (nitpick). Heuristic findings are always P3."},
                "category":    {"type": "string", "description": "text_overflow | text_truncation | contrast | alignment | spacing | overlap | missing_content | font_size | visual_hierarchy | chart_readability | table_readability | image_quality | layout_balance | color_consistency | border_style | footer_clearance | aspect_ratio"},
                "description": {"type": "string"},
                "location":    {"type": "string"},
                "source":      {"type": "string", "enum": ["vision", "heuristic"], "description": "Which checker produced this finding. Heuristic findings are advisory and may have higher false-positive rates."},
                "suggested_fixes": {
                  "type": "array",
                  "description": "repair_slide fix kinds pre-mapped from category (via SuggestedFixesForCategory). Empty for review-only categories (image_quality, aspect_ratio, border_style).",
                  "items": {
                    "type": "object",
                    "properties": {
                      "kind":   {"type": "string"},
                      "params": {"type": "object"}
                    },
                    "required": ["kind"]
                  }
                }
              },
              "required": ["slide_index", "severity", "category", "description"]
            }
          }
        },
        "required": ["slide_index", "findings"]
      }
    },
    "total_p0":     {"type": "integer"},
    "total_p1":     {"type": "integer"},
    "total_p2":     {"type": "integer"},
    "total_p3":     {"type": "integer"},
    "total_issues": {"type": "integer"},
    "findings": ` + findingEnvelopeSchema + `
  },
  "required": ["slide_count", "results", "total_issues", "findings"]
}`)

// --- score_deck ---
var outputSchemaScoreDeck = json.RawMessage(`{
  "type": "object",
  "properties": {
    "overall_score": {"type": "integer"},
    "per_slide": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index": {"type": "integer"},
          "score":       {"type": "integer"},
          "findings":    {"type": "array", "items": {"type": "object"}}
        }
      }
    },
    "composition": {
      "type": "object",
      "description": "Deck-level composition score (rhythm, variety, accent balance). Separate axis from per-slide correctness.",
      "properties": {
        "score":       {"type": "integer", "description": "0-100 composition quality score"},
        "diagnostics": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "code":     {"type": "string"},
              "severity": {"type": "string"},
              "message":  {"type": "string"}
            }
          }
        }
      }
    },
    "summary": {
      "type": "object",
      "properties": {
        "top_codes":            {"type": "array", "items": {"type": "object"}},
        "slide_count":          {"type": "integer"},
        "problem_slides_count": {"type": "integer"}
      }
    },
    "quality_gate": {
      "type": "object",
      "description": "Machine-readable definition of done. passed=true iff every criterion is satisfied; reasons enumerates the unmet ones. Agents (and the visual-QA loop) should stop calling repair tools when passed=true.",
      "properties": {
        "passed":  {"type": "boolean", "description": "True iff the deck meets ship-quality thresholds. When true, the agent should stop iterating."},
        "reasons": {"type": "array", "items": {"type": "string"}, "description": "Human-readable list of unmet criteria. Empty when passed=true."},
        "criteria": {
          "type": "object",
          "description": "Thresholds applied. Echoed in every response so agents can pin a known gate version in their own tests.",
          "properties": {
            "min_score":                  {"type": "integer", "description": "Minimum overall_score (default 80)."},
            "max_p0_findings":            {"type": "integer", "description": "Maximum refuse-action findings tolerated (default 0)."},
            "max_p1_findings":            {"type": "integer", "description": "Maximum shrink_or_split-action findings tolerated (default 0)."},
            "require_takeaway_on_charts": {"type": "boolean", "description": "Whether takeaway_missing findings fail the gate (default true)."},
            "allow_accent_overload":      {"type": "boolean", "description": "Whether accent_overload findings are permitted (default false)."}
          },
          "required": ["min_score", "max_p0_findings", "max_p1_findings", "require_takeaway_on_charts", "allow_accent_overload"]
        }
      },
      "required": ["passed", "reasons", "criteria"]
    },
    "mode_used": {"type": "string"},
    "render_evidence": ` + renderEvidenceSchema + `
  },
  "required": ["overall_score", "per_slide", "summary", "mode_used"]
}`)

// --- score_candidates ---
var outputSchemaScoreCandidates = json.RawMessage(`{
  "type": "object",
  "properties": {
    "slide_index": {"type": "integer", "description": "0-based slide position the candidates were scored against."},
    "candidates": {
      "type": "array",
      "description": "Candidates ranked best→worst by score; rank is 1-based.",
      "items": {
        "type": "object",
        "properties": {
          "index":          {"type": "integer", "description": "0-based position of this candidate in the request candidates array."},
          "rank":           {"type": "integer", "description": "1-based rank after sorting (1 = best)."},
          "score":          {"type": "integer", "description": "Combined deterministic score 0-100 (slide_score - rhythm_penalty)."},
          "slide_score":    {"type": "integer", "description": "100 - sum(severity weights) of fit findings scoped to the target slide."},
          "rhythm_penalty": {"type": "integer", "description": "Penalty subtracted for pattern repetition. 0, 5, or 15."},
          "findings":       {"type": "array", "items": {"type": "object"}, "description": "Deterministic findings (overflow, contrast, occupancy, etc.) scoped to the target slide for this candidate."},
          "notes":          {"type": "array", "items": {"type": "string"}, "description": "Human-readable rhythm/occupancy notes that explain the rhythm_penalty."},
          "parse_error":    {"type": "string", "description": "Set when the candidate JSON failed to decode; score will be 0 and the candidate ranks last."}
        },
        "required": ["index", "rank", "score", "slide_score", "rhythm_penalty"]
      }
    },
    "mode_used": {"type": "string"}
  },
  "required": ["slide_index", "candidates", "mode_used"]
}`)

// --- preview_presentation_plan ---
var outputSchemaPreviewPlan = json.RawMessage(`{
  "type": "object",
  "properties": {
    "resolved_slides": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index":           {"type": "integer"},
          "layout_id":             {"type": "string"},
          "layout_id_source":      {"type": "string"},
          "layout_name":           {"type": "string"},
          "slide_type":            {"type": "string"},
          "placeholders":          {"type": "array", "items": {"type": "object"}},
          "expanded_pattern":      {"type": "object"},
          "expanded_compose": {
            "type": "object",
            "description": "Per-segment expansion of a compose envelope. Lets agents attribute findings (segment_index) and inspect each child's bounds_pct + row/col ranges in the merged grid.",
            "properties": {
              "direction": {"type": "string", "enum": ["vertical", "horizontal"]},
              "segments": {
                "type": "array",
                "items": {
                  "type": "object",
                  "properties": {
                    "index":                 {"type": "integer"},
                    "pattern":               {"type": "string"},
                    "cells_after_expansion": {"type": "integer"},
                    "bounds_pct": {
                      "type": "object",
                      "properties": {
                        "x_pct":      {"type": "number"},
                        "y_pct":      {"type": "number"},
                        "width_pct":  {"type": "number"},
                        "height_pct": {"type": "number"}
                      },
                      "required": ["x_pct", "y_pct", "width_pct", "height_pct"]
                    },
                    "row_range": {"type": "array", "items": {"type": "integer"}, "minItems": 2, "maxItems": 2},
                    "col_range": {"type": "array", "items": {"type": "integer"}, "minItems": 2, "maxItems": 2}
                  },
                  "required": ["index", "pattern", "cells_after_expansion", "bounds_pct"]
                }
              }
            },
            "required": ["direction", "segments"]
          },
          "shape_grid_resolution": {
            "type": "object",
            "properties": {
              "virtual_layout_used": {"type": "boolean"},
              "layout_id":           {"type": "string"},
              "geometry":            {"type": "object"},
              "cells": {
                "type": "array",
                "description": "Per-cell wireframe rectangles in EMU (914400 EMU = 1 inch). Resolved from the shape_grid (including patterns/compose) without rendering.",
                "items": {
                  "type": "object",
                  "properties": {
                    "row":  {"type": "integer"},
                    "col":  {"type": "integer"},
                    "x":    {"type": "integer"},
                    "y":    {"type": "integer"},
                    "w":    {"type": "integer"},
                    "h":    {"type": "integer"},
                    "kind": {"type": "string", "enum": ["shape", "table", "icon", "image", "diagram"]}
                  },
                  "required": ["row", "col", "x", "y", "w", "h", "kind"]
                }
              }
            }
          },
          "applied_defaults":      {"type": "object"}
        },
        "required": ["slide_index", "layout_id", "layout_id_source", "placeholders"]
      }
    },
    "warnings":    {"type": "array", "items": {"type": "string"}},
    "errors":      {"type": "array", "items": {"type": "string"}},
    "fit_findings": {"type": "array", "items": {"type": "object"}},
    "response_fingerprint": {"type": "string", "description": "Lowercase sha256 hex (64 chars) over the canonical JSON of this response (with the field zeroed). Use as cache key / drift detector."}
  },
  "required": ["resolved_slides"]
}`)

// --- examine_template ---
// Mirrors examine.Report (internal/examine/examine.go), the same shape the CLI
// writes to report.json. The findings field embeds the shared FindingEnvelope.
var outputSchemaExamineTemplate = json.RawMessage(`{
  "type": "object",
  "properties": {
    "template":     {"type": "string", "description": "Template display name (base file name)."},
    "sha256":       {"type": "string", "description": "SHA-256 of the template bytes."},
    "aspect_ratio": {"type": "string", "description": "Slide aspect ratio (e.g. 16:9)."},
    "slide": {
      "type": "object",
      "properties": {
        "width_emu":  {"type": "integer"},
        "height_emu": {"type": "integer"},
        "width_in":   {"type": "number"},
        "height_in":  {"type": "number"}
      },
      "required": ["width_emu", "height_emu", "width_in", "height_in"]
    },
    "theme": {
      "type": "object",
      "properties": {
        "name":       {"type": "string"},
        "title_font": {"type": "string"},
        "body_font":  {"type": "string"},
        "colors":     {"type": "object", "additionalProperties": {"type": "string"}, "description": "Scheme color name → hex (e.g. {\"accent1\":\"#336699\"})."}
      },
      "required": ["name", "title_font", "body_font", "colors"]
    },
    "masters": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name":     {"type": "string"},
          "xml_path": {"type": "string", "description": "Path of the master XML within the PPTX package."}
        },
        "required": ["name", "xml_path"]
      }
    },
    "canonical_coverage": {
      "type": "object",
      "description": "Per-family coverage of the four content-bearing canonical layout families. Keyed by family name.",
      "additionalProperties": {
        "type": "object",
        "properties": {
          "family":  {"type": "string"},
          "present": {"type": "boolean"},
          "layouts": {"type": "array", "items": {"type": "string"}, "description": "Names of the layouts providing this family."}
        },
        "required": ["family", "present", "layouts"]
      }
    },
    "derivable_layouts": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name":    {"type": "string"},
          "ready":   {"type": "boolean"},
          "missing": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["name", "ready"]
      }
    },
    "layouts": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "index":                {"type": "integer"},
          "id":                   {"type": "string"},
          "name":                 {"type": "string"},
          "tags":                 {"type": "array", "items": {"type": "string"}},
          "canonical_type":       {"type": "string"},
          "canonical_family":     {"type": "string"},
          "canonical_confidence": {"type": "number"},
          "asset_base":           {"type": "string", "description": "Shared base name the CLI uses for this layout's artifacts."},
          "xml_path":             {"type": "string"},
          "content_zone": {
            "type": "object",
            "description": "Derived safe content area in EMU (title-bottom, footer-top, side margins).",
            "properties": {
              "left_emu":   {"type": "integer"},
              "top_emu":    {"type": "integer"},
              "right_emu":  {"type": "integer"},
              "bottom_emu": {"type": "integer"}
            },
            "required": ["left_emu", "top_emu", "right_emu", "bottom_emu"]
          },
          "placeholders": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "id":              {"type": "string"},
                "type":            {"type": "string"},
                "role":            {"type": "string"},
                "role_confidence": {"type": "number"},
                "index":           {"type": "integer"},
                "z_index":         {"type": "integer", "description": "Document order in the layout shape tree (later = drawn on top)."},
                "font_pt":         {"type": "number", "description": "Font-aware point size the engine uses for fit decisions."},
                "max_chars":       {"type": "integer", "description": "Font-aware character budget."},
                "bounds": {
                  "type": "object",
                  "properties": {
                    "x_emu": {"type": "integer"},
                    "y_emu": {"type": "integer"},
                    "w_emu": {"type": "integer"},
                    "h_emu": {"type": "integer"},
                    "x_in":  {"type": "number"},
                    "y_in":  {"type": "number"},
                    "w_in":  {"type": "number"},
                    "h_in":  {"type": "number"}
                  },
                  "required": ["x_emu", "y_emu", "w_emu", "h_emu", "x_in", "y_in", "w_in", "h_in"]
                }
              },
              "required": ["id", "type", "role", "index", "z_index", "font_pt", "max_chars", "bounds"]
            }
          }
        },
        "required": ["index", "id", "name", "canonical_type", "canonical_family", "asset_base", "xml_path", "content_zone", "placeholders"]
      }
    },
    "findings": ` + findingEnvelopeSchema + `
  },
  "required": ["template", "aspect_ratio", "slide", "theme", "canonical_coverage", "layouts", "findings"]
}`)

// --- apply_deck_patch ---
var outputSchemaApplyDeckPatch = json.RawMessage(`{
  "type": "object",
  "properties": {
    "patched_deck": {"type": "object", "description": "The deck after applying every operation. Same schema as generate_presentation's presentation input — feed it straight into validate_input / generate_presentation / repair_slide. Present only on success; a rejected (atomic) patch returns an error envelope instead, with no patched_deck."},
    "applied_ops": {
      "type": "array",
      "description": "One entry per operation, in request order. On a success response every entry has applied=true (the patch is atomic).",
      "items": {
        "type": "object",
        "properties": {
          "op":      {"type": "string", "enum": ["insert_slide", "remove_slide", "replace_slide", "move_slide", "duplicate_slide", "replace_field"]},
          "applied": {"type": "boolean"},
          "message": {"type": "string"}
        },
        "required": ["op", "applied"]
      }
    },
    "findings": ` + findingEnvelopeSchema + `
  },
  "required": ["patched_deck", "applied_ops", "findings"]
}`)

// findingEnvelopeSchema is the JSON Schema fragment for a diagnostics.
// FindingEnvelope (docs/api/finding-envelope.schema.json). Surfaces that embed
// the envelope concatenate this fragment so the shape stays in one place.
const findingEnvelopeSchema = `{
      "type": "object",
      "properties": {
        "schema_version": {"type": "string"},
        "tool":           {"type": "string"},
        "subcommand":     {"type": "string"},
        "input_sha256":   {"type": "string"},
        "template":       {"type": "string"},
        "ok":             {"type": "boolean"},
        "summary":        {"type": "string"},
        "findings":       {"type": "array", "items": {"type": "object"}}
      },
      "required": ["schema_version", "tool", "subcommand", "ok", "summary", "findings"]
    }`

// --- repair_slide ---
var outputSchemaRepairSlide = json.RawMessage(`{
  "type": "object",
  "properties": {
    "patched_deck": {"type": "object"},
    "applied_fixes": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "kind":            {"type": "string"},
          "applied":         {"type": "boolean"},
          "message":         {"type": "string"},
          "code":            {"type": "string"},
          "supported_kinds": {"type": "array", "items": {"type": "string"}},
          "next_tool_call": {
            "type": "object",
            "properties": {
              "tool":          {"type": "string"},
              "args_template": {"type": "object"}
            },
            "required": ["tool", "args_template"]
          }
        },
        "required": ["kind", "applied"]
      }
    },
    "findings": ` + findingEnvelopeSchema + `
  },
  "required": ["patched_deck", "applied_fixes", "findings"]
}`)

// --- repair_slides_batch ---
var outputSchemaRepairSlidesBatch = json.RawMessage(`{
  "type": "object",
  "properties": {
    "patched_deck": {"type": "object"},
    "applied_fixes": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index":     {"type": "integer"},
          "kind":            {"type": "string"},
          "applied":         {"type": "boolean"},
          "message":         {"type": "string"},
          "code":            {"type": "string"},
          "supported_kinds": {"type": "array", "items": {"type": "string"}},
          "next_tool_call": {
            "type": "object",
            "properties": {
              "tool":          {"type": "string"},
              "args_template": {"type": "object"}
            },
            "required": ["tool", "args_template"]
          }
        },
        "required": ["slide_index", "kind", "applied"]
      }
    },
    "findings": ` + findingEnvelopeSchema + `
  },
  "required": ["patched_deck", "applied_fixes", "findings"]
}`)

// --- auto_repair ---
// visualQAQualityModeSchema and visualQAResultSchema are shared by the
// auto_repair and make_deck output schemas so the two stay in lockstep. They
// describe the truth-labeled quality_mode and the opt-in visual_qa phase report.
const visualQAQualityModeSchema = `{"type": "string", "enum": ["deterministic", "deterministic+visual_qa"], "description": "Truth-label for which inspection regime ran. \"deterministic\" (default) = static + render-fit findings only, no rendering or API key. \"deterministic+visual_qa\" = the deterministic loop followed by the opt-in vision/heuristic visual refinement phase (set visual_qa.enabled=true)."}`

const visualQAResultSchema = `{
      "type": "object",
      "description": "Visual-QA phase report. Present only when visual_qa mode was requested. Records the inspection backend, per-pass thumbnail paths/findings/repairs, and optional palette audit. Any repairs applied here are also reflected in final_presentation.",
      "properties": {
        "requested":       {"type": "boolean"},
        "artifact_consistent": {"type": "boolean", "description": "True when final_presentation matches the PPTX at the response path. Each visual-repair pass is staged: applied, re-rendered, and rolled back in memory if its re-render fails, so JSON and PPTX always advance together. False ONLY in the defensive case where a re-render failed AND the in-memory repairs could not be reverted — final_presentation then reflects changes the PPTX does not, a blocking notes[] entry explains it, and the artifact must not be shipped."},
        "inspection_mode": {"type": "string", "enum": ["vision", "heuristic", "skipped"], "description": "vision = Claude vision API (ANTHROPIC_API_KEY set); heuristic = pure-Go fallback (no key); skipped = render tools unavailable, no inspection ran."},
        "model":           {"type": "string", "description": "Resolved vision model (empty in heuristic/skipped modes)."},
        "requirements": {
          "type": "object",
          "description": "Preconditions and cost of vision-backed inspection.",
          "properties": {
            "api_key_env":         {"type": "string"},
            "api_key_present":     {"type": "boolean"},
            "default_model":       {"type": "string"},
            "render_dependencies": {"type": "array", "items": {"type": "string"}},
            "render_available":    {"type": "boolean"},
            "render_missing":      {"type": "array", "items": {"type": "string"}},
            "cost_note":           {"type": "string"}
          },
          "required": ["api_key_env", "api_key_present", "default_model", "render_dependencies", "render_available", "cost_note"]
        },
        "passes": {
          "type": "array",
          "description": "Per-pass record of the render→inspect→repair iterations.",
          "items": {
            "type": "object",
            "properties": {
              "pass":            {"type": "integer"},
              "inspection_mode": {"type": "string"},
              "thumbnail_paths": {"type": "array", "items": {"type": "string"}, "description": "Stable on-disk paths to the inspected thumbnails."},
              "visual_findings": {"type": "array", "items": {"type": "object"}, "description": "All visualqa.Finding objects (every severity), in slide order."},
              "proposed_repairs": {
                "type": "array",
                "items": {"type": "object", "properties": {"slide_index": {"type": "integer"}, "kind": {"type": "string"}, "category": {"type": "string"}}}
              },
              "repairs_applied": {"type": "array", "items": {"type": "string"}}
            },
            "required": ["pass", "inspection_mode", "thumbnail_paths", "visual_findings", "proposed_repairs", "repairs_applied"]
          }
        },
        "palette_audit": {
          "type": "object",
          "description": "Deterministic palette ΔE audit. Present only when visual_qa.audit_palette=true.",
          "properties": {
            "available":  {"type": "boolean"},
            "violations": {"type": "integer"},
            "findings":   {"type": "object", "description": "FindingEnvelope of RENDER.palette_drift findings."},
            "note":       {"type": "string"}
          },
          "required": ["available", "violations", "findings"]
        },
        "notes": {"type": "array", "items": {"type": "string"}, "description": "Human-readable explanations for transparent fallbacks (missing render tools, missing API key, re-render failures, rolled-back repairs)."}
      },
      "required": ["requested", "artifact_consistent", "inspection_mode", "requirements", "passes"]
    }`

// renderEvidenceSchema documents the render_evidence block emitted by
// score_deck / auto_repair / make_deck when the render pass that backs the
// score did not complete. Its presence is the unambiguous "not a clean pass"
// signal.
const renderEvidenceSchema = `{
      "type": "object",
      "description": "Present only when the render pass that backs the score did NOT complete (slide conversion, temp-dir creation, or generation failed). Its presence means the score and gate reflect static analysis only; an explicit RENDER_EVIDENCE_INCOMPLETE finding accompanies it.",
      "properties": {
        "complete": {"type": "boolean", "description": "False when render-time evidence is incomplete (the block is only emitted in that case)."},
        "stage":    {"type": "string", "enum": ["convert", "tempdir", "generate"], "description": "Which render stage failed."},
        "detail":   {"type": "string", "description": "Underlying error text for the failed stage."},
        "degraded": {"type": "boolean", "description": "True when the caller passed allow_degraded_scoring=true, so the incompleteness is advisory rather than blocking. The score is still labeled degraded and must not be treated as a clean pass."}
      },
      "required": ["complete"]
    }`

// outputValidationSchema documents the final structural/output validation block
// emitted by auto_repair / make_deck. A publishable / gate-passed result
// requires ran=true and valid=true.
const outputValidationSchema = `{
      "type": "object",
      "description": "Final structural/output validation of the rendered PPTX (pptx.ValidateOutputFile), run after any visual-QA re-render and regardless of degraded scoring. A gate-passed / publishable result requires ran=true and valid=true.",
      "properties": {
        "ran":      {"type": "boolean", "description": "Whether the validation suite ran."},
        "valid":    {"type": "boolean", "description": "True when the rendered PPTX has no blocking structural findings."},
        "blocking": {"type": "array", "items": {"type": "string"}, "description": "Formatted blocking findings that forced the gate open. Present only when valid=false."}
      },
      "required": ["ran", "valid"]
    }`

var outputSchemaAutoRepair = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path":        {"type": "string", "description": "Absolute path to the final rendered PPTX. Written regardless of whether the gate ultimately passed."},
    "final_score": {"type": "integer", "description": "Overall_score of the last (post-repair) pass, in [0, 100]."},
    "gate_passed": {"type": "boolean", "description": "True iff the gate criteria were met within max_passes iterations."},
    "passes":      {"type": "integer", "description": "Number of iterations actually run (≤ max_passes)."},
    "trace": {
      "type": "array",
      "description": "Per-pass record of score, finding count, and the repairs applied DURING that pass (empty on the final converged pass).",
      "items": {
        "type": "object",
        "properties": {
          "pass":            {"type": "integer"},
          "score":           {"type": "integer"},
          "findings_count":  {"type": "integer"},
          "repairs_applied": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["pass", "score", "findings_count", "repairs_applied"]
      }
    },
    "gate_reasons": {
      "type": "array",
      "description": "Human-readable list of unmet gate criteria. Present only when gate_passed=false.",
      "items": {"type": "string"}
    },
    "final_presentation": {"type": "object", "description": "The full repaired deck JSON after the convergence loop (same schema as generate_presentation's presentation input). Always present on success, including zero-repair runs. Reflects any visual_qa repairs. Feed it straight back into validate_input / generate_presentation / repair_slide to continue editing without reconstructing state from the trace."},
    "quality_mode": ` + visualQAQualityModeSchema + `,
    "visual_qa": ` + visualQAResultSchema + `,
    "evidence_complete": {"type": "boolean", "description": "True only when the render pass that backed the score completed AND final structural output validation passed. gate_passed cannot be true on incomplete evidence unless allow_degraded_scoring was set, in which case render_evidence.degraded labels the result and evidence_complete stays false."},
    "render_evidence": ` + renderEvidenceSchema + `,
    "output_validation": ` + outputValidationSchema + `,
    "idempotent_replay": {"type": "boolean", "description": "True when this response was served from the idempotency cache (the caller passed an idempotency_key that matched a prior successful call)."}
  },
  "required": ["final_score", "gate_passed", "passes", "trace", "quality_mode", "final_presentation", "evidence_complete"]
}`)

// --- make_deck ---
var outputSchemaMakeDeck = json.RawMessage(`{
  "type": "object",
  "properties": {
    "path":        {"type": "string", "description": "Absolute path to the final rendered PPTX. Written regardless of whether the gate passed."},
    "final_score": {"type": "integer", "description": "overall_score of the last (post-repair) pass, in [0, 100]."},
    "gate_passed": {"type": "boolean", "description": "True iff the gate criteria were met within max_repair_passes iterations."},
    "passes":      {"type": "integer", "description": "Number of auto_repair iterations actually run (≤ max_repair_passes)."},
    "trace": {
      "type": "array",
      "description": "Per-pass record of score, finding count, and repairs applied DURING that pass. Same shape as auto_repair.trace.",
      "items": {
        "type": "object",
        "properties": {
          "pass":            {"type": "integer"},
          "score":           {"type": "integer"},
          "findings_count":  {"type": "integer"},
          "repairs_applied": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["pass", "score", "findings_count", "repairs_applied"]
      }
    },
    "gate_reasons": {
      "type": "array",
      "description": "Human-readable list of unmet gate criteria. Present only when gate_passed=false.",
      "items": {"type": "string"}
    },
    "plan": {
      "type": "object",
      "description": "Snapshot of the planner's decisions for this deck. Use slides[].slide_index to target follow-up repair_slide calls.",
      "properties": {
        "template":     {"type": "string"},
        "slide_budget": {"type": "integer"},
        "slides": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "slide_index":         {"type": "integer"},
              "narrative_role":      {"type": "string", "enum": ["opening", "evidence", "comparison", "emphasis", "framework", "closing"]},
              "recommended_pattern": {"type": "string"},
              "title":               {"type": "string"}
            },
            "required": ["slide_index", "narrative_role", "recommended_pattern", "title"]
          }
        }
      },
      "required": ["template", "slide_budget", "slides"]
    },
    "final_presentation": {"type": "object", "description": "The full deck JSON the engine authored and repaired (same schema as generate_presentation's presentation input). Always present on success. Reflects any visual_qa repairs. Feed it straight back into validate_input / generate_presentation / repair_slide to continue editing without rebuilding it from the plan summary or trace."},
    "quality_mode": ` + visualQAQualityModeSchema + `,
    "visual_qa": ` + visualQAResultSchema + `,
    "evidence_complete": {"type": "boolean", "description": "True only when the render pass that backed the score completed AND final structural output validation passed. Mirrors auto_repair.evidence_complete."},
    "render_evidence": ` + renderEvidenceSchema + `,
    "output_validation": ` + outputValidationSchema + `,
    "idempotent_replay": {"type": "boolean", "description": "True when this response was served from the idempotency cache (the caller passed an idempotency_key that matched a prior successful call)."}
  },
  "required": ["final_score", "gate_passed", "passes", "trace", "quality_mode", "plan", "final_presentation", "evidence_complete"]
}`)

// --- table_density_guide ---
var outputSchemaTableDensityGuide = json.RawMessage(`{
  "type": "object",
  "properties": {
    "tiers": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "data_rows":    {"type": "string"},
          "font_size":    {"type": "string"},
          "max_columns":  {"type": "integer"},
          "tdr_ceiling":  {"type": "integer"},
          "notes":        {"type": "string"}
        },
        "required": ["data_rows", "font_size", "max_columns", "tdr_ceiling"]
      }
    },
    "limits": {
      "type": "object",
      "properties": {
        "max_rows":     {"type": "integer"},
        "max_columns":  {"type": "integer"},
        "min_font_pt":  {"type": "integer"},
        "split_advice": {"type": "string"}
      }
    },
    "multiline_note": {"type": "string"},
    "table_styles":   {"type": "array", "items": {"type": "object"}},
    "template":       {"type": "string"}
  },
  "required": ["tiers", "limits", "multiline_note"]
}`)

// --- resolve_theme ---
var outputSchemaResolveTheme = json.RawMessage(`{
  "type": "object",
  "properties": {
    "template":     {"type": "string"},
    "colors":       {"type": "object", "additionalProperties": {"type": "string"}, "description": "Map of scheme name → hex (e.g., {\"accent1\":\"#336699\"}). Convenient for lookups."},
    "theme_colors": {
      "type": "array",
      "description": "Same palette as colors, but in the [{name,rgb}] shape svggen-mcp's StyleSpec.theme_colors expects. Copy this array straight into render_diagram's style.theme_colors to keep native OOXML and SVG renders on the same palette.",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string", "description": "Scheme color name (accent1..accent6, dk1, dk2, lt1, lt2, hlink, folHlink)."},
          "rgb":  {"type": "string", "description": "Hex value with leading '#' (e.g., \"#336699\")."}
        },
        "required": ["name", "rgb"]
      }
    },
    "color_roles":  {"type": "object"},
    "fonts": {
      "type": "object",
      "properties": {
        "major": {"type": "object", "properties": {"latin": {"type": "string"}}},
        "minor": {"type": "object", "properties": {"latin": {"type": "string"}}}
      }
    },
    "resolved_for": {"type": "array", "items": {"type": "string"}},
    "unknown":      {"type": "array", "items": {"type": "object"}},
    "applied_theme_override": {
      "type": "object",
      "properties": {
        "colors":     {"type": "object", "additionalProperties": {"type": "string"}},
        "title_font": {"type": "string"},
        "body_font":  {"type": "string"}
      }
    },
    "warnings": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["template", "colors", "theme_colors", "fonts"]
}`)

// --- list_template_settings ---
var outputSchemaListTemplateSettings = json.RawMessage(`{
  "type": "object",
  "properties": {
    "template":     {"type": "string"},
    "table_styles": {"type": "object"},
    "cell_styles":  {"type": "object"}
  },
  "required": ["template", "table_styles", "cell_styles"]
}`)

// --- register_template_setting ---
var outputSchemaRegisterTemplateSetting = json.RawMessage(`{
  "type": "object",
  "properties": {
    "written":  {"type": "boolean"},
    "path":     {"type": "string"},
    "template": {"type": "string"},
    "kind":     {"type": "string"},
    "name":     {"type": "string"}
  },
  "required": ["written", "path", "template", "kind", "name"]
}`)

// --- delete_template_setting ---
var outputSchemaDeleteTemplateSetting = json.RawMessage(`{
  "type": "object",
  "properties": {
    "removed":  {"type": "boolean"},
    "template": {"type": "string"},
    "kind":     {"type": "string"},
    "name":     {"type": "string"}
  },
  "required": ["removed", "template", "kind", "name"]
}`)

// --- get_capabilities ---
var outputSchemaGetCapabilities = json.RawMessage(`{
  "type": "object",
  "properties": {
    "schema_version":     {"type": "string"},
    "tool_version":       {"type": "string"},
    "changelog_url":      {"type": "string"},
    "mcp_tools_available": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name":     {"type": "string"},
          "added_in": {"type": "string"},
          "kind":     {"type": "string", "enum": ["primitive", "workflow_facade", "diagnostic"]},
          "phase":    {"type": "string", "enum": ["discovery", "plan", "vary", "render", "repair", "settings"]},
          "mutates_state":     {"type": "boolean"},
          "writes_files":      {"type": "boolean"},
          "render_dependency": {"type": "boolean"},
          "api_key_dependency": {"type": "boolean"},
          "cli_counterpart":   {"type": "string"},
          "mcp_only_reason":   {"type": "string"},
          "primitive_alternatives": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["name", "added_in", "kind", "phase"]
      }
    },
    "tool_list": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name":        {"type": "string"},
          "description": {"type": "string"}
        },
        "required": ["name", "description"]
      }
    },
    "registry": {
      "type": "object",
      "properties": {
        "charts":   {"type": "array", "items": {"type": "string"}},
        "diagrams": {"type": "array", "items": {"type": "string"}},
        "patterns": {"type": "array", "items": {"type": "string"}}
      },
      "required": ["charts", "diagrams", "patterns"]
    },
    "deprecated_fields": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "path":        {"type": "string"},
          "replacement": {"type": "string"},
          "removed_in":  {"type": "string"}
        },
        "required": ["path", "replacement", "removed_in"]
      }
    },
    "deprecations": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "path":        {"type": "string"},
          "replacement": {"type": "string"},
          "removed_in":  {"type": "string"}
        },
        "required": ["path", "replacement", "removed_in"]
      }
    },
    "features": {
      "type": "object",
      "properties": {
        "strict_fit":             {"type": "array", "items": {"type": "string"}},
        "compact_responses":      {"type": "boolean"},
        "fit_report": {
          "type": "object",
          "properties": {
            "supported":  {"type": "boolean"},
            "default_in": {"type": "object", "additionalProperties": {"type": "boolean"}}
          },
          "required": ["supported", "default_in"]
        },
        "strict_unknown_keys":    {"type": "boolean"},
        "output_validation":      {"type": "array", "items": {"type": "string"}},
        "named_patterns":         {"type": "boolean"},
        "template_settings":      {"type": "boolean"},
        "supports_inline_markup": {"type": "array", "items": {"type": "string"}},
        "supports_speaker_notes": {"type": "boolean"},
        "compose": {
          "type": "object",
          "properties": {
            "max_segments":              {"type": "integer"},
            "max_nesting_depth":         {"type": "integer"},
            "max_leaf_patterns":         {"type": "integer"},
            "directions":                {"type": "array", "items": {"type": "string"}},
            "supports_smart_compose":    {"type": "boolean"},
            "supports_nested_compose":   {"type": "boolean"},
            "supports_diagram_segments": {"type": "boolean"}
          },
          "required": ["max_segments", "max_nesting_depth", "max_leaf_patterns", "directions", "supports_smart_compose", "supports_nested_compose", "supports_diagram_segments"]
        },
        "base_dir":              {"type": "array", "items": {"type": "string"}},
        "deck_chrome": {
          "type": "object",
          "properties": {
            "supported":  {"type": "boolean"},
            "version":    {"type": "string"},
            "usage_hint": {"type": "string"}
          },
          "required": ["supported", "version", "usage_hint"]
        },
        "page_numbers": {
          "type": "object",
          "properties": {
            "supported":  {"type": "boolean"},
            "version":    {"type": "string"},
            "usage_hint": {"type": "string"}
          },
          "required": ["supported", "version", "usage_hint"]
        },
        "section_structure": {
          "type": "object",
          "properties": {
            "supported":  {"type": "boolean"},
            "version":    {"type": "string"},
            "usage_hint": {"type": "string"}
          },
          "required": ["supported", "version", "usage_hint"]
        },
        "section_crumb": {
          "type": "object",
          "properties": {
            "supported":  {"type": "boolean"},
            "version":    {"type": "string"},
            "usage_hint": {"type": "string"}
          },
          "required": ["supported", "version", "usage_hint"]
        },
        "quality_modes": {
          "type": "object",
          "description": "auto_repair / make_deck inspection regimes. default is the deterministic loop; visual_qa_opt_in marks the opt-in vision/heuristic phase.",
          "properties": {
            "default":          {"type": "string"},
            "modes":            {"type": "array", "items": {"type": "string"}},
            "visual_qa_opt_in": {"type": "boolean"},
            "version":          {"type": "string"},
            "usage_hint":       {"type": "string"}
          },
          "required": ["default", "modes", "visual_qa_opt_in", "version", "usage_hint"]
        },
        "feature_versions":       {"type": "object", "additionalProperties": {"type": "string"}}
      },
      "required": ["deck_chrome", "page_numbers", "section_structure", "section_crumb", "quality_modes"]
    },
    "runtime": {
      "type": "object",
      "properties": {
        "settings_write_enabled":  {"type": "boolean"},
        "render_available":        {"type": "boolean"},
        "render_missing_commands": {"type": "array", "items": {"type": "string"}},
        "templates_dir":           {"type": "string"},
        "output_dir":              {"type": "string"},
        "schema_fingerprint":      {"type": "string"}
      },
      "required": ["settings_write_enabled", "render_available", "render_missing_commands", "templates_dir", "output_dir", "schema_fingerprint"]
    },
    "vocabularies": {
      "type": "object",
      "properties": {
        "repair_fix_kinds":     {"type": "array", "items": {"type": "string"}},
        "fit_finding_codes":    {"type": "array", "items": {"type": "string"}},
        "content_types":        {"type": "array", "items": {"type": "string"}},
        "slide_transitions":    {"type": "array", "items": {"type": "string"}},
        "transition_speeds":    {"type": "array", "items": {"type": "string"}},
        "build_animations":     {"type": "array", "items": {"type": "string"}},
        "chart_types":          {"type": "array", "items": {"type": "string"}},
        "diagram_types":        {"type": "array", "items": {"type": "string"}},
        "placeholder_aliases":  {"type": "object", "additionalProperties": {"type": "array", "items": {"type": "string"}}},
        "pattern_names":        {"type": "array", "items": {"type": "string"}},
        "pattern_aliases":      {"type": "object", "additionalProperties": {"type": "string"}}
      }
    },
    "error_codes": {"type": "array", "items": {"type": "string"}},
    "cli_only_commands": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name":            {"type": "string"},
          "cli_only_reason": {"type": "string"}
        },
        "required": ["name", "cli_only_reason"]
      }
    }
  },
  "required": ["schema_version", "tool_version", "changelog_url", "mcp_tools_available", "tool_list", "registry", "deprecations", "features", "runtime", "vocabularies", "error_codes", "cli_only_commands"]
}`)

// --- analyze_deck_rhythm ---
var outputSchemaAnalyzeDeckRhythm = json.RawMessage(`{
  "type": "object",
  "properties": {
    "per_slide": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index":     {"type": "integer"},
          "pattern":         {"type": "string"},
          "density_class":   {"type": "string", "enum": ["low", "med", "high"]},
          "accent_role":     {"type": "string"},
          "dominant_visual": {"type": "string"},
          "within_slide_accent_variety": {"type": "integer"}
        },
        "required": ["slide_index", "pattern", "density_class", "dominant_visual", "within_slide_accent_variety"]
      }
    },
    "aggregates": {
      "type": "object",
      "properties": {
        "pattern_runs": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name":  {"type": "string"},
              "start": {"type": "integer"},
              "len":   {"type": "integer"}
            },
            "required": ["name", "start", "len"]
          }
        },
        "longest_run":      {"type": "integer"},
        "repetition_index": {"type": "number"},
        "accent_balance":   {"type": "object", "additionalProperties": {"type": "number"}},
        "density_cv":       {"type": "number"},
        "density_distribution": {
          "type": "object",
          "properties": {
            "underfilled_cells": {"type": "integer"},
            "optimal_cells":    {"type": "integer"},
            "overflow_cells":   {"type": "integer"}
          },
          "required": ["underfilled_cells", "optimal_cells", "overflow_cells"]
        }
      },
      "required": ["pattern_runs", "longest_run", "repetition_index", "accent_balance", "density_cv", "density_distribution"]
    },
    "recommendations": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index":               {"type": "integer"},
          "message":                   {"type": "string"},
          "recommended_break_patterns": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["slide_index", "message", "recommended_break_patterns"]
      }
    },
    "composition_score": {"type": "integer"}
  },
  "required": ["per_slide", "aggregates", "recommendations", "composition_score"]
}`)

// --- read_presentation ---
var outputSchemaReadPresentation = json.RawMessage(`{
  "type": "object",
  "properties": {
    "slide_count": {"type": "integer"},
    "slides": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "index":          {"type": "integer"},
          "layout_id":      {"type": "string"},
          "placeholders":   {"type": "array", "items": {"type": "object"}},
          "shapes":         {"type": "array", "items": {"type": "object"}},
          "tables":         {"type": "array", "items": {"type": "object"}},
          "speaker_notes":  {"type": "string"}
        }
      }
    }
  },
  "required": ["slide_count", "slides"]
}`)

// --- plan_deck ---
var outputSchemaPlanDeck = json.RawMessage(`{
  "type": "object",
  "properties": {
    "slides": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index":          {"type": "integer"},
          "narrative_role":       {"type": "string", "enum": ["opening", "evidence", "comparison", "emphasis", "framework", "closing"]},
          "recommended_pattern":  {"type": "string"},
          "content_seed":         {"type": "string"},
          "rationale":            {"type": "string"},
          "suggested_pattern":    {"type": "string", "description": "First-choice pattern (same value as recommended_pattern; kept as a separate field for the suggested_pattern / suggested_pattern_fallback / skeleton agent-facing triplet)."},
          "suggested_pattern_fallback": {"type": "string", "description": "Second-choice pattern when the suggested pattern's content shape does not fit. Drawn from alternatives[0] when available."},
          "skeleton":             {"type": "object", "description": "Partial SlideInput JSON with __FILL__ tokens for every agent-supplied string. Copy and replace tokens rather than authoring the slide structure from scratch. Validates as-is with validate_input (stays valid=true), but any __FILL__ left unreplaced is reported as an unresolved_placeholder warning — replace every token before publishable generation, or pass placeholder_policy=strict to block on it."},
          "predicted_cell_budgets": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "columns":          {"type": "integer"},
                "rows":             {"type": "integer"},
                "body_max_chars":   {"type": "integer"},
                "header_max_chars": {"type": "integer"}
              },
              "required": ["columns", "rows", "body_max_chars", "header_max_chars"]
            }
          },
          "predicted_findings": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "code":           {"type": "string"},
                "path":           {"type": "string"},
                "message":        {"type": "string"},
                "action":         {"type": "string"},
                "next_tool_call": {"type": "string"}
              },
              "required": ["code", "message"]
            }
          },
          "alternatives": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "pattern_name": {"type": "string"},
                "score":        {"type": "number"},
                "rationale":    {"type": "string"},
                "template_support": {"$ref": "#/$defs/template_support"}
              },
              "required": ["pattern_name", "score", "rationale"]
            }
          },
          "template_support": {"$ref": "#/$defs/template_support"}
        },
        "required": ["slide_index", "narrative_role", "recommended_pattern", "suggested_pattern", "content_seed", "rationale"]
      }
    },
    "brief":        {"type": "string"},
    "slide_budget":  {"type": "integer"},
    "template":      {"type": "string", "description": "Echo of the template name the plan was vetted against (the 'template' argument). Present only when template context was supplied."},
    "rhythm_check": {
      "type": "object",
      "properties": {
        "longest_pattern_run": {"type": "integer"},
        "has_emphasis":        {"type": "boolean"},
        "emphasis_count":      {"type": "integer"},
        "pattern_variety":     {"type": "integer"}
      },
      "required": ["longest_pattern_run", "has_emphasis", "emphasis_count", "pattern_variety"]
    },
    "response_fingerprint": {"type": "string", "description": "Lowercase sha256 hex (64 chars) over the canonical JSON of this response (with the field zeroed). Use as cache key / drift detector."}
  },
  "required": ["slides", "brief", "slide_budget", "rhythm_check"],
  "$defs": {
    "template_support": {
      "type": "object",
      "description": "Per-pattern feasibility for the template passed in the 'template' argument. Present only when template context was supplied. The recommended pattern is never left unsupported when a supported alternative exists — it is swapped during planning. Computed by the same shared helper recommend_visual uses, so the two tools agree for identical template constraints.",
      "properties": {
        "status":          {"type": "string", "enum": ["supported", "risky", "unsupported"], "description": "supported = the template natively covers the needed layout/capability; risky = producible only via a synthesised/derived layout or close to a capacity/content-zone limit; unsupported = requires an absent canonical/derivable layout."},
        "reasons":         {"type": "array", "items": {"type": "string"}, "description": "Why the status applies: which layouts cover the candidate, what is synthesised, which capacity/content-zone constraint bites, or what is missing."},
        "required_layout": {"type": "string", "description": "The canonical layout or derivable capability the pattern needs (e.g. \"grid base\"). Omitted when there is no specific layout requirement."}
      },
      "required": ["status"]
    }
  }
}`)

// --- validate_presentation_output ---
var outputSchemaValidateOutput = json.RawMessage(`{
  "type": "object",
  "properties": {
    "is_valid":  {"type": "boolean"},
    "file_path": {"type": "string"},
    "summary":   {"type": "string"},
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "code":        {"type": "string"},
          "severity":    {"type": "string", "enum": ["blocking", "warning"]},
          "path":        {"type": "string"},
          "message":     {"type": "string"},
          "phase":       {"type": "string"},
          "validator":   {"type": "string"},
          "slide_index": {"type": "integer"},
          "source_path": {"type": "string"},
          "scope":       {"type": "string", "enum": ["source", "template", "generator"]}
        },
        "required": ["code", "severity", "message", "phase", "validator", "slide_index", "scope"]
      }
    }
  },
  "required": ["is_valid", "file_path", "summary"]
}`)

// --- get_started ---
var outputSchemaGetStarted = json.RawMessage(`{
  "type": "object",
  "properties": {
    "task":            {"type": "string"},
    "available_tasks": {"type": "array", "items": {"type": "string"}},
    "fast_path": {
      "type": "object",
      "description": "The recommended single-call workflow facade for this task (make_deck for brief, auto_repair for revise). Present only for tasks that have a facade; omitted for validate-only. Reach for this before the manual sequence when you do not need per-step control.",
      "properties": {
        "tool":          {"type": "string", "description": "The facade tool to call first (e.g. make_deck)."},
        "when_to_call":  {"type": "string", "description": "When to use the facade versus dropping to the manual sequence."},
        "falls_back_to": {"type": "array", "items": {"type": "string"}, "description": "The manual primitive tool names (this response's sequence) the facade collapses into one call."}
      },
      "required": ["tool", "when_to_call", "falls_back_to"]
    },
    "sequence": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "tool":         {"type": "string"},
          "when_to_call": {"type": "string"}
        },
        "required": ["tool", "when_to_call"]
      }
    },
    "notes": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["task", "sequence", "available_tasks"]
}`)

// --- get_input_schema ---
var outputSchemaGetInputSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "digest":       {"type": "string"},
    "not_modified": {"type": "boolean"},
    "schema":       {"type": "object"}
  },
  "required": ["digest"]
}`)

// --- propose_repairs ---
var outputSchemaProposeRepairs = json.RawMessage(`{
  "type": "object",
  "properties": {
    "slides": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "slide_index":   {"type": "integer"},
          "finding_count": {"type": "integer"},
          "directives": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "kind":   {"type": "string"},
                "params": {"type": "object"},
                "rank":   {"type": "integer"},
                "source": {
                  "type": "object",
                  "properties": {
                    "type":     {"type": "string", "enum": ["fit", "visual"]},
                    "code":     {"type": "string"},
                    "category": {"type": "string"},
                    "severity": {"type": "string"},
                    "action":   {"type": "string"},
                    "path":     {"type": "string"},
                    "message":  {"type": "string"}
                  },
                  "required": ["type"]
                },
                "tool_call": {
                  "type": "object",
                  "properties": {
                    "tool":          {"type": "string"},
                    "args_template": {"type": "object"}
                  },
                  "required": ["tool", "args_template"]
                }
              },
              "required": ["kind", "rank", "source"]
            }
          },
          "batch_tool_call": {
            "type": "object",
            "properties": {
              "tool":          {"type": "string"},
              "args_template": {"type": "object"}
            },
            "required": ["tool", "args_template"]
          }
        },
        "required": ["slide_index", "finding_count", "directives"]
      }
    },
    "unmapped": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "reason":      {"type": "string"},
          "code":        {"type": "string"},
          "category":    {"type": "string"},
          "slide_index": {"type": "integer"},
          "path":        {"type": "string"},
          "message":     {"type": "string"}
        },
        "required": ["reason"]
      }
    },
    "summary": {
      "type": "object",
      "properties": {
        "total_findings":    {"type": "integer"},
        "mapped_findings":   {"type": "integer"},
        "unmapped_findings": {"type": "integer"},
        "total_directives": {"type": "integer"},
        "slides_affected":   {"type": "integer"}
      },
      "required": ["total_findings", "mapped_findings", "unmapped_findings", "total_directives", "slides_affected"]
    }
  },
  "required": ["slides", "summary"]
}`)

// --- audit_palette ---
// region describes one sampled pic/shape rectangle; pair is a (pic, shape) ΔE
// comparison. findings projects every threshold violation into the shared
// FindingEnvelope so agents can branch on findings.ok.
var outputSchemaAuditPalette = json.RawMessage(`{
  "type": "object",
  "properties": {
    "pptx":                {"type": "string"},
    "slide_count":         {"type": "integer"},
    "max_delta_e_allowed": {"type": "number", "description": "ΔE threshold a (pic, shape) pair must not exceed."},
    "chroma_min":          {"type": "integer"},
    "density":             {"type": "integer"},
    "violations":          {"type": "integer", "description": "Number of (pic, shape) pairs whose ΔE exceeded the threshold across all slides."},
    "slides": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "index":       {"type": "integer", "description": "1-based slide index."},
          "pic_count":   {"type": "integer"},
          "shape_count": {"type": "integer"},
          "pair_count":  {"type": "integer"},
          "max_delta_e": {"type": "number", "description": "Largest ΔE among this slide's pairs."},
          "pairs": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "slide":   {"type": "integer"},
                "pic":     {"$ref": "#/$defs/audit_region"},
                "shape":   {"$ref": "#/$defs/audit_region"},
                "delta_e": {"type": "number"},
                "pass":    {"type": "boolean", "description": "True when delta_e <= max_delta_e_allowed."}
              },
              "required": ["slide", "pic", "shape", "delta_e", "pass"]
            }
          }
        },
        "required": ["index", "pic_count", "shape_count", "pair_count"]
      }
    },
    "findings": ` + findingEnvelopeSchema + `
  },
  "required": ["pptx", "slide_count", "violations", "findings"],
  "$defs": {
    "audit_region": {
      "type": "object",
      "properties": {
        "kind":         {"type": "string", "enum": ["pic", "shape"]},
        "name":         {"type": "string"},
        "bounds_emu":   {"type": "array", "items": {"type": "integer"}, "minItems": 4, "maxItems": 4},
        "bounds_px":    {"type": "array", "items": {"type": "integer"}, "minItems": 4, "maxItems": 4},
        "dominant_hex": {"type": "string"},
        "r":            {"type": "integer"},
        "g":            {"type": "integer"},
        "b":            {"type": "integer"},
        "pixel_count":  {"type": "integer"},
        "declared_hex": {"type": "string", "description": "Explicit srgbClr or schemeClr indirection for shape regions; blank for pics."}
      },
      "required": ["kind", "dominant_hex", "pixel_count"]
    }
  }
}`)
