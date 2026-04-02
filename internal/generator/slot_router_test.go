package generator

import (
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestFilterContentPlaceholders(t *testing.T) {
	tests := []struct {
		name         string
		placeholders []types.PlaceholderInfo
		wantCount    int
		wantTypes    []types.PlaceholderType
	}{
		{
			name: "filters out title and other",
			placeholders: []types.PlaceholderInfo{
				{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
				{ID: "Footer", Type: types.PlaceholderOther, Index: 10},
				{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
			},
			wantCount: 2,
			wantTypes: []types.PlaceholderType{types.PlaceholderBody, types.PlaceholderContent},
		},
		{
			name: "includes visual placeholders",
			placeholders: []types.PlaceholderInfo{
				{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "Image 1", Type: types.PlaceholderImage, Index: 1},
				{ID: "Chart 1", Type: types.PlaceholderChart, Index: 2},
				{ID: "Table 1", Type: types.PlaceholderTable, Index: 3},
			},
			wantCount: 3,
			wantTypes: []types.PlaceholderType{types.PlaceholderImage, types.PlaceholderChart, types.PlaceholderTable},
		},
		{
			name: "sorts by index",
			placeholders: []types.PlaceholderInfo{
				{ID: "Content 2", Type: types.PlaceholderBody, Index: 5},
				{ID: "Content 1", Type: types.PlaceholderBody, Index: 2},
				{ID: "Content 3", Type: types.PlaceholderContent, Index: 3},
			},
			wantCount: 3,
			wantTypes: []types.PlaceholderType{types.PlaceholderBody, types.PlaceholderContent, types.PlaceholderBody},
		},
		{
			name:         "empty input",
			placeholders: []types.PlaceholderInfo{},
			wantCount:    0,
			wantTypes:    nil,
		},
		{
			name: "only non-content placeholders",
			placeholders: []types.PlaceholderInfo{
				{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "Subtitle", Type: types.PlaceholderSubtitle, Index: 1},
				{ID: "Footer", Type: types.PlaceholderOther, Index: 10},
			},
			wantCount: 0,
			wantTypes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterContentPlaceholders(tt.placeholders)

			if len(got) != tt.wantCount {
				t.Errorf("FilterContentPlaceholders() count = %d, want %d", len(got), tt.wantCount)
			}

			for i, ph := range got {
				if i < len(tt.wantTypes) && ph.Type != tt.wantTypes[i] {
					t.Errorf("FilterContentPlaceholders()[%d].Type = %v, want %v", i, ph.Type, tt.wantTypes[i])
				}
			}
		})
	}
}

func TestRouteTypesSlotContent_Text(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 1,
		RawContent: "Hello, World!",
		Type:       types.SlotContentText,
		Text:       "Hello, World!",
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("routeTypesSlotContent() returned %d items, want 1", len(items))
	}

	if items[0].Type != ContentText {
		t.Errorf("Type = %v, want %v", items[0].Type, ContentText)
	}

	if items[0].PlaceholderID != "Content 1" {
		t.Errorf("PlaceholderID = %v, want %v", items[0].PlaceholderID, "Content 1")
	}

	if items[0].Value != "Hello, World!" {
		t.Errorf("Value = %v, want %v", items[0].Value, "Hello, World!")
	}
}

func TestRouteTypesSlotContent_Bullets(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 1,
		RawContent: "- Bullet 1\n- Bullet 2\n- Bullet 3",
		Type:       types.SlotContentBullets,
		Bullets:    []string{"Bullet 1", "Bullet 2", "Bullet 3"},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("returned %d items, want 1", len(items))
	}

	if items[0].Type != ContentBullets {
		t.Errorf("Type = %v, want %v", items[0].Type, ContentBullets)
	}

	bullets, ok := items[0].Value.([]string)
	if !ok {
		t.Fatalf("Value is not []string")
	}

	if len(bullets) != 3 {
		t.Errorf("len(bullets) = %d, want 3", len(bullets))
	}
}

