// Command gfm-hotview is a dependency-free, offline GitHub-Flavored Markdown
// viewer that serves a directory tree with a GitHub-like multi-panel UI, live
// reload, and optional browser auto-open.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/local/gfm-hotview/internal/browser"
	"github.com/local/gfm-hotview/internal/config"
	"github.com/local/gfm-hotview/internal/server"
	"github.com/local/gfm-hotview/internal/watcher"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gfm-hotview: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gfm-hotview", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gfm-hotview [options] [path]\n\nOptions:\n")
		fs.PrintDefaults()
	}

	var (
		port     = fs.Int("port", config.DefaultPort, "TCP port to bind (auto-selects next free if in use)")
		host     = fs.String("host", config.DefaultHost, "hostname/interface to bind")
		noOpen   = fs.Bool("no-open", false, "do not auto-open the browser")
		noReload = fs.Bool("no-reload", false, "disable live reload")
		theme    = fs.String("theme", string(config.ThemeAuto), "color theme: auto|light|dark")
		mode     = fs.String("mode", string(config.ModeGFM), "markdown dialect: gfm|markdown")
		show     = fs.String("show", "", "comma-separated globs of files shown in the tree")
		hidden   = fs.Bool("hidden", false, "include dotfiles/dot-directories")
		ignore   = fs.String("ignore", "", "additional comma-separated ignore globs")
		openPage = fs.String("open-page", "", "document to render first (relative to root)")
		cfgPath  = fs.String("config", "", "path to config file (default: .gfm-hotview/config.*)")
		noConfig = fs.Bool("no-config", false, "ignore config file and .gfm-hotview overrides")
		quiet    = fs.Bool("quiet", false, "suppress non-error log output")
		verbose  = fs.Bool("verbose", false, "verbose request/watch logging")
		showVer  = fs.Bool("version", false, "print version and exit")
	)
	// Short aliases.
	fs.IntVar(port, "p", config.DefaultPort, "alias for --port")
	fs.StringVar(host, "H", config.DefaultHost, "alias for --host")
	fs.StringVar(theme, "t", string(config.ThemeAuto), "alias for --theme")
	fs.StringVar(cfgPath, "c", "", "alias for --config")
	fs.BoolVar(quiet, "q", false, "alias for --quiet")
	fs.BoolVar(verbose, "v", false, "alias for --verbose")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVer {
		fmt.Println("gfm-hotview " + version)
		return nil
	}

	// Determine root and optional initial file.
	target := "."
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	root, initialFile, err := resolveTarget(target)
	if err != nil {
		return err
	}

	// Build flag set honoring "only override if explicitly set".
	fl := config.Flags{
		ConfigPath: *cfgPath,
		NoConfig:   *noConfig,
	}
	setFlags := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })
	isSet := func(names ...string) bool {
		for _, n := range names {
			if setFlags[n] {
				return true
			}
		}
		return false
	}
	if isSet("port", "p") {
		fl.Port = port
	}
	if isSet("host", "H") {
		fl.Host = host
	}
	if isSet("no-open") {
		fl.NoOpen = noOpen
	}
	if isSet("no-reload") {
		fl.NoReload = noReload
	}
	if isSet("theme", "t") {
		fl.Theme = theme
	}
	if isSet("mode") {
		fl.Mode = mode
	}
	if isSet("show") {
		fl.Show = show
	}
	if isSet("hidden") {
		fl.Hidden = hidden
	}
	if isSet("ignore") {
		fl.Ignore = ignore
	}
	if isSet("quiet", "q") {
		fl.Quiet = quiet
	}
	if isSet("verbose", "v") {
		fl.Verbose = verbose
	}
	// open-page: explicit flag wins, else the initial file (if a file was given).
	switch {
	case isSet("open-page"):
		fl.OpenPage = openPage
	case initialFile != "":
		fl.OpenPage = &initialFile
	}

	cfg, err := config.Resolve(root, fl)
	if err != nil {
		return err
	}

	logger := log.New(os.Stderr, "", 0)
	if cfg.Quiet {
		logger.SetOutput(devNull{})
	}

	srv, err := server.New(cfg, logger)
	if err != nil {
		return err
	}

	// Bind listener (with auto port selection).
	ln, addr, err := listen(cfg.Host, cfg.Port)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s/", addr)

	// Live reload watcher.
	var w *watcher.Watcher
	if !cfg.NoReload {
		w, err = watcher.New(cfg.Root, cfg.CSSDir, cfg.Ignore)
		if err != nil {
			logger.Printf("warning: live reload disabled (%v)", err)
		} else {
			go w.Run()
			go func() {
				for ev := range w.Events {
					if ev.Has(watcher.KindCSS) {
						srv.NotifyCSS()
					}
					if ev.Has(watcher.KindTree) {
						srv.NotifyTree()
					}
					if ev.Has(watcher.KindContent) {
						srv.NotifyContent()
					}
				}
			}()
		}
	}

	httpSrv := &http.Server{Handler: srv.Handler()}

	logger.Printf("gfm-hotview serving %s", cfg.Root)
	logger.Printf("listening on %s", url)

	// Auto-open browser.
	if !cfg.NoOpen {
		go func() {
			time.Sleep(150 * time.Millisecond)
			if err := browser.Open(url); err != nil {
				logger.Printf("could not open browser automatically: %v", err)
				logger.Printf("open this URL manually: %s", url)
			}
		}()
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	select {
	case <-ctx.Done():
		logger.Printf("shutting down…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		if w != nil {
			_ = w.Close()
		}
		return nil
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// resolveTarget returns the absolute root directory and, if target is a file,
// the file's path relative to that root.
func resolveTarget(target string) (root, initialFile string, err error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", "", fmt.Errorf("%q: %w", target, err)
	}
	if st.IsDir() {
		return abs, "", nil
	}
	dir := filepath.Dir(abs)
	rel, err := filepath.Rel(dir, abs)
	if err != nil {
		return "", "", err
	}
	return dir, filepath.ToSlash(rel), nil
}

// listen binds host:port, auto-selecting the next free port if the requested
// one is taken (unless port is 0, which lets the OS choose).
func listen(host string, port int) (net.Listener, string, error) {
	if port == 0 {
		ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
		if err != nil {
			return nil, "", err
		}
		return ln, ln.Addr().String(), nil
	}
	for p := port; p < port+100 && p <= 65535; p++ {
		ln, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", p)))
		if err == nil {
			return ln, fmt.Sprintf("%s:%d", host, p), nil
		}
		var ne *net.OpError
		if !errors.As(err, &ne) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("no free port found near %d", port)
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }
