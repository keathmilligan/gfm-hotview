// Package server wires the HTTP layer: embedded assets, the SPA-lite shell,
// JSON/HTML fragment APIs, raw file serving (path-contained), and SSE live
// reload. All file access is confined to the configured root.
package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/local/gfm-hotview/internal/config"
	"github.com/local/gfm-hotview/internal/render"
	"github.com/local/gfm-hotview/internal/tree"
	"github.com/local/gfm-hotview/web"
)

// Server is the HTTP application.
type Server struct {
	cfg      *config.Config
	renderer *render.Renderer
	tmpl     *template.Template
	assets   fs.FS
	hub      *sseHub
	logger   *log.Logger
}

// New constructs a Server.
func New(cfg *config.Config, logger *log.Logger) (*Server, error) {
	tmpl, err := template.ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	assets, err := fs.Sub(web.Assets, "assets")
	if err != nil {
		return nil, fmt.Errorf("loading assets: %w", err)
	}
	return &Server{
		cfg:      cfg,
		renderer: render.New(cfg.Mode == config.ModeGFM),
		tmpl:     tmpl,
		assets:   assets,
		hub:      newSSEHub(),
		logger:   logger,
	}, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/view/", s.handleView)
	mux.HandleFunc("/raw/", s.handleRaw)
	mux.HandleFunc("/api/tree", s.handleAPITree)
	mux.HandleFunc("/api/tree-html", s.handleAPITreeHTML)
	mux.HandleFunc("/api/render", s.handleAPIRender)
	mux.HandleFunc("/user.css", s.handleUserCSS)
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(s.assets))))
	if !s.cfg.NoReload {
		mux.HandleFunc("/events", s.hub.serveHTTP)
	}
	return s.logMiddleware(mux)
}

// NotifyContent / NotifyTree / NotifyCSS push live-reload events.
func (s *Server) NotifyContent() { s.hub.broadcast("content", "1") }
func (s *Server) NotifyTree()    { s.hub.broadcast("tree", "1") }
func (s *Server) NotifyCSS()     { s.hub.broadcast("css", "1") }

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Verbose {
			s.logger.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

// ---- Page shell ----

type pageData struct {
	Title            string
	BrandName        string
	Theme            string
	TreeHTML         template.HTML
	BreadcrumbHTML   template.HTML
	ContentHTML      template.HTML
	TOCHTML          template.HTML
	InitialPathJSON  template.JS
	Reload           bool
	HasVendorKatex   bool
	HasVendorMermaid bool
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	rel := s.cfg.OpenPage
	if rel == "" {
		rel = s.detectIndex("")
	}
	s.renderShell(w, r, rel)
}

func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/view/")
	rel = decodePath(rel)
	s.renderShell(w, r, rel)
}

func (s *Server) renderShell(w http.ResponseWriter, r *http.Request, rel string) {
	treeHTML, err := s.treeHTML()
	if err != nil {
		http.Error(w, "tree error", http.StatusInternalServerError)
		return
	}

	doc := s.renderDoc(rel)

	jsonPath, _ := json.Marshal(rel)
	data := pageData{
		Title:            orDefault(doc.title, "gfm-hotview"),
		BrandName:        filepath.Base(s.cfg.Root),
		Theme:            string(s.cfg.Theme),
		TreeHTML:         template.HTML(treeHTML),
		BreadcrumbHTML:   template.HTML(doc.breadcrumb),
		ContentHTML:      template.HTML(doc.html),
		TOCHTML:          "",
		InitialPathJSON:  template.JS(jsonPath),
		Reload:           !s.cfg.NoReload,
		HasVendorKatex:   s.vendorExists("vendor/katex.min.js"),
		HasVendorMermaid: s.vendorExists("vendor/mermaid.min.js"),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "page.html", data); err != nil {
		s.logger.Printf("template error: %v", err)
	}
}

// ---- APIs ----

func (s *Server) handleAPITree(w http.ResponseWriter, r *http.Request) {
	node, err := s.buildTree()
	if err != nil {
		http.Error(w, "tree error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, node)
}

func (s *Server) handleAPITreeHTML(w http.ResponseWriter, r *http.Request) {
	html, err := s.treeHTML()
	if err != nil {
		http.Error(w, "tree error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

type renderResponse struct {
	HTML       string           `json:"html"`
	Title      string           `json:"title"`
	Breadcrumb string           `json:"breadcrumb"`
	Headings   []render.Heading `json:"headings"`
}

func (s *Server) handleAPIRender(w http.ResponseWriter, r *http.Request) {
	rel := decodePath(r.URL.Query().Get("path"))
	doc := s.renderDoc(rel)
	if doc.notFound {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, renderResponse{
		HTML:       doc.html,
		Title:      doc.title,
		Breadcrumb: doc.breadcrumb,
		Headings:   doc.headings,
	})
}

func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	rel := decodePath(strings.TrimPrefix(r.URL.Path, "/raw/"))
	abs, err := resolveSafe(s.cfg.Root, rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if s.isExcluded(rel) {
		http.NotFound(w, r)
		return
	}
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(abs)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeFile(w, r, abs)
}

func (s *Server) handleUserCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	if s.cfg.CSSDir == "" {
		return
	}
	entries, err := os.ReadDir(s.cfg.CSSDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".css") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.cfg.CSSDir, e.Name()))
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "/* %s */\n", e.Name())
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n"))
	}
}

