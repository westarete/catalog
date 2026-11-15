// Command catalog keeps a catalog.md current in every folder that holds a
// .catalog/ folder, inside the folders listed in global.toml.
//
// Usage:
//
//	catalog daemon --once <folder>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
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

const usage = "usage: catalog daemon --once <folder>"

// Exit codes: 1 for a failed run, 2 for a command line we don't understand.
const (
	exitFailed = 1
	exitUsage  = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
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
	if !*once || flags.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	if err := runOnce(flags.Arg(0)); err != nil {
		_, _ = fmt.Fprintf(stderr, "catalog: %v\n", err)
		return exitFailed
	}
	return 0
}

// runOnce crawls folder as the only watched folder and summarizes every file
// waiting in its catalogs.
func runOnce(folder string) error {
	stateDir, err := settings.StateDir()
	if err != nil {
		return err
	}
	g, err := loadGlobal(stateDir)
	if err != nil {
		return err
	}
	g.Watch = []string{folder}

	s, err := store.Open(filepath.Join(stateDir, "store.db"))
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	d := &daemon.Daemon{
		Global:      g,
		Store:       s,
		Provider:    models.NewAnthropic(g.APIKey, g.Model),
		JournalPath: filepath.Join(stateDir, "journal.jsonl"),
		FS:          os.DirFS,
	}
	return d.Once(ctx, time.Now)
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
