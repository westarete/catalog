package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"westarete.com/catalog/internal/model"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func sameState(a, b model.FileState) bool {
	return a.Rel == b.Rel && a.Size == b.Size && a.ModTime.Equal(b.ModTime) &&
		a.Hash == b.Hash && a.ChangedAt.Equal(b.ChangedAt)
}

func state(rel string, size int64, hash model.Hash) model.FileState {
	return model.FileState{
		Rel:       rel,
		Size:      size,
		ModTime:   time.Date(2026, 11, 15, 9, 30, 1, 123456789, time.UTC),
		Hash:      hash,
		ChangedAt: time.Date(2026, 11, 15, 9, 31, 2, 987654321, time.UTC),
	}
}

func TestFilesRoundTrip(t *testing.T) {
	s, _ := openTemp(t)
	a, b := state("a.md", 10, "h1"), state("sub/b.txt", 20, "h2")
	if err := s.SetFiles("/cat", []model.FileState{a, b}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Files("/cat")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !sameState(got["a.md"], a) || !sameState(got["sub/b.txt"], b) {
		t.Fatalf("got %+v", got)
	}

	if err := s.SetFiles("/cat", []model.FileState{a}); err != nil {
		t.Fatal(err)
	}
	got, err = s.Files("/cat")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !sameState(got["a.md"], a) {
		t.Fatalf("after replace: %+v", got)
	}
}

func TestFilesAreSeparatePerDir(t *testing.T) {
	s, _ := openTemp(t)
	if err := s.SetFiles("/one", []model.FileState{state("a.md", 1, "h")}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFiles("/two", []model.FileState{state("b.md", 2, "h")}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFiles("/two", nil); err != nil {
		t.Fatal(err)
	}
	one, err := s.Files("/one")
	if err != nil {
		t.Fatal(err)
	}
	two, err := s.Files("/two")
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || len(two) != 0 {
		t.Fatalf("one=%v two=%v", one, two)
	}
}

func TestFilesEmptyDirIsEmptyMap(t *testing.T) {
	s, _ := openTemp(t)
	got, err := s.Files("/none")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil map", got)
	}
}

func TestZeroTimesRoundTrip(t *testing.T) {
	s, _ := openTemp(t)
	f := model.FileState{Rel: "z.md", Size: 1, Hash: "h"}
	if err := s.SetFiles("/cat", []model.FileState{f}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Files("/cat")
	if err != nil {
		t.Fatal(err)
	}
	if !got["z.md"].ModTime.IsZero() || !got["z.md"].ChangedAt.IsZero() {
		t.Fatalf("got %+v, want zero times", got["z.md"])
	}
}

func TestSetFilesRollsBackOnError(t *testing.T) {
	s, _ := openTemp(t)
	a := state("a.md", 1, "h")
	if err := s.SetFiles("/cat", []model.FileState{a}); err != nil {
		t.Fatal(err)
	}
	dup := []model.FileState{state("x.md", 1, "h"), state("x.md", 2, "h")}
	if err := s.SetFiles("/cat", dup); err == nil {
		t.Fatal("want error for duplicate rel")
	}
	got, err := s.Files("/cat")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !sameState(got["a.md"], a) {
		t.Fatalf("old set not preserved: %+v", got)
	}
	// A write after the failure shows the failed transaction let go of its lock.
	if err := s.SetFiles("/cat", []model.FileState{a}); err != nil {
		t.Fatalf("write after failed SetFiles: %v", err)
	}
}

func TestSummaryByHashAndRecipe(t *testing.T) {
	s, _ := openTemp(t)
	if err := s.PutSummary("h", "A", "text A"); err != nil {
		t.Fatal(err)
	}
	text, ok, err := s.Summary("h", "A")
	if err != nil || !ok || text != "text A" {
		t.Fatalf("Summary(h,A) = %q %v %v", text, ok, err)
	}
	if _, ok, err := s.Summary("h", "B"); err != nil || ok {
		t.Fatalf("Summary(h,B) ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.Summary("other", "A"); err != nil || ok {
		t.Fatalf("Summary(other,A) ok=%v err=%v", ok, err)
	}
	text, recipe, ok, err := s.AnySummary("h")
	if err != nil || !ok || text != "text A" || recipe != "A" {
		t.Fatalf("AnySummary = %q %q %v %v", text, recipe, ok, err)
	}
	if _, _, ok, err := s.AnySummary("other"); err != nil || ok {
		t.Fatalf("AnySummary(other) ok=%v err=%v", ok, err)
	}
}

func TestPutSummaryReplaces(t *testing.T) {
	s, _ := openTemp(t)
	for _, text := range []string{"first", "second"} {
		if err := s.PutSummary("h", "A", text); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := s.Summary("h", "A")
	if err != nil || got != "second" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestFailureLifecycle(t *testing.T) {
	s, _ := openTemp(t)
	if _, ok, err := s.Failure("/cat", "a.md"); err != nil || ok {
		t.Fatalf("before record: ok=%v err=%v", ok, err)
	}
	want := Failure{Count: 3, LastError: "boom", RetryAt: time.Date(2026, 11, 15, 10, 0, 0, 5, time.UTC)}
	if err := s.RecordFailure("/cat", "a.md", want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Failure("/cat", "a.md")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Count != want.Count || got.LastError != want.LastError || !got.RetryAt.Equal(want.RetryAt) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if _, ok, _ := s.Failure("/cat", "b.md"); ok {
		t.Fatal("other rel should have no failure")
	}
	if _, ok, _ := s.Failure("/other", "a.md"); ok {
		t.Fatal("other dir should have no failure")
	}

	want.Count = 4
	if err := s.RecordFailure("/cat", "a.md", want); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Failure("/cat", "a.md"); got.Count != 4 {
		t.Fatalf("update: got %+v", got)
	}

	if err := s.ClearFailure("/cat", "a.md"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Failure("/cat", "a.md"); err != nil || ok {
		t.Fatalf("after clear: ok=%v err=%v", ok, err)
	}
}

func TestReopen(t *testing.T) {
	s, path := openTemp(t)
	a := state("a.md", 5, "h")
	if err := s.SetFiles("/cat", []model.FileState{a}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutSummary("h", "A", "kept"); err != nil {
		t.Fatal(err)
	}
	f := Failure{Count: 1, LastError: "e", RetryAt: time.Unix(100, 7)}
	if err := s.RecordFailure("/cat", "a.md", f); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	files, err := s2.Files("/cat")
	if err != nil || len(files) != 1 || !sameState(files["a.md"], a) {
		t.Fatalf("files %+v err %v", files, err)
	}
	if text, ok, _ := s2.Summary("h", "A"); !ok || text != "kept" {
		t.Fatalf("summary %q %v", text, ok)
	}
	if got, ok, _ := s2.Failure("/cat", "a.md"); !ok || got.Count != 1 {
		t.Fatalf("failure %+v %v", got, ok)
	}
}

func TestWALMode(t *testing.T) {
	s, _ := openTemp(t)
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q", mode)
	}
}

func TestBusyTimeout(t *testing.T) {
	s, _ := openTemp(t)
	var ms int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&ms); err != nil {
		t.Fatal(err)
	}
	if ms != busyTimeoutMillis {
		t.Fatalf("busy_timeout = %d, want %d", ms, busyTimeoutMillis)
	}
}

func TestOpenHandlesSpecialCharactersInPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Application Support?x#y%z", "store.db")
	if err := mkdirAll(path); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.PutSummary("h", "A", "x"); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if _, ok, _ := s.Summary("h", "A"); !ok {
		t.Fatal("data not at that path")
	}
}

func TestOpenFailsForNonDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	if err := writeFile(path, "this is not a database, just some text that is long enough"); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(path); err == nil {
		_ = s.Close()
		t.Fatal("want error")
	}
}

func mkdirAll(file string) error { return os.MkdirAll(filepath.Dir(file), 0o755) }

func writeFile(path, text string) error { return os.WriteFile(path, []byte(text), 0o644) }

// assertOwnerOnly fails unless the file at path exists with mode 0600.
func assertOwnerOnly(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("%s mode = %o, want 600", filepath.Base(path), mode)
	}
}

// The store holds summaries of private documents, so only its owner may
// read the database or its write-ahead log.
func TestOpenCreatesFilesOnlyOwnerCanRead(t *testing.T) {
	s, path := openTemp(t)
	if err := s.PutSummary("h", "r", "a summary"); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		assertOwnerOnly(t, p)
	}
}

// A store created by an earlier version may be readable by anyone, and a
// daemon that was killed leaves its write-ahead log behind with data in it;
// Open makes all of it private.
func TestOpenTightensAnExistingStore(t *testing.T) {
	live, livePath := openTemp(t)
	if err := live.PutSummary("h", "r", "a summary"); err != nil {
		t.Fatal(err)
	}
	// Copying the files of a store that is still open is what a killed
	// daemon leaves behind: a database and a log that holds the summary.
	path := filepath.Join(t.TempDir(), "store.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(livePath + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+suffix, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path+suffix, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("leftover log is empty or missing: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		assertOwnerOnly(t, p)
	}
	if text, ok, err := s.Summary("h", "r"); err != nil || !ok || text != "a summary" {
		t.Errorf("Summary = %q, %v, %v; want the summary from the leftover log", text, ok, err)
	}
}
