package main

import (
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
)

// Pagination defaults for MCP list-style tools. The opaque cursor is an
// integer-offset string; callers should treat it as opaque and pass it
// back verbatim on the next call.
const (
	paginationDefaultPageSize = 50
	paginationMaxPageSize     = 200
)

// paginationParams parses cursor and page_size from an MCP request.
// Returns offset (0 if absent), clamped pageSize, and an error message
// string for invalid input (empty when no error). An empty cursor maps
// to offset 0. page_size is clamped to [1, paginationMaxPageSize].
func paginationParams(request mcp.CallToolRequest) (offset, pageSize int, errField, errMsg string) {
	pageSize = paginationDefaultPageSize
	if v, ok := request.GetArguments()["page_size"]; ok {
		switch x := v.(type) {
		case float64:
			pageSize = int(x)
		case int:
			pageSize = x
		case string:
			if x != "" {
				n, err := strconv.Atoi(x)
				if err != nil {
					return 0, 0, "page_size", fmt.Sprintf("page_size must be an integer, got %q", x)
				}
				pageSize = n
			}
		}
	}
	if pageSize < 1 {
		pageSize = 1
	}
	if pageSize > paginationMaxPageSize {
		pageSize = paginationMaxPageSize
	}

	if c, err := request.RequireString("cursor"); err == nil && c != "" {
		n, parseErr := strconv.Atoi(c)
		if parseErr != nil || n < 0 {
			return 0, 0, "cursor", fmt.Sprintf("invalid cursor %q: must be a non-negative integer string", c)
		}
		offset = n
	}
	return offset, pageSize, "", ""
}

// paginationSlice returns the [start, end) slice indices and the next
// cursor for a list of total items, given an offset and pageSize.
// nextCursor is "" when there is no next page.
func paginationSlice(total, offset, pageSize int) (start, end int, nextCursor string) {
	if offset >= total {
		return total, total, ""
	}
	start = offset
	end = offset + pageSize
	if end > total {
		end = total
	}
	if end < total {
		nextCursor = strconv.Itoa(end)
	}
	return start, end, nextCursor
}
