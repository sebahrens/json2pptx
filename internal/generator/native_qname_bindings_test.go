package generator

import (
	"bytes"
	"strings"
	"testing"
)

func TestLocalizeNativeQNameBindings(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"ordinary", `<root><child lang="de-DE"/></root>`, `<root><child lang="de-DE"/></root>`},
		{"empty", `<root xmlns:q="urn:choice"><child mode="q:value"/></root>`, `<root xmlns:q="urn:choice"><child mode="q:value" xmlns:q="urn:choice"/></root>`},
		{"paired", `<root xmlns:q="urn:choice"><child mode='q:value'>text</child></root>`, `<root xmlns:q="urn:choice"><child mode='q:value' xmlns:q="urn:choice">text</child></root>`},
		{"override", `<root xmlns:q="urn:outer"><branch xmlns:q="urn:inner"><child mode="q:value"/></branch><child mode="q:value"/></root>`, `<root xmlns:q="urn:outer"><branch xmlns:q="urn:inner"><child mode="q:value" xmlns:q="urn:inner"/></branch><child mode="q:value" xmlns:q="urn:outer"/></root>`},
		{"local", `<root xmlns:q="urn:outer"><child mode="q:value" xmlns:q="urn:inner"/></root>`, `<root xmlns:q="urn:outer"><child mode="q:value" xmlns:q="urn:inner"/></root>`},
		{"deduplicate", `<root xmlns:q="urn:choice"><child modes="q:first q:second"/></root>`, `<root xmlns:q="urn:choice"><child modes="q:first q:second" xmlns:q="urn:choice"/></root>`},
		{"uri", `<root xmlns:q="urn:choice"><child href="q://path" unknown="u:value"/></root>`, `<root xmlns:q="urn:choice"><child href="q://path" unknown="u:value"/></root>`},
		{"escaped", `<root xmlns:q="urn:choice?a=1&amp;b=2"><child mode="q:value"/></root>`, `<root xmlns:q="urn:choice?a=1&amp;b=2"><child mode="q:value" xmlns:q="urn:choice?a=1&amp;b=2"/></root>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := localizeNativeQNameBindings([]byte(tc.source))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s; want %s", got, tc.want)
			}
			again, err := localizeNativeQNameBindings(got)
			if err != nil || !bytes.Equal(got, again) {
				t.Fatalf("not idempotent: %s (%v)", again, err)
			}
		})
	}
}

func TestLocalizeNativeQNameBindingsRejectsMalformedXML(t *testing.T) {
	for _, source := range []string{`<root><child></root>`, `<root mode="q:value"`, `<root>&invalid;</root>`} {
		if _, err := localizeNativeQNameBindings([]byte(source)); err == nil || !strings.Contains(err.Error(), "native QName scope") {
			t.Errorf("malformed XML accepted or uncontextualized: %q (%v)", source, err)
		}
	}
}
