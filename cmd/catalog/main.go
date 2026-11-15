// Command catalog keeps a catalog.md current in every folder that holds a
// .catalog/ folder, inside the folders listed in global.toml.
//
// Usage:
//
//	catalog daemon
//	catalog daemon --once <folder>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"westarete.com/catalog/internal/daemon"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/settings"
	"westarete.com/catalog/internal/store"
)

const usage = "usage: catalog daemon [--once <folder>]"

// Exit codes: 1 for a failed run, 2 for a command line we don't understand.
const (
	exitFailed = 1
	exitUsage  = 2
)

// crawlEvery is how often the daemon crawls the watched folders.
const crawlEvery = time.Minute

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr)
	stop()
	os.Exit(code)
}

// run carries out the command line in args until it's done or ctx ends.
func run(ctx context.Context, args []string, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "daemon" {
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	flags := flag.NewFlagSet("catalog daemon", flag.ContinueOnError)
	flags.SetOutput(stderr)
	once := flags.Bool("once", false, "summarize every waiting file in `folder` once, then exit")
	if err := flags.Parse(args[1:]); err != nil {
		return exitUsage
	}
	var err error
	switch {
	case *once && flags.NArg() == 1:
		err = runOnce(ctx, flags.Arg(0))
	case !*once && flags.NArg() == 0:
		err = runLoop(ctx, stderr)
	default:
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "catalog: %v\n", err)
		return exitFailed
	}
	return 0
}

// runOnce crawls folder as the only watched folder and summarizes every file
// waiting in its catalogs.
func runOnce(ctx context.Context, folder string) error {
	stateDir, g, err := setup()
	if err != nil {
		return err
	}
	g.Watch = []string{folder}
	d, err := newDaemon(stateDir, g)
	if err != nil {
		return err
	}
	defer func() { _ = d.Store.Close() }()
	return d.Once(ctx, time.Now)
}

// runLoop keeps the catalogs in the folders global.toml watches current,
// rereading global.toml on each crawl, until ctx ends. It writes the errors
// it can't journal to stderr.
func runLoop(ctx context.Context, stderr io.Writer) error {
	stateDir, g, err := setup()
	if err != nil {
		return err
	}
	d, err := newDaemon(stateDir, g)
	if err != nil {
		return err
	}
	defer func() { _ = d.Store.Close() }()
	d.GlobalPath = filepath.Join(stateDir, settings.GlobalFile)
	d.Log = log.New(stderr, "catalog: ", log.LstdFlags)
	ticker := time.NewTicker(crawlEvery)
	defer ticker.Stop()
	return d.Run(ctx, time.Now, ticker.C)
}

// setup finds the state folder and loads global.toml from it.
func setup() (stateDir string, g settings.Global, err error) {
	stateDir, err = settings.StateDir()
	if err != nil {
		return "", g, err
	}
	g, err = loadGlobal(stateDir)
	return stateDir, g, err
}

// newDaemon opens the Store in stateDir and builds a daemon on it with the
// settings in g. The caller closes the Store.
func newDaemon(stateDir string, g settings.Global) (*daemon.Daemon, error) {
	s, err := store.Open(filepath.Join(stateDir, "store.db"))
	if err != nil {
		return nil, err
	}
	return &daemon.Daemon{
		Global:      g,
		Store:       s,
		Provider:    models.NewAnthropic(g.APIKey, g.Model),
		NewProvider: models.NewAnthropic,
		JournalPath: filepath.Join(stateDir, "journal.jsonl"),
		FS:          os.DirFS,
	}, nil
}

// loadGlobal reads global.toml, which must exist and hold an API key.
func loadGlobal(stateDir string) (settings.Global, error) {
	path := filepath.Join(stateDir, settings.GlobalFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings.Global{}, fmt.Errorf("no settings file at %s; create it with your api_key", path)
	}
	if err != nil {
		return settings.Global{}, err
	}
	g, err := settings.ParseGlobal(data)
	if err != nil {
		return settings.Global{}, fmt.Errorf("%s: %w", path, err)
	}
	if g.APIKey == "" {
		return settings.Global{}, fmt.Errorf("%s has no api_key", path)
	}
	return g, nil
}
