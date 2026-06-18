package tree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildFiltersAndSorts(t *testing.T) {
	root := t.TempDir()
	mk := func(p string) {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("README.md")
	mk("zeta.md")
	mk("alpha.md")
	mk("notes.txt")        // not shown by default
	mk("sub/page.md")      // dir with md -> kept
	mk("empty/ignore.txt") // dir without md -> dropped
	mk(".gfm-hotview/config.toml")
	mk(".git/HEAD")

	node, err := Build(Options{
		Root:   root,
		Show:   []string{"*.md"},
		Ignore: []string{".git", ".gfm-hotview"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Expect: dir "sub" first, then files alpha, README, zeta (case-insensitive).
	var names []string
	for _, c := range node.Children {
		names = append(names, c.Name)
	}
	want := []string{"sub", "alpha.md", "README.md", "zeta.md"}
	if len(names) != len(want) {
		t.Fatalf("children = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("children = %v, want %v", names, want)
		}
	}
}

func TestBuildHidesDotfilesByDefault(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".secret.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "visible.md"), []byte("x"), 0o644)

	node, err := Build(Options{Root: root, Show: []string{"*.md"}, Ignore: nil})
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Children) != 1 || node.Children[0].Name != "visible.md" {
		t.Fatalf("dotfile not hidden: %+v", node.Children)
	}
}
