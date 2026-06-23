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

func TestMakeMountsSingleHasEmptyLabel(t *testing.T) {
	m := MakeMounts([]string{"/tmp/proj"})
	if len(m) != 1 || m[0].Label != "" || m[0].Abs != "/tmp/proj" {
		t.Fatalf("single root should get empty label, got %+v", m)
	}
	if MakeMounts(nil) != nil {
		t.Fatal("nil roots should yield nil mounts")
	}
}

func TestMakeMountsDisambiguates(t *testing.T) {
	m := MakeMounts([]string{"/a/proj", "/b/proj", "/c/other"})
	if len(m) != 3 {
		t.Fatalf("want 3 mounts, got %+v", m)
	}
	labels := map[string]bool{}
	for _, mm := range m {
		if labels[mm.Label] {
			t.Fatalf("duplicate label %q in %+v", mm.Label, m)
		}
		labels[mm.Label] = true
	}
	if m[0].Label != "proj" || m[1].Label != "proj-2" || m[2].Label != "other" {
		t.Fatalf("unexpected labels: %+v", m)
	}
}

func TestBuildMultiNamespacesPaths(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	mk := func(root, p string) {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(a, "README.md")
	mk(a, "sub/page.md")
	mk(b, "guide.md")

	mounts := MakeMounts([]string{a, b})
	node, err := BuildMulti(mounts, Options{Show: []string{"*.md"}, Ignore: nil})
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Children) != 2 {
		t.Fatalf("want 2 mounts, got %d", len(node.Children))
	}
	ma, mb := node.Children[0], node.Children[1]
	if ma.Name == "" || mb.Name == "" || ma.Name == mb.Name {
		t.Fatalf("mount labels must be non-empty and unique: %+v", node.Children)
	}
	if ma.Path != ma.Name || !ma.IsDir {
		t.Fatalf("mount node should be a dir labeled by its name: %+v", ma)
	}
	// Paths within each mount are prefixed by the mount label.
	paths := map[string]bool{}
	var collect func(n *Node)
	collect = func(n *Node) {
		if n.Path != "" {
			paths[n.Path] = true
		}
		for _, c := range n.Children {
			collect(c)
		}
	}
	collect(ma)
	collect(mb)
	want := []string{ma.Name + "/README.md", ma.Name + "/sub", ma.Name + "/sub/page.md", mb.Name + "/guide.md"}
	for _, w := range want {
		if !paths[w] {
			t.Fatalf("missing namespaced path %q in %+v", w, paths)
		}
	}
}
