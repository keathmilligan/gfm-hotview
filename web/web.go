// Package web embeds the frontend templates and static assets so the binary is
// fully self-contained and works offline.
package web

import "embed"

// Templates holds HTML templates.
//
//go:embed templates/*.html
var Templates embed.FS

// Assets holds static assets (CSS/JS and vendor bundles).
//
//go:embed assets
var Assets embed.FS
