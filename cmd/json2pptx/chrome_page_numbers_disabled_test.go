package main

import "testing"

// go-slide-creator-1iy0x: page_numbers.enabled:false must suppress the slide
// number. It used to set PageNumberFormat = "", which the footer renderer reads
// as "plain slide number", so the switch was a no-op.
func TestChromeToFooterConfig_PageNumbersDisabled(t *testing.T) {
	off := false
	cfg := chromeToFooterConfig(&ChromeInput{ClientName: "Acme", PageNumbers: &PageNumbersInput{Enabled: &off, Format: "{current} / {total}"}}, 5, nil)
	if !cfg.HidePageNumber {
		t.Fatalf("page_numbers.enabled:false did not hide the page number: %+v", cfg)
	}
	if cfg.LeftText != "Acme" {
		t.Errorf("disabling page numbers must keep the footer line, got %q", cfg.LeftText)
	}
	on := chromeToFooterConfig(&ChromeInput{ClientName: "Acme"}, 5, nil)
	if on.HidePageNumber {
		t.Error("chrome without page_numbers must keep the default slide number")
	}
}
