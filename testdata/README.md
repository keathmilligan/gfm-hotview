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

### Bash

```bash
#!/bin/bash
set -euo pipefail

echo "building with $CORES cores"
make -j"${CORES:-4}" build
```

### Go

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds server settings loaded from a TOML file.
type Config struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &cfg, nil
}
```

### TOML

```toml
[server]
host = "0.0.0.0"
port = 8080

[logging]
level = "info"
format = "json"

[features]
  hot_reload = true
  dark_mode = "auto"

[limits]
max_body_size = "10MB"
timeout = "30s"

[[plugins]]
name = "metrics"
enabled = true
path = "./plugins/metrics.so"

[[plugins]]
name = "auth"
enabled = false
```

### YAML

```yaml
server:
  host: "0.0.0.0"
  port: 8080

logging:
  level: info
  format: json

features:
  hot_reload: true
  dark_mode: auto

limits:
  max_body_size: 10MB
  timeout: 30s

plugins:
  - name: metrics
    enabled: true
    path: ./plugins/metrics.so
  - name: auth
    enabled: false
```

### Python

```python
#!/usr/bin/env python3
"""Hot-reload file watcher using inotify."""

import asyncio
import sys
from pathlib import Path
from typing import AbstractSet

WATCH_MASK = 0  # placeholder


async def watch(root: Path, exts: AbstractSet[str]) -> None:
    """Watch a directory tree for changes."""
    pending: set[Path] = set()

    for path in root.rglob("*"):
        if path.suffix in exts and not path.name.startswith("."):
            pending.add(path)

    while True:
        if not pending:
            await asyncio.sleep(0.5)
            continue
        target = pending.pop()
        print(f"changed: {target}")
```

### Rust

```rust
/// A thread-safe file-tree node with incremental build support.
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Arc;

#[derive(Debug, Clone)]
pub struct Node {
    pub name: String,
    pub path: PathBuf,
    pub is_dir: bool,
    pub children: HashMap<String, Arc<Node>>,
}

impl Node {
    pub fn new_dir(name: &str, path: PathBuf) -> Self {
        Self {
            name: name.to_string(),
            path,
            is_dir: true,
            children: HashMap::new(),
        }
    }

    pub fn insert(&mut self, child: Arc<Node>) -> Option<Arc<Node>> {
        self.children.insert(child.name.clone(), child)
    }
}
```

### JavaScript

```javascript
// Debounced SSE-based hot-reload client
const BACKOFF = [500, 1000, 2000, 5000, 10000];

export function connectReload(url) {
  let attempt = 0;
  const source = new EventSource(url);

  source.addEventListener("content", (e) => {
    const path = JSON.parse(e.data).path;
    if (path === window.location.pathname) location.reload();
  });

  source.addEventListener("css", () => {
    document.querySelectorAll('link[href*="user.css"]').forEach((l) => {
      l.href = l.href.replace(/\?.*/, "") + "?" + Date.now();
    });
  });

  source.onerror = () => {
    source.close();
    attempt = Math.min(attempt + 1, BACKOFF.length - 1);
    setTimeout(() => connectReload(url), BACKOFF[attempt]);
  };
}
```

### C

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MAX_PATH 4096
#define INITIAL_CAP 16

typedef struct {
    char **items;
    size_t len;
    size_t cap;
} StringList;

static int list_push(StringList *list, const char *s) {
    if (list->len >= list->cap) {
        size_t new_cap = list->cap * 2;
        char **tmp = realloc(list->items, new_cap * sizeof(char *));
        if (!tmp) return -1;
        list->items = tmp;
        list->cap = new_cap;
    }
    list->items[list->len] = strdup(s);
    if (!list->items[list->len]) return -1;
    list->len++;
    return 0;
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
