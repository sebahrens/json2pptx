package quality

import (
	"encoding/xml"
	"os"
	"testing"
)

func TestAptosFallbackRetainsOriginalFontPriority(t *testing.T) {
	data, err := os.ReadFile("fixtures/fontconfig-aptos-fallback.conf")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Aliases []struct {
			Family string   `xml:"family"`
			Accept []string `xml:"accept>family"`
			Prefer []string `xml:"prefer>family"`
		} `xml:"alias"`
		Dirs     []string `xml:"dir"`
		CacheDir []string `xml:"cachedir"`
	}
	if err := xml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Aliases) != 2 || len(config.Dirs) != 0 || len(config.CacheDir) != 0 {
		t.Fatalf("fallback fixture must contain only two aliases, no host paths: %+v", config)
	}
	want := map[string]bool{"Aptos": true, "Aptos Light": true}
	for _, alias := range config.Aliases {
		if !want[alias.Family] {
			t.Fatalf("unexpected or duplicate original family %q", alias.Family)
		}
		delete(want, alias.Family)
		if len(alias.Prefer) != 0 || len(alias.Accept) != 1 || alias.Accept[0] != "Arial" {
			t.Fatalf("Arial must follow, not replace, installed %s: %+v", alias.Family, alias)
		}
	}
}
