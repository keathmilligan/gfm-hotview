// Package config resolves effective settings from built-in defaults, an
// optional .gfm-hotview config file, and command-line flags (in that precedence
// order, later overriding earlier).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// DirName is the per-project configuration/theming directory at the served root.
const DirName = ".gfm-hotview"

// Defaults.
const (
	DefaultPort = 6419
	DefaultHost = "localhost"
)

// DefaultShow are the default glob patterns of files shown in the tree.
var DefaultShow = []string{"*.md", "*.markdown"}

// DefaultIgnore are always-excluded names/patterns.
var DefaultIgnore = []string{".git", ".hg", ".svn", "node_modules", ".DS_Store", DirName}

// Theme controls color theme behavior.
type Theme string

const (
	ThemeAuto  Theme = "auto"
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

// Mode controls the Markdown dialect.
type Mode string

const (
	ModeGFM      Mode = "gfm"
	ModeMarkdown Mode = "markdown"
)

// Config holds the fully-resolved effective settings.
type Config struct {
	Root     string // absolute path to the served directory
	Host     string
	Port     int
	NoOpen   bool
	NoReload bool
	Theme    Theme
	Mode     Mode
	Show     []string
	Hidden   bool
	Ignore   []string
	OpenPage string // relative path; empty => auto-detect README/index
	Quiet    bool
	Verbose  bool

	// CSSDir is the absolute path to .gfm-hotview/css if it exists, else "".
	CSSDir string
}

// fileConfig is the on-disk schema. Only a forward-compatible subset is honored
// in v1; unknown keys are ignored (optionally warned in verbose mode).
type fileConfig struct {
	Server struct {
		Host *string `toml:"host" yaml:"host" json:"host"`
		Port *int    `toml:"port" yaml:"port" json:"port"`
	} `toml:"server" yaml:"server" json:"server"`
}

// Flags carries values parsed from the command line. A nil pointer means the
// flag was not explicitly set, so it must not override config-file values.
type Flags struct {
	Host     *string
	Port     *int
	NoOpen   *bool
	NoReload *bool
	Theme    *string
	Mode     *string
	Show     *string
	Hidden   *bool
	Ignore   *string
	OpenPage *string
	Quiet    *bool
	Verbose  *bool

	ConfigPath string // explicit --config path ("" => auto-detect)
	NoConfig   bool   // --no-config: ignore config file and .gfm-hotview overrides
}

// Default returns a Config populated with built-in defaults for the given root.
func Default(root string) *Config {
	return &Config{
		Root:     root,
		Host:     DefaultHost,
		Port:     DefaultPort,
		Theme:    ThemeAuto,
		Mode:     ModeGFM,
		Show:     append([]string(nil), DefaultShow...),
		Ignore:   append([]string(nil), DefaultIgnore...),
		OpenPage: "",
	}
}

// Resolve builds the effective configuration following the precedence in §3.5:
// defaults -> config file -> flags. root must already be an absolute directory.
func Resolve(root string, f Flags) (*Config, error) {
	cfg := Default(root)

	if !f.NoConfig {
		fc, path, err := loadFile(root, f.ConfigPath)
		if err != nil {
			return nil, err
		}
		if fc != nil {
			if fc.Server.Host != nil {
				cfg.Host = *fc.Server.Host
			}
			if fc.Server.Port != nil {
				cfg.Port = *fc.Server.Port
			}
		}
		_ = path // discovery path is available for logging by the caller if needed

		// Discover CSS overrides directory.
		cssDir := filepath.Join(root, DirName, "css")
		if st, err := os.Stat(cssDir); err == nil && st.IsDir() {
			cfg.CSSDir = cssDir
		}
	}

	// Flags override.
	if f.Host != nil {
		cfg.Host = *f.Host
	}
	if f.Port != nil {
		cfg.Port = *f.Port
	}
	if f.NoOpen != nil {
		cfg.NoOpen = *f.NoOpen
	}
	if f.NoReload != nil {
		cfg.NoReload = *f.NoReload
	}
	if f.Theme != nil {
		cfg.Theme = Theme(*f.Theme)
	}
	if f.Mode != nil {
		cfg.Mode = Mode(*f.Mode)
	}
	if f.Show != nil {
		cfg.Show = splitCSV(*f.Show)
	}
	if f.Hidden != nil {
		cfg.Hidden = *f.Hidden
	}
	if f.Ignore != nil {
		cfg.Ignore = append(append([]string(nil), DefaultIgnore...), splitCSV(*f.Ignore)...)
	}
	if f.OpenPage != nil {
		cfg.OpenPage = *f.OpenPage
	}
	if f.Quiet != nil {
		cfg.Quiet = *f.Quiet
	}
	if f.Verbose != nil {
		cfg.Verbose = *f.Verbose
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch c.Theme {
	case ThemeAuto, ThemeLight, ThemeDark:
	default:
		return fmt.Errorf("invalid theme %q (want auto|light|dark)", c.Theme)
	}
	switch c.Mode {
	case ModeGFM, ModeMarkdown:
	default:
		return fmt.Errorf("invalid mode %q (want gfm|markdown)", c.Mode)
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d (want 0-65535)", c.Port)
	}
	return nil
}

// loadFile finds and parses the config file. It returns (nil, "", nil) when no
// config file is present. An explicit path that does not exist is an error.
func loadFile(root, explicit string) (*fileConfig, string, error) {
	var path string
	if explicit != "" {
		if filepath.IsAbs(explicit) {
			path = explicit
		} else {
			path = filepath.Join(root, explicit)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, "", fmt.Errorf("config file %q: %w", explicit, err)
		}
	} else {
		dir := filepath.Join(root, DirName)
		for _, name := range []string{"config.toml", "config.yaml", "config.yml", "config.json"} {
			cand := filepath.Join(dir, name)
			if _, err := os.Stat(cand); err == nil {
				path = cand
				break
			}
		}
		if path == "" {
			return nil, "", nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading config %q: %w", path, err)
	}

	var fc fileConfig
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		if err := toml.Unmarshal(data, &fc); err != nil {
			return nil, "", fmt.Errorf("parsing TOML config %q: %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, "", fmt.Errorf("parsing YAML config %q: %w", path, err)
		}
	case ".json":
		// yaml.v3 parses JSON as a YAML superset.
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, "", fmt.Errorf("parsing JSON config %q: %w", path, err)
		}
	default:
		return nil, "", fmt.Errorf("unsupported config extension %q", ext)
	}
	return &fc, path, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ErrNoConfig is returned by callers when no config file is found (informational).
var ErrNoConfig = errors.New("no config file found")