func TestRouteTypesSlotContent_Table(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 1,
		RawContent: "| Name | Value |\n|------|-------|\n| A    | 1     |\n| B    | 2     |",
		Type:       types.SlotContentTable,
		Table: &types.TableSpec{
			Headers: []string{"Name", "Value"},
			Rows: [][]types.TableCell{
				{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "1", ColSpan: 1, RowSpan: 1}},
				{{Content: "B", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
			},
			Style: types.DefaultTableStyle,
		},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("returned %d items, want 1", len(items))
	}

	if items[0].Type != ContentTable {
		t.Errorf("Type = %v, want %v", items[0].Type, ContentTable)
	}

	table, ok := items[0].Value.(*types.TableSpec)
	if !ok {
		t.Fatalf("Value is not *types.TableSpec")
	}

	if len(table.Headers) != 2 {
		t.Errorf("len(table.Headers) = %d, want 2", len(table.Headers))
	}

	if len(table.Rows) != 2 {
		t.Errorf("len(table.Rows) = %d, want 2", len(table.Rows))
	}
}

func TestRouteTypesSlotContent_NilSlot(t *testing.T) {
	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	items, err := routeTypesSlotContent(nil, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent(nil) error = %v", err)
	}

	if items != nil {
		t.Errorf("routeTypesSlotContent(nil) = %v, want nil", items)
	}
}

func TestBuildSlotContentItems(t *testing.T) {
	layout := types.LayoutMetadata{
		Name: "Test Layout",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
			{ID: "Footer", Type: types.PlaceholderOther, Index: 10},
		},
	}

	slots := map[int]*types.SlotContent{
		1: {
			SlotNumber: 1,
			RawContent: "Text for slot 1",
			Type:       types.SlotContentText,
			Text:       "Text for slot 1",
		},
		2: {
			SlotNumber: 2,
			RawContent: "- Bullet A\n- Bullet B",
			Type:       types.SlotContentBullets,
			Bullets:    []string{"Bullet A", "Bullet B"},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	if result == nil {
		t.Fatal("BuildSlotContentItems() returned nil result")
	}

	if len(result.ContentItems) != 2 {
		t.Errorf("len(result.ContentItems) = %d, want 2", len(result.ContentItems))
	}
}

func TestBuildSlotContentItems_SlotOutOfRange(t *testing.T) {
	layout := types.LayoutMetadata{
		Name: "Test Layout",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
		},
	}

	// Slot 3 is out of range (only 1 content placeholder)
	slots := map[int]*types.SlotContent{
		3: {
			SlotNumber: 3,
			RawContent: "Out of range",
			Type:       types.SlotContentText,
			Text:       "Out of range",
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() unexpected error = %v", err)
	}

	// Should produce a warning, not an error
	if len(result.Warnings) == 0 {
		t.Error("BuildSlotContentItems() expected warning for out of range slot")
	}

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "out of range") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("BuildSlotContentItems() warnings %v missing 'out of range'", result.Warnings)
	}
}

func TestBuildSlotContentItems_NoContentPlaceholders(t *testing.T) {
	layout := types.LayoutMetadata{
		Name: "Title Only Layout",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Footer", Type: types.PlaceholderOther, Index: 10},
		},
	}

	slots := map[int]*types.SlotContent{
		1: {
			SlotNumber: 1,
			RawContent: "Some content",
			Type:       types.SlotContentText,
			Text:       "Some content",
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() unexpected error = %v", err)
	}

	// Should produce a warning about no content placeholders
	if len(result.Warnings) == 0 {
		t.Error("BuildSlotContentItems() expected warning for layout with no content placeholders")
	}
}

func TestBuildSlotContentItems_EmptySlots(t *testing.T) {
	layout := types.LayoutMetadata{
		Name: "Test Layout",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
		},
	}

	slots := map[int]*types.SlotContent{}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	if result == nil {
		t.Fatal("BuildSlotContentItems() returned nil result")
	}

	if len(result.ContentItems) != 0 {
		t.Errorf("len(result.ContentItems) = %d, want 0", len(result.ContentItems))
	}
}

