package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("XRAY_API_ADDR", "")
	t.Setenv("WEB_LISTEN", "")

	cfg := Load()
	if cfg.XrayAPIAddr != defaultXrayAPIAddr {
		t.Errorf("XrayAPIAddr = %q, want default %q", cfg.XrayAPIAddr, defaultXrayAPIAddr)
	}
	if cfg.ListenAddr != defaultWebListen {
		t.Errorf("ListenAddr = %q, want default %q", cfg.ListenAddr, defaultWebListen)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("XRAY_API_ADDR", "1.2.3.4:9090")
	t.Setenv("WEB_LISTEN", ":9999")

	cfg := Load()
	if cfg.XrayAPIAddr != "1.2.3.4:9090" {
		t.Errorf("XrayAPIAddr = %q, want %q", cfg.XrayAPIAddr, "1.2.3.4:9090")
	}
	if cfg.ListenAddr != ":9999" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9999")
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("SOME_KEY", "")
	if got := envOr("SOME_KEY", "fb"); got != "fb" {
		t.Errorf("envOr empty = %q, want fallback", got)
	}
	t.Setenv("SOME_KEY", "val")
	if got := envOr("SOME_KEY", "fb"); got != "val" {
		t.Errorf("envOr set = %q, want %q", got, "val")
	}
}
