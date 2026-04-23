package jsonschema

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TableInput represents a table with headers, rows, and optional styling.
type TableInput struct {
	Headers          []string           `json:"headers"`
	Rows             [][]TableCellInput `json:"rows"`
	Style            *TableStyleInput   `json:"style,omitempty"`
	ColumnAlignments []string           `json:"column_alignments,omitempty"`
}

// ToTableSpec converts TableInput to types.TableSpec.
func (t *TableInput) ToTableSpec() *types.TableSpec {
	if t == nil {
		return nil
	}
	spec := &types.TableSpec{
		Headers:          t.Headers,
		ColumnAlignments: t.ColumnAlignments,
	}
	for _, row := range t.Rows {
		cells := make([]types.TableCell, len(row))
		for j, cell := range row {
			cells[j] = types.TableCell{
				Content: cell.Content,
				ColSpan: cell.ColSpan,
				RowSpan: cell.RowSpan,
			}
		}
		spec.Rows = append(spec.Rows, cells)
	}
	if t.Style != nil {
		spec.Style = types.TableStyle{
			Borders:       t.Style.Borders,
			Striped:       t.Style.Striped, // nil means unset (default banding on)
			UseTableStyle: t.Style.UseTableStyle,
			StyleID:       t.Style.StyleID,
		}
		if t.Style.HeaderBackground != nil {
			spec.Style.HeaderBackground = *t.Style.HeaderBackground
		}
		// Default StyleID when not explicitly set
		if spec.Style.StyleID == "" {
			spec.Style.StyleID = types.DefaultTableStyleID
		}
	} else {
		spec.Style = types.DefaultTableStyle
	}
	return spec
}

// TableCellInput supports both string shorthand and full object form.
type TableCellInput struct {
	Content string `json:"content"`
	ColSpan int    `json:"col_span,omitempty"`
	RowSpan int    `json:"row_span,omitempty"`
}

// UnmarshalJSON supports string shorthand: "cell text" or {"content":"cell text","col_span":2}.
func (c *TableCellInput) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Content = s
		c.ColSpan = 1
		c.RowSpan = 1
		return nil
	}
	type alias TableCellInput
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("TableCellInput must be string or {content, col_span, row_span}: %w", err)
	}
	*c = TableCellInput(a)
	if c.ColSpan == 0 {
		c.ColSpan = 1
	}
	if c.RowSpan == 0 {
		c.RowSpan = 1
	}
	return nil
}

// TableStyleInput maps to types.TableStyle.
type TableStyleInput struct {
	HeaderBackground *string `json:"header_background,omitempty"`
	Borders          string  `json:"borders,omitempty"`
	Striped          *bool   `json:"striped,omitempty"`
	UseTableStyle    bool    `json:"use_table_style,omitempty"`
	StyleID          string  `json:"style_id,omitempty"`
}
