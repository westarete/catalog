// Package control lets `catalog status` ask the running daemon what it's
// doing. The daemon serves a Snapshot of its state on a Unix socket, and each
// connection gets the latest snapshot as one JSON object.
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

// Snapshot is the daemon's state at the end of its last crawl or summary.
type Snapshot struct {
	Started, LastCrawl time.Time
	// ConfigErrors holds one line for each config file that didn't load,
	// naming the file or the catalog's folder, then the error.
	ConfigErrors []string
	// PauseReason says why the daemon isn't summarizing, if it isn't. When
	// it's waiting out a service problem, PausedUntil says until when; when
	// it can't summarize until someone fixes global.toml, PausedUntil is
	// zero.
	PausedUntil time.Time
	PauseReason string
	Catalogs    []CatalogStatus
	Failing     []FailingFile
}

// CatalogStatus counts the files in the catalog in Dir. Every file is either
// summarized, with a summary in catalog.md, or waiting for its first one.
// ToUpdate counts the summarized files whose summary the daemon will write
// again: one written before its neighbors had summaries, or one under an
// older recipe.
type CatalogStatus struct {
	Dir                                  string
	Files, Summarized, Waiting, ToUpdate int
}

// FailingFile is a file or folder the daemon couldn't handle. Rel is empty
// when the problem is with the folder Dir itself, such as a catalog.md the
// daemon can't write. RetryAt is zero when the daemon tries again on every
// crawl.
type FailingFile struct {
	Dir, Rel, Error string
	RetryAt         time.Time
}

// ErrNotRunning means no daemon is listening on the socket.
var ErrNotRunning = errors.New("the catalog daemon isn't running")

// ErrNotSocket means something other than a socket is at the socket's path.
var ErrNotSocket = errors.New("not a socket")

// timeout bounds how long either side waits on the other.
const timeout = 5 * time.Second

// Serve listens on a Unix socket at socketPath and writes current() to each
// connection until ctx ends. Then it closes the socket, removes it, and
// returns nil. It first removes a socket left by a daemon that has stopped,
// but it returns an error if a daemon still answers there or if the path
// holds something other than a socket.
func Serve(ctx context.Context, socketPath string, current func() Snapshot) error {
	if err := removeStale(socketPath); err != nil {
		return err
	}
	// Closing a listener that net.Listen created also removes its socket.
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() { _ = l.Close() }()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return err
	}
	serveOn(ctx, l, current)
	return nil
}

// serveOn answers each connection to l until ctx ends, then closes l. When
// Accept fails, as it does when the process runs out of file descriptors,
// serveOn waits and tries again, as net/http's server does.
func serveOn(ctx context.Context, l net.Listener, current func() Snapshot) {
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	var answering sync.WaitGroup
	defer answering.Wait()
	var delay time.Duration
	for {
		conn, err := l.Accept()
		if err != nil {
			// Once ctx ends, the closed listener fails every Accept, and
			// wait returns false at once.
			delay = nextAcceptDelay(delay)
			if !wait(ctx, delay) {
				return
			}
			continue
		}
		delay = 0
		answering.Go(func() { answer(conn, current()) })
	}
}

// nextAcceptDelay returns how long to wait after a failed Accept, given the
// wait after the one before: 5 milliseconds at first, doubling up to a
// second.
func nextAcceptDelay(prev time.Duration) time.Duration {
	if prev == 0 {
		return 5 * time.Millisecond
	}
	return min(2*prev, time.Second)
}

// wait waits for d to pass and reports true, or reports false if ctx ends
// first.
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// answer writes s to conn and closes it. If the asker has gone, there's no
// one to tell.
func answer(conn net.Conn, s Snapshot) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	_ = json.NewEncoder(conn).Encode(s)
}

// removeStale removes the socket at path if no daemon answers on it.
func removeStale(path string) error {
	found, err := socketAt(path)
	if !found {
		return err
	}
	conn, err := net.DialTimeout("unix", path, timeout)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("another catalog daemon is answering on %s", path)
	}
	if !notRunning(err) {
		return err
	}
	return os.Remove(path)
}

// socketAt reports whether there's a socket at path. If something else is
// there, the error wraps ErrNotSocket. Checking this before connecting gives
// the same answer on every OS; Linux, for one, may refuse a connection to a
// regular file just as it does to a socket with no listener.
func socketAt(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode().Type() != fs.ModeSocket {
		return false, fmt.Errorf("%s is %w; move it aside so the daemon can listen there", path, ErrNotSocket)
	}
	return true, nil
}

// Ask connects to the daemon at socketPath and returns its snapshot. If no
// daemon is listening there, the error wraps ErrNotRunning.
func Ask(socketPath string) (Snapshot, error) {
	found, err := socketAt(socketPath)
	if err != nil {
		return Snapshot{}, err
	}
	if !found {
		return Snapshot{}, fmt.Errorf("%w: no socket at %s", ErrNotRunning, socketPath)
	}
	conn, err := net.DialTimeout("unix", socketPath, timeout)
	if notRunning(err) {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrNotRunning, err)
	}
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	var s Snapshot
	if err := json.NewDecoder(conn).Decode(&s); err != nil {
		return Snapshot{}, fmt.Errorf("read status from %s: %w", socketPath, err)
	}
	return s, nil
}

// notRunning reports whether a failed connection means no daemon is
// listening: the socket is missing, or nothing accepts on it.
func notRunning(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED)
}
