package template

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

const ProfileParserVersion = "1"

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
	Diagnostics   []ProfileDiagnostic                  `json:"diagnostics,omitempty"`
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
	if width <= 0 || height <= 0 {
		return "unknown"
	}
	r := float64(width) / float64(height)
	if r > 1.7 && r < 1.82 {
		return "16:9"
	}
	if r > 1.3 && r < 1.36 {
		return "4:3"
	}
	return fmt.Sprintf("%.3f:1", r)
}

func cloneProfile(in *TemplateProfile) *TemplateProfile {
	if in == nil {
		return nil
	}
	out := *in
	out.Layouts = append([]types.LayoutMetadata(nil), in.Layouts...)
	out.RoleBindings = make(map[types.CanonicalLayoutType]string, len(in.RoleBindings))
	for k, v := range in.RoleBindings {
		out.RoleBindings[k] = v
	}
	out.Diagnostics = append([]ProfileDiagnostic(nil), in.Diagnostics...)
	return &out
}
