---
id: mermaid-zoom-view
created: 2026-08-30
updated: 2026-08-30
---

# Plan: Mermaid diagram zoom view

| Created | Updated |
| --- | --- |
| 2026-08-30 | 2026-08-30 |

## 1. Overlay chrome

- [x] 1.1 Add a full-screen dialog to `page.html` (viewport, canvas, close button)
- [x] 1.2 Style the overlay, close control, grab/grabbing cursor, and scroll lock in `app.css`

## 2. Open from diagrams

- [x] 2.1 Wrap each Mermaid block and add a hover zoom button (copy-button pattern)
- [x] 2.2 Clicking the button or the diagram opens the overlay with a cloned SVG
- [x] 2.3 Prefix IDs in the clone so markers/gradients do not clash with the original

## 3. Pan, zoom, close

- [x] 3.1 Left-click-drag pans; wheel zooms toward the cursor; scale is clamped
- [x] 3.2 Escape, close button, and click-without-drag on the backdrop close the overlay
- [x] 3.3 Close on navigation / content reload so a stale diagram is not left open

## 4. Docs

- [x] 4.1 Mention zoom in the README features list

## Change history

| Date | Change |
| --- | --- |
| 2026-08-30 | Initial plan |
| 2026-08-30 | Implementation complete |
