package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDefaults(t *testing.T) {
	root := t.TempDir()
	cfg, err := Resolve([]string{root}, Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != DefaultPort || cfg.Host != DefaultHost {
		t.Fatalf("got %s:%d, want %s:%d", cfg.Host, cfg.Port, DefaultHost, DefaultPort)
	}
	if cfg.Theme != ThemeAuto || cfg.Mode != ModeGFM {
		t.Fatalf("unexpected theme/mode: %v %v", cfg.Theme, cfg.Mode)
	}
}

func TestResolveConfigFileTOML(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[server]\nhost = \"127.0.0.1\"\nport = 8123\n"
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Resolve([]string{root}, Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 8123 {
		t.Fatalf("config not applied: got %s:%d", cfg.Host, cfg.Port)
	}
}

func TestFlagsOverrideConfig(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DirName)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[server]\nport = 8123\n"), 0o644)

	port := 9099
	cfg, err := Resolve([]string{root}, Flags{Port: &port})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9099 {
		t.Fatalf("flag did not override config: got %d", cfg.Port)
	}
}

func TestNoConfigIgnoresFileAndCSS(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DirName)
	_ = os.MkdirAll(filepath.Join(dir, "css"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[server]\nport = 8123\n"), 0o644)

	cfg, err := Resolve([]string{root}, Flags{NoConfig: true})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("--no-config should use default port, got %d", cfg.Port)
	}
	if len(cfg.CSSDirs) != 0 {
		t.Fatalf("--no-config should ignore css dir, got %v", cfg.CSSDirs)
	}
}

func TestInvalidConfigIsError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, DirName)
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[server\nport = bad"), 0o644)

	if _, err := Resolve([]string{root}, Flags{}); err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestInvalidThemeRejected(t *testing.T) {
	root := t.TempDir()
	bad := "purple"
	if _, err := Resolve([]string{root}, Flags{Theme: &bad}); err == nil {
		t.Fatal("expected error for invalid theme")
	}
}
