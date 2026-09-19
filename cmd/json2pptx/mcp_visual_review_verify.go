// mcp_visual_review_verify.go binds a submitted visual review to the artifact's
// own rendered pixels (go-slide-creator-jltp).
//
// submit_visual_review is the documented completion path when no vision provider
// is configured, and "inspect every rendered image yourself" is the only
// definition of done — so the review's images are the evidence. Before this,
// only the PPTX sha256 was checked: a review of six slides all pointing at
// slide-01.png, or six images taken from a different deck, both returned
// status "visually_reviewed_current_revision". The completion contract was an
// honour system a cost-minimising agent short-circuits in one call.
//
// Verification compares each submitted pixel hash against the renders this
// server already produced for that exact PPTX (the render cache keys by file
// hash + density, so every density the deck was rendered at counts). It never
// invokes LibreOffice: a deck with no cached render is *unverifiable*, not
// *wrong*, and an unverifiable review is recorded without the completion status
// instead of being rejected.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

const (
	// imageVerificationVerified: every slide's submitted image is this
	// artifact's own rendered pixels for that index.
	imageVerificationVerified = "verified"
	// imageVerificationUnverifiable: this server has no render of the artifact
	// to compare against, so the images can be neither confirmed nor refuted.
	imageVerificationUnverifiable = "unverifiable"
)

// howToVerify is the one instruction that turns an unverifiable review into a
// verifiable one.
const howToVerify = "render this exact PPTX with render_deck_thumbnails (or render_slide_image per slide) and submit the returned slides[].path / content_hash — verification compares your pixel hashes against this server's own render of the artifact."

// imageVerification reports whether the review's images are the artifact's
// pixels. It is always present in the submit_visual_review response: an agent
// reading "verified_slides: 0" learns that its review did not carry evidence.
type imageVerification struct {
	Status         string   `json:"status"`
	Method         string   `json:"method"`
	VerifiedSlides int      `json:"verified_slides"`
	TotalSlides    int      `json:"total_slides"`
	Reasons        []string `json:"reasons,omitempty"`
	HowToVerify    string   `json:"how_to_verify,omitempty"`
}

// verified reports whether every slide was matched to the artifact's own render.
func (v *imageVerification) verified() bool {
	return v != nil && v.Status == imageVerificationVerified
}

// cachedSlideHashes is indirected so tests can supply a render set without
// LibreOffice on PATH.
var cachedSlideHashes = render.CachedSlideHashes

// verifyReviewImages compares each reviewed slide's pixel hash against the
// cached renders of artifactHash.
//
// A slide whose index has a cached render and whose hash does not match it is a
// hard rejection (wrapping errVisualReviewRejected): the image is positively not
// that slide of this deck. A slide whose index has no cached render is counted
// unverified — absence of a render is not evidence of forgery.
func verifyReviewImages(slides []visualqa.ReviewSlide, artifactHash string, total int) (*imageVerification, error) {
	v := &imageVerification{
		Status:         imageVerificationVerified,
		Method:         "render_cache",
		TotalSlides:    total,
		VerifiedSlides: 0,
	}
	known := cachedSlideHashes(artifactHash)
	if len(known) == 0 {
		v.Status = imageVerificationUnverifiable
		v.Reasons = append(v.Reasons, fmt.Sprintf("this server has no cached render of artifact %s, so the submitted images could not be compared against its pixels", shortHash(artifactHash)))
		v.HowToVerify = howToVerify
		return v, nil
	}

	// Reverse index: which slide of this deck a hash actually renders to. It
	// turns "wrong image" into "that is slide 0, submitted as slide 4".
	source := map[string][]int{}
	for idx, hashes := range known {
		for _, h := range hashes {
			source[h] = append(source[h], idx)
		}
	}

	var unverified []int
	for _, s := range slides {
		expected, ok := known[s.Index]
		if !ok || len(expected) == 0 {
			unverified = append(unverified, s.Index)
			continue
		}
		if containsHash(expected, s.ImageSHA256) {
			v.VerifiedSlides++
			continue
		}
		return nil, fmt.Errorf("%w: slides[%d] image does not show slide %d of this deck: %s", errVisualReviewRejected, s.Index, s.Index, mismatchDetail(s, source, artifactHash))
	}

	if len(unverified) > 0 {
		sort.Ints(unverified)
		v.Status = imageVerificationUnverifiable
		v.Reasons = append(v.Reasons, fmt.Sprintf("no cached render of artifact %s covers slide(s) %s, so their images could not be compared against its pixels", shortHash(artifactHash), joinInts(unverified)))
		v.HowToVerify = howToVerify
	}
	return v, nil
}

// mismatchDetail explains a rejected image: which slide it really is when this
// deck's render produced it, otherwise that it is not from this artifact at all.
func mismatchDetail(s visualqa.ReviewSlide, source map[string][]int, artifactHash string) string {
	if got, ok := source[s.ImageSHA256]; ok {
		sort.Ints(got)
		return fmt.Sprintf("pixel hash %s is slide %s of this deck (submitted as slide %d) — submit each slide's own image", shortHash(s.ImageSHA256), joinInts(got), s.Index)
	}
	where := "image_sha256"
	if s.ImagePath != "" {
		where = s.ImagePath
	}
	return fmt.Sprintf("pixel hash %s (%s) matches no render of artifact %s. %s", shortHash(s.ImageSHA256), where, shortHash(artifactHash), howToVerify)
}

func containsHash(hashes []string, want string) bool {
	for _, h := range hashes {
		if strings.EqualFold(h, want) {
			return true
		}
	}
	return false
}

// shortHash abbreviates a digest for a message; a full sha256 per slide would
// bury the point.
func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12] + "…"
}

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprint(x)
	}
	return strings.Join(parts, ", ")
}
