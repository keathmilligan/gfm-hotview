# gfm-hotview — Specification

## 1. Overview

`gfm-hotview` is a small, dependency-free local web server that renders and
serves a directory tree of GitHub-Flavored Markdown (GFM) documents with a
look and feel close to GitHub.com. It is intended as a modern replacement for
the Ruby-based [Madness](https://github.com/DannyBen/madness) server: same
core idea (point it at a directory, browse and read the Markdown inside a
browser) but with no language runtime to install, more faithful GFM rendering,
and a single self-contained binary that runs identically on Windows, Linux,
and macOS.

It draws inspiration from
[thiagokokada/gh-gfm-preview](https://github.com/thiagokokada/gh-gfm-preview)
for offline GFM rendering and live reload, but its primary mode is a
**multi-panel directory browser** (GitHub-style file tree on the left, rendered
document on the right) rather than single-file preview.

### 1.1 Goals

- Faithfully render GFM, matching GitHub.com appearance as closely as
  practical, fully offline.
- Provide a GitHub-like multi-panel UI: a file-tree sidebar plus a rendered
  document pane.
- Be trivial to run and deploy: a single static binary, zero configuration,
  zero external services, no internet access required at runtime.
- Run identically on Windows, Linux, and macOS (amd64 and arm64).
- Live-reload the rendered view when files on disk change.
- Optionally open the user's default browser automatically on startup.

### 1.2 Non-Goals

- Editing Markdown in the browser (read-only viewer).
- Authentication / multi-user accounts / access control beyond host binding.
- Features that require GitHub's backend (issue/PR references, @mentions,
  permalink-to-line, user avatars).
- A general-purpose static-site generator or build pipeline.
- Acting as a public-facing production web server.

## 2. Implementation Constraints

- **Language / runtime:** Go (1.22+). Chosen for static single-binary output,
  straightforward cross-compilation, and a mature offline GFM ecosystem.
- **Dependency-free at runtime:** The shipped artifact is one statically linked
  executable. No interpreter, no Node, no system libraries, no network calls.
- **Embedded assets:** All HTML templates, CSS, JavaScript, fonts, icons, and
  syntax-highlight styles are compiled into the binary via Go's `embed`
  package. Nothing is fetched from a CDN.
- **Build-time dependencies** (compiled in, allowed): a GFM Markdown library
  and syntax highlighter. Recommended:
  - [`yuin/goldmark`](https://github.com/yuin/goldmark) + GFM extension for
    CommonMark/GFM rendering.
  - [`alecthomas/chroma`](https://github.com/alecthomas/chroma) for offline
    syntax highlighting.
  - `goldmark` extensions / custom renderers for alerts, task lists, emoji,
    autolinks, footnotes, math, and Mermaid fenced blocks.
  These are vendored and statically linked; they do not add runtime
  dependencies.
- **Offline by contract:** The binary MUST function with no network. Any
  feature that would require remote resources MUST degrade gracefully and MUST
  be documented as such (see §5.3).

## 3. Command-Line Interface

### 3.1 Invocation

```
gfm-hotview [options] [path]
```

- `path` (optional, default `.`): the root directory to serve. If a file is
  given instead of a directory, its containing directory becomes the root and
  that file is opened as the initial document.
- Starting `gfm-hotview` with no arguments serves the current working directory.

### 3.2 Options

| Flag | Default | Description |
| --- | --- | --- |
| `-p, --port <int>` | `6419` | TCP port to bind. If in use, auto-select the next free port (see §3.3). |
| `-H, --host <string>` | `localhost` | Hostname/interface to bind. |
| `--no-open` | off | Do not auto-open the browser on startup. |
| `--no-reload` | off | Disable live reload (no file watching, no SSE/WebSocket). |
| `-t, --theme <auto\|light\|dark>` | `auto` | Force a color theme; `auto` follows the browser/OS preference. |
| `--mode <gfm\|markdown>` | `gfm` | `gfm` enables all GitHub extensions; `markdown` renders plain CommonMark. |
| `--show <globs>` | `*.md,*.markdown` | Comma-separated glob patterns of files shown in the tree. |
| `--hidden` | off | Include dotfiles/dot-directories in the tree. |
| `--ignore <globs>` | (see §4.3) | Additional glob patterns to exclude from the tree. |
| `--open-page <path>` | `README`-detect | Document to render first (relative to root). Defaults to a detected index file. |
| `-c, --config <path>` | auto-detect | Path to a config file. Defaults to `.gfm-hotview/config.*` under the root (see §11). |
| `--no-config` | off | Ignore any config file and `.gfm-hotview` directory (use built-in defaults + flags only). |
| `-q, --quiet` | off | Suppress non-error log output. |
| `-v, --verbose` | off | Verbose request/watch logging. |
| `--version` | | Print version and exit. |
| `-h, --help` | | Print usage and exit. |

### 3.3 Port selection

If the requested port is unavailable, the server probes successive ports
(`port`, `port+1`, …) until it finds a free one, logs the chosen port, and
uses it. `--port 0` lets the OS pick an ephemeral port.

### 3.4 Browser auto-open

On startup (unless `--no-open`), open the served URL in the user's default
browser using the OS-native mechanism:

- Windows: `rundll32 url.dll,FileProtocolHandler <url>` (or `cmd /c start`).
- macOS: `open <url>`.
- Linux: `xdg-open <url>` (fallback gracefully if absent; just log the URL).

If opening fails, log the URL and continue running.

### 3.5 Settings precedence

Effective settings are resolved in this order, later sources overriding earlier
ones:

1. Built-in defaults.
2. Config file (`.gfm-hotview/config.*`; see §11), unless `--no-config`.
3. Command-line flags.

A flag explicitly passed on the command line always wins over the same key in
the config file. `--no-config` disables both the config file and all
`.gfm-hotview` overrides (CSS theming included), falling back to defaults + flags.

## 4. Directory Tree & File Handling

### 4.1 Root and traversal

The server walks the root directory and all subdirectories recursively. The
root is fixed at startup; the server MUST NOT serve or traverse outside the
root (see §8 Security).

### 4.2 Tree contents

- By default the tree shows Markdown files (`--show` globs) plus any directory
  that (transitively) contains a shown file.
- Empty directories (no matching descendants) are hidden by default.
- The tree is sorted: directories first, then files, each alphabetically,
  case-insensitive.
- File and directory names are displayed as-is; `.md`/`.markdown` extensions
  MAY be shown.

### 4.3 Default ignores

Always excluded unless explicitly re-included: `.git`, `.hg`, `.svn`,
`node_modules`, `.DS_Store`, the `.gfm-hotview` configuration directory (§11), and
(unless `--hidden`) all dotfiles and dot-directories. `--ignore` adds further patterns. `.gitignore` is **not**
honored by default (keep behavior predictable and dependency-free); this MAY
be a future opt-in.

### 4.4 Index/README detection

When a directory node is opened (or as the default landing page for the root),
the server looks for a default document in this order, case-insensitive:
`README.md`, `index.md`, `readme.markdown`, `index.markdown`. If none exists,
the directory view shows the rendered tree/listing for that directory only.

### 4.5 Non-Markdown files

- Images referenced by Markdown (e.g. `![](diagram.png)`) and other static
  assets within the root are served directly with correct MIME types so
  documents render with their local images intact.
- Relative links between Markdown files resolve to the in-app viewer route so
  navigation stays inside the panel UI.
- Links to non-Markdown text files MAY be shown as raw text; binary files are
  served for download/inline display by the browser.

## 5. Markdown Rendering

### 5.1 Baseline

Render the full [GitHub Flavored Markdown spec](https://github.github.com/gfm/)
(CommonMark + GFM extensions): headings, lists, blockquotes, tables,
strikethrough, task lists, fenced code, autolinks, hard line breaks, and HTML
blocks.

### 5.2 GitHub-specific features (offline-capable)

The following MUST render and visually match GitHub as closely as practical,
entirely offline:

- **Tables** with alignment.
- **Task lists** (`- [ ]` / `- [x]`), rendered as (read-only) checkboxes.
- **Strikethrough**, **autolinks**.
- **Footnotes**.
- **Syntax-highlighted code blocks** via embedded Chroma using a GitHub-like
  style for light and dark themes. Unknown languages fall back to plain.
- **Alerts / admonitions** (`> [!NOTE]`, `> [!TIP]`, `> [!IMPORTANT]`,
  `> [!WARNING]`, `> [!CAUTION]`) with GitHub styling and icons (icons embedded
  as inline SVG).
- **Section links / heading anchors:** each heading gets a GitHub-compatible
  slug `id` and an anchor link on hover.
- **Emoji shortcodes** (`:smile:`) mapped to Unicode from an embedded table.
  Shortcodes that map only to GitHub custom images are left as text.
- **Math** (`$…$`, `$$…$$`, and ```` ```math ````) rendered with an embedded
  math typesetter (e.g. KaTeX assets compiled into the binary). No CDN.
- **Mermaid diagrams** (```` ```mermaid ````) rendered client-side using an
  embedded Mermaid bundle. No CDN.
- **Raw HTML** in Markdown is passed through (see §8 for the security note).

### 5.3 Features that cannot be fully offline

These MUST degrade gracefully and be documented:

- **GeoJSON/TopoJSON maps** require map tiles; offline they render as a
  formatted code block (or an optional best-effort local renderer with no base
  map). Disabled by default offline.
- **GitHub-backend features** (@mentions, issue/PR `#123` autolinking, commit
  SHA links, user avatars) are out of scope and rendered as literal text.

### 5.4 Frontmatter

YAML/TOML frontmatter delimited by `---`/`+++` at the very top of a file is
parsed and hidden from the body. `title` (if present) MAY set the document
title; otherwise the first H1 or the filename is used.

## 6. User Interface

### 6.1 Layout

A two-pane (multi-panel) responsive layout served as a single embedded SPA-lite
page:

- **Left sidebar — file tree:** the recursive directory tree of shown files,
  collapsible folders, the current file highlighted. Includes a quick filter
  box that fuzzy-filters the tree by filename. Resizable and collapsible; width
  persists in `localStorage`.
- **Main pane — rendered document:** the GFM-rendered HTML inside a
  GitHub-styled "markdown body" container, constrained to a readable max width,
  with a sticky breadcrumb header showing the current path.
- **Optional right rail — table of contents:** auto-generated from the
  document's headings, with scroll-spy highlighting the active section. May be
  hidden on narrow viewports.

### 6.2 Styling

- GitHub-equivalent Markdown CSS (e.g. based on
  [`github-markdown-css`](https://github.com/sindresorhus/github-markdown-css)),
  embedded for both light and dark themes.
- Theme selection follows `--theme`; `auto` respects
  `prefers-color-scheme` and offers an in-page toggle (choice persisted in
  `localStorage`).
- All fonts, icons, and styles are embedded; the page must look correct with no
  network.

### 6.3 Custom CSS theming

The viewer supports optional, fully offline CSS theming via the `.gfm-hotview`
directory (see §11):

- On startup the server looks for a `.gfm-hotview/css` directory under the root.
  If present, every `*.css` file in it (sorted alphabetically, non-recursive
  unless otherwise configured) is collected as **user style overrides**.
- These overrides are served from a dedicated embedded-vs-user CSS route and
  linked into the page **after** all built-in stylesheets, so user rules win by
  normal CSS cascade order without `!important` gymnastics.
- Overrides apply to both the chrome (sidebar, header, TOC) and the rendered
  `.markdown-body` content. To keep targeting stable, the built-in markup
  exposes documented, stable class names / CSS custom properties (e.g. theme
  color variables) that overrides may redefine.
- Theming is additive and optional: with no `.gfm-hotview/css` directory the
  built-in themes are used unchanged. `--no-config` disables user CSS overrides.
- User CSS is local content and is served as-is (not sanitized). It cannot
  reference remote URLs and still satisfy the offline guarantee; remote
  `@import`/`url()` references will simply fail to load offline and MUST NOT be
  relied upon.
- Changes to files under `.gfm-hotview/css` participate in live reload (§7): the
  page re-applies styles without a full reload when an override file changes.

### 6.4 Navigation behavior

- Clicking a file in the tree loads and renders it in the main pane without a
  full page reload (history `pushState`, deep-linkable URLs).
- Clicking a folder expands/collapses it and, if it has an index/README, shows
  that document.
- In-document relative links to other Markdown files navigate within the app;
  heading anchors scroll within the document; external `http(s)` links open in
  a new tab.
- Browser back/forward navigate document history.

### 6.5 Graceful degradation

Core reading works without JavaScript: each document is also reachable via a
plain server-rendered HTML route so the content is viewable if JS is disabled
(live reload, fuzzy filter, and client-side nav are JS enhancements).

## 7. Live Reload

- When enabled (default), the server watches the root tree for create / modify
  / delete / rename events using OS-native file-system notifications, with a
  short debounce to coalesce bursts.
- The browser holds an open channel to the server (Server-Sent Events
  preferred for simplicity; WebSocket acceptable). On a relevant change the
  server pushes an event and the client re-fetches and re-renders only the
  affected view:
  - Change to the currently viewed file → re-render the main pane (preserving
    scroll position where feasible).
  - Tree-structure change (add/remove/rename) → refresh the sidebar tree.
  - Change to a `.gfm-hotview/css` override file → re-apply user styles without a
    full reload. A change to the config file is reported in the log; settings
    that cannot be applied live (e.g. bind host/port) require a restart and the
    server says so rather than silently ignoring them.
- If the live-reload channel drops, the client retries with backoff and shows a
  subtle "reconnecting" indicator.
- `--no-reload` disables watching and the client channel entirely.

## 8. Security

- **Path containment:** all file access is confined to the root directory.
  Resolve and validate every requested path; reject anything that escapes via
  `..`, absolute paths, or symlinks pointing outside the root. Return `404`
  for out-of-root or excluded paths.
- **Default binding:** bind to `localhost` by default. Binding to a non-loopback
  interface is allowed via `--host` but documented as exposing the served
  directory to the network (read-only).
- **Raw HTML rendering:** to faithfully mirror GitHub's advanced formatting,
  raw HTML in Markdown is rendered (not sanitized) by default. Because content
  is local and trusted, this is acceptable; a `--safe`/sanitize option MAY be
  offered for serving untrusted content. This trade-off MUST be documented.
- **`.gfm-hotview` directory:** the config file and `.gfm-hotview/css` overrides are
  read at startup from a path confined to the root (§8 path-containment rules
  apply). The `.gfm-hotview` directory itself is excluded from the browsable file
  tree and is never served via `/raw`. User-supplied CSS is treated as trusted
  local content and is served unsanitized, consistent with the raw-HTML policy
  above.
- No telemetry, no analytics, no outbound network requests of any kind.

## 9. HTTP Endpoints (internal contract)

Routes are an implementation detail but the following capabilities are
required:

- `GET /` — app shell; renders the default/landing document.
- `GET /view/<relpath>` — app view for a specific Markdown file (deep-linkable).
- `GET /raw/<relpath>` — served file bytes (images/assets/raw text) with correct
  MIME type, path-contained.
- `GET /api/tree` — JSON of the (filtered) directory tree for the sidebar.
- `GET /api/render?path=<relpath>` — rendered HTML fragment + metadata (title,
  headings/TOC) for client-side navigation.
- `GET /events` — live-reload stream (SSE), when reload is enabled.
- `GET /assets/*` — embedded static assets (CSS/JS/fonts/icons).
- `GET /user.css` — concatenated user CSS overrides from `.gfm-hotview/css` (§11),
  empty if none; linked after all built-in stylesheets.

All `<relpath>` inputs are validated for path containment (§8).

## 10. Cross-Platform & Distribution

- **Targets:** `windows/amd64`, `windows/arm64`, `linux/amd64`, `linux/arm64`,
  `darwin/amd64`, `darwin/arm64`.
- **Artifacts:** one self-contained binary per target (`.exe` on Windows),
  produced via Go cross-compilation. No CGO required (pure-Go file watching and
  highlighting) so builds are fully static and portable.
- **Install paths:** prebuilt release binaries, `go install`, and (optionally) a
  `gh` CLI extension wrapper analogous to `gh-gfm-preview`.
- **Line endings:** source uses LF; the tool handles both LF and CRLF input
  Markdown transparently.
- **Reproducibility:** version/commit embedded at build time via `-ldflags`.

## 11. Configuration & Theming Directory (`.gfm-hotview`)

`gfm-hotview` supports an optional, per-project `.gfm-hotview` directory located at
the root of the served tree. It is entirely optional: when absent, built-in
defaults and command-line flags fully determine behavior. The directory is
read at startup, is excluded from the browsable tree, and is never served via
`/raw` (see §8). `--no-config` ignores the directory entirely.

```
<root>/
  .gfm-hotview/
    config.toml        # optional config file (see §11.1)
    css/               # optional CSS overrides (see §11.2)
      theme.css
      ...
```

### 11.1 Config file

- **Location / discovery:** by default the server looks for a config file named
  `config.toml`, `config.yaml`, `config.yml`, or `config.json` inside
  `.gfm-hotview/` (in that preference order; the first found wins). An explicit
  `--config <path>` overrides discovery and may point anywhere within the root.
- **Format:** TOML is the recommended/primary format; YAML and JSON are
  accepted equivalents. The chosen library MUST be pure-Go and compiled in (no
  runtime dependency).
- **Purpose (v1):** override the bind **port** and **address/host**. The schema
  is intentionally forward-compatible so additional settings can be added later
  without breaking existing files; unknown keys are ignored with a warning in
  verbose mode rather than causing a fatal error.
- **Precedence:** config values override built-in defaults but are themselves
  overridden by explicitly-passed command-line flags (§3.5).
- **Validation:** invalid syntax or out-of-range values are reported as a clear
  startup error; the server does not start with an unparseable config rather
  than silently falling back.

Recommended v1 keys (TOML shown; YAML/JSON equivalent):

```toml
# .gfm-hotview/config.toml
[server]
host = "127.0.0.1"   # bind address; overrides default "localhost"
port = 8080          # bind port; overrides default 6419

# Reserved namespaces for future settings (ignored if unknown in v1):
# [ui]
# theme = "dark"
# [tree]
# show = ["*.md", "*.markdown"]
```

Settings that change the listening socket (`host`, `port`) are applied at
startup only; changing them in the config file while running requires a restart
(§7).

### 11.2 CSS overrides

- If a `.gfm-hotview/css` directory exists, every `*.css` file directly within it
  is loaded as user style overrides, concatenated in alphabetical filename
  order, and exposed via `GET /user.css`.
- The page links `/user.css` **after** all built-in stylesheets so user rules
  win by normal cascade order (see §6.3). Overrides may target the documented
  stable class names and theme CSS custom properties.
- User CSS is trusted local content served unsanitized (§8) and must be
  self-contained to preserve offline operation; remote `@import`/`url()` will
  not load offline.
- Files under `.gfm-hotview/css` participate in live reload: edits re-apply styles
  without a full page reload (§7).

## 12. Acceptance Criteria

1. Running `gfm-hotview` in a directory with no flags starts a local server,
   opens the browser, and shows a GitHub-styled view with the file tree on the
   left and the detected README (or directory listing) rendered on the right —
   with no network access.
2. The sidebar lists all Markdown files in the root and its subdirectories
   (respecting default ignores), folders are collapsible, and clicking a file
   renders it without a full page reload and updates the URL.
3. A representative GFM document renders tables, task lists, footnotes,
   alerts, emoji, heading anchors, syntax-highlighted code, math, and Mermaid
   diagrams correctly and offline, visually close to GitHub.
4. Editing a viewed file on disk updates the rendered pane within ~1 second
   without a manual refresh; adding/removing files updates the tree.
5. Requests for paths outside the root (including via `..` or out-of-root
   symlinks) return `404` and never read those files.
6. A `.gfm-hotview/config.toml` setting `host`/`port` changes the bind address and
   port; an explicit `--port`/`--host` flag overrides the config file value.
7. A `.gfm-hotview/css/theme.css` override visibly changes the rendered styling,
   applies after the built-in CSS, and re-applies on edit via live reload;
   removing the file (or `--no-config`) restores the built-in theme.
8. The same binary, cross-compiled, runs and passes criteria 1–7 on Windows,
   Linux, and macOS with no additional installation.
9. With JavaScript disabled, documents are still readable via server-rendered
   HTML routes.

## 13. Future Considerations (out of scope for v1)

- Optional `.gitignore` honoring.
- Full-text search across the served tree.
- Print/export to standalone HTML or PDF.
- A user-level/global `.gfm-hotview` (e.g. in the home/config dir) merged beneath
  the per-project one.
- Additional config-driven settings (theme, tree filters, mode) beyond
  host/port.
- Best-effort offline map rendering for GeoJSON/TopoJSON.
- PlantUML / additional diagram dialects.
