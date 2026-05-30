package pptx

// IsIllegalXMLChar reports whether r is illegal in XML 1.0 document content and
// therefore must be stripped before it is written into a raw OOXML fragment.
//
// The legal character set per the XML 1.0 spec is:
//
//	#x9 | #xA | #xD | [#x20-#xD7FF] | [#xE000-#xFFFD] | [#x10000-#x10FFFF]
//
// Everything else is illegal. The cases that legal JSON string input can
// realistically carry are the C0 control characters (U+0000-U+0008, U+000B,
// U+000C, U+000E-U+001F) and the noncharacters U+FFFE/U+FFFF; left unstripped
// they make Office reject the generated file as non-well-formed (repair dialog
// or open failure). Tab, newline, and carriage return remain legal.
//
// Centralizing this predicate keeps every raw-OOXML escaper (a:t text, table
// cell text, attribute values, notes) consistent; previously only the notes
// path stripped these characters.
func IsIllegalXMLChar(r rune) bool {
	switch {
	case r == '\t' || r == '\n' || r == '\r':
		return false
	case r >= 0x20 && r <= 0xD7FF:
		return false
	case r >= 0xE000 && r <= 0xFFFD:
		return false
	case r >= 0x10000 && r <= 0x10FFFF:
		return false
	default:
		return true
	}
}
