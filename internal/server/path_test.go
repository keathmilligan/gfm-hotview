package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSafe(t *testing.T) {
	root := t.TempDir()
	// Create a file inside root.
	if err := os.WriteFile(filepath.Join(root, "ok.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{"empty is root", "", false},
		{"simple file", "ok.md", false},
		{"nested", "a/b/c.md", false},
		{"dotdot escape", "../secret", true},
		{"dotdot deep", "a/../../etc/passwd", true},
		// A leading slash is treated as relative-to-root (HTTP path semantics),
		// so it stays contained rather than escaping.
		{"absolute-style stays in root", "/etc/passwd", false},
		{"leading slash trimmed", "/ok.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveSafe(root, tc.rel)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.rel)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.rel, err)
			}
		})
	}
}

func TestResolveSafeSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if _, err := resolveSafe(root, "link.md"); err == nil {
		t.Fatal("expected symlink escaping root to be rejected")
	}
}
