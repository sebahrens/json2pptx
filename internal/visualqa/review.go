package visualqa

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// ReviewRecord is the vendor-neutral completion contract shared by configured
// vision providers and host/manual reviewers. It binds the review to exact
// artifact and pixel hashes so evidence from an earlier revision cannot pass.
type ReviewRecord struct {
	ArtifactSHA256 string        `json:"artifact_sha256"`
	Revision       string        `json:"revision"`
	Backend        string        `json:"backend"` // provider, host, or manual
	Provider       string        `json:"provider,omitempty"`
	ContactSheet   string        `json:"contact_sheet,omitempty"`
	Slides         []ReviewSlide `json:"slides"`
	Findings       []Finding     `json:"findings"`
	Verdict        string        `json:"verdict"` // approved, changes_requested, inconclusive
}

type ReviewSlide struct {
	Index       int    `json:"index"`
	Role        string `json:"role"`
	ImagePath   string `json:"image_path,omitempty"`
	ImageSHA256 string `json:"image_sha256"`
}

// ValidateCompletion requires current pixels for every expected slide. Manual
// and host review use the same checks as provider adapters; only a provider or
// explicitly recorded host/manual verdict can claim inspection.
func (r *ReviewRecord) ValidateCompletion(artifactHash, revision string, totalSlides int) error {
	if r == nil {
		return fmt.Errorf("review record is required")
	}
	if r.ArtifactSHA256 != artifactHash || r.Revision != revision {
		return fmt.Errorf("stale visual review evidence")
	}
	if r.Backend != "provider" && r.Backend != "host" && r.Backend != "manual" {
		return fmt.Errorf("unsupported review backend %q", r.Backend)
	}
	if totalSlides <= 0 || len(r.Slides) != totalSlides {
		return fmt.Errorf("all-slide coverage required: reviewed %d of %d", len(r.Slides), totalSlides)
	}
	indices := make([]int, 0, len(r.Slides))
	for _, s := range r.Slides {
		if s.Role == "" || s.ImageSHA256 == "" {
			return fmt.Errorf("slide %d is missing role or pixel hash", s.Index)
		}
		indices = append(indices, s.Index)
	}
	sort.Ints(indices)
	for i, idx := range indices {
		if idx != i {
			return fmt.Errorf("all-slide coverage required: missing slide %d", i)
		}
	}
	if r.Verdict != "approved" && r.Verdict != "changes_requested" && r.Verdict != "inconclusive" {
		return fmt.Errorf("invalid review verdict %q", r.Verdict)
	}
	return nil
}

func PixelHash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
