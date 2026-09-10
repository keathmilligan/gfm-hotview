---
id: image-zoom-view
status: accepted
features: [ui]
created: 2026-08-30
updated: 2026-09-08
---

# Image zoom view

| Created | Updated |
| --- | --- |
| 2026-08-30 | 2026-09-08 |

## Why

Mermaid diagrams already open in a full-screen pan/zoom overlay. Inline
images have the same problem when they are large or detailed.

## What changes

- Content images use the same overlay as Mermaid: click or hover zoom button
  to open, left-click-drag to pan, wheel to zoom, Escape / × / backdrop to
  close.
- Linked images still follow the link on click; the zoom button opens the
  overlay.

## Scope

- In: markdown `img` in the document, shared overlay, samples/README copy
- Out: pinch/touch, in-document zoom, images outside `#content`

## Impacted specifications

- `ui` (existing, from mermaid-zoom-view)

## Open questions

- None

## Change history

| Date | Change |
| --- | --- |
| 2026-08-30 | Initial proposal; implementing on explicit request |
| 2026-08-30 | Implementation complete; awaiting review |
| 2026-09-08 | Accepted; `ui` spec updated; archived |
