---
feature: ui
created: 2026-09-08
updated: 2026-09-08
---

# UI

| Created | Updated |
| --- | --- |
| 2026-09-08 | 2026-09-08 |

## Purpose

User-facing presentation: document chrome, sidebar file tree, theme, and the
full-screen media zoom overlay for diagrams and images.

## Requirements

### File tree sidebar

The system SHALL present a sidebar file tree of matching Markdown files. The
sidebar SHALL stay usable when the served tree contains thousands of files in
one directory or nested directories. The document SHALL not include a DOM row
for every file; only rows in (or immediately around) the sidebar viewport SHALL
be present.

File and folder rows SHALL use shared icons, including in content-pane
directory listings when a directory has no index document.

#### Expand and collapse

- GIVEN a directory row in the sidebar
- WHEN the user clicks it
- THEN its children are shown if it was collapsed, or hidden if it was expanded

- GIVEN the expand-all control
- WHEN the user clicks it
- THEN every directory in the tree is expanded

- GIVEN the collapse-all control
- WHEN the user clicks it
- THEN directory children are hidden

Served roots SHALL start expanded. Opening a file SHALL expand its ancestor
directories.

#### Filter

- GIVEN the sidebar filter field
- WHEN the user types a substring
- THEN only files whose names contain that substring (case-insensitive) are
  shown, along with their ancestor directories
- AND directories with no matching file descendant stay hidden
- AND matching branches are expanded while the filter is active

#### Selection and scroll

- GIVEN a file is open
- WHEN the tree is shown
- THEN that file is marked selected
- AND the selected row is scrolled into the visible window on first load and
  on navigation

#### Tree live reload

- GIVEN live reload is enabled and the served tree changes on disk
- WHEN the client receives a tree update
- THEN the sidebar reflects the new tree without replacing the page
- AND existing expand/collapse state is kept for paths that still exist

### Media zoom overlay

The system SHALL provide a full-screen overlay for inspecting a Mermaid
diagram or a content image at a larger size. Overlay styling SHALL follow the
current light or dark theme. Opening a new document or reloading content SHALL
close the overlay.

#### Open from a Mermaid diagram

- GIVEN a rendered Mermaid diagram in the document
- WHEN the user clicks the diagram or its zoom button
- THEN the overlay opens with a copy of that diagram

#### Open from an image

- GIVEN a content image in the document that is not inside a link
- WHEN the user clicks the image or its zoom button
- THEN the overlay opens with that image

#### Linked images

- GIVEN a content image wrapped in a link
- WHEN the user clicks the image
- THEN the link is followed and the overlay does not open
- WHEN the user clicks the zoom button
- THEN the overlay opens with that image

#### Pan, zoom, and close

- GIVEN the overlay is open
- WHEN the user left-click-drags
- THEN the media pans
- WHEN the user scrolls the mouse wheel
- THEN the media zooms toward the cursor
- WHEN the user presses Escape, clicks the close control, or clicks the
  backdrop without dragging
- THEN the overlay closes and the document is shown again

## Change history

| Date | Change |
| --- | --- |
| 2026-09-08 | Added media zoom for Mermaid diagrams and content images |
| 2026-09-08 | Added virtualized file tree sidebar, filter, and tree live reload |
