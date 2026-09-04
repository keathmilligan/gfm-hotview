package server

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/gfm-hotview/internal/config"
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
