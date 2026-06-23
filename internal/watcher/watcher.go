// Package watcher watches the served tree (and the .gfm-hotview/css overrides) for
// changes and emits debounced events describing what changed.
package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Kind classifies a coalesced change event.
type Kind int

const (
	// KindContent means a watched file's content changed (re-render the view).
	KindContent Kind = iota
	// KindTree means the directory structure changed (refresh the sidebar).
	KindTree
	// KindCSS means a .gfm-hotview/css override changed (re-apply styles).
	KindCSS
)

// Event is a debounced change notification.
type Event struct {
	Kinds map[Kind]bool
}

// Has reports whether the event includes the given kind.
func (e Event) Has(k Kind) bool { return e.Kinds[k] }

// Watcher recursively watches one or more root directories.
type Watcher struct {
	roots   []string
	cssDirs []string
	ignore  []string
	fsw     *fsnotify.Watcher
	Events  chan Event
	done    chan struct{}
	pending map[Kind]bool
	logger  *log.Logger
}

// New creates a Watcher over the given roots. cssDirs lists CSS override
// directories (may be empty). ignore lists base names of directories to skip
// (e.g. .git, node_modules). logger, when non-nil, receives detailed trace
// output (raw fs events, watch additions, debounced flushes).
func New(roots []string, cssDirs []string, ignore []string, logger *log.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		roots:   roots,
		cssDirs: cssDirs,
		ignore:  ignore,
		fsw:     fsw,
		Events:  make(chan Event, 8),
		done:    make(chan struct{}),
		pending: map[Kind]bool{},
		logger:  logger,
	}
	for _, r := range roots {
		if err := w.addRecursive(r); err != nil {
			_ = fsw.Close()
			return nil, err
		}
	}
	return w, nil
}

// Run processes raw fs events, debouncing bursts into coalesced Events. It
// blocks until Close is called.
func (w *Watcher) Run() {
	const debounce = 150 * time.Millisecond
	var timer *time.Timer
	var timerC <-chan time.Time

	flush := func() {
		if len(w.pending) == 0 {
			return
		}
		if w.logger != nil {
			var ks []string
			for k := range w.pending {
				ks = append(ks, kindName(k))
			}
			w.logger.Printf("watch: flush %s", strings.Join(ks, ","))
		}
		ev := Event{Kinds: w.pending}
		w.pending = map[Kind]bool{}
		select {
		case w.Events <- ev:
		case <-w.done:
		}
	}

	for {
		select {
		case <-w.done:
			return
		case e, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if w.logger != nil {
				w.logger.Printf("watch: %s %s", e.Op, e.Name)
			}
			w.classify(e)
			if timer == nil {
				timer = time.NewTimer(debounce)
				timerC = timer.C
			} else {
				timer.Reset(debounce)
			}
		case <-timerC:
			flush()
			timer = nil
			timerC = nil
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) classify(e fsnotify.Event) {
	name := e.Name

	// New directories must be watched; removed structure changes the tree.
	if e.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
		w.pending[KindTree] = true
		if e.Op&fsnotify.Create != 0 {
			if st, err := os.Stat(name); err == nil && st.IsDir() && !w.skip(name) {
				_ = w.addRecursive(name)
			}
		}
	}

	if strings.HasSuffix(strings.ToLower(name), ".css") {
		for _, cd := range w.cssDirs {
			if strings.HasPrefix(name, cd) {
				w.pending[KindCSS] = true
				return
			}
		}
	}

	if e.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
		w.pending[KindContent] = true
	}
}

func (w *Watcher) addRecursive(dir string) error {
	return filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if !d.IsDir() {
			return nil
		}
		if !w.isRoot(p) && w.skip(p) {
			return filepath.SkipDir
		}
		_ = w.fsw.Add(p)
		if w.logger != nil {
			w.logger.Printf("watch: add %s", p)
		}
		return nil
	})
}

// kindName returns a human-readable name for a change Kind.
func kindName(k Kind) string {
	switch k {
	case KindContent:
		return "content"
	case KindTree:
		return "tree"
	case KindCSS:
		return "css"
	}
	return "unknown"
}

// isRoot reports whether p is one of the watched roots.
func (w *Watcher) isRoot(p string) bool {
	for _, r := range w.roots {
		if p == r {
			return true
		}
	}
	return false
}

// skip reports whether a directory should not be watched. The .gfm-hotview dir is
// watched only for its css subdir.
func (w *Watcher) skip(dir string) bool {
	base := filepath.Base(dir)
	for _, ig := range w.ignore {
		if base == ig {
			// Still allow watching .gfm-hotview so css changes are seen.
			if ig == ".gfm-hotview" {
				return false
			}
			return true
		}
	}
	return false
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsw.Close()
}
