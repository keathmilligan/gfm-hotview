package render

import (
	"strings"
	"testing"
)

func TestRenderGFMFeatures(t *testing.T) {
	r := New(true)
	src := []byte(strings.Join([]string{
		"# Title",
		"",
		"| a | b |",
		"| - | - |",
		"| 1 | 2 |",
		"",
		"- [x] done",
		"- [ ] todo",
		"",
		"> [!NOTE]",
		"> Heads up.",
		"",
		"```mermaid",
		"graph TD",
		"A-->B",
		"```",
		"",
		":rocket:",
	}, "\n"))

	res, err := r.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	h := res.HTML

	checks := map[string]string{
		"table":    "<table>",
		"checkbox": "type=\"checkbox\"",
		"alert":    "markdown-alert-note",
		"mermaid":  `class="mermaid"`,
		"emoji":    "&#x1f680;",
	}
	for name, want := range checks {
		if !strings.Contains(h, want) {
			t.Errorf("%s: expected HTML to contain %q\n---\n%s", name, want, h)
		}
	}
	if res.Title != "Title" {
		t.Errorf("title = %q, want Title", res.Title)
	}
	if len(res.Headings) == 0 {
		t.Error("expected at least one heading")
	}
}

func TestFrontmatterStripped(t *testing.T) {
	r := New(true)
	src := []byte("---\ntitle: X\n---\n# Body\n")
	res, err := r.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.HTML, "title: X") {
		t.Errorf("frontmatter leaked into output: %s", res.HTML)
	}
	if !strings.Contains(res.HTML, "Body") {
		t.Errorf("body missing: %s", res.HTML)
	}
}

func TestHeadingIDsAssigned(t *testing.T) {
	r := New(true)
	res, err := r.Render([]byte("# Hello World\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Headings) != 1 || res.Headings[0].ID != "hello-world" {
		t.Fatalf("unexpected headings: %+v", res.Headings)
	}
}
