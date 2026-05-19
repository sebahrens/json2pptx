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
var outputSchemaValidate = json.RawMessage(`{
  "type": "object",
  "properties": {
    "valid":               {"type": "boolean"},
    "slide_count":         {"type": "integer"},
    "chart_count":         {"type": "integer"},
    "diagram_count":       {"type": "integer"},
    "table_count":         {"type": "integer"},
    "shape_count":         {"type": "integer"},
    "warnings":            {"type": "array", "items": {"type": "string"}},
    "validation_warnings": {"type": "array", "items": {"type": "object"}},
    "errors":              {"type": "array", "items": {"type": "string"}},
    "diagnostics":         {"type": "array", "items": {"$ref": "#/$defs/diagnostic"}},
    "slides":              {"type": "array", "items": {"type": "object"}},
    "fit_findings":        {"type": "array", "items": {"type": "object"}}
  },
  "required": ["valid"],
  "$defs": {
    "diagnostic": {
      "type": "object",
      "properties": {
        "code":     {"type": "string"},
        "path":     {"type": "string"},
        "message":  {"type": "string"},
        "severity": {"type": "string", "enum": ["error", "warning"]},
        "fix":      {"type": "object"},
        "next_tool_call": {
          "type": "object",
          "properties": {
            "tool":          {"type": "string"},
            "args_template": {"type": "object"}
          },
          "required": ["tool", "args_template"]
        }
      },
      "required": ["code", "message", "severity"]
    }
  }
}`)

// --- list_templates ---
var outputSchemaListTemplates = json.RawMessage(`{
  "type": "object",
  "properties": {
    "tool":            {"type": "object", "properties": {"name": {"type": "string"}, "version": {"type": "string"}}},
    "templates":       {"type": "array", "items": {"type": "object"}},
    "supported_types": {"type": "object"},
    "input_formats":   {"type": "array", "items": {"type": "string"}},
    "output_formats":  {"type": "array", "items": {"type": "string"}}
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
            "narrative_role":   {"type": "string"},
            "pairs_with":       {"type": "array", "items": {"type": "string"}},
            "composes_with":    {"type": "array", "items": {"type": "string"}},
            "role_on_slide":    {"type": "array", "items": {"type": "string"}},
            "density_class":    {"type": "string"},
            "accent_weight":    {"type": "string"},
            "supports_callout": {"type": "boolean"}
          },
          "required": ["name", "cells", "use_when", "not_when", "category"]
        }
      }
    },
    "required": ["category", "patterns"]
  }
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
        "icon_support": {"type": "string", "enum": ["none", "text_only", "svg_only", "svg_and_text"], "description": "What icon rendering modes the pattern supports."},
        "icon_modes":   {"type": "string", "description": "Available icon_mode override values, if applicable."}
      },
      "required": ["icon_support"]
    }
  },
  "required": ["name", "description", "use_when", "not_when", "version", "schema"]
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
          "placement":        {"$ref": "#/$defs/placement_guidance"}
        },
        "required": ["category", "name", "score", "rationale", "confidence_band"]
      }
    },
    "query_understood_as": {"type": "string"},
    "disambiguating_questions": {
      "type": "array",
      "items": {"type": "string"}
    }
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
    }
  }
}`)

// --- list_icons ---
var outputSchemaListIcons = json.RawMessage(`{
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "set":   {"type": "string"},
      "count": {"type": "integer"},
      "names": {"type": "array", "items": {"type": "string"}}
    },
    "required": ["set", "count", "names"]
  }
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

// --- render_slide_image ---
var outputSchemaRenderSlideImage = json.RawMessage(`{
  "type": "object",
  "properties": {
    "index":      {"type": "integer"},
    "png_base64": {"type": "string"},
    "path":       {"type": "string"},
    "width":      {"type": "integer"},
    "height":     {"type": "integer"},
    "size_error": {"type": "string"}
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
          "index":      {"type": "integer"},
          "png_base64": {"type": "string"},
          "path":       {"type": "string"},
          "width":      {"type": "integer"},
          "height":     {"type": "integer"}
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
    "total_issues": {"type": "integer"}
  },
  "required": ["slide_count", "results", "total_issues"]
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
    "mode_used": {"type": "string"}
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
    "fit_findings": {"type": "array", "items": {"type": "object"}}
  },
  "required": ["resolved_slides"]
}`)

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
    "new_findings": {"type": "array", "items": {"type": "object"}}
  },
  "required": ["patched_deck", "applied_fixes"]
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
          "added_in": {"type": "string"}
        },
        "required": ["name", "added_in"]
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
        "feature_versions":       {"type": "object", "additionalProperties": {"type": "string"}}
      }
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
    "error_codes": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["schema_version", "tool_version", "changelog_url", "mcp_tools_available", "tool_list", "registry", "deprecations", "features", "runtime", "vocabularies", "error_codes"]
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
                "rationale":    {"type": "string"}
              },
              "required": ["pattern_name", "score", "rationale"]
            }
          }
        },
        "required": ["slide_index", "narrative_role", "recommended_pattern", "content_seed", "rationale"]
      }
    },
    "brief":        {"type": "string"},
    "slide_budget":  {"type": "integer"},
    "rhythm_check": {
      "type": "object",
      "properties": {
        "longest_pattern_run": {"type": "integer"},
        "has_emphasis":        {"type": "boolean"},
        "emphasis_count":      {"type": "integer"},
        "pattern_variety":     {"type": "integer"}
      },
      "required": ["longest_pattern_run", "has_emphasis", "emphasis_count", "pattern_variety"]
    }
  },
  "required": ["slides", "brief", "slide_budget", "rhythm_check"]
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
