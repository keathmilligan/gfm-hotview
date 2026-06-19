//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

var reToken = regexp.MustCompile(`^\s*(?:/\*.*?\*/\s*)?(\.chroma\s+\.\w+)\s*\{([^}]*)\}$`)

// Custom dark-mode colors for tokens present in the light "github" theme but
// absent from "github-dark". Tokens like NameAttribute (.na) and Punctuation
// (.p) would otherwise fall back to light-theme colors (e.g. #1f2328) that
// are nearly invisible on dark backgrounds.
var missingDarkColors = map[string]string{
	".bp": "color: #9198a1",
	".na": "color: #c9d1d9",
	".nb": "color: #d2a8ff",
	".nx": "color: #c9d1d9",
	".p":  "color: #c9d1d9",
}

func main() {
	var buf bytes.Buffer

	buf.WriteString("/* Generated — do not edit by hand. */\n\n")

	// Shared structural rules.
	buf.WriteString(".chroma { -webkit-text-size-adjust: none; }\n\n")

	// Light — both default (no attribute) and explicit.
	buf.WriteString("/* Light (default) */\n")
	writeTokens(&buf, "github", "", nil)
	buf.WriteString("\n/* Light (explicit attribute) */\n")
	writeTokens(&buf, "github", `html[data-theme="light"] `, nil)

	// Dark — explicit toggle. Track which tokens are emitted.
	darkSeen := make(map[string]bool)
	buf.WriteString("\n/* Dark (explicit toggle) */\n")
	writeTokens(&buf, "github-dark", `html[data-theme="dark"] `, darkSeen)
	addMissingDark(&buf, `html[data-theme="dark"] `, darkSeen)

	// Dark — auto via OS.  Reset map for the second pass.
	darkSeen = make(map[string]bool)
	buf.WriteString("\n/* Dark (auto via OS preference) */\n")
	buf.WriteString("@media (prefers-color-scheme: dark) {\n")
	writeTokens(&buf, "github-dark", "", darkSeen)
	addMissingDark(&buf, "", darkSeen)
	buf.WriteString("}\n")

	os.WriteFile("web/assets/chroma.css", buf.Bytes(), 0o644)
	fmt.Println("wrote web/assets/chroma.css")
}

func writeTokens(w io.Writer, themeName, prefix string, seen map[string]bool) {
	style := styles.Get(themeName)
	if style == nil {
		fmt.Fprintf(os.Stderr, "theme %q not found\n", themeName)
		return
	}
	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.TabWidth(2),
	)
	var tmp bytes.Buffer
	formatter.WriteCSS(&tmp, style)

	for _, line := range strings.Split(tmp.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := reToken.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		selector := m[1]
		decl := strings.TrimSpace(m[2])
		fmt.Fprintf(w, "%s%s { %s }\n", prefix, selector, decl)
		if seen != nil {
			parts := strings.Fields(selector)
			if len(parts) >= 2 {
				seen[parts[1]] = true
			}
		}
	}
}

func addMissingDark(w io.Writer, prefix string, seen map[string]bool) {
	for sel, rule := range missingDarkColors {
		if seen[sel] {
			continue
		}
		fmt.Fprintf(w, "%s.chroma %s { %s }\n", prefix, sel, rule)
	}
}
