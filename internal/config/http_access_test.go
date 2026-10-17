package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPAccessConfiguration(t *testing.T) {
	if cfg := DefaultConfig(); cfg.Server.Bind != "127.0.0.1" || cfg.Server.APIToken != "" || len(cfg.Server.CORSAllowedOrigins) != 0 {
		t.Fatal("unsafe HTTP defaults")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  bind: 192.0.2.4\n  cors_allowed_origins: [https://configured.example]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JSON2PPTX_BIND", "::1")
	t.Setenv("JSON2PPTX_API_TOKEN", "secret")
	t.Setenv("JSON2PPTX_CORS_ALLOWED_ORIGINS", "https://agent.example, http://localhost:3000")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Bind != "::1" || cfg.Server.APIToken != "secret" || len(cfg.Server.CORSAllowedOrigins) != 2 {
		t.Fatalf("env override lost: %+v", cfg.Server)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://example.org/path", "javascript:evil", "https://user@example.org", "https://example.org?q=1"} {
		cfg.Server.CORSAllowedOrigins = []string{origin}
		if cfg.Validate() == nil {
			t.Errorf("invalid origin accepted: %s", origin)
		}
	}
}