func TestPopulateTableInShape(t *testing.T) {
	table := &types.TableSpec{
		Headers: []string{"Name", "Value"},
		Rows: [][]types.TableCell{
			{{Content: "A", ColSpan: 1, RowSpan: 1}, {Content: "1", ColSpan: 1, RowSpan: 1}},
			{{Content: "B", ColSpan: 1, RowSpan: 1}, {Content: "2", ColSpan: 1, RowSpan: 1}},
		},
		Style: types.DefaultTableStyle,
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
		Bounds: types.BoundingBox{
			X:      914400,  // 1 inch
			Y:      914400,  // 1 inch
			Width:  5486400, // 6 inches
			Height: 3657600, // 4 inches
		},
	}

	result, err := PopulateTableInShape(table, placeholder, nil)
	if err != nil {
		t.Fatalf("PopulateTableInShape() error = %v", err)
	}

	if result == nil {
		t.Fatal("PopulateTableInShape() returned nil result")
	}

	if result.XML == "" {
		t.Error("PopulateTableInShape().XML is empty")
	}

	// Check that the XML contains table structure
	if !strings.Contains(result.XML, "<a:tbl>") {
		t.Error("PopulateTableInShape().XML missing <a:tbl>")
	}

	if !strings.Contains(result.XML, "Name") {
		t.Error("PopulateTableInShape().XML missing header 'Name'")
	}
}

func TestPopulateTableInShape_NilTable(t *testing.T) {
	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	_, err := PopulateTableInShape(nil, placeholder, nil)
	if err == nil {
		t.Error("PopulateTableInShape(nil) expected error")
	}
}

func TestRouteTypesSlotContent_Chart(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 2,
		RawContent: "```chart\ntype: bar\ntitle: Revenue\n```",
		Type:       types.SlotContentChart,
		DiagramSpec: &types.DiagramSpec{
			Type:  "bar_chart",
			Title: "Revenue",
			Data:  map[string]any{},
		},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 2",
		Type: types.PlaceholderContent,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent(chart) error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("returned %d items, want 1", len(items))
	}

	if items[0].Type != ContentDiagram {
		t.Errorf("Type = %v, want %v", items[0].Type, ContentDiagram)
	}

	if items[0].PlaceholderID != "Content 2" {
		t.Errorf("PlaceholderID = %v, want %v", items[0].PlaceholderID, "Content 2")
	}

	spec, ok := items[0].Value.(*types.DiagramSpec)
	if !ok {
		t.Fatalf("Value is not *types.DiagramSpec, got %T", items[0].Value)
	}

	if spec.Type != "bar_chart" {
		t.Errorf("DiagramSpec.Type = %q, want %q", spec.Type, "bar_chart")
	}

	if spec.Title != "Revenue" {
		t.Errorf("DiagramSpec.Title = %q, want %q", spec.Title, "Revenue")
	}
}

func TestRouteTypesSlotContent_Chart_NilDiagram(t *testing.T) {
	// DiagramSpec is nil (resolve step failed or didn't find a chart)
	slot := &types.SlotContent{
		SlotNumber: 1,
		RawContent: "```chart\n: invalid yaml [\n```",
		Type:       types.SlotContentChart,
		// DiagramSpec intentionally nil
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	_, err := routeTypesSlotContent(slot, placeholder)
	if err == nil {
		t.Error("routeTypesSlotContent(nil diagram) expected error")
	}

	if !strings.Contains(err.Error(), "no valid chart") {
		t.Errorf("routeTypesSlotContent(nil diagram) error = %v, want 'no valid chart'", err)
	}
}

func TestRouteTypesSlotContent_Chart_NoChart(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 1,
		RawContent: "Just some text with no chart block",
		Type:       types.SlotContentChart,
		// DiagramSpec intentionally nil
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	_, err := routeTypesSlotContent(slot, placeholder)
	if err == nil {
		t.Error("routeTypesSlotContent(no chart) expected error")
	}

	if !strings.Contains(err.Error(), "no valid chart") {
		t.Errorf("routeTypesSlotContent(no chart) error = %v, want 'no valid chart'", err)
	}
}

func TestRouteTypesSlotContent_Infographic(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 2,
		RawContent: "```diagram\ntype: timeline\ntitle: Project Timeline\n```",
		Type:       types.SlotContentInfographic,
		DiagramSpec: &types.DiagramSpec{
			Type:  "timeline",
			Title: "Project Timeline",
			Data:  map[string]any{},
		},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 2",
		Type: types.PlaceholderContent,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent(infographic) error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("returned %d items, want 1", len(items))
	}

	if items[0].Type != ContentDiagram {
		t.Errorf("Type = %v, want %v", items[0].Type, ContentDiagram)
	}

	spec, ok := items[0].Value.(*types.DiagramSpec)
	if !ok {
		t.Fatalf("Value is not *types.DiagramSpec, got %T", items[0].Value)
	}

	if spec.Type != "timeline" {
		t.Errorf("DiagramSpec.Type = %q, want %q", spec.Type, "timeline")
	}
}

