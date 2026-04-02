// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"cmp"
	"encoding/xml"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Common relationship type URIs used in PPTX files.
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
	RelTypeNotesSlide    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"
	RelTypeHandoutMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/handoutMaster"
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
// If target already exists, returns the existing relationship ID.
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

// All returns all relationships.
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
	slices.SortFunc(sorted, func(a, b RelationshipXML) int {
		// Sort numerically by rId number
		numA := parseRelID(a.ID)
		numB := parseRelID(b.ID)
		if numA != 0 && numB != 0 {
			return cmp.Compare(numA, numB)
		}
		// Fall back to string comparison for non-standard IDs
		return cmp.Compare(a.ID, b.ID)
	})

	relsXML := RelationshipsXML{
		XMLName:       xml.Name{Space: NsPackageRels, Local: "Relationships"},
		Xmlns:         NsPackageRels,
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

// GetRelsPath returns the .rels path for a given part path.
// For example: "ppt/slides/slide1.xml" -> "ppt/slides/_rels/slide1.xml.rels"
func GetRelsPath(partPath string) string {
	dir := ""
	file := partPath

	if idx := strings.LastIndex(partPath, "/"); idx >= 0 {
		dir = partPath[:idx+1]
		file = partPath[idx+1:]
	}

	return dir + "_rels/" + file + ".rels"
}

// GetPartPath returns the part path from a .rels path.
// For example: "ppt/slides/_rels/slide1.xml.rels" -> "ppt/slides/slide1.xml"
func GetPartPath(relsPath string) string {
	// Remove .rels suffix
	if !strings.HasSuffix(relsPath, ".rels") {
		return relsPath
	}
	path := strings.TrimSuffix(relsPath, ".rels")

	// Remove _rels/ from path
	path = strings.Replace(path, "/_rels/", "/", 1)
	path = strings.Replace(path, "_rels/", "", 1)

	return path
}

// PackageRels returns the path to the package-level relationships file.
func PackageRels() string {
	return "_rels/.rels"
}

// PresentationRels returns the path to the presentation relationships file.
func PresentationRels() string {
	return "ppt/_rels/presentation.xml.rels"
}
