// Package render converts GitHub-Flavored Markdown into HTML that visually
// matches GitHub, fully offline. It wires goldmark with GFM, footnotes, task
// lists, emoji, syntax highlighting (chroma), math (passthrough to KaTeX),
// mermaid (client-side), heading anchors, and GitHub-style alerts.
package render

import (
	"bytes"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// Result is a rendered document plus extracted metadata.
type Result struct {
	HTML     string    `json:"html"`
	Title    string    `json:"title"`
	Headings []Heading `json:"headings"`
}

// Heading is one entry of the table of contents.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id"`
}

// Renderer renders Markdown to HTML.
type Renderer struct {
	md goldmark.Markdown
	// gfm indicates whether GitHub extensions are enabled.
	gfm bool
}

// New builds a Renderer. When gfm is false, only CommonMark is rendered.
func New(gfm bool) *Renderer {
	exts := []goldmark.Extender{
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.TabWidth(2),
			),
		),
	}
	if gfm {
		exts = append(exts,
			extension.GFM,      // tables, strikethrough, linkify, tasklist
			extension.Footnote, // footnotes
			emoji.Emoji,        // :smile: -> unicode
			mathjax.MathJax,    // $...$ and $$...$$ passthrough for KaTeX
			&mermaidExtender{}, // ```mermaid fenced blocks (client-side)
			&alertExtender{},   // > [!NOTE] style admonitions
		)
	}

	md := goldmark.New(
		goldmark.WithExtensions(exts...),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // raw HTML passthrough (trusted local content, per spec §8)
		),
	)
	return &Renderer{md: md, gfm: gfm}
}

// Render converts src to HTML and extracts the title and headings.
func (r *Renderer) Render(src []byte) (Result, error) {
	src = stripFrontmatter(src)

	doc := r.md.Parser().Parse(text.NewReader(src))
	headings, title := extractHeadings(doc, src)

	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, src, doc); err != nil {
		return Result{}, err
	}

	return Result{
		HTML:     buf.String(),
		Title:    title,
		Headings: headings,
	}, nil
}

// extractHeadings walks the parsed AST collecting heading text/id/level and the
// document title (first H1, else first heading).
func extractHeadings(doc ast.Node, src []byte) ([]Heading, string) {
	var headings []Heading
	var title string

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		txt := nodeText(h, src)
		id := ""
		if v, ok := h.AttributeString("id"); ok {
			switch s := v.(type) {
			case []byte:
				id = string(s)
			case string:
				id = s
			}
		}
		headings = append(headings, Heading{Level: h.Level, Text: txt, ID: id})
		if title == "" && h.Level == 1 {
			title = txt
		}
		return ast.WalkContinue, nil
	})

	if title == "" && len(headings) > 0 {
		title = headings[0].Text
	}
	return headings, title
}

// nodeText collects the visible text content of a node by walking its
// descendants and concatenating *ast.Text values (with soft line breaks as
// newlines). Replaces the deprecated ast.BaseNode.Text.
func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(nn ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := nn.(*ast.Text); ok {
			b.Write(t.Value(src))
			if t.SoftLineBreak() {
				b.WriteByte('\n')
			}
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// stripFrontmatter removes a leading YAML (---) or TOML (+++) frontmatter block.
func stripFrontmatter(src []byte) []byte {
	s := src
	// Tolerate a UTF-8 BOM.
	s = bytes.TrimPrefix(s, []byte{0xEF, 0xBB, 0xBF})
	trimmed := s
	var fence string
	switch {
	case bytes.HasPrefix(trimmed, []byte("---\n")) || bytes.HasPrefix(trimmed, []byte("---\r\n")):
		fence = "---"
	case bytes.HasPrefix(trimmed, []byte("+++\n")) || bytes.HasPrefix(trimmed, []byte("+++\r\n")):
		fence = "+++"
	default:
		return src
	}

	lines := strings.SplitAfter(string(trimmed), "\n")
	// lines[0] is the opening fence. Find the closing fence.
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		if line == fence {
			rest := strings.Join(lines[i+1:], "")
			return []byte(rest)
		}
	}
	return src // unterminated frontmatter: leave as-is
}
