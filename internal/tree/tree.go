// Package tree builds a filtered directory tree of the served root for the
// sidebar, honoring show globs, default ignores, and the --hidden flag.
package tree

import (
	"io/fs"
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

func build(opts Options, absDir, relDir string) ([]*Node, error) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	var dirs, files []*Node
	for _, e := range entries {
		name := e.Name()
		if opts.ignored(name) {
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