func TestRouteTypesSlotContent_Image(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 2,
		RawContent: "![Company Logo](images/logo.png)",
		Type:       types.SlotContentImage,
		ImagePath:  "images/logo.png",
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 2",
		Type: types.PlaceholderContent,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent(image) error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("returned %d items, want 1", len(items))
	}

	if items[0].Type != ContentImage {
		t.Errorf("Type = %v, want %v", items[0].Type, ContentImage)
	}

	imgContent, ok := items[0].Value.(ImageContent)
	if !ok {
		t.Fatalf("Value is not ImageContent, got %T", items[0].Value)
	}

	if imgContent.Path != "images/logo.png" {
		t.Errorf("ImageContent.Path = %q, want %q", imgContent.Path, "images/logo.png")
	}
}

func TestRouteTypesSlotContent_Image_NoPath(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 1,
		RawContent: "Just text, no image reference",
		Type:       types.SlotContentImage,
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	_, err := routeTypesSlotContent(slot, placeholder)
	if err == nil {
		t.Error("expected error")
	}

	if !strings.Contains(err.Error(), "no valid image path") {
		t.Errorf("error = %v, want 'no valid image path'", err)
	}
}

func TestRouteTypesSlotContent_BulletGroups(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 2,
		RawContent: "**Revenue Growth**\n- Up 15% YoY\n- APAC leading\n**Cost Reduction**\n- OpEx down 8%\n- Automation savings",
		Type:       types.SlotContentBulletGroups,
		BulletGroups: []types.BulletGroup{
			{Header: "**Revenue Growth**", Bullets: []string{"Up 15% YoY", "APAC leading"}},
			{Header: "**Cost Reduction**", Bullets: []string{"OpEx down 8%", "Automation savings"}},
		},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 2",
		Type: types.PlaceholderContent,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("routeTypesSlotContent(bullet_groups) error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("returned %d items, want 1", len(items))
	}

	if items[0].Type != ContentBulletGroups {
		t.Errorf("Type = %v, want %v", items[0].Type, ContentBulletGroups)
	}

	bgc, ok := items[0].Value.(BulletGroupsContent)
	if !ok {
		t.Fatalf("Value is not BulletGroupsContent, got %T", items[0].Value)
	}

	if len(bgc.Groups) != 2 {
		t.Fatalf("len(bgc.Groups) = %d, want 2", len(bgc.Groups))
	}

	if bgc.Groups[0].Header != "**Revenue Growth**" {
		t.Errorf("Groups[0].Header = %q, want %q", bgc.Groups[0].Header, "**Revenue Growth**")
	}

	if len(bgc.Groups[0].Bullets) != 2 {
		t.Errorf("len(Groups[0].Bullets) = %d, want 2", len(bgc.Groups[0].Bullets))
	}
}

