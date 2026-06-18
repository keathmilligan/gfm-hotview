package render

import (
	stdhtml "html"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// mermaidExtender renders ```mermaid fenced code blocks as
// <pre class="mermaid">…</pre> for client-side rendering by mermaid.js. This
// avoids any server-side/browser-automation dependency and keeps the binary
// small and fully offline. If the mermaid bundle is absent in the browser, the
// diagram source is simply shown as preformatted text.
//
// It works by transforming matching fenced code blocks into a custom AST node
// before rendering, so non-mermaid code blocks are still handled by the syntax
// highlighter.
type mermaidExtender struct{}

func (e *mermaidExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&mermaidTransformer{}, 90),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&mermaidRenderer{}, 90),
	))
}

var mermaidKind = ast.NewNodeKind("MermaidBlock")

type mermaidNode struct {
	ast.BaseBlock
	code string
}

func (n *mermaidNode) Kind() ast.NodeKind         { return mermaidKind }
func (n *mermaidNode) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

type mermaidTransformer struct{}

func (t *mermaidTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var targets []*ast.FencedCodeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if fcb, ok := n.(*ast.FencedCodeBlock); ok {
			if fcb.Info != nil && string(fcb.Info.Segment.Value(src)) == "mermaid" {
				targets = append(targets, fcb)
			}
		}
		return ast.WalkContinue, nil
	})

	for _, fcb := range targets {
		var code string
		lines := fcb.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			code += string(seg.Value(src))
		}
		mn := &mermaidNode{code: code}
		if parent := fcb.Parent(); parent != nil {
			parent.ReplaceChild(parent, fcb, mn)
		}
	}
}

type mermaidRenderer struct{}

func (r *mermaidRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(mermaidKind, r.render)
}

func (r *mermaidRenderer) render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*mermaidNode)
		_, _ = w.WriteString(`<pre class="mermaid">`)
		_, _ = w.WriteString(stdhtml.EscapeString(n.code))
		_, _ = w.WriteString("</pre>\n")
	}
	return ast.WalkSkipChildren, nil
}
