package svggen

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

func fontMetadataFixture() []byte {
	font := make([]byte, 104)
	copy(font[:4], []byte{0, 1, 0, 0})
	binary.BigEndian.PutUint16(font[4:6], 2)
	copy(font[12:16], "head")
	binary.BigEndian.PutUint32(font[20:24], 44)
	binary.BigEndian.PutUint32(font[24:28], 54)
	copy(font[28:32], "glyf")
	binary.BigEndian.PutUint32(font[36:40], 100)
	binary.BigEndian.PutUint32(font[40:44], 4)
	binary.BigEndian.PutUint64(font[64:72], 123)
	binary.BigEndian.PutUint64(font[72:80], 456)
	copy(font[100:], []byte{10, 20, 30, 40})
	return font
}

func TestStableFontMetadataPreservesPayloadAndChecksums(t *testing.T) {
	source := fontMetadataFixture()
	original := append([]byte(nil), source...)
	got, err := stableFontMetadata(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, original) {
		t.Fatal("mutated source font")
	}
	if binary.BigEndian.Uint64(got[72:80]) != 123 {
		t.Fatal("modified time not stable creation time")
	}
	if fontChecksum(got) != 0xB1B0AFBA {
		t.Fatal("invalid whole-font checksum")
	}
	head := append([]byte(nil), got[44:98]...)
	binary.BigEndian.PutUint32(head[8:12], 0)
	if binary.BigEndian.Uint32(got[16:20]) != fontChecksum(head) {
		t.Fatal("invalid head table checksum")
	}
	normalized := append([]byte(nil), got...)
	for _, span := range [][2]int{{16, 20}, {52, 56}, {72, 80}} {
		copy(normalized[span[0]:span[1]], original[span[0]:span[1]])
	}
	if !bytes.Equal(normalized, original) {
		t.Fatal("changed font data outside timestamps and checksums")
	}
	again, err := stableFontMetadata(got)
	if err != nil || !bytes.Equal(again, got) {
		t.Fatal("normalization not idempotent")
	}
}

func TestStableFontMetadataRejectsMalformedInput(t *testing.T) {
	for _, name := range []string{"short", "signature", "directory", "bounds", "short head", "missing head", "duplicate head", "overlap", "directory overlap"} {
		t.Run(name, func(t *testing.T) {
			font := fontMetadataFixture()
			switch name {
			case "short":
				font = font[:8]
			case "signature":
				copy(font[:4], "NOPE")
			case "directory":
				binary.BigEndian.PutUint16(font[4:6], 200)
			case "bounds":
				binary.BigEndian.PutUint32(font[20:24], 1000)
			case "short head":
				binary.BigEndian.PutUint32(font[24:28], 12)
			case "missing head":
				copy(font[12:16], "xxxx")
			case "duplicate head":
				copy(font[28:32], "head")
			case "overlap":
				binary.BigEndian.PutUint32(font[36:40], 48)
			case "directory overlap":
				binary.BigEndian.PutUint32(font[20:24], 12)
			}
			if _, err := stableFontMetadata(font); err == nil {
				t.Fatal("accepted malformed font")
			}
		})
	}
}

func TestFontRulesPreserveDuplicateCascadeAndDrawing(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString(fontMetadataFixture())
	rule := func(family, program string) string {
		return "\n@font-face{font-family:'" + family + "';src:url('data:font/ttf;base64," + program + "');}"
	}
	alternate := fontMetadataFixture()
	alternate[100] = 99
	input := "<svg><text>Keep me</text><style>" + rule("Z", payload) + rule("A", payload) + rule("Z", base64.StdEncoding.EncodeToString(alternate)) + "\n</style></svg>"
	got, err := normalizeEmittedFontDeterminism([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	rules := emittedFontRules.Find(got)
	lines := bytes.Split(rules[1:], []byte("\n"))
	if len(lines) != 3 || !strings.Contains(string(lines[0]), "family:'A'") {
		t.Fatal("distinct font rules not sorted")
	}
	for index, want := range []byte{10, 99} {
		match := emittedFontURI.FindSubmatch(lines[index+1])
		font, err := base64.StdEncoding.DecodeString(string(match[1]))
		if err != nil || font[100] != want {
			t.Fatal("duplicate font cascade reordered")
		}
	}
	if !bytes.HasPrefix(got, []byte("<svg><text>Keep me</text><style>")) || !bytes.HasSuffix(got, []byte("\n</style></svg>")) {
		t.Fatal("changed drawing bytes")
	}
}

func TestFontRuleEquivalentDefaultDescriptorsRetainOrder(t *testing.T) {
	implicit := []byte("@font-face{font-family:'Aptos';src:url('FIRST');}")
	explicit := []byte("@font-face{font-family:'aptos';font-weight:400;font-style:normal;src:url('SECOND');}")
	if !bytes.Equal(fontRuleDescriptor(implicit), fontRuleDescriptor(explicit)) {
		t.Fatal("equivalent CSS defaults not recognized")
	}
	input := append(append(append([]byte("<style>\n"), implicit...), []byte("\n")...), explicit...)
	input = append(input, []byte("\n</style>")...)
	got, err := normalizeEmittedFontDeterminism(input)
	if err != nil || !bytes.Equal(got, input) {
		t.Fatal("equivalent font descriptors changed cascade order")
	}
}

func TestFontRuleDescriptorPreservesQuotedFamilyContent(t *testing.T) {
	named := []byte("@font-face{font-family:'Aptos;font-weight:400';src:url('FIRST');}")
	plain := []byte("@font-face{font-family:'Aptos';src:url('SECOND');}")
	if bytes.Equal(fontRuleDescriptor(named), fontRuleDescriptor(plain)) {
		t.Fatal("font-family text was mistaken for a CSS descriptor")
	}
}

func TestNormalizeEmittedFontsFailureDoesNotPublishPartialSVG(t *testing.T) {
	for _, payload := range []string{"AAAA=", base64.StdEncoding.EncodeToString([]byte("invalid font"))} {
		source := []byte("<svg><style>@font-face{src:url('data:font/ttf;base64," + payload + "');}</style></svg>")
		original := append([]byte(nil), source...)
		got, err := normalizeEmittedFontDeterminism(source)
		if err == nil || got != nil {
			t.Fatal("invalid embedded font returned publishable SVG")
		}
		if !bytes.Equal(source, original) {
			t.Fatal("error path mutated source")
		}
	}
	source := []byte("<svg><text>Unchanged plain drawing</text></svg>")
	got, err := normalizeEmittedFontDeterminism(source)
	if err != nil || !bytes.Equal(got, source) {
		t.Fatal("font-free drawing changed")
	}
}
