package patterns

import (
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// Picture references inside pattern values (go-slide-creator-hdpq).
//
// image-text-split established the contract — {path | url, alt}, resolved by
// the host exactly like a shape_grid image cell and cover-cropped into its
// frame. team-bios and pull-quote now take one too, so an "Our team" page and a
// customer quote with a headshot no longer have to be hand-built as a raw
// shape_grid, abandoning the pattern's own sizing.
//
// These helpers keep the three patterns saying the same thing about the same
// field rather than each spelling its own validation and schema.

// PhotoSchema is the {path | url, alt} reference shared by every pattern that
// takes a picture in its values. describe names the slot; altMax is the alt
// budget in runes.
func PhotoSchema(describe string, altMax int) *Schema {
	return ObjectSchema(map[string]*Schema{
		"path": StringSchema(0).WithDescription("Local .png / .jpg file (relative paths resolve against the deck JSON directory)"),
		"url":  StringSchema(0).WithDescription("HTTPS image URL (downloaded and cached)"),
		"alt":  StringSchema(altMax).WithDescription("Alt text for screen readers"),
	}, nil).WithAdditionalProperties(false).WithDescription(describe)
}

// validatePatternPhoto reports what is wrong with one picture reference: a
// reference with neither path nor url (which would silently render nothing),
// an over-long alt, or the overlay/text keys that only a shape_grid image cell
// accepts. A nil photo is valid — every pattern taking one falls back to a
// placeholder.
func validatePatternPhoto(pattern, path string, img *jsonschema.GridImageInput, altMax int) []error {
	if img == nil {
		return nil
	}
	var errs []error
	if strings.TrimSpace(img.Path) == "" && strings.TrimSpace(img.URL) == "" {
		errs = append(errs, newValidationError(pattern, path, ErrCodeRequired,
			pattern+": "+path+" needs a path or url (omit it to render the placeholder)",
			ProvideValueFix(path+".path")))
	}
	if runeLen(img.Alt) > altMax {
		errs = append(errs, errMaxLength(pattern, path+".alt", altMax, runeLen(img.Alt)))
	}
	if img.Overlay != nil || img.Text != nil {
		errs = append(errs, errUnknownKey(pattern, path, "overlay/text", "path, url, alt"))
	}
	return errs
}

// patternPhotoCell builds the image cell for a resolved picture reference, or
// nil when the reference carries no source. defaultAlt is used when the author
// gave none.
func patternPhotoCell(img *jsonschema.GridImageInput, defaultAlt string) *jsonschema.GridCellInput {
	if img == nil || (strings.TrimSpace(img.Path) == "" && strings.TrimSpace(img.URL) == "") {
		return nil
	}
	alt := strings.TrimSpace(img.Alt)
	if alt == "" {
		alt = defaultAlt
	}
	return &jsonschema.GridCellInput{
		Image: &jsonschema.GridImageInput{Path: img.Path, URL: img.URL, Alt: alt},
	}
}
