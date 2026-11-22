// Command catalog keeps a catalog.md current in every folder that holds a
// .catalog/ folder, inside the folders listed in global.toml.
//
// Usage:
//
//	catalog daemon
//	catalog daemon --once <folder>
//	catalog status
//	catalog setup [--agent-only]
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

	"westarete.com/catalog/internal/control"
	"westarete.com/catalog/internal/daemon"
	"westarete.com/catalog/internal/install"
	"westarete.com/catalog/internal/models"
	"westarete.com/catalog/internal/settings"
	"westarete.com/catalog/internal/store"
)

const usage = "usage: catalog daemon [--once <folder>]\n       catalog status\n       catalog setup [--agent-only]"

// socketFile is the name of the daemon's status socket in the state folder,
// and journalFile the name of its journal.
const (
	socketFile  = "control.sock"
	journalFile = "journal.jsonl"
)

// Exit codes: 1 for a failed run or a daemon that isn't running, 2 for a
// command line we don't understand.
const (
	exitFailed = 1
	exitUsage  = 2
)

// crawlEvery is how often the daemon crawls the watched folders.
const crawlEvery = time.Minute

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run carries out the command line in args until it's done or ctx ends.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var err error
	switch {
	case len(args) == 1 && args[0] == "status":
		return runStatus(stdout, stderr)
	case len(args) > 0 && args[0] == "setup":
		err = runSetup(args[1:], stdin, stdout, stderr)
	case len(args) > 0 && args[0] == "daemon":
		err = runDaemon(ctx, args[1:], stdout, stderr)
	default:
		err = errUsage
	}
	if errors.Is(err, errUsage) {
		_, _ = fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "catalog: %v\n", err)
		return exitFailed
	}
	return 0
}

var errUsage = errors.New("usage")

// runDaemon carries out `catalog daemon` with the arguments that follow it.
func runDaemon(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("catalog daemon", flag.ContinueOnError)
	flags.SetOutput(stderr)
	once := flags.Bool("once", false, "summarize every waiting file in `folder` once, then exit")
	if err := flags.Parse(args); err != nil {
		return errUsage
	}
	switch {
	case *once && flags.NArg() == 1:
		return runOnce(ctx, flags.Arg(0), stdout)
	case !*once && flags.NArg() == 0:
		return runLoop(ctx, stderr)
	default:
		return errUsage
	}
}

// runStatus asks the daemon for its status and prints it.
func runStatus(stdout, stderr io.Writer) int {
	stateDir, err := settings.StateDir()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "catalog: %v\n", err)
		return exitFailed
	}
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "catalog: %v\n", err)
		return exitFailed
	}
	_, _ = fmt.Fprintln(stdout, install.Describe(home, settingsState(stateDir), onMac))
	s, err := control.Ask(filepath.Join(stateDir, socketFile))
	if errors.Is(err, control.ErrNotRunning) {
		_, _ = fmt.Fprintln(stdout, "The catalog daemon isn't running.")
		return exitFailed
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "catalog: %v\n", err)
		return exitFailed
	}
	_, _ = fmt.Fprint(stdout, control.Format(s))
	return 0
}

// runOnce crawls folder as the only watched folder, summarizes every file
// waiting in its catalogs, and prints what it found, as status would. It
// stops at once if global.toml is missing or has no API key, and fails if it
// finds no catalog.
func runOnce(ctx context.Context, folder string, stdout io.Writer) error {
	stateDir, err := privateStateDir()
	if err != nil {
		return err
	}
	g, err := loadGlobal(stateDir)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	g.Watch = []string{abs}
	d, err := openDaemon(stateDir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Store.Close() }()
	d.Global = g
	d.Provider = models.NewAnthropic(g.APIKey, g.Model)
	err = d.Once(ctx, time.Now)
	s := d.Snapshot()
	_, _ = fmt.Fprint(stdout, control.Format(s))
	if err == nil && len(s.Catalogs) == 0 {
		err = fmt.Errorf("no catalog in %s: it can't be read, or nothing in it has a .catalog/ folder", abs)
	}
	return err
}

// runLoop keeps the catalogs in the folders global.toml watches current,
// rereading global.toml on each crawl, and answers status requests, until
// ctx ends. It fails at once if another daemon answers on the socket. A
// missing or broken global.toml doesn't stop it: launchd would only start it
// again, and status reports the problem instead. It writes the errors it
// can't journal to stderr.
func runLoop(ctx context.Context, stderr io.Writer) error {
	stateDir, err := privateStateDir()
	if err != nil {
		return err
	}
	socket := filepath.Join(stateDir, socketFile)
	// A second daemon would crawl and summarize everything the first one
	// does, so it stops before it starts.
	if _, err := control.Ask(socket); err == nil {
		return fmt.Errorf("another catalog daemon is already running; it answers on %s", socket)
	}
	d, err := openDaemon(stateDir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Store.Close() }()
	d.GlobalPath = filepath.Join(stateDir, settings.GlobalFile)
	d.Log = log.New(stderr, "catalog: ", log.LstdFlags)

	serving, stopServing := context.WithCancel(ctx)
	served := make(chan struct{})
	go func() {
		defer close(served)
		if err := control.Serve(serving, socket, d.Snapshot); err != nil {
			d.Log.Printf("status won't answer: %v", err)
		}
	}()
	defer func() {
		stopServing()
		<-served
	}()

	ticker := time.NewTicker(crawlEvery)
	defer ticker.Stop()
	return d.Run(ctx, time.Now, ticker.C)
}

// openDaemon opens the Store in stateDir and builds a daemon on it, with no
// settings or Provider yet. The caller closes the Store.
func openDaemon(stateDir string) (*daemon.Daemon, error) {
	s, err := store.Open(filepath.Join(stateDir, "store.db"))
	if err != nil {
		return nil, err
	}
	return &daemon.Daemon{
		Store:       s,
		NewProvider: models.NewAnthropic,
		JournalPath: filepath.Join(stateDir, journalFile),
		FS:          os.DirFS,
	}, nil
}

// privateStateDir returns the state folder, after creating it if it's
// missing and making it and the journal in it private. It does that before
// anything else can fail, so a run that stops at once, such as one with no
// global.toml yet, still fixes what an earlier version left readable. The
// Store makes its own files private when it opens them.
func privateStateDir() (string, error) {
	dir, err := settings.StateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// gosec's G302 expects a file; a folder needs its execute bit to be
	// opened at all, so 0700 is the private mode for one.
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // see above
		return "", err
	}
	journal := filepath.Join(dir, journalFile)
	if err := os.Chmod(journal, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return dir, nil
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
