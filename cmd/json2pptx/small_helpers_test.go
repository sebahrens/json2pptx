package main

import (
	"testing"
)

func TestIsColorKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"color", true},
		{"text_color", true},
		{"backgroundColor", true},
		{"fill", true},
		{"shape_fill", true},
		{"primary_bg", true},
		{"label_fg", true},
		{"size", false},
		{"text", false},
		{"width", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isColorKey(tc.key); got != tc.want {
			t.Errorf("isColorKey(%q) = %v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestIsBodyNMarker(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"body_2", true},
		{"body_3", true},
		{"body_10", true},
		{"body", false},
		{"body_", false},
		{"body_x", false},
		{"title", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isBodyNMarker(tc.id); got != tc.want {
			t.Errorf("isBodyNMarker(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestIsSlotMarker(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"slot1", true},
		{"slot2", true},
		{"slot42", true},
		{"slot", false},
		{"slot_", false},
		{"slotx", false},
		{"body_1", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isSlotMarker(tc.id); got != tc.want {
			t.Errorf("isSlotMarker(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestComposeChromeLine(t *testing.T) {
	cases := []struct {
		name string
		in   *ChromeInput
		want string
	}{
		{"empty", &ChromeInput{}, ""},
		{"only client", &ChromeInput{ClientName: "Acme Corp"}, "Acme Corp"},
		{"client + date", &ChromeInput{ClientName: "Acme Corp", FooterDate: "May 2026"}, "Acme Corp | May 2026"},
		{"with confidentiality", &ChromeInput{
			Confidentiality: "Strictly confidential",
			ClientName:      "Acme Corp",
			FooterDate:      "May 2026",
		}, "Strictly confidential — Acme Corp | May 2026"},
		{"only confidentiality", &ChromeInput{Confidentiality: "Internal"}, "Internal"},
		{"project prefix", &ChromeInput{ProjectCode: "Aurora", ClientName: "Acme"}, "Project Aurora | Acme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := composeChromeLine(tc.in); got != tc.want {
				t.Errorf("composeChromeLine = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChromeToFooterConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := chromeToFooterConfig(&ChromeInput{ClientName: "Acme"}, 12)
		if !cfg.Enabled {
			t.Error("expected Enabled=true")
		}
		if cfg.LeftText != "Acme" {
			t.Errorf("LeftText = %q, want %q", cfg.LeftText, "Acme")
		}
		if cfg.TotalSlides != 12 {
			t.Errorf("TotalSlides = %d, want 12", cfg.TotalSlides)
		}
	})

	t.Run("explicit format", func(t *testing.T) {
		cfg := chromeToFooterConfig(&ChromeInput{
			ClientName:  "Acme",
			PageNumbers: &PageNumbersInput{Format: "{current} / {total}"},
		}, 5)
		if cfg.PageNumberFormat != "{current} / {total}" {
			t.Errorf("PageNumberFormat = %q", cfg.PageNumberFormat)
		}
	})

	t.Run("disabled clears format", func(t *testing.T) {
		f := false
		cfg := chromeToFooterConfig(&ChromeInput{
			PageNumbers: &PageNumbersInput{Enabled: &f, Format: "{current}"},
		}, 5)
		if cfg.PageNumberFormat != "" {
			t.Errorf("expected empty format when disabled, got %q", cfg.PageNumberFormat)
		}
	})
}