// ---- Rendering helpers ----

type docResult struct {
	html       string
	title      string
	breadcrumb string
	headings   []render.Heading
	notFound   bool
}

var headingRe = regexp.MustCompile(`(?s)<h([1-6]) id="([^"]+)">`)

// renderDoc renders the markdown at rel, or a directory listing if rel is a
// directory (after index detection), or a not-found marker.
func (s *Server) renderDoc(rel string) docResult {
	if rel == "" {
		// landing with no detectable index: show root listing
		return docResult{
			html:       s.dirListingHTML(""),
			title:      filepath.Base(s.cfg.Root),
			breadcrumb: s.breadcrumbHTML(""),
		}
	}

	abs, err := resolveSafe(s.cfg.Root, rel)
	if err != nil || s.isExcluded(rel) {
		return docResult{notFound: true}
	}
	st, err := os.Stat(abs)
	if err != nil {
		return docResult{notFound: true}
	}
	if st.IsDir() {
		if idx := s.detectIndex(rel); idx != "" {
			return s.renderDoc(idx)
		}
		return docResult{
			html:       s.dirListingHTML(rel),
			title:      filepath.Base(abs),
			breadcrumb: s.breadcrumbHTML(rel),
		}
	}

	src, err := os.ReadFile(abs)
	if err != nil {
		return docResult{notFound: true}
	}
	res, err := s.renderer.Render(src)
	if err != nil {
		return docResult{
			html:       "<p class=\"error\">render error: " + html.EscapeString(err.Error()) + "</p>",
			title:      filepath.Base(abs),
			breadcrumb: s.breadcrumbHTML(rel),
		}
	}

	// Inject heading anchor links (GitHub-style) for headings that carry an id.
	htmlOut := headingRe.ReplaceAllString(res.HTML,
		`<h$1 id="$2"><a class="heading-anchor" href="#$2" aria-label="Permalink">#</a>`)

	// Tag task-list items so the GitHub-style CSS applies.
	htmlOut = strings.ReplaceAll(htmlOut, `<li><input `, `<li class="task-list-item"><input `)

	title := res.Title
	if title == "" {
		title = filepath.Base(abs)
	}
	return docResult{
		html:       htmlOut,
		title:      title,
		breadcrumb: s.breadcrumbHTML(rel),
		headings:   res.Headings,
	}
}

// detectIndex returns the relative path of a README/index file within dir
// (relative path), or "" if none.
func (s *Server) detectIndex(dir string) string {
	absDir, err := resolveSafe(s.cfg.Root, dir)
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return ""
	}
	want := []string{"readme.md", "index.md", "readme.markdown", "index.markdown"}
	lower := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			lower[strings.ToLower(e.Name())] = e.Name()
		}
	}
	for _, w := range want {
		if real, ok := lower[w]; ok {
			if dir == "" {
				return real
			}
			return dir + "/" + real
		}
	}
	return ""
}

func (s *Server) dirListingHTML(rel string) string {
	node, err := s.buildTree()
	if err != nil {
		return "<p>unable to list directory</p>"
	}
	target := node
	if rel != "" {
		target = findNode(node, rel)
	}
	if target == nil || len(target.Children) == 0 {
		return "<p>No markdown files found in this directory.</p>"
	}
	var b strings.Builder
	b.WriteString("<h1>" + html.EscapeString(orDefault(rel, filepath.Base(s.cfg.Root))) + "</h1>")
	b.WriteString(`<ul class="dir-listing">`)
	for _, c := range target.Children {
		icon := iconFile
		if c.IsDir {
			icon = iconFolder
		}
		href := "/view/" + pathEscape(c.Path)
		b.WriteString(`<li>` + icon + ` <a href="` + href + `" data-path="` + html.EscapeString(c.Path) + `">` + html.EscapeString(c.Name) + `</a></li>`)
	}
	b.WriteString("</ul>")
	return b.String()
}

