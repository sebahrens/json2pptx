package svggen

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var emittedFontURI = regexp.MustCompile(`data:font/(?:ttf|otf|sfnt);base64,([A-Za-z0-9+/=]+)`)
var emittedFontRules = regexp.MustCompile(`(?:\n@font-face\{[^\n]*\})+`)
var emittedFontDescriptor = regexp.MustCompile(`^@font-face\{font-family:'([^']*)'((?:;font-weight:[0-9]+|;font-style:italic|;font-style:normal)*)$`)

// normalizeEmittedFontDeterminism changes only generated font metadata and
// ordering of canvas's contiguous font rules, never glyphs or drawing data.
func normalizeEmittedFontDeterminism(source []byte) ([]byte, error) {
	var failure error
	content := emittedFontURI.ReplaceAllFunc(source, func(uri []byte) []byte {
		if failure != nil {
			return uri
		}
		comma := bytes.IndexByte(uri, ',')
		font, err := base64.StdEncoding.DecodeString(string(uri[comma+1:]))
		if err != nil {
			failure = fmt.Errorf("decode embedded font: %w", err)
			return uri
		}
		font, err = stableFontMetadata(font)
		if err != nil {
			failure = err
			return uri
		}
		encoded := base64.StdEncoding.EncodeToString(font)
		return append(append([]byte(nil), uri[:comma+1]...), encoded...)
	})
	if failure != nil {
		return nil, failure
	}
	content = emittedFontRules.ReplaceAllFunc(content, func(block []byte) []byte {
		rules := bytes.Split(block[1:], []byte("\n"))
		// Stable sorting by descriptors preserves the relative order of duplicate
		// declarations: their CSS cascade can intentionally depend on that order.
		sort.SliceStable(rules, func(i, j int) bool {
			return bytes.Compare(fontRuleDescriptor(rules[i]), fontRuleDescriptor(rules[j])) < 0
		})
		return append([]byte("\n"), bytes.Join(rules, []byte("\n"))...)
	})
	return content, nil
}

func fontRuleDescriptor(rule []byte) []byte {
	if index := bytes.Index(rule, []byte(";src:")); index >= 0 {
		descriptor := rule[:index]
		fields := emittedFontDescriptor.FindSubmatch(descriptor)
		if len(fields) != 3 {
			return descriptor
		}
		weight, style := "400", "normal"
		for _, property := range strings.Split(string(fields[2]), ";") {
			if value, ok := strings.CutPrefix(property, "font-weight:"); ok {
				weight = value
			}
			if value, ok := strings.CutPrefix(property, "font-style:"); ok {
				style = value
			}
		}
		return []byte(strings.ToLower(string(fields[1])) + "\x00" + weight + "\x00" + style)
	}
	return rule
}

func stableFontMetadata(source []byte) ([]byte, error) {
	if len(source) < 12 {
		return nil, fmt.Errorf("embedded font has truncated SFNT header")
	}
	signature := string(source[:4])
	if signature != "\x00\x01\x00\x00" && signature != "OTTO" && signature != "true" {
		return nil, fmt.Errorf("unsupported embedded font signature")
	}
	count := int(binary.BigEndian.Uint16(source[4:6]))
	if count > (len(source)-12)/16 {
		return nil, fmt.Errorf("embedded font has truncated table directory")
	}
	headRecord, headOffset, headLength := -1, 0, 0
	spans := make([][2]uint64, 0, count)
	for index := 0; index < count; index++ {
		record := 12 + 16*index
		offset := uint64(binary.BigEndian.Uint32(source[record+8 : record+12]))
		length := uint64(binary.BigEndian.Uint32(source[record+12 : record+16]))
		if offset < uint64(12+16*count) || offset+length > uint64(len(source)) {
			return nil, fmt.Errorf("embedded font has invalid table bounds")
		}
		for _, span := range spans {
			if length > 0 && span[0] < offset+length && offset < span[1] {
				return nil, fmt.Errorf("embedded font has overlapping tables")
			}
		}
		spans = append(spans, [2]uint64{offset, offset + length})
		if string(source[record:record+4]) == "head" {
			if headRecord >= 0 || length < 54 {
				return nil, fmt.Errorf("embedded font has invalid head table")
			}
			headRecord, headOffset, headLength = record, int(offset), int(length)
		}
	}
	if headRecord < 0 {
		return nil, fmt.Errorf("embedded font has no head table")
	}
	font := append([]byte(nil), source...)
	// Use the original font creation time, not wall-clock generation time.
	copy(font[headOffset+28:headOffset+36], font[headOffset+20:headOffset+28])
	binary.BigEndian.PutUint32(font[headOffset+8:headOffset+12], 0)
	binary.BigEndian.PutUint32(font[headRecord+4:headRecord+8], fontChecksum(font[headOffset:headOffset+headLength]))
	binary.BigEndian.PutUint32(font[headOffset+8:headOffset+12], 0xB1B0AFBA-fontChecksum(font))
	return font, nil
}

func fontChecksum(data []byte) uint32 {
	var checksum uint32
	for len(data) >= 4 {
		checksum += binary.BigEndian.Uint32(data[:4])
		data = data[4:]
	}
	var tail [4]byte
	copy(tail[:], data)
	return checksum + binary.BigEndian.Uint32(tail[:])
}
