---
id: mermaid-zoom-view
status: accepted
features: [ui]
created: 2026-08-30
updated: 2026-09-08
---

# Mermaid diagram zoom view

| Created | Updated |
| --- | --- |
| 2026-08-30 | 2026-09-08 |

## Why

Rendered Mermaid diagrams stay inline in the document. Large graphs, sequence
diagrams, and class charts are hard to read at that size. Users need a way to
inspect a diagram full-screen, pan around, and zoom in.

## What changes

- Each rendered Mermaid diagram gets a control that opens a full-screen overlay
  containing a copy of that diagram.
- In the overlay, left-click and drag pans the diagram; the mouse wheel zooms
  in and out (toward the cursor).
- Escape closes the overlay and returns to the document.
- Overlay styling follows the current light/dark theme.
- Client-side only (`app.js` / CSS). Server-side Mermaid rendering is unchanged.

## Scope

- In: open overlay from a diagram, pan (left-click drag), wheel zoom, Escape to
  close, theme-aware backdrop, grab/grabbing cursor while panning
- Out: pinch/touch gestures, keyboard pan/zoom, exporting/saving the diagram,
  zooming diagrams in-document (without the overlay)

## Impacted specifications

- `ui` (new)

## Open questions

- None — approved with: hover zoom button plus click-to-open; backdrop click
  closes; visible close control in addition to Escape.

## Change history

| Date | Change |
| --- | --- |
| 2026-08-30 | Initial proposal |
| 2026-08-30 | Approved; open questions resolved; implementing |
| 2026-08-30 | Implementation complete; awaiting review |
| 2026-09-08 | Accepted; `ui` spec created; archived |