func (s *Server) breadcrumbHTML(rel string) string {
	var b strings.Builder
	b.WriteString(`<a href="/" data-path="">` + html.EscapeString(filepath.Base(s.cfg.Root)) + `</a>`)
	if rel == "" {
		return b.String()
	}
	parts := strings.Split(rel, "/")
	acc := ""
	for i, p := range parts {
		if acc == "" {
			acc = p
		} else {
			acc += "/" + p
		}
		b.WriteString(" / ")
		if i == len(parts)-1 {
			b.WriteString(html.EscapeString(p))
		} else {
			b.WriteString(`<a href="/view/` + pathEscape(acc) + `" data-path="` + html.EscapeString(acc) + `">` + html.EscapeString(p) + `</a>`)
		}
	}
	return b.String()
}

// ---- Tree HTML ----

func (s *Server) buildTree() (*tree.Node, error) {
	return tree.Build(tree.Options{
		Root:   s.cfg.Root,
		Show:   s.cfg.Show,
		Ignore: s.cfg.Ignore,
		Hidden: s.cfg.Hidden,
	})
}

func (s *Server) treeHTML() (string, error) {
	node, err := s.buildTree()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<ul class="tree-list">`)
	for _, c := range node.Children {
		writeTreeNode(&b, c)
	}
	b.WriteString("</ul>")
	return b.String(), nil
}

// Outline-style icons drawn with currentColor strokes (no fill); they inherit
// text color.
const (
	caretRight = `<svg class="tree-caret" viewBox="0 0 12 12" width="12" height="12" aria-hidden="true"><path d="M4.5 2.5 8 6l-3.5 3.5"/></svg>`
	iconFolder = `<svg class="tree-svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M1.5 3.25c0-.41.34-.75.75-.75h3.19c.2 0 .39.08.53.22l1.06 1.06h7.22c.41 0 .75.34.75.75v7.94c0 .41-.34.75-.75.75H2.25a.75.75 0 0 1-.75-.75V3.25Z"/></svg>`
	iconFile   = `<svg class="tree-svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M3 1.75c0-.14.11-.25.25-.25h6.19l3.31 3.31v9.44c0 .14-.11.25-.25.25H3.25a.25.25 0 0 1-.25-.25V1.75Z"/><path d="M9.25 1.75V4.5c0 .14.11.25.25.25h2.75"/></svg>`)

func writeTreeNode(b *strings.Builder, n *tree.Node) {
	if n.IsDir {
		// Collapsed by default.
		b.WriteString(`<li class="tree-item collapsed" data-dir="true" data-name="` + html.EscapeString(n.Name) + `">`)
		b.WriteString(`<span class="tree-label"><span class="tree-toggle">` + caretRight + `</span><span class="tree-icon">` + iconFolder + `</span>` + html.EscapeString(n.Name) + `</span>`)
		b.WriteString(`<ul class="tree-list">`)
		for _, c := range n.Children {
			writeTreeNode(b, c)
		}
		b.WriteString("</ul></li>")
		return
	}
	href := "/view/" + pathEscape(n.Path)
	b.WriteString(`<li class="tree-item" data-dir="false" data-name="` + html.EscapeString(n.Name) + `">`)
	b.WriteString(`<a class="tree-label" href="` + href + `" data-path="` + html.EscapeString(n.Path) + `"><span class="tree-toggle"></span><span class="tree-icon">` + iconFile + `</span>` + html.EscapeString(n.Name) + `</a>`)
	b.WriteString("</li>")
}

// ---- misc ----

func (s *Server) isExcluded(rel string) bool {
	if rel == "" {
		return false
	}
	for _, part := range strings.Split(rel, "/") {
		if !s.cfg.Hidden && strings.HasPrefix(part, ".") {
			return true
		}
		for _, ig := range s.cfg.Ignore {
			if part == ig {
				return true
			}
			if ok, _ := path.Match(ig, part); ok {
				return true
			}
		}
	}
	return false
}

func (s *Server) vendorExists(name string) bool {
	f, err := s.assets.Open(name)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func findNode(n *tree.Node, rel string) *tree.Node {
	if n.Path == rel {
		return n
	}
	for _, c := range n.Children {
		if found := findNode(c, rel); found != nil {
			return found
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func decodePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if dec, err := decodeSegment(seg); err == nil {
			parts[i] = dec
		}
	}
	return strings.TrimPrefix(strings.Join(parts, "/"), "/")
}

func decodeSegment(s string) (string, error) {
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			var b byte
			_, err := fmt.Sscanf(s[i+1:i+3], "%02x", &b)
			if err != nil {
				return s, err
			}
			buf.WriteByte(b)
			i += 2
		} else {
			buf.WriteByte(s[i])
		}
	}
	return buf.String(), nil
}

func pathEscape(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = urlEscapeSegment(s)
	}
	return strings.Join(segs, "/")
}

func urlEscapeSegment(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
