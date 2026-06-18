# gfm-hotview

A small, dependency-free, **offline** GitHub-Flavored Markdown (GFM) viewer.
Point it at a directory and it serves a GitHub-style multi-panel UI — a file
tree on the left, the rendered document on the right — with live reload and
optional browser auto-open. Ships as a single static binary for Windows,
Linux, and macOS.

See [`docs/spec.md`](docs/spec.md) for the full specification.

## Features

- Faithful GFM rendering, fully offline (no network, no CDN).
  - Tables, task lists, footnotes, strikethrough, autolinks
  - GitHub-style alerts (`> [!NOTE]`, `[!TIP]`, `[!IMPORTANT]`, `[!WARNING]`, `[!CAUTION]`)
  - Heading anchors, emoji shortcodes (`:rocket:`)
  - Syntax highlighting via embedded Chroma (light/dark)
  - Math (`$…$`, `$$…$$`) and Mermaid blocks rendered client-side when the
    optional vendor bundles are present (see below); otherwise shown as source
- Multi-panel UI: file-tree sidebar with fuzzy filter, rendered pane, TOC with
  scroll-spy, light/dark/auto themes
- Live reload over Server-Sent Events (file watching with debounce)
- Auto-opens your default browser; auto-selects a free port
- Path-contained: never serves files outside the served root
- Single static binary, no runtime dependencies

## Install / Build

Requires Go 1.22+.

```sh
go build -o gfm-hotview .
```

Cross-compile (fully static, `CGO_ENABLED=0`):

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o gfm-hotview.exe .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o gfm-hotview-macos .
```

## Usage

```sh
gfm-hotview [options] [path]
```

`path` defaults to the current directory. If you pass a file, its directory
becomes the root and the file opens first.

### Options

| Flag | Default | Description |
| --- | --- | --- |
| `-p, --port` | `6419` | Port to bind (auto-selects next free if taken; `0` = OS-chosen) |
| `-H, --host` | `localhost` | Host/interface to bind |
| `--no-open` | off | Do not auto-open the browser |
| `--no-reload` | off | Disable live reload |
| `-t, --theme` | `auto` | `auto` \| `light` \| `dark` |
| `--mode` | `gfm` | `gfm` \| `markdown` (plain CommonMark) |
| `--show` | `*.md,*.markdown` | Comma-separated globs shown in the tree |
| `--hidden` | off | Include dotfiles/dot-directories |
| `--ignore` | — | Additional comma-separated ignore globs |
| `--open-page` | README-detect | Initial document (relative to root) |
| `-c, --config` | auto | Path to config file (default `.gfm-hotview/config.*`) |
| `--no-config` | off | Ignore config file and `.gfm-hotview` overrides |
| `-q, --quiet` | off | Suppress non-error logs |
| `-v, --verbose` | off | Verbose logging |
| `--version` | | Print version |

## Configuration & theming (`.gfm-hotview`)

Create an optional `.gfm-hotview` directory at the root you serve:

```
<root>/.gfm-hotview/
  config.toml      # optional: override host/port (and future settings)
  css/             # optional: CSS overrides applied after built-in styles
    theme.css
```

`config.toml` (TOML primary; YAML/JSON also accepted):

```toml
[server]
host = "127.0.0.1"
port = 8080
```

Precedence: built-in defaults → config file → command-line flags. Any CSS files
in `.gfm-hotview/css` are concatenated and served at `/user.css`, linked after the
built-in stylesheets so they win by cascade. Both are ignored with
`--no-config`. The `.gfm-hotview` directory is never shown in the tree or served
as raw content.

## Optional offline Math/Mermaid bundles

To render Math and Mermaid in the browser fully offline, drop the minified
bundles into `web/assets/vendor/` before building (they are then embedded):

```
web/assets/vendor/katex.min.css
web/assets/vendor/katex.min.js
web/assets/vendor/auto-render.min.js
web/assets/vendor/mermaid.min.js
```

If absent, math is emitted as `\(…\)`/`\[…\]` text and Mermaid as a code block
(`<pre class="mermaid">`); everything else still works.

## Development

```sh
go test ./...      # unit tests
go vet ./...
gofmt -l .
```
