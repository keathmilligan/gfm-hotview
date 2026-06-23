// Package config resolves effective settings from built-in defaults, an
// optional user-level config file (~/.config/gfm-hotview), an optional
// per-project .gfm-hotview config file, and command-line flags (in that
// precedence order, later overriding earlier).
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

// AppName is the app-specific subdirectory under the user config directory
// (e.g. ~/.config/gfm-hotview on Linux, %APPDATA%/gfm-hotview on Windows).
const AppName = "gfm-hotview"

const exampleConfigTOML = `# gfm-hotview configuration
# Uncomment settings below to customize.
# [server]
# host = "localhost"
# port = 6419
# ignore = [".git", ".hg", ".svn", "node_modules", "vendor", "bower_components", "dist", "build", "target", "out", "__pycache__", ".venv*", "venv", ".pytest_cache", ".cargo", ".cache", "coverage", ".nyc_output", ".DS_Store", ".gfm-hotview"]
`

// Defaults.
const (
	DefaultPort = 6419
	DefaultHost = "localhost"
)

// DefaultShow are the default glob patterns of files shown in the tree.
var DefaultShow = []string{"*.md", "*.markdown"}

// DefaultIgnore are always-excluded directory names/patterns.
var DefaultIgnore = []string{
	// Version control
	".git", ".hg", ".svn",
	// Package managers
	"node_modules", "vendor", "bower_components",
	// Build output
	"dist", "build", "target", "out",
	// Python
	"__pycache__", ".venv*", "venv", ".pytest_cache",
	// Rust
	".cargo",
	// Cache
	".cache",
	// Coverage
	"coverage", ".nyc_output",
	// OS
	".DS_Store",
	// Self
	DirName,
}

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
	Root     string   // absolute path to the primary served directory (Roots[0])
	Roots    []string // absolute paths to all served root directories
	Host     string
	Port     int
	NoOpen   bool
	NoReload bool
	Theme    Theme
	Mode     Mode
	Show     []string
	Ignore   []string
	OpenPage string // relative path; empty => auto-detect README/index
	Quiet    bool
	Verbose  bool
	Debug    bool

	// CSSDirs holds the absolute paths to user and project CSS override
	// directories (in order: user first, project last for cascade precedence).
	CSSDirs []string
}

// fileConfig is the on-disk schema. Only a forward-compatible subset is honored
// in v1; unknown keys are ignored (optionally warned in verbose mode).
type fileConfig struct {
	Server struct {
		Host *string `toml:"host" yaml:"host" json:"host"`
		Port *int    `toml:"port" yaml:"port" json:"port"`
	} `toml:"server" yaml:"server" json:"server"`
	Ignore *[]string `toml:"ignore" yaml:"ignore" json:"ignore"`
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
	Ignore   *string
	OpenPage *string
	Quiet    *bool
	Verbose  *bool
	Debug    *bool

	ConfigPath string // explicit --config path ("" => auto-detect)
	NoConfig   bool   // --no-config: ignore config file and .gfm-hotview overrides
}

// Default returns a Config populated with built-in defaults for the given root.
func Default(root string) *Config {
	return &Config{
		Root:     root,
		Roots:    []string{root},
		Host:     DefaultHost,
		Port:     DefaultPort,
		Theme:    ThemeAuto,
		Mode:     ModeGFM,
		Show:     append([]string(nil), DefaultShow...),
		Ignore:   append([]string(nil), DefaultIgnore...),
		OpenPage: "",
	}
}

// UserConfigDir returns the app-specific subdirectory inside the OS user config
// directory, or "" if it cannot be determined or does not exist.
func UserConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, AppName)
}

// ensureConfigDir creates dir (and css/ subdirectory) if they do not exist.
// If no config file is present, an example config.toml is written with all
// settings commented out.
func ensureConfigDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	hasConfig := false
	for _, name := range []string{"config.toml", "config.yaml", "config.yml", "config.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			hasConfig = true
			break
		}
	}
	if !hasConfig {
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(exampleConfigTOML), 0o644); err != nil {
			return err
		}
	}
	cssDir := filepath.Join(dir, "css")
	return os.MkdirAll(cssDir, 0o755)
}

// Resolve builds the effective configuration following the precedence:
// defaults -> user config -> project config -> flags.
// roots must contain at least one absolute directory; roots[0] is the primary
// root used for project config-file discovery. CSS overrides are discovered
// from all roots.
func Resolve(roots []string, f Flags) (*Config, error) {
	if len(roots) == 0 {
		return nil, fmt.Errorf("no root directories specified")
	}
	primary := roots[0]
	cfg := Default(primary)
	cfg.Roots = append([]string(nil), roots...)

	if !f.NoConfig {
		if f.ConfigPath == "" {
			// 1. User-level config (~/.config/gfm-hotview).
			if ud := UserConfigDir(); ud != "" {
				if err := ensureConfigDir(ud); err != nil {
					return nil, err
				}
				cfg.CSSDirs = append(cfg.CSSDirs, filepath.Join(ud, "css"))

				fc, _, err := loadFile(ud, "")
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
					if fc.Ignore != nil {
						cfg.Ignore = *fc.Ignore
					}
				}
			}

			// 2. Project-level config (<primary>/.gfm-hotview) overrides user.
			projectDir := filepath.Join(primary, DirName)
			fc, _, err := loadFile(projectDir, "")
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
				if fc.Ignore != nil {
					cfg.Ignore = *fc.Ignore
				}
			}

			// Project CSS overrides from all roots.
			for _, r := range roots {
				cssDir := filepath.Join(r, DirName, "css")
				if st, err := os.Stat(cssDir); err == nil && st.IsDir() {
					cfg.CSSDirs = append(cfg.CSSDirs, cssDir)
				}
			}
		} else {
			// Explicit --config path overrides user + project auto-discovery.
			fc, _, err := loadFile(primary, f.ConfigPath)
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
				if fc.Ignore != nil {
					cfg.Ignore = *fc.Ignore
				}
			}

			// Still discover CSS overrides from all roots.
			for _, r := range roots {
				cssDir := filepath.Join(r, DirName, "css")
				if st, err := os.Stat(cssDir); err == nil && st.IsDir() {
					cfg.CSSDirs = append(cfg.CSSDirs, cssDir)
				}
			}
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
	if f.Ignore != nil {
		cfg.Ignore = splitCSV(*f.Ignore)
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
	if f.Debug != nil {
		cfg.Debug = *f.Debug
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

// loadFile finds and parses a config file in dir. If explicit is set, it is
// treated as an override path. Returns (nil, "", nil) when no config file is
// present. An explicit path that does not exist is an error.
func loadFile(dir, explicit string) (*fileConfig, string, error) {
	var path string
	if explicit != "" {
		if filepath.IsAbs(explicit) {
			path = explicit
		} else {
			path = filepath.Join(dir, explicit)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, "", fmt.Errorf("config file %q: %w", explicit, err)
		}
	} else {
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
