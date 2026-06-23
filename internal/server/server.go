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
	"time"

	"github.com/local/gfm-hotview/internal/config"
	"github.com/local/gfm-hotview/internal/render"
	"github.com/local/gfm-hotview/internal/tree"
	"github.com/local/gfm-hotview/web"
)

// Server is the HTTP application.
type Server struct {
	cfg      *config.Config
	mounts   []tree.Mount
	renderer *render.Renderer
	tmpl     *template.Template
	assets   fs.FS
	hub      *sseHub
	logger   *log.Logger
	version  string
}

// New constructs a Server.
func New(cfg *config.Config, logger *log.Logger, version string) (*Server, error) {
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
		mounts:   tree.MakeMounts(cfg.Roots),
		renderer: render.New(cfg.Mode == config.ModeGFM),
		tmpl:     tmpl,
		assets:   assets,
		hub:      newSSEHub(),
		logger:   logger,
		version:  version,
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
func (s *Server) NotifyContent() {
	if s.cfg.Debug {
		s.logger.Printf("sse: broadcast content")
	}
	s.hub.broadcast("content", "1")
}
func (s *Server) NotifyTree() {
	if s.cfg.Debug {
		s.logger.Printf("sse: broadcast tree")
	}
	s.hub.broadcast("tree", "1")
}
func (s *Server) NotifyCSS() {
	if s.cfg.Debug {
		s.logger.Printf("sse: broadcast css")
	}
	s.hub.broadcast("css", "1")
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if s.cfg.Debug {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			s.logger.Printf("req: %s %s -> %d (%dms)", r.Method, r.URL.Path, rec.status, time.Since(start).Milliseconds())
			return
		}
		if s.cfg.Verbose {
			s.logger.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder wraps http.ResponseWriter to capture the response status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

// ---- Page shell ----

type pageData struct {
	Title           string
	BrandName       string
	Version         string
	Theme           string
	TreeHTML        template.HTML
	BreadcrumbHTML  template.HTML
	ContentHTML     template.HTML
	TOCHTML         template.HTML
	InitialPathJSON template.JS
	Reload          bool
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	rel := s.cfg.OpenPage
	if rel == "" {
		if len(s.mounts) <= 1 {
			rel = s.detectIndex("")
		} // multi-root: leave "" to render the roots listing
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
		Title:           orDefault(doc.title, "gfm-hotview"),
		BrandName:       "gfm-hotview",
		Version:         s.version,
		Theme:           string(s.cfg.Theme),
		TreeHTML:        template.HTML(treeHTML),
		BreadcrumbHTML:  template.HTML(doc.breadcrumb),
		ContentHTML:     template.HTML(doc.html),
		TOCHTML:         "",
		InitialPathJSON: template.JS(jsonPath),
		Reload:          !s.cfg.NoReload,
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
	m, rest, ok := s.mountForRel(rel)
	if !ok {
		http.NotFound(w, r)
		return
	}
	abs, err := resolveSafe(m.Abs, rest)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if s.isExcluded(rest) {
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
	for _, dir := range s.cfg.CSSDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".css") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "/* %s */\n", e.Name())
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n"))
		}
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
var imgSrcRe = regexp.MustCompile(`<img([^>]*)\ssrc="([^"]*)"`)

// renderDoc renders the markdown at rel, or a directory listing if rel is a
// directory (after index detection), or a not-found marker.
func (s *Server) renderDoc(rel string) docResult {
	if s.cfg.Debug {
		s.logger.Printf("render: %q", rel)
	}
	if rel == "" {
		// landing with no detectable index: show root listing
		title := filepath.Base(s.cfg.Root)
		if len(s.mounts) > 1 {
			title = "roots"
		}
		return docResult{
			html:       s.dirListingHTML(""),
			title:      title,
			breadcrumb: s.breadcrumbHTML(""),
		}
	}

	m, rest, ok := s.mountForRel(rel)
	if !ok {
		return docResult{notFound: true}
	}
	abs, err := resolveSafe(m.Abs, rest)
	if err != nil || s.isExcluded(rest) {
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

	// Rewrite relative image src to /raw/ paths.
	docDir := path.Dir(rel)
	htmlOut = imgSrcRe.ReplaceAllStringFunc(htmlOut, func(m string) string {
		match := imgSrcRe.FindStringSubmatch(m)
		if match == nil {
			return m
		}
		attrs := match[1]
		src := match[2]
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") ||
			strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "#") ||
			strings.HasPrefix(src, "/") {
			return m
		}
		resolved, err := s.resolveAbs(path.Join(docDir, src))
		if err != nil {
			return m
		}
		relPath, err := s.absToNamespacedRel(resolved)
		if err != nil {
			return m
		}
		return fmt.Sprintf(`<img%s src="/raw/%s"`, attrs, pathEscape(relPath))
	})

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
	m, rest, ok := s.mountForRel(dir)
	if !ok {
		return ""
	}
	absDir, err := resolveSafe(m.Abs, rest)
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
	heading := rel
	if rel == "" {
		if len(s.mounts) > 1 {
			heading = "roots"
		} else {
			heading = filepath.Base(s.cfg.Root)
		}
	}
	var b strings.Builder
	b.WriteString("<h1>" + html.EscapeString(orDefault(heading, filepath.Base(s.cfg.Root))) + "</h1>")
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
	if len(s.mounts) > 1 {
		b.WriteString(`<a href="/" data-path="">roots</a>`)
	} else {
		b.WriteString(`<a href="/" data-path="">` + html.EscapeString(filepath.Base(s.cfg.Root)) + `</a>`)
	}
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
	opts := tree.Options{
		Show:   s.cfg.Show,
		Ignore: s.cfg.Ignore,
		Hidden: s.cfg.Hidden,
	}
	if s.cfg.Debug {
		opts.Logger = s.logger
	}
	if len(s.mounts) <= 1 {
		opts.Root = s.cfg.Root
		return tree.Build(opts)
	}
	return tree.BuildMulti(s.mounts, opts)
}

func (s *Server) treeHTML() (string, error) {
	node, err := s.buildTree()
	if err != nil {
		return "", err
	}
	// Top-level entries: for a single root the root node itself (expanded); for
	// multiple roots, one expanded folder per mount.
	top := node.Children
	if len(s.mounts) <= 1 {
		top = []*tree.Node{node}
	}
	// Map each top-level entry name to its absolute path for display.
	absPath := map[string]string{}
	if len(s.mounts) <= 1 {
		absPath[node.Name] = s.cfg.Root
	} else {
		for _, m := range s.mounts {
			absPath[m.Label] = m.Abs
		}
	}
	var b strings.Builder
	b.WriteString(`<ul class="tree-list">`)
	for _, c := range top {
		b.WriteString(`<li class="tree-item" data-dir="true" data-name="` + html.EscapeString(c.Name) + `">`)
		b.WriteString(`<span class="tree-label"><span class="tree-toggle">` + caretRight + `</span><span class="tree-icon">` + iconFolder + `</span>` + html.EscapeString(c.Name))
		if ap := absPath[c.Name]; ap != "" {
			b.WriteString(`<span class="tree-root-path">` + html.EscapeString(ap) + `</span>`)
		}
		b.WriteString(`</span>`)
		b.WriteString(`<ul class="tree-list">`)
		for _, cc := range c.Children {
			writeTreeNode(&b, cc)
		}
		b.WriteString("</ul></li>")
	}
	b.WriteString("</ul>")
	return b.String(), nil
}

// Outline-style icons drawn with currentColor strokes (no fill); they inherit
// text color.
const (
	caretRight = `<svg class="tree-caret" viewBox="0 0 12 12" width="12" height="12" aria-hidden="true"><path d="M4.5 2.5 8 6l-3.5 3.5"/></svg>`
	iconFolder = `<svg class="tree-svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M1.5 3.25c0-.41.34-.75.75-.75h3.19c.2 0 .39.08.53.22l1.06 1.06h7.22c.41 0 .75.34.75.75v7.94c0 .41-.34.75-.75.75H2.25a.75.75 0 0 1-.75-.75V3.25Z"/></svg>`
	iconFile   = `<svg class="tree-svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M3 1.75c0-.14.11-.25.25-.25h6.19l3.31 3.31v9.44c0 .14-.11.25-.25.25H3.25a.25.25 0 0 1-.25-.25V1.75Z"/><path d="M9.25 1.75V4.5c0 .14.11.25.25.25h2.75"/></svg>`
)

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

// mountForRel splits a slash-separated request path into the mount it belongs
// to and the remainder within that mount. For a single root (label ""), rel is
// returned unchanged. ok is false when rel does not map to any mount.
func (s *Server) mountForRel(rel string) (m tree.Mount, rest string, ok bool) {
	rel = strings.TrimPrefix(rel, "/")
	if len(s.mounts) == 1 && s.mounts[0].Label == "" {
		return s.mounts[0], rel, true
	}
	var label string
	if i := strings.IndexByte(rel, '/'); i < 0 {
		label, rest = rel, ""
	} else {
		label, rest = rel[:i], rel[i+1:]
	}
	for _, m = range s.mounts {
		if m.Label == label {
			return m, rest, true
		}
	}
	return tree.Mount{}, "", false
}

// resolveAbs maps a (possibly namespaced) relative path to an absolute path
// confined to its mount.
func (s *Server) resolveAbs(rel string) (string, error) {
	m, rest, ok := s.mountForRel(rel)
	if !ok {
		return "", errOutOfRoot
	}
	return resolveSafe(m.Abs, rest)
}

// absToNamespacedRel is the inverse of resolveAbs: it maps an absolute path
// back to a slash-separated, mount-namespaced relative path.
func (s *Server) absToNamespacedRel(abs string) (string, error) {
	absClean := filepath.Clean(abs)
	for _, m := range s.mounts {
		mAbs := filepath.Clean(m.Abs)
		if absClean == mAbs {
			return m.Label, nil
		}
		if strings.HasPrefix(absClean, mAbs+string(filepath.Separator)) {
			rel, err := filepath.Rel(mAbs, absClean)
			if err != nil {
				return "", err
			}
			rel = filepath.ToSlash(rel)
			if m.Label == "" {
				return rel, nil
			}
			return m.Label + "/" + rel, nil
		}
	}
	return "", errOutOfRoot
}

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