func TestBuildSlotContentItems_ChartAndBulletGroups(t *testing.T) {
	layout := types.LayoutMetadata{
		Name: "Two Content Layout",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	slots := map[int]*types.SlotContent{
		1: {
			SlotNumber: 1,
			RawContent: "```chart\ntype: waterfall\ntitle: Bridge\n```",
			Type:       types.SlotContentChart,
			DiagramSpec: &types.DiagramSpec{
				Type:  "waterfall",
				Title: "Bridge",
				Data:  map[string]any{},
			},
		},
		2: {
			SlotNumber: 2,
			RawContent: "**Revenue**\n- Up 15%\n**Cost**\n- Down 8%",
			Type:       types.SlotContentBulletGroups,
			BulletGroups: []types.BulletGroup{
				{Header: "**Revenue**", Bullets: []string{"Up 15%"}},
				{Header: "**Cost**", Bullets: []string{"Down 8%"}},
			},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	if len(result.ContentItems) != 2 {
		t.Fatalf("len(result.ContentItems) = %d, want 2", len(result.ContentItems))
	}

	// Slot 1 = diagram
	if result.ContentItems[0].Type != ContentDiagram {
		t.Errorf("ContentItems[0].Type = %v, want %v", result.ContentItems[0].Type, ContentDiagram)
	}

	// Slot 2 = bullet_groups
	if result.ContentItems[1].Type != ContentBulletGroups {
		t.Errorf("ContentItems[1].Type = %v, want %v", result.ContentItems[1].Type, ContentBulletGroups)
	}

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

func TestRouteTypesSlotContent_ChartWithBodyText(t *testing.T) {
	slot := &types.SlotContent{
		SlotNumber: 1,
		RawContent: "**FY24 Mix**\n```chart\ntype: pie\ntitle: FY24\n```",
		Type:       types.SlotContentChart,
		Body:       "**FY24 Mix**",
		DiagramSpec: &types.DiagramSpec{
			Type:  "pie_chart",
			Title: "FY24",
			Data:  map[string]any{},
		},
	}

	placeholder := types.PlaceholderInfo{
		ID:   "Content 1",
		Type: types.PlaceholderBody,
	}

	items, err := routeTypesSlotContent(slot, placeholder)
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("returned %d items, want 2 (text + diagram)", len(items))
	}

	// First item should be the body text
	if items[0].Type != ContentText {
		t.Errorf("items[0].Type = %v, want %v", items[0].Type, ContentText)
	}
	if items[0].Value != "**FY24 Mix**" {
		t.Errorf("items[0].Value = %v, want %q", items[0].Value, "**FY24 Mix**")
	}
	if items[0].PlaceholderID != "Content 1" {
		t.Errorf("items[0].PlaceholderID = %v, want %q", items[0].PlaceholderID, "Content 1")
	}

	// Second item should be the diagram
	if items[1].Type != ContentDiagram {
		t.Errorf("items[1].Type = %v, want %v", items[1].Type, ContentDiagram)
	}

	spec, ok := items[1].Value.(*types.DiagramSpec)
	if !ok {
		t.Fatalf("items[1].Value is not *types.DiagramSpec, got %T", items[1].Value)
	}
	if spec.Type != "pie_chart" {
		t.Errorf("DiagramSpec.Type = %q, want %q", spec.Type, "pie_chart")
	}
}

func TestBuildSlotContentItems_ChartAndText(t *testing.T) {
	layout := types.LayoutMetadata{
		Name: "Two Content Layout",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	slots := map[int]*types.SlotContent{
		1: {
			SlotNumber: 1,
			RawContent: "Key highlights from Q4",
			Type:       types.SlotContentText,
			Text:       "Key highlights from Q4",
		},
		2: {
			SlotNumber: 2,
			RawContent: "```chart\ntype: bar\ntitle: Revenue\n```",
			Type:       types.SlotContentChart,
			DiagramSpec: &types.DiagramSpec{
				Type:  "bar_chart",
				Title: "Revenue",
				Data:  map[string]any{},
			},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	if len(result.ContentItems) != 2 {
		t.Fatalf("len(result.ContentItems) = %d, want 2", len(result.ContentItems))
	}

	// Slot 1 = text
	if result.ContentItems[0].Type != ContentText {
		t.Errorf("ContentItems[0].Type = %v, want %v", result.ContentItems[0].Type, ContentText)
	}

	// Slot 2 = diagram
	if result.ContentItems[1].Type != ContentDiagram {
		t.Errorf("ContentItems[1].Type = %v, want %v", result.ContentItems[1].Type, ContentDiagram)
	}

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}
