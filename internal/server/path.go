package server

import (
	"errors"
	"path/filepath"
	"strings"
)

// errOutOfRoot indicates a requested path escapes the served root.
var errOutOfRoot = errors.New("path escapes root")

// resolveSafe maps a slash-separated relative request path to an absolute path
// confined to root. It rejects absolute inputs, "..", and any result (after
// resolving symlinks) that lands outside root.
func resolveSafe(root, rel string) (string, error) {
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return root, nil
	}
	// Disallow absolute or drive-letter style inputs.
	if filepath.IsAbs(rel) {
		return "", errOutOfRoot
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errOutOfRoot
	}
	abs := filepath.Join(root, clean)

	// Lexical containment check.
	rootClean := filepath.Clean(root)
	if abs != rootClean && !strings.HasPrefix(abs, rootClean+string(filepath.Separator)) {
		return "", errOutOfRoot
	}

	// Resolve symlinks and re-check containment, when the target exists.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		rootResolved, rerr := filepath.EvalSymlinks(rootClean)
		if rerr != nil {
			rootResolved = rootClean
		}
		if resolved != rootResolved && !strings.HasPrefix(resolved, rootResolved+string(filepath.Separator)) {
			return "", errOutOfRoot
		}
	}
	return abs, nil
}
