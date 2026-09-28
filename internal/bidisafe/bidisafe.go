// Package bidisafe neutralises invisible Unicode control characters in deck
// input at the JSON boundary (go-slide-creator-7oz3c).
//
// The embedding / override / isolate bidi controls (U+202A–U+202E,
// U+2066–U+2069) and the byte-order mark / zero-width no-break space
// (U+FEFF) have no visible effect a slide author wants, but they can spoof
// the reading order of rendered text ("Trojan Source") and, followed by an
// emoji and a combining mark, they crash the HarfBuzz shaper used for text
// measurement. They are removed from every JSON string value before the deck
// is decoded, and each string that changed is reported so the caller can
// surface a diagnostic.
//
// Directional marks (U+200E LRM, U+200F RLM, U+061C ALM) are legitimate in
// right-to-left prose and are left alone; the measurement layer guards the
// shaper against them separately.
package bidisafe

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// Site is one JSON string value that had control characters removed.
type Site struct {
	// Path is the RFC 6901 JSON Pointer of the string value, e.g.
	// "/slides/0/content/1/text_value".
	Path string `json:"path"`
	// Removed is the number of control characters removed from the value.
	Removed int `json:"removed"`
}

// IsControl reports whether r is one of the neutralised control characters.
func IsControl(r rune) bool {
	return (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) || r == 0xFEFF
}

// StripString removes the neutralised control characters from s and returns
// the result and the number removed.
func StripString(s string) (string, int) {
	if !strings.ContainsFunc(s, IsControl) {
		return s, 0
	}
	n := 0
	out := strings.Map(func(r rune) rune {
		if IsControl(r) {
			n++
			return -1
		}
		return r
	}, s)
	return out, n
}

// MayContain is a cheap pre-check: false guarantees data (a JSON document)
// holds none of the neutralised characters, either as raw UTF-8 or as a
// \uXXXX escape. A true result may be a false positive.
func MayContain(data []byte) bool {
	// Raw UTF-8: U+202A–U+202E = E2 80 AA..AE, U+2066–U+2069 = E2 81 A6..A9,
	// U+FEFF = EF BB BF.
	for i := 0; i+2 < len(data); i++ {
		switch data[i] {
		case 0xE2:
			b1, b2 := data[i+1], data[i+2]
			if (b1 == 0x80 && b2 >= 0xAA && b2 <= 0xAE) || (b1 == 0x81 && b2 >= 0xA6 && b2 <= 0xA9) {
				return true
			}
		case 0xEF:
			if data[i+1] == 0xBB && data[i+2] == 0xBF {
				return true
			}
		case '\\':
			if data[i+1] == 'u' && i+5 < len(data) {
				esc := strings.ToLower(string(data[i+2 : i+6]))
				if strings.HasPrefix(esc, "202") || strings.HasPrefix(esc, "206") || esc == "feff" {
					return true
				}
			}
		}
	}
	return false
}

// Neutralize removes the control characters from every string value in the
// JSON document data. When nothing needs removing it returns data unchanged
// and nil sites without re-encoding. Otherwise it returns the re-encoded
// document (numbers preserved verbatim) and the changed sites in path order.
// A leading UTF-8 byte-order mark before the document is dropped too.
//
// Malformed JSON is returned unchanged so the caller's own
// decoder reports the syntax error in its usual shape.
func Neutralize(data []byte) ([]byte, []Site) {
	trimmed := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !MayContain(trimmed) {
		return trimmed, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return data, nil
	}
	var sites []Site
	doc = walk(doc, "", &sites)
	if len(sites) == 0 {
		return trimmed, nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return data, nil
	}
	sort.SliceStable(sites, func(i, j int) bool { return sites[i].Path < sites[j].Path })
	return bytes.TrimRight(buf.Bytes(), "\n"), sites
}

func walk(v any, path string, sites *[]Site) any {
	switch t := v.(type) {
	case string:
		s, n := StripString(t)
		if n > 0 {
			*sites = append(*sites, Site{Path: path, Removed: n})
		}
		return s
	case []any:
		for i := range t {
			t[i] = walk(t[i], path+"/"+strconv.Itoa(i), sites)
		}
		return t
	case map[string]any:
		for k, child := range t {
			t[k] = walk(child, path+"/"+escapePointer(k), sites)
		}
		return t
	default:
		return v
	}
}

func escapePointer(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}
