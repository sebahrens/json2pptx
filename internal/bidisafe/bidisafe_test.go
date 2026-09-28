package bidisafe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

var rlo = string(rune(0x202E))

func TestNeutralizeRawAndEscapedControls(t *testing.T) {
	in := `{"title":"` + rlo + `Q3","n":12345678901234567890,"s":["ok","\u2067x\ufeff"],"k/~":"\u202Ay","esc":"\\u202e literal"}`
	out, sites := Neutralize([]byte(in))
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if got["title"] != "Q3" || got["k/~"] != "y" {
		t.Errorf("controls not stripped: %v", got)
	}
	if s := got["s"].([]any); s[1] != "x" {
		t.Errorf("array string not stripped: %q", s[1])
	}
	// An escaped backslash followed by "u202e" is literal text, not a control.
	if got["esc"] != `\u202e literal` {
		t.Errorf("literal backslash-u text altered: %q", got["esc"])
	}
	// Big integers survive verbatim (UseNumber).
	if !strings.Contains(string(out), "12345678901234567890") {
		t.Errorf("number precision lost: %s", out)
	}
	want := []Site{{Path: "/k~1~0", Removed: 1}, {Path: "/s/1", Removed: 2}, {Path: "/title", Removed: 1}}
	if !reflect.DeepEqual(sites, want) {
		t.Errorf("sites = %+v, want %+v", sites, want)
	}
}

func TestNeutralizeCleanInputIsUntouched(t *testing.T) {
	in := []byte(`{"title":"Plain text – with an en dash and ` + string(rune(0x200F)) + ` RLM","x":[1,2]}`)
	out, sites := Neutralize(in)
	if sites != nil || string(out) != string(in) {
		t.Errorf("clean input changed: sites=%v out=%s", sites, out)
	}
}

func TestNeutralizeLeadingBOMAndMalformed(t *testing.T) {
	out, sites := Neutralize([]byte("\xEF\xBB\xBF{\"a\":1}"))
	if string(out) != `{"a":1}` || sites != nil {
		t.Errorf("leading BOM: out=%s sites=%v", out, sites)
	}
	bad := []byte(`{"a":"` + rlo + `"`)
	if out, _ := Neutralize(bad); string(out) != string(bad) {
		t.Errorf("malformed JSON must pass through unchanged for the caller's decoder to report")
	}
}

func TestStripString(t *testing.T) {
	s, n := StripString("a" + rlo + "b" + string(rune(0x2069)) + string(rune(0xFEFF)))
	if s != "ab" || n != 3 {
		t.Errorf("StripString = %q,%d", s, n)
	}
	if s, n := StripString("plain"); s != "plain" || n != 0 {
		t.Errorf("plain changed: %q %d", s, n)
	}
}
