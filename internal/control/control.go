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
// summarized under the current recipe or waiting to be.
type CatalogStatus struct {
	Dir                        string
	Files, Summarized, Waiting int
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
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	var answering sync.WaitGroup
	defer answering.Wait()
	for {
		conn, err := l.Accept()
		if err != nil && ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		answering.Go(func() { answer(conn, current()) })
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
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().Type() != fs.ModeSocket {
		return fmt.Errorf("%s isn't a socket; move it aside so the daemon can listen there", path)
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

// Ask connects to the daemon at socketPath and returns its snapshot. If no
// daemon is listening there, the error wraps ErrNotRunning.
func Ask(socketPath string) (Snapshot, error) {
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
