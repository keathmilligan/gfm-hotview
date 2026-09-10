package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/gfm-hotview/internal/config"
	"github.com/local/gfm-hotview/internal/tree"
)

func newMultiRootServer(t *testing.T, roots []string) *Server {
	t.Helper()
	cfg := config.Default(roots[0])
	cfg.Roots = roots
	s, err := New(cfg, log.New(os.Stderr, "", 0), "test")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func getReq(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestMultiRootTreeRenderAndRaw(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.WriteFile(filepath.Join(a, "README.md"), []byte("# A\nhello a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "guide.md"), []byte("# B\nhello b"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newMultiRootServer(t, []string{a, b})
	h := s.Handler()
	la, lb := filepath.Base(a), filepath.Base(b)

	// The tree lists both mounts.
	rr := getReq(t, h, "/api/tree")
	if !strings.Contains(rr.Body.String(), la) || !strings.Contains(rr.Body.String(), lb) {
		t.Fatalf("tree missing mount labels %q/%q: %s", la, lb, rr.Body.String())
	}

	// A namespaced path renders content from the right root.
	rr = getReq(t, h, "/api/render?path="+la+"/README.md")
	if rr.Code != http.StatusOK {
		t.Fatalf("render status %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "hello a") {
		t.Fatalf("render body missing content: %s", rr.Body.String())
	}

	// /raw/ serves a namespaced file from the second root.
	rr = getReq(t, h, "/raw/"+lb+"/guide.md")
	if rr.Code != http.StatusOK {
		t.Fatalf("raw status %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "hello b") {
		t.Fatalf("raw body missing content: %s", rr.Body.String())
	}

	// Landing renders a roots listing (no auto README from primary root).
	rr = getReq(t, h, "/")
	if !strings.Contains(rr.Body.String(), "roots") {
		t.Fatalf("landing should show roots listing: %s", rr.Body.String())
	}
}

func TestMultiRootRejectsEscape(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if err := os.WriteFile(filepath.Join(a, "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newMultiRootServer(t, []string{a, b})
	h := s.Handler()
	la := filepath.Base(a)

	// ".." within a mount must not escape into the other root or outside. The
	// render API takes the path as a query param, so it reaches the handler
	// without the mux cleaning/redirecting it.
	rr := getReq(t, h, "/api/render?path="+la+"/../../etc/passwd")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("escape should be 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRelativeSVGImageIsRenderedAndServed(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("![diagram](images/diagram.svg)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle cx="5" cy="5" r="5"/></svg>`
	if err := os.WriteFile(filepath.Join(root, "images", "diagram.svg"), []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newMultiRootServer(t, []string{root})
	h := s.Handler()

	rr := getReq(t, h, "/api/render?path=README.md")
	if rr.Code != http.StatusOK {
		t.Fatalf("render status %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `src=\"/raw/images/diagram.svg\"`) {
		t.Fatalf("SVG source was not rewritten: %s", rr.Body.String())
	}

	rr = getReq(t, h, "/raw/images/diagram.svg")
	if rr.Code != http.StatusOK {
		t.Fatalf("raw SVG status %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("SVG Content-Type = %q, want image/svg+xml", got)
	}
}

func TestTreeJSONPayloadMuchSmallerThanHTML(t *testing.T) {
	root := t.TempDir()
	const n = 1000
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("note-%04d.md", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := newMultiRootServer(t, []string{root})
	h := s.Handler()

	rr := getReq(t, h, "/api/tree")
	if rr.Code != http.StatusOK {
		t.Fatalf("tree status %d: %s", rr.Code, rr.Body.String())
	}
	jsonBytes := rr.Body.Len()
	var node tree.Node
	if err := json.Unmarshal(rr.Body.Bytes(), &node); err != nil {
		t.Fatalf("tree JSON: %v", err)
	}
	legacy := legacyTreeHTMLSize(&node)
	if jsonBytes*5 > legacy {
		t.Fatalf("JSON payload %d bytes is not far smaller than legacy HTML %d bytes", jsonBytes, legacy)
	}

	page := getReq(t, h, "/")
	if page.Code != http.StatusOK {
		t.Fatalf("page status %d", page.Code)
	}
	body := page.Body.String()
	if strings.Contains(body, `class="tree-svg"`) {
		t.Fatal("page still embeds per-row tree SVGs")
	}
	if !strings.Contains(body, `"isDir"`) {
		t.Fatal("page should embed tree JSON")
	}
	if strings.Count(body, "<svg") > 20 {
		t.Fatalf("page has too many inline SVGs (%d); tree rows should use CSS icons", strings.Count(body, "<svg"))
	}
}

func TestTreeHTMLEndpointRemoved(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newMultiRootServer(t, []string{root})
	rr := getReq(t, s.Handler(), "/api/tree-html")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("/api/tree-html should be gone, got %d", rr.Code)
	}
}

func TestDirListingUsesCSSIcons(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.md"), []byte("# a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newMultiRootServer(t, []string{root})
	rr := getReq(t, s.Handler(), "/api/render?path=sub")
	if rr.Code != http.StatusOK {
		t.Fatalf("render status %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `class=\"listing-icon\"`) && !strings.Contains(body, `class="listing-icon"`) {
		t.Fatalf("directory listing missing CSS icon class: %s", body)
	}
	if strings.Contains(body, `class=\"tree-svg\"`) || strings.Contains(body, `class="tree-svg"`) {
		t.Fatalf("directory listing still uses inline SVG: %s", body)
	}
}

// legacyTreeHTMLSize is the old per-row HTML encoding (inline SVGs) used as a
// baseline so the JSON payload test stays an order-of-magnitude check.
func legacyTreeHTMLSize(node *tree.Node) int {
	const (
		caretRight = `<svg class="tree-caret" viewBox="0 0 12 12" width="12" height="12" aria-hidden="true"><path d="M4.5 2.5 8 6l-3.5 3.5"/></svg>`
		iconFolder = `<svg class="tree-svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M1.5 3.25c0-.41.34-.75.75-.75h3.19c.2 0 .39.08.53.22l1.06 1.06h7.22c.41 0 .75.34.75.75v7.94c0 .41-.34.75-.75.75H2.25a.75.75 0 0 1-.75-.75V3.25Z"/></svg>`
		iconFile   = `<svg class="tree-svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M3 1.75c0-.14.11-.25.25-.25h6.19l3.31 3.31v9.44c0 .14-.11.25-.25.25H3.25a.25.25 0 0 1-.25-.25V1.75Z"/><path d="M9.25 1.75V4.5c0 .14.11.25.25.25h2.75"/></svg>`
	)
	var b strings.Builder
	var write func(n *tree.Node)
	write = func(n *tree.Node) {
		if n.IsDir {
			b.WriteString(`<li class="tree-item collapsed" data-dir="true" data-name="` + n.Name + `">`)
			b.WriteString(`<span class="tree-label"><span class="tree-toggle">` + caretRight + `</span><span class="tree-icon">` + iconFolder + `</span>` + n.Name + `</span>`)
			b.WriteString(`<ul class="tree-list">`)
			for _, c := range n.Children {
				write(c)
			}
			b.WriteString("</ul></li>")
			return
		}
		b.WriteString(`<li class="tree-item" data-dir="false" data-name="` + n.Name + `">`)
		b.WriteString(`<a class="tree-label" href="/view/` + n.Path + `" data-path="` + n.Path + `"><span class="tree-toggle"></span><span class="tree-icon">` + iconFile + `</span>` + n.Name + `</a></li>`)
	}
	b.WriteString(`<ul class="tree-list"><li class="tree-item" data-dir="true" data-name="` + node.Name + `">`)
	b.WriteString(`<span class="tree-label"><span class="tree-toggle">` + caretRight + `</span><span class="tree-icon">` + iconFolder + `</span>` + node.Name + `</span><ul class="tree-list">`)
	for _, c := range node.Children {
		write(c)
	}
	b.WriteString("</ul></li></ul>")
	return b.Len()
}
