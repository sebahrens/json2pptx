# Relationships Manager

Centralize OOXML relationship ID management to prevent collisions.

## Scope

This specification covers ONLY relationship management utilities. It does NOT cover:
- Changing how relationships are stored in PPTX
- Adding new relationship types

## Purpose

OOXML uses relationship files (`.rels`) to link content. Each relationship has a unique ID (rId1, rId2, etc.). Current code parses and generates these IDs ad-hoc, risking collisions.

## Current Problem

In `internal/generator/images.go`:

```go
// Ad-hoc ID generation - works but fragile
nextRelID := 1
for _, rel := range existingRels.Relationships {
    var num int
    if _, err := fmt.Sscanf(rel.ID, "rId%d", &num); err == nil {
        if num >= nextRelID {
            nextRelID = num + 1
        }
    }
}
newRelID := fmt.Sprintf("rId%d", nextRelID)
```

This logic is duplicated and error-prone.

## Implementation

Located at `internal/pptx/relationships.go`. Uses `RelationshipXML` type from `internal/pptx/xml_types.go`.

### XML Types (in xml_types.go)

```go
// RelationshipsXML represents the root element of a .rels file.
type RelationshipsXML struct {
    XMLName       xml.Name          `xml:"Relationships"`
    Xmlns         string            `xml:"xmlns,attr,omitempty"`
    Relationships []RelationshipXML `xml:"Relationship"`
}

// RelationshipXML represents a single relationship entry.
type RelationshipXML struct {
    ID         string `xml:"Id,attr"`
    Type       string `xml:"Type,attr"`
    Target     string `xml:"Target,attr"`
    TargetMode string `xml:"TargetMode,attr,omitempty"`
}
```

### Relationships Manager (in relationships.go)

