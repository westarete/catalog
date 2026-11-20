package control

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// socketPath returns a path for a socket in a new short temporary folder.
// macOS limits a socket's path to 103 bytes, and t.TempDir's paths are
// longer than that.
func socketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cat")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "control.sock")
}

// serve runs Serve on path until the test ends, once it's answering. The
// returned function stops it and returns Serve's result.
func serve(t *testing.T, path string, current func() Snapshot) func() error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, path, current) }()
	stop := sync.OnceValue(func() error {
		cancel()
		return <-done
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := Ask(path)
		if err == nil {
			break
		}
		select {
		case serveErr := <-done:
			cancel()
			t.Fatalf("Serve returned before answering: %v", serveErr)
		default:
		}
		if time.Now().After(deadline) {
			_ = stop()
			t.Fatalf("Serve never answered: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

func fixed(s Snapshot) func() Snapshot { return func() Snapshot { return s } }

var started = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

func sample() Snapshot {
	return Snapshot{
		Started:      started,
		LastCrawl:    started.Add(41 * time.Minute),
		ConfigErrors: []string{"/state/global.toml: parse global.toml: bad"},
		PausedUntil:  started.Add(45 * time.Minute),
		PauseReason:  "summarize /notes/a.md: 529 Overloaded",
		Catalogs:     []CatalogStatus{{Dir: "/notes", Files: 12, Summarized: 10, Waiting: 2, ToUpdate: 3}},
		Failing: []FailingFile{
			{Dir: "/notes", Rel: "b.pdf", Error: "model returned no profile text", RetryAt: started.Add(time.Hour)},
		},
	}
}

func TestServeAndAsk(t *testing.T) {
	path := socketPath(t)
	want := sample()
	serve(t, path, fixed(want))

	got, err := Ask(path)

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Ask = %+v\nwant %+v", got, want)
	}
}

// Each connection gets the snapshot current when it connects.
func TestServeAnswersWithCurrentSnapshot(t *testing.T) {
	path := socketPath(t)
	var n atomic.Int64
	serve(t, path, func() Snapshot { return Snapshot{Catalogs: []CatalogStatus{{Files: int(n.Add(1))}}} })

	first, err := Ask(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Ask(path)
	if err != nil {
		t.Fatal(err)
	}

	if second.Catalogs[0].Files != first.Catalogs[0].Files+1 {
		t.Errorf("files = %d then %d, want a fresh snapshot each time", first.Catalogs[0].Files, second.Catalogs[0].Files)
	}
}

func TestAskNoDaemon(t *testing.T) {
	_, err := Ask(socketPath(t))

	if !errors.Is(err, ErrNotRunning) {
		t.Errorf("Ask = %v, want ErrNotRunning", err)
	}
}

// staleSocket leaves a socket file at path that nothing listens on, as a
// daemon that crashed would.
func staleSocket(t *testing.T, path string) {
	t.Helper()
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAskRefused(t *testing.T) {
	path := socketPath(t)
	staleSocket(t, path)

	_, err := Ask(path)

	if !errors.Is(err, ErrNotRunning) {
		t.Errorf("Ask = %v, want ErrNotRunning", err)
	}
}

// A file at the socket's path that isn't a socket is ErrNotSocket, not a
// daemon that isn't running, on every OS.
func TestAskNotASocket(t *testing.T) {
	path := socketPath(t)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Ask(path)

	if !errors.Is(err, ErrNotSocket) || errors.Is(err, ErrNotRunning) {
		t.Errorf("Ask = %v, want ErrNotSocket", err)
	}
}

// Ask can't tell whether a daemon is running if it can't look at the path.
func TestAskCantCheckSocket(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission tests don't work as root")
	}
	locked := filepath.Join(filepath.Dir(socketPath(t)), "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, err := Ask(filepath.Join(locked, "control.sock"))

	if err == nil || errors.Is(err, ErrNotRunning) || errors.Is(err, ErrNotSocket) {
		t.Errorf("Ask = %v, want a permission error", err)
	}
}

// A listener that closes without writing a snapshot is an error.
func TestAskEmptyAnswer(t *testing.T) {
	path := socketPath(t)
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	go func() {
		if conn, err := l.Accept(); err == nil {
			_ = conn.Close()
		}
	}()

	_, err = Ask(path)

	if err == nil || errors.Is(err, ErrNotRunning) {
		t.Errorf("Ask = %v, want a read error", err)
	}
}

func TestServeReplacesStaleSocket(t *testing.T) {
	path := socketPath(t)
	staleSocket(t, path)

	serve(t, path, fixed(sample()))

	if _, err := Ask(path); err != nil {
		t.Errorf("Ask = %v", err)
	}
}

func TestServeKeepsRegularFile(t *testing.T) {
	path := socketPath(t)
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Serve(context.Background(), path, fixed(sample()))

	if !errors.Is(err, ErrNotSocket) {
		t.Fatalf("Serve = %v, want ErrNotSocket", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "keep" {
		t.Errorf("file now holds %q", data)
	}
}

func TestServeLeavesLiveDaemonAlone(t *testing.T) {
	path := socketPath(t)
	want := sample()
	serve(t, path, fixed(want))

	err := Serve(context.Background(), path, fixed(Snapshot{}))

	if err == nil {
		t.Fatal("second Serve = nil, want an error")
	}
	got, err := Ask(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("first daemon after second Serve: %+v, %v", got, err)
	}
}

func TestServeFailsWithoutFolder(t *testing.T) {
	path := filepath.Join(filepath.Dir(socketPath(t)), "missing", "control.sock")

	if err := Serve(context.Background(), path, fixed(sample())); err == nil {
		t.Error("Serve = nil, want an error")
	}
}

// Serve can't tell whether a socket it can't stat is stale, so it stops.
func TestServeFailsWhenSocketCantBeChecked(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission tests don't work as root")
	}
	dir := filepath.Dir(socketPath(t))
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	if err := Serve(context.Background(), filepath.Join(locked, "control.sock"), fixed(sample())); err == nil {
		t.Error("Serve = nil, want an error")
	}
}

func TestServeSocketIsPrivate(t *testing.T) {
	path := socketPath(t)
	serve(t, path, fixed(sample()))

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Type() != fs.ModeSocket || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want a socket with 0600", info.Mode())
	}
}

func TestServeStopsAndRemovesSocket(t *testing.T) {
	path := socketPath(t)
	stop := serve(t, path, fixed(sample()))

	if err := stop(); err != nil {
		t.Errorf("Serve = %v, want nil after cancel", err)
	}

	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("socket after stop: %v, want it gone", err)
	}
	if _, err := Ask(path); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Ask after stop = %v, want ErrNotRunning", err)
	}
}

func TestFormat(t *testing.T) {
	useLocal(t, time.UTC)

	got := Format(sample())

	want := strings.Join([]string{
		"Started Nov 15 09:00; last crawled Nov 15 09:41.",
		"Config error: /state/global.toml: parse global.toml: bad",
		"Summarizing paused until Nov 15 09:45: summarize /notes/a.md: 529 Overloaded",
		"",
		"Catalogs:",
		"  /notes: 12 files, 10 summarized, 2 waiting, 3 to update",
		"",
		"Failing:",
		"  /notes/b.pdf: model returned no profile text (retry Nov 15 10:00)",
		"",
	}, "\n")
	if got != want {
		t.Errorf("Format =\n%s\nwant\n%s", got, want)
	}
}

// Until the first crawl ends, status says only that the daemon is starting,
// not that it found no catalogs.
func TestFormatBeforeFirstCrawl(t *testing.T) {
	useLocal(t, time.UTC)
	tests := map[string]struct {
		s    Snapshot
		want string
	}{
		"started":        {Snapshot{Started: started}, "Started Nov 15 09:00; the first crawl is under way.\n"},
		"not yet":        {Snapshot{}, "Starting; the first crawl is under way.\n"},
		"with a problem": {Snapshot{PauseReason: "x", ConfigErrors: []string{"y"}}, "Starting; the first crawl is under way.\n"},
	}
	for name, tt := range tests {
		if got := Format(tt.s); got != tt.want {
			t.Errorf("%s: Format =\n%q\nwant\n%q", name, got, tt.want)
		}
	}
}

func TestFormatNoCatalogs(t *testing.T) {
	useLocal(t, time.UTC)

	got := Format(Snapshot{Started: started, LastCrawl: started})

	want := "Started Nov 15 09:00; last crawled Nov 15 09:00.\n\nNo catalogs found.\n"
	if got != want {
		t.Errorf("Format =\n%q\nwant\n%q", got, want)
	}
}

// A problem with a folder rather than a file has no Rel, and one the daemon
// retries on every crawl has no RetryAt. Summarizing that is stopped rather
// than paused has a reason but no end.
func TestFormatFolderProblemAndStop(t *testing.T) {
	useLocal(t, time.UTC)
	s := Snapshot{
		Started:     started,
		LastCrawl:   started,
		PauseReason: "no API key",
		Failing:     []FailingFile{{Dir: "/src/private", Error: "permission denied"}},
	}

	got := Format(s)

	for _, line := range []string{
		"Not summarizing: no API key\n",
		"  /src/private: permission denied (retry next crawl)\n",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("Format doesn't contain %q:\n%s", line, got)
		}
	}
}

func TestFormatUsesLocalTime(t *testing.T) {
	useLocal(t, time.FixedZone("EST", -5*60*60))

	got := Format(Snapshot{Started: time.Date(2026, 11, 15, 14, 0, 0, 0, time.UTC)})

	if !strings.HasPrefix(got, "Started Nov 15 09:00;") {
		t.Errorf("Format =\n%s\nwant the start in local time, 09:00", got)
	}
}

// useLocal sets the local time zone for one test. No test in this package
// runs in parallel.
func useLocal(t *testing.T, loc *time.Location) {
	t.Helper()
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

// failingListener fails its first few Accepts, as a listener does when the
// process runs out of file descriptors.
type failingListener struct {
	net.Listener
	failures atomic.Int64
}

var errTooManyFiles = errors.New("accept: too many open files")

func (l *failingListener) Accept() (net.Conn, error) {
	if l.failures.Add(-1) >= 0 {
		return nil, errTooManyFiles
	}
	return l.Listener.Accept()
}

// An Accept error doesn't stop status for good: serveOn waits and tries
// again.
func TestServeRetriesAccept(t *testing.T) {
	path := socketPath(t)
	inner, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	l := &failingListener{Listener: inner}
	l.failures.Store(3)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		serveOn(ctx, l, fixed(sample()))
		close(done)
	}()

	got, err := Ask(path)

	cancel()
	<-done
	if err != nil || !reflect.DeepEqual(got, sample()) {
		t.Errorf("Ask after failed Accepts = %+v, %v", got, err)
	}
}

// A listener that keeps failing doesn't keep serveOn from stopping.
func TestServeStopsWhileRetrying(t *testing.T) {
	inner, err := net.Listen("unix", socketPath(t))
	if err != nil {
		t.Fatal(err)
	}
	l := &failingListener{Listener: inner}
	l.failures.Store(1 << 40)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		serveOn(ctx, l, fixed(sample()))
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serveOn didn't stop within a second of cancel")
	}
}

func TestAcceptDelay(t *testing.T) {
	tests := []struct{ prev, want time.Duration }{
		{0, 5 * time.Millisecond},
		{5 * time.Millisecond, 10 * time.Millisecond},
		{640 * time.Millisecond, time.Second},
		{time.Second, time.Second},
	}
	for _, tt := range tests {
		if got := nextAcceptDelay(tt.prev); got != tt.want {
			t.Errorf("nextAcceptDelay(%v) = %v, want %v", tt.prev, got, tt.want)
		}
	}
}
