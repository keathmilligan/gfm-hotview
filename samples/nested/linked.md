---
title: Nested sample
kind: demo
---

# Nested page

This file lives in `samples/nested/` so the sidebar tree has a folder, and so
relative links have to walk up a directory.

Frontmatter above (`title`, `kind`) is stripped before render — you should not
see those keys in the preview.

## Links

- Up to the [samples hub](../README.md)
- Sibling features: [images](../images.md), [mermaid](../mermaid.md)

## Image from parent folder

![Landscape from parent images/](../images/landscape.svg)

Relative `../images/landscape.svg` should display here the same way it does on
the images page.