```go
package pptx

import (
    "encoding/xml"
    "fmt"
    "sort"
    "strconv"
    "strings"
)

// Relationships manages a .rels file with ID allocation.
// It tracks existing relationship IDs and allocates new ones deterministically.
type Relationships struct {
    rels     []RelationshipXML
    maxID    int               // highest rId number seen
    byID     map[string]int    // rId -> index in rels slice
    byTarget map[string]string // target -> rId (for deduplication)
}

// NewRelationships creates an empty relationships manager.
func NewRelationships() *Relationships {
    return &Relationships{
        rels:     make([]RelationshipXML, 0),
        maxID:    0,
        byID:     make(map[string]int),
        byTarget: make(map[string]string),
    }
}

// ParseRelationships parses a .rels XML file into a Relationships manager.
func ParseRelationships(data []byte) (*Relationships, error) {
    var relsXML RelationshipsXML
    if err := xml.Unmarshal(data, &relsXML); err != nil {
        return nil, fmt.Errorf("failed to parse relationships XML: %w", err)
    }

    r := &Relationships{
        rels:     make([]RelationshipXML, len(relsXML.Relationships)),
        maxID:    0,
        byID:     make(map[string]int),
        byTarget: make(map[string]string),
    }

    for i, rel := range relsXML.Relationships {
        r.rels[i] = rel
        r.byID[rel.ID] = i
        r.byTarget[rel.Target] = rel.ID

        // Track max ID for allocation
        if num := parseRelID(rel.ID); num > r.maxID {
            r.maxID = num
        }
    }

    return r, nil
}

// parseRelID extracts the numeric part of an rId string.
// Returns 0 if the ID doesn't match the rIdN pattern.
func parseRelID(id string) int {
    if !strings.HasPrefix(id, "rId") {
        return 0
    }
    numStr := strings.TrimPrefix(id, "rId")
    num, err := strconv.Atoi(numStr)
    if err != nil {
        return 0
    }
    return num
}

// AllocID allocates a new unique relationship ID.
// IDs are allocated in sequence: rId1, rId2, rId3, etc.
func (r *Relationships) AllocID() string {
    r.maxID++
    return fmt.Sprintf("rId%d", r.maxID)
}

// NextID returns what the next allocated ID would be without allocating it.
// Useful for planning operations that need to know IDs in advance.
func (r *Relationships) NextID() string {
    return fmt.Sprintf("rId%d", r.maxID+1)
}

// Add creates a new relationship and returns its allocated ID.
// If target already exists, returns the existing relationship ID (deduplication).
func (r *Relationships) Add(relType, target string) string {
    // Check for existing relationship with same target
    if existingID, exists := r.byTarget[target]; exists {
        return existingID
    }

    id := r.AllocID()
    rel := RelationshipXML{
        ID:     id,
        Type:   relType,
        Target: target,
    }

    r.byID[id] = len(r.rels)
    r.byTarget[target] = id
    r.rels = append(r.rels, rel)

    return id
}

// AddWithID adds a relationship with a specific ID.
// Use this when restoring relationships or maintaining existing IDs.
// Returns an error if the ID is already in use.
func (r *Relationships) AddWithID(id, relType, target string) error {
    if _, exists := r.byID[id]; exists {
        return fmt.Errorf("relationship ID already exists: %s", id)
    }

    rel := RelationshipXML{
        ID:     id,
        Type:   relType,
        Target: target,
    }

    r.byID[id] = len(r.rels)
    r.byTarget[target] = id
    r.rels = append(r.rels, rel)

    // Update maxID if this ID is higher
    if num := parseRelID(id); num > r.maxID {
        r.maxID = num
    }

    return nil
}

// AddExternal creates an external relationship (e.g., hyperlink to external URL).
func (r *Relationships) AddExternal(relType, target string) string {
    id := r.AllocID()
    rel := RelationshipXML{
        ID:         id,
        Type:       relType,
        Target:     target,
        TargetMode: "External",
    }

    r.byID[id] = len(r.rels)
    r.byTarget[target] = id
    r.rels = append(r.rels, rel)

    return id
}

// Get returns a relationship by ID, or nil if not found.
func (r *Relationships) Get(id string) *RelationshipXML {
    idx, ok := r.byID[id]
    if !ok {
        return nil
    }
    return &r.rels[idx]
}

// FindByTarget returns the relationship ID for a given target path.
// Returns empty string if no relationship points to that target.
func (r *Relationships) FindByTarget(target string) string {
    return r.byTarget[target]
}

// FindByType returns all relationships of a given type.
func (r *Relationships) FindByType(relType string) []RelationshipXML {
    var result []RelationshipXML
    for _, rel := range r.rels {
        if rel.Type == relType {
            result = append(result, rel)
        }
    }
    return result
}

// Remove deletes a relationship by ID.
// Returns true if the relationship was found and removed.
func (r *Relationships) Remove(id string) bool {
    idx, ok := r.byID[id]
    if !ok {
        return false
    }

    rel := r.rels[idx]
    delete(r.byTarget, rel.Target)
    delete(r.byID, id)

    // Remove from slice
    r.rels = append(r.rels[:idx], r.rels[idx+1:]...)

    // Update indices in byID map
    for i := idx; i < len(r.rels); i++ {
        r.byID[r.rels[i].ID] = i
    }

    return true
}

// All returns all relationships (copy of internal slice).
func (r *Relationships) All() []RelationshipXML {
    result := make([]RelationshipXML, len(r.rels))
    copy(result, r.rels)
    return result
}

// Count returns the number of relationships.
func (r *Relationships) Count() int {
    return len(r.rels)
}

// Marshal serializes the relationships to XML.
// Output is deterministic: relationships are sorted by ID.
func (r *Relationships) Marshal() ([]byte, error) {
    // Sort relationships by ID for deterministic output
    sorted := make([]RelationshipXML, len(r.rels))
    copy(sorted, r.rels)
    sort.Slice(sorted, func(i, j int) bool {
        numI := parseRelID(sorted[i].ID)
        numJ := parseRelID(sorted[j].ID)
        if numI != 0 && numJ != 0 {
            return numI < numJ
        }
        return sorted[i].ID < sorted[j].ID
    })

    relsXML := RelationshipsXML{
        XMLName:       xml.Name{Space: NsPackageRels, Local: "Relationships"},
        Relationships: sorted,
    }

    data, err := xml.MarshalIndent(relsXML, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("failed to marshal relationships: %w", err)
    }

    return append([]byte(xml.Header), data...), nil
}

// Clone creates an independent copy of the relationships.
func (r *Relationships) Clone() *Relationships {
    clone := &Relationships{
        rels:     make([]RelationshipXML, len(r.rels)),
        maxID:    r.maxID,
        byID:     make(map[string]int),
        byTarget: make(map[string]string),
    }

    copy(clone.rels, r.rels)
    for k, v := range r.byID {
        clone.byID[k] = v
    }
    for k, v := range r.byTarget {
        clone.byTarget[k] = v
    }

    return clone
}
```

