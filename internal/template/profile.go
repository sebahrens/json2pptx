package template

import (
	"encoding/xml"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ProfileParserVersion is part of the profile cache key; bump it whenever the
// profile shape or derivation changes so cached profiles are rebuilt.
// v2: resolved footer regions + per-layout chrome geometry.
// v3: effective solid layout/master background for contrast preflight.
// v4: master/layout decorative regions and side-art chrome exclusions.
const ProfileParserVersion = "4"

type ProfileDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// TemplateProfile is the effective, relationship-aware description shared by
// layout selection, geometry checks, and discovery tools.
type TemplateProfile struct {
	TemplateHash  string                               `json:"template_hash"`
	ParserVersion string                               `json:"parser_version"`
	SlideWidth    int64                                `json:"slide_width"`
	SlideHeight   int64                                `json:"slide_height"`
	AspectRatio   string                               `json:"aspect_ratio"`
	Layouts       []types.LayoutMetadata               `json:"layouts"`
	RoleBindings  map[types.CanonicalLayoutType]string `json:"role_bindings"`
	// Geometry is the per-layout chrome geometry (footer regions, content
	// area, takeaway and source bands) derived from the layout placeholders.
	// It is the single source the generator, shape_grid reservation, and
	// examine_template read, so all three agree on where chrome sits.
	Geometry    []LayoutGeometry    `json:"geometry"`
	Diagnostics []ProfileDiagnostic `json:"diagnostics,omitempty"`
}

// LayoutGeometry is the profiled chrome geometry of one layout. Frame is
// resolved with both the takeaway and source bands reserved (the worst case);
// callers needing another combination use TemplateProfile.ChromeFrame.
type LayoutGeometry struct {
	LayoutID      string               `json:"layout_id"`
	FooterRegions []types.ChromeRegion `json:"footer_regions"`
	Frame         ChromeFrame          `json:"frame"`
}

// Layout returns the profiled layout with the given ID, or nil.
func (p *TemplateProfile) Layout(id string) *types.LayoutMetadata {
	if p == nil {
		return nil
	}
	return FindLayout(p.Layouts, id)
}

// ReferenceLayout returns the layout bound to the One Content role, the
// reference geometry for chrome on layouts without a body placeholder.
func (p *TemplateProfile) ReferenceLayout() *types.LayoutMetadata {
	if p == nil {
		return nil
	}
	if id, ok := p.RoleBindings[types.CanonicalLayoutOneContent]; ok {
		return p.Layout(id)
	}
	return nil
}

// ChromeFrame resolves the chrome frame of a slide on layoutID. Unknown IDs
// (e.g. synthesized layouts) resolve against the reference layout.
func (p *TemplateProfile) ChromeFrame(layoutID string, hasTakeaway, hasSource bool) ChromeFrame {
	if p == nil {
		return ResolveChromeFrame(nil, nil, 0, 0, hasTakeaway, hasSource)
	}
	return ResolveChromeFrame(p.Layout(layoutID), p.ReferenceLayout(), p.SlideWidth, p.SlideHeight, hasTakeaway, hasSource)
}

var profileCache = struct {
	sync.RWMutex
	m map[string]*TemplateProfile
}{m: make(map[string]*TemplateProfile)}

// BuildProfile resolves each layout through layout -> master -> theme
// relationships and validates that canonical role bindings are unambiguous and
// geometrically usable. Cache identity is content hash plus parser version, so
// replacing a template at the same path cannot return stale metadata.
func BuildProfile(reader *Reader) (*TemplateProfile, error) {
	if reader == nil {
		return nil, fmt.Errorf("template reader is required")
	}
	key := reader.Hash() + ":" + ProfileParserVersion
	profileCache.RLock()
	cached := profileCache.m[key]
	profileCache.RUnlock()
	if cached != nil {
		return cloneProfile(cached), nil
	}

	layouts, err := ParseLayouts(reader)
	if err != nil {
		return nil, err
	}
	width, height := ParseSlideDimensions(reader)
	p := &TemplateProfile{
		TemplateHash: reader.Hash(), ParserVersion: ProfileParserVersion,
		SlideWidth: width, SlideHeight: height, AspectRatio: aspectRatio(width, height),
		Layouts: layouts, RoleBindings: make(map[types.CanonicalLayoutType]string),
	}

	roleCandidates := make(map[types.CanonicalLayoutType][]int)
	for i := range p.Layouts {
		master, themePath, relErr := resolveLayoutRelationships(reader, p.Layouts[i].ID)
		if relErr != nil {
			p.Diagnostics = append(p.Diagnostics, ProfileDiagnostic{Code: "LAYOUT_RELATIONSHIP_INVALID", Severity: "error", Message: fmt.Sprintf("layout %s: %v", p.Layouts[i].ID, relErr)})
		} else {
			p.Layouts[i].MasterPath = master
			p.Layouts[i].ThemePath = themePath
			if themePath != "" {
				data, readErr := reader.ReadFile(themePath)
				if readErr == nil {
					p.Layouts[i].Theme = parseThemeData(data)
				}
			}
		}
		role := EffectiveCanonicalType(&p.Layouts[i])
		if role != types.CanonicalLayoutUnknown {
			roleCandidates[role] = append(roleCandidates[role], i)
		}
		for _, ph := range p.Layouts[i].Placeholders {
			if ph.Role == types.PlaceholderRoleBody && (ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0) {
				p.Diagnostics = append(p.Diagnostics, ProfileDiagnostic{Code: "UNUSABLE_GEOMETRY", Severity: "error", Message: fmt.Sprintf("layout %s body placeholder %s has unresolved bounds", p.Layouts[i].ID, ph.ID)})
			}
		}
	}

	roles := make([]string, 0, len(roleCandidates))
	byName := make(map[string]types.CanonicalLayoutType)
	for role := range roleCandidates {
		name := string(role)
		roles = append(roles, name)
		byName[name] = role
	}
	sort.Strings(roles)
	for _, name := range roles {
		role := byName[name]
		indices := roleCandidates[role]
		sort.SliceStable(indices, func(i, j int) bool {
			a, b := p.Layouts[indices[i]], p.Layouts[indices[j]]
			if a.CanonicalConfidence == b.CanonicalConfidence {
				return a.ID < b.ID
			}
			return a.CanonicalConfidence > b.CanonicalConfidence
		})
		p.RoleBindings[role] = p.Layouts[indices[0]].ID
		if len(indices) > 1 && p.Layouts[indices[0]].CanonicalConfidence == p.Layouts[indices[1]].CanonicalConfidence {
			p.Diagnostics = append(p.Diagnostics, ProfileDiagnostic{Code: "AMBIGUOUS_CANONICAL_ROLE", Severity: "warning", Message: fmt.Sprintf("role %s matches %s and %s with equal confidence", role, p.Layouts[indices[0]].ID, p.Layouts[indices[1]].ID)})
		}
	}

	ref := p.ReferenceLayout()
	p.Geometry = make([]LayoutGeometry, len(p.Layouts))
	for i := range p.Layouts {
		l := &p.Layouts[i]
		regions := l.FooterRegions
		if regions == nil {
			regions = []types.ChromeRegion{}
		}
		p.Geometry[i] = LayoutGeometry{
			LayoutID:      l.ID,
			FooterRegions: regions,
			Frame:         ResolveChromeFrame(l, ref, width, height, true, true),
		}
	}

	profileCache.Lock()
	profileCache.m[key] = cloneProfile(p)
	profileCache.Unlock()
	return p, nil
}

func resolveLayoutRelationships(reader *Reader, layoutID string) (string, string, error) {
	relsPath := fmt.Sprintf("ppt/slideLayouts/_rels/%s.xml.rels", layoutID)
	data, err := reader.ReadFile(relsPath)
	if err != nil {
		return "", "", err
	}
	var rels pptx.RelationshipsXML
	if err := xml.Unmarshal(data, &rels); err != nil {
		return "", "", err
	}
	master := ""
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeSlideMaster {
			master = ResolveRelativePath("ppt/slideLayouts", rel.Target)
			break
		}
	}
	if master == "" {
		return "", "", fmt.Errorf("missing slideMaster relationship")
	}
	masterRels := filepath.ToSlash(filepath.Join(filepath.Dir(master), "_rels", filepath.Base(master)+".rels"))
	data, err = reader.ReadFile(masterRels)
	if err != nil {
		return master, "", err
	}
	if err := xml.Unmarshal(data, &rels); err != nil {
		return master, "", err
	}
	for _, rel := range rels.Relationships {
		if rel.Type == pptx.RelTypeTheme {
			return master, ResolveRelativePath(filepath.Dir(master), rel.Target), nil
		}
	}
	return master, "", nil
}

