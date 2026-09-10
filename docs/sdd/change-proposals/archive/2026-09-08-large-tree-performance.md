---
id: large-tree-performance
status: accepted
features: [ui]
created: 2026-09-08
updated: 2026-09-08
---

# Large tree performance

| Created | Updated |
| --- | --- |
| 2026-09-08 | 2026-09-08 |

## What

### Why

Opening a folder with thousands of Markdown files makes the viewer sluggish:
the first page is huge, the sidebar stutters, and typing in the filter walks
every row. The document itself is not the problem. The sidebar currently
embeds the entire file tree as HTML, with an inline SVG icon on every row and
a click listener on every label.

Measured on this machine: 5,000 `.md` files scan in ~12 ms, but the tree HTML
is ~2.3 MiB. Nested trees are just as large because collapsed folders still
serialize their children.

### Requirements

- The sidebar SHALL stay usable when a served tree contains thousands of
  Markdown files in one directory or spread across nested directories.
- First paint SHALL not include per-file HTML for rows that are not in the
  sidebar viewport.
- Filter, expand/collapse, expand-all, collapse-all, and selection SHALL keep
  their current behavior, including substring filtering of file names.
- Live reload of the tree SHALL not replace the whole sidebar HTML document.
- The client SHALL not add a new JavaScript library.

### Scope

- In: tree payload, sidebar rendering, filter, tree live-reload, file/folder
  icons, content-pane directory listing icons
- Out: markdown render performance, inotify watch limits on huge mixed
  repositories, virtualizing the document pane, changing `--show` / `--ignore`

### Open questions

- None

## How

### Approach

Cache the tree as compact JSON. The page carries that JSON; the client holds
it as the model, flattens expanded rows, and virtualizes the sidebar viewport.
Clicks go through one listener on `#tree`. Icons are a single sprite or CSS,
not markup copied onto every row. Details and measurements:
[design/large-tree-performance.md](../design/large-tree-performance.md).

### Impacted specifications

- `ui` (existing)

### Plan

#### 1. Payload

- [x] 1.1 Embed compact tree JSON in the page shell instead of `TreeHTML`
- [x] 1.2 Keep `/api/tree` as the live-reload payload; stop using
      `/api/tree-html`
- [x] 1.3 Add a test that a 1,000-file tree’s page/JSON payload stays far
      smaller than today’s HTML encoding (order-of-magnitude, not a brittle
      byte count)

#### 2. Client tree

- [x] 2.1 Build a client model from the JSON and flatten expanded rows
- [x] 2.2 Virtualize `#tree`: render only the viewport window at a fixed row
      height, indent by depth
- [x] 2.3 Delegate click, selection, expand/collapse, expand-all, and
      collapse-all to `#tree` (no per-row listeners)
- [x] 2.4 Run the existing filter semantics on the model and virtualize the
      filtered list
- [x] 2.5 Scroll the selected file into the virtual window on navigation and
      on first load

#### 3. Icons and listings

- [x] 3.1 Replace per-row inline SVGs with a sprite or CSS icons in the
      sidebar
- [x] 3.2 Use the same icons in content-pane directory listings (no per-item
      inline SVG)

#### 4. Server cleanup

- [x] 4.1 Remove `renderTreeHTML` / `/api/tree-html` once unused
- [x] 4.2 Optionally skip `DirEntry.Info()` except for symlinks
- [x] 4.3 Update server tests that asserted on tree HTML

## Change history

| Date | Change |
| --- | --- |
| 2026-09-08 | Initial proposal |
| 2026-09-08 | Approved; implementing |
| 2026-09-08 | Implementation complete; awaiting review |
| 2026-09-08 | Accepted; `ui` spec updated; archived |
