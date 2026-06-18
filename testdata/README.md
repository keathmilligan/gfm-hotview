# gfm-hotview test fixture

This document exercises the renderer.

## Tables

| Feature | Supported |
| --- | :---: |
| Tables | yes |
| Alerts | yes |

## Task list

- [x] done
- [ ] todo

## Alert

> [!NOTE]
> This is a note alert.

> [!WARNING]
> Be careful.

## Code

```go
package main

func main() {
    println("hi")
}
```

## Math

Inline $a^2 + b^2 = c^2$ and block:

$$
\int_0^1 x^2 \, dx = \frac{1}{3}
$$

## Mermaid

```mermaid
graph TD
  A --> B
```

## Emoji

:rocket: :tada:

A [relative link](sub/page.md).

## GFM Features Exercise

### Definition Lists

Render Pipeline
: The component responsible for converting raw markdown into styled output blocks.

Plugin System with *extensions*
: A hot-reloadable architecture that supports **custom parsers** and `render hooks`.

### Strikethrough

The ~~legacy ASCII renderer~~ has been replaced with a full GFM-compliant engine. Users may notice that ~~some deprecated syntax~~ is no longer supported in tables and blockquotes.