### Path Utility Functions

```go
// GetRelsPath returns the .rels path for a given part path.
// For example: "ppt/slides/slide1.xml" -> "ppt/slides/_rels/slide1.xml.rels"
func GetRelsPath(partPath string) string

// GetPartPath returns the part path from a .rels path.
// For example: "ppt/slides/_rels/slide1.xml.rels" -> "ppt/slides/slide1.xml"
func GetPartPath(relsPath string) string

// PackageRels returns the path to the package-level relationships file.
func PackageRels() string  // returns "_rels/.rels"

// PresentationRels returns the path to the presentation relationships file.
func PresentationRels() string  // returns "ppt/_rels/presentation.xml.rels"
```

## Common Relationship Types

```go
const (
    RelTypeSlide         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"
    RelTypeSlideLayout   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout"
    RelTypeSlideMaster   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster"
    RelTypeImage         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
    RelTypeHyperlink     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
    RelTypeTheme         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"
    RelTypePresProps     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps"
    RelTypeViewProps     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps"
    RelTypeTableStyles   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles"
    RelTypeNotesMaster   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster"
    RelTypeHandoutMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/handoutMaster"
)
```

## Acceptance Criteria

### AC1: Parse Existing Relationships
- Given `.rels` file with rId1, rId3, rId5
- When parsed
- Then maxID is 5

### AC2: Add New Relationship
- Given relationships with maxID 5
- When Add() called
- Then returns "rId6"

### AC3: No ID Collision
- Given 10 AllocID() calls on empty relationships
- When marshaled
- Then all IDs are unique (rId1 through rId10)

### AC4: Find By Target
- Given relationship with Target="../media/image1.png"
- When FindByTarget("../media/image1.png")
- Then returns the relationship ID

### AC5: External Relationships
- Given AddExternal() call for hyperlink
- When marshaled
- Then includes TargetMode="External"

### AC6: Round-Trip
- Given existing .rels file
- When parsed, modified, and marshaled
- Then original relationships preserved, new ones added

### AC7: Deduplication
- Given Add() called twice with same target
- When Count() called
- Then returns 1 (deduplicated)

### AC8: Deterministic Output
- Given relationships added in any order
- When Marshal() called
- Then output is sorted by rId number

### AC9: Clone Independence
- Given cloned relationships
- When original modified
- Then clone unchanged

## Integration

Update `internal/generator/images.go` to use `Relationships` manager:

```go
// Before
nextRelID := 1
for _, rel := range existingRels.Relationships { ... }
newRelID := fmt.Sprintf("rId%d", nextRelID)

// After
rels, _ := pptx.ParseRelationships(relsData)
newRelID := rels.Add(pptx.RelTypeImage, "../media/image1.png")
relsData, _ = rels.Marshal()
```

## Testing

Tests located at `internal/pptx/relationships_test.go` covering:
- Empty relationships
- Parsing existing rels
- Adding multiple relationships
- Adding with specific ID
- Finding by target and type
- Removing relationships
- Round-trip validation
- Deterministic marshaling (sorted output)
- Cloning independence
- Path utility functions
- Benchmark tests for Add and Parse/Marshal operations
