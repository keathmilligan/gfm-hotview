---
feature: samples
created: 2026-09-08
updated: 2026-09-08
---

# Samples

| Created | Updated |
| --- | --- |
| 2026-09-08 | 2026-09-08 |

## Purpose

In-repo Markdown documents that exercise GitHub-Flavored Markdown features for
preview and visual regression.

## Requirements

### Sample tree

The repository SHALL include a `samples/` directory that is not served unless
the user points the viewer at it.

- GIVEN the repository checkout
- WHEN a user runs the viewer on `samples`
- THEN they can open a hub README and pages covering GFM basics, alerts, code,
  math, Mermaid, images, and a nested relative link

The sample tree SHALL include SVG and raster images and several Mermaid
diagram types. The project README SHALL point at `samples/` as the way to
preview those features.

## Change history

| Date | Change |
| --- | --- |
| 2026-09-08 | Added samples directory requirements |