func aspectRatio(width, height int64) string {
	return AspectRatio(width, height)
}

// namedAspectRatios are the ratios worth reporting by name, with the tolerance
// each is recognised within. Order matters only for readability; the bands do
// not overlap.
var namedAspectRatios = []struct {
	name      string
	ratio     float64
	tolerance float64
}{
	{"4:3", 4.0 / 3.0, 0.03},
	{"16:10", 1.6, 0.02},
	{"16:9", 16.0 / 9.0, 0.06},
	{"21:9", 21.0 / 9.0, 0.06},
}

// AspectRatio reports a slide size as an aspect-ratio string: a recognised name
// ("4:3", "16:10", "16:9", "21:9") when the dimensions land within tolerance of
// one, otherwise a decimal "W.WWW:1" form.
//
// Callers used to hardcode "16:9" and only override it from template metadata,
// so a 10x7.5in template was reported as widescreen — and list_templates
// fields=compact exposes nothing but the ratio, so an agent sizing columns and
// text for a 4:3 corporate deck was told it had 16:9 to work with
// (go-slide-creator-r9nn).
func AspectRatio(width, height int64) string {
	if width <= 0 || height <= 0 {
		return "unknown"
	}
	r := float64(width) / float64(height)
	for _, named := range namedAspectRatios {
		if math.Abs(r-named.ratio) <= named.tolerance {
			return named.name
		}
	}
	return fmt.Sprintf("%.3f:1", r)
}

func cloneProfile(in *TemplateProfile) *TemplateProfile {
	if in == nil {
		return nil
	}
	out := *in
	out.Layouts = append([]types.LayoutMetadata(nil), in.Layouts...)
	for i := range out.Layouts {
		out.Layouts[i].DecorRegions = append([]types.DecorRegion(nil), in.Layouts[i].DecorRegions...)
	}
	out.RoleBindings = make(map[types.CanonicalLayoutType]string, len(in.RoleBindings))
	for k, v := range in.RoleBindings {
		out.RoleBindings[k] = v
	}
	out.Geometry = append([]LayoutGeometry(nil), in.Geometry...)
	out.Diagnostics = append([]ProfileDiagnostic(nil), in.Diagnostics...)
	return &out
}
