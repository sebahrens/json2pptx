package main

import (
	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// Type aliases for shape grid and table types now defined in internal/jsonschema.
// These aliases allow existing code in package main to continue using the
// unqualified type names (e.g., ShapeGridInput instead of jsonschema.ShapeGridInput).
// ---------------------------------------------------------------------------

type ShapeGridInput = jsonschema.ShapeGridInput
type GridBoundsInput = jsonschema.GridBoundsInput
type GridRowInput = jsonschema.GridRowInput
type ConnectorSpecInput = jsonschema.ConnectorSpecInput
type GridCellInput = jsonschema.GridCellInput
type AccentBarInput = jsonschema.AccentBarInput
type GridImageInput = jsonschema.GridImageInput
type GridOverlayInput = jsonschema.GridOverlayInput
type GridImageTextInput = jsonschema.GridImageTextInput
type IconInput = jsonschema.IconInput
type ShapeSpecInput = jsonschema.ShapeSpecInput
type ShapeFillInput = jsonschema.ShapeFillInput
type TableInput = jsonschema.TableInput
type TableCellInput = jsonschema.TableCellInput
type TableStyleInput = jsonschema.TableStyleInput
type OverlayShapeInput = jsonschema.OverlayShapeInput
type OverlayPointInput = jsonschema.OverlayPointInput
type OverlayAnchorCellInput = jsonschema.OverlayAnchorCellInput

// ---------------------------------------------------------------------------
// Type aliases for the raw deck input model now defined in internal/deckinput.
// The types (and their methods: PresentationInput.UnmarshalJSON,
// ThemeInput.ToThemeOverride, ContentInput.UsesLegacyValue/ResolveValue) moved
// out of package main so internal packages can import them. These aliases keep
// all existing package-main call sites compiling unchanged.
// ---------------------------------------------------------------------------

type PresentationInput = deckinput.PresentationInput
type ChromeInput = deckinput.ChromeInput
type PageNumbersInput = deckinput.PageNumbersInput
type StructureInput = deckinput.StructureInput
type SectionInput = deckinput.SectionInput
type DefaultsInput = deckinput.DefaultsInput
type GridConfig = deckinput.GridConfig
type ThemeInput = deckinput.ThemeInput
type SlideInput = deckinput.SlideInput
type BackgroundInput = deckinput.BackgroundInput
type ContentInput = deckinput.ContentInput
type BodyAndBulletsInput = deckinput.BodyAndBulletsInput
type BodyAndLeadInput = deckinput.BodyAndLeadInput
type BulletGroupsInput = deckinput.BulletGroupsInput
type BulletGroupInput = deckinput.BulletGroupInput
type ImageInput = deckinput.ImageInput
