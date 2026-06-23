// Package tree builds a filtered directory tree of the served root for the
// sidebar, honoring show globs, default ignores, and the --hidden flag.
package tree

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Node is a single entry in the tree. Path is the slash-separated path relative
// to the root ("" for the root itself).
type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	IsDir    bool    `json:"isDir"`
	Children []*Node `json:"children,omitempty"`
}

// Options controls tree construction.
type Options struct {
	Root   string
	Show   []string // glob patterns matched against base filename
	Ignore []string // names/patterns excluded entirely
	Hidden bool     // include dotfiles/dot-directories
	Logger *log.Logger
}

// Mount pairs a display label with an absolute root path. In multi-root mode,
// each root is shown as a top-level tree entry whose paths are prefixed by
// Label+"/". For a single root, Label is "" so paths stay relative to root
// (preserving the classic single-root URL scheme).
type Mount struct {
	Label string
	Abs   string
}

// MakeMounts builds the mount table for the given roots. A single root gets an
// empty label. Multiple roots are labeled by basename; collisions are
// disambiguated with a "-2", "-3", ... suffix so labels stay unique.
func MakeMounts(roots []string) []Mount {
	if len(roots) <= 1 {
		if len(roots) == 1 {
			return []Mount{{Label: "", Abs: filepath.Clean(roots[0])}}
		}
		return nil
	}
	used := make(map[string]bool, len(roots))
	mounts := make([]Mount, 0, len(roots))
	for _, r := range roots {
		base := filepath.Base(r)
		label := base
		n := 2
		for used[label] {
			label = fmt.Sprintf("%s-%d", base, n)
			n++
		}
		used[label] = true
		mounts = append(mounts, Mount{Label: label, Abs: filepath.Clean(r)})
	}
	return mounts
}

// Build walks the root and returns the filtered tree. Directories that contain
// no shown files (transitively) are omitted. The returned root Node has Path ""
// and IsDir true.
func Build(opts Options) (*Node, error) {
	root := &Node{Name: filepath.Base(opts.Root), Path: "", IsDir: true}
	children, err := build(opts, opts.Root, "")
	if err != nil {
		return nil, err
	}
	root.Children = children
	return root, nil
}

// BuildMulti builds a virtual root whose children are one directory node per
// mount. Paths within each mount are prefixed by the mount's label. The
// returned node has Path "" and IsDir true; its Children are the per-mount
// nodes (each with Path = m.Label). Unreadable roots are skipped.
func BuildMulti(mounts []Mount, opts Options) (*Node, error) {
	virtual := &Node{Name: "", Path: "", IsDir: true}
	for _, m := range mounts {
		children, err := build(opts, m.Abs, m.Label)
		if err != nil {
			continue
		}
		virtual.Children = append(virtual.Children, &Node{
			Name:     m.Label,
			Path:     m.Label,
			IsDir:    true,
			Children: children,
		})
	}
	return virtual, nil
}

func build(opts Options, absDir, relDir string) ([]*Node, error) {
	if opts.Logger != nil {
		opts.Logger.Printf("scan: %s", absDir)
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	var dirs, files []*Node
	for _, e := range entries {
		name := e.Name()
		if opts.ignored(name) {
			if opts.Logger != nil {
				opts.Logger.Printf("  skip: %s (ignored)", filepath.Join(absDir, name))
			}
			continue
		}
		// Resolve symlinks for type but never traverse outside root.
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		rel := joinRel(relDir, name)

		if e.IsDir() || (info.Mode()&fs.ModeSymlink != 0 && isDir(filepath.Join(absDir, name))) {
			childAbs := filepath.Join(absDir, name)
			kids, berr := build(opts, childAbs, rel)
			if berr != nil {
				// Skip unreadable directories rather than failing the whole tree.
				continue
			}
			if len(kids) == 0 {
				continue // hide empty directories
			}
			dirs = append(dirs, &Node{Name: name, Path: rel, IsDir: true, Children: kids})
		} else {
			if opts.shown(name) {
				if opts.Logger != nil {
					opts.Logger.Printf("  file: %s", filepath.Join(absDir, name))
				}
				files = append(files, &Node{Name: name, Path: rel, IsDir: false})
			}
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return append(dirs, files...), nil
}

func (o Options) ignored(name string) bool {
	if !o.Hidden && strings.HasPrefix(name, ".") {
		return true
	}
	for _, pat := range o.Ignore {
		if name == pat {
			return true
		}
		if ok, _ := path.Match(pat, name); ok {
			return true
		}
	}
	return false
}

func (o Options) shown(name string) bool {
	for _, pat := range o.Show {
		if pat == "*" {
			return true
		}
		if ok, _ := path.Match(pat, name); ok {
			return true
		}
	}
	return false
}

func joinRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func isDir(p string) bool {
	st, err := os.Stat(p) // follows symlinks
	return err == nil && st.IsDir()
}
