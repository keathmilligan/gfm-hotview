---
id: gfm-samples
status: review
features: [samples]
created: 2026-08-30
updated: 2026-08-30
---

# GFM samples directory

| Created | Updated |
| --- | --- |
| 2026-08-30 | 2026-08-30 |

## Why

There is no in-repo document tree that exercises GFM features end-to-end
(images, Mermaid zoom, alerts, math, relative links). A `samples/` tree makes
those easy to preview and to regression-check by eye.

## What changes

- Add `samples/` with a README hub, feature pages, nested links, SVG and raster
  images, and several Mermaid diagram types.
- Point at it from the project README.

## Scope

- In: sample markdown, images, a nested page, README pointer
- Out: automated visual tests, serving samples by default

## Impacted specifications

- `samples` (new)

## Open questions

- None

## Change history

| Date | Change |
| --- | --- |
| 2026-08-30 | Initial proposal; implementing on explicit create request |
| 2026-08-30 | Implementation complete; awaiting review |
