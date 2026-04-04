package generator

import (
	"strings"
	"testing"
)

func TestGenerateNotesSlideXML(t *testing.T) {
	tests := []struct {
		name       string
		notesText  string
		wantParts  []string // Substrings that must appear in the XML
		wantAbsent []string // Substrings that must NOT appear
	}{
		{
			name:      "simple notes text",
			notesText: "Remember to talk about Q4 results",
			wantParts: []string{
				`<p:notes`,
				`<p:ph type="sldImg"/>`,
				`<p:ph type="body" idx="1"/>`,
				`Remember to talk about Q4 results`,
				`<a:rPr lang="en-US"`,
			},
		},
		{
			name:      "multi-line notes",
			notesText: "First point\nSecond point\nThird point",
			wantParts: []string{
				"First point",
				"Second point",
				"Third point",
			},
		},
		{
			name:      "notes with XML special characters",
			notesText: "Use <b> tags & \"quotes\" in 'test'",
			wantParts: []string{
				"&lt;b&gt;",
				"&amp;",
				"&quot;quotes&quot;",
				"&apos;test&apos;",
			},
			wantAbsent: []string{
				"<b>", // Must be escaped
			},
		},
		{
			name:      "empty notes",
			notesText: "",
			wantParts: []string{
				`<a:endParaRPr lang="en-US"/>`,
			},
		},
		{
			name:      "notes with illegal XML control characters",
			notesText: "before\x01\x08\x0B\x0C\x0E\x1Fafter",
			wantParts: []string{
				"beforeafter",
			},
			wantAbsent: []string{
				"\x01", "\x08", "\x0B", "\x0C", "\x0E", "\x1F",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateNotesSlideXML(tt.notesText)
			xmlStr := string(result)

			for _, part := range tt.wantParts {
				if !strings.Contains(xmlStr, part) {
					t.Errorf("missing expected substring %q in:\n%s", part, xmlStr)
				}
			}

			for _, absent := range tt.wantAbsent {
				if strings.Contains(xmlStr, absent) {
					t.Errorf("found unwanted substring %q in:\n%s", absent, xmlStr)
				}
			}
		})
	}
}

func TestGenerateNotesSlideRels(t *testing.T) {
	tests := []struct {
		name     string
		slideNum int
		wantRel  string
	}{
		{
			name:     "slide 1 notes rels",
			slideNum: 1,
			wantRel:  "../slides/slide1.xml",
		},
		{
			name:     "slide 5 notes rels",
			slideNum: 5,
			wantRel:  "../slides/slide5.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := generateNotesSlideRels(tt.slideNum)
			if err != nil {
				t.Fatalf("generateNotesSlideRels() error = %v", err)
			}

			xmlStr := string(data)
			if !strings.Contains(xmlStr, tt.wantRel) {
				t.Errorf("missing expected target %q in:\n%s", tt.wantRel, xmlStr)
			}
			if !strings.Contains(xmlStr, "notesSlide") {
				// Verify the relationship type mentions notesSlide (via the slide rel type)
				if !strings.Contains(xmlStr, "relationships/slide") {
					t.Errorf("missing slide relationship type in:\n%s", xmlStr)
				}
			}
		})
	}
}

func TestXmlEscapeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no escaping needed", input: "hello world", want: "hello world"},
		{name: "ampersand", input: "A & B", want: "A &amp; B"},
		{name: "less than", input: "a < b", want: "a &lt; b"},
		{name: "greater than", input: "a > b", want: "a &gt; b"},
		{name: "double quote", input: `say "hello"`, want: "say &quot;hello&quot;"},
		{name: "single quote", input: "it's", want: "it&apos;s"},
		{name: "multiple escapes", input: `<a href="x">&</a>`, want: "&lt;a href=&quot;x&quot;&gt;&amp;&lt;/a&gt;"},
		{name: "strips null byte", input: "hello\x00world", want: "helloworld"},
		{name: "strips control chars", input: "a\x01b\x08c\x0Bd\x0Ce\x0Ef\x1Fg", want: "abcdefg"},
		{name: "preserves tab", input: "a\tb", want: "a\tb"},
		{name: "preserves newline", input: "a\nb", want: "a\nb"},
		{name: "preserves carriage return", input: "a\rb", want: "a\rb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xmlEscapeString(tt.input)
			if got != tt.want {
				t.Errorf("xmlEscapeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNotesSlideXMLPaths(t *testing.T) {
	if got := NotesSlideXMLPath(1); got != "ppt/notesSlides/notesSlide1.xml" {
		t.Errorf("NotesSlideXMLPath(1) = %q", got)
	}
	if got := NotesSlideXMLPath(5); got != "ppt/notesSlides/notesSlide5.xml" {
		t.Errorf("NotesSlideXMLPath(5) = %q", got)
	}
	if got := NotesSlideRelsPath(1); got != "ppt/notesSlides/_rels/notesSlide1.xml.rels" {
		t.Errorf("NotesSlideRelsPath(1) = %q", got)
	}
}
