package publish

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func catalogDirWithDot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".catalog"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPublishWritesAndSkips(t *testing.T) {
	dir := catalogDirWithDot(t)
	path := filepath.Join(dir, ".catalog", "catalog.md")

	changed, err := Publish(dir, "one\n")
	if err != nil || !changed {
		t.Fatalf("first call = %v, %v; want true, nil", changed, err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "one\n" {
		t.Fatalf("content = %q", got)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	changed, err = Publish(dir, "one\n")
	if err != nil || changed {
		t.Fatalf("same text = %v, %v; want false, nil", changed, err)
	}
	info, _ = os.Stat(path)
	if !info.ModTime().Equal(old) {
		t.Errorf("mtime changed")
	}

	changed, err = Publish(dir, "two\n")
	if err != nil || !changed {
		t.Fatalf("new text = %v, %v; want true, nil", changed, err)
	}
	got, _ = os.ReadFile(path)
	if string(got) != "two\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestPublishLeavesNoTempFiles(t *testing.T) {
	dir := catalogDirWithDot(t)
	if _, err := Publish(dir, "x"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, ".catalog"))
	if len(entries) != 1 || entries[0].Name() != "catalog.md" {
		t.Errorf("entries = %v", entries)
	}
}

func TestPublishUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := catalogDirWithDot(t)
	dot := filepath.Join(dir, ".catalog")
	path := filepath.Join(dot, "catalog.md")
	if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dot, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dot, 0o755) })

	changed, err := Publish(dir, "new")
	if err == nil || changed {
		t.Fatalf("got %v, %v; want false and an error", changed, err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "keep" {
		t.Errorf("existing file = %q", got)
	}
}

func TestPublishNoCatalogFolder(t *testing.T) {
	dir := t.TempDir()
	changed, err := Publish(dir, "x")
	if err == nil || changed {
		t.Fatalf("got %v, %v; want false and an error", changed, err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".catalog")); statErr == nil {
		t.Errorf("Publish created .catalog/")
	}
}

func TestPublishRenameFailureRemovesTemp(t *testing.T) {
	dir := catalogDirWithDot(t)
	dot := filepath.Join(dir, ".catalog")
	// A directory at the target makes the rename fail.
	if err := os.MkdirAll(filepath.Join(dot, "catalog.md", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(dir, "x"); err == nil {
		t.Fatal("want error")
	}
	entries, _ := os.ReadDir(dot)
	if len(entries) != 1 {
		t.Errorf("temp file left behind: %v", entries)
	}
}

func TestWriteAndCloseReportsWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path) // read-only: the write fails but Close succeeds
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAndClose(f, "x"); err == nil {
		t.Error("want an error writing to a read-only file")
	}
}

// A process killed between writing its temporary file and renaming it
// leaves the file behind; the next publish removes it and nothing else.
func TestPublishRemovesLeftoverTempFiles(t *testing.T) {
	dir := catalogDirWithDot(t)
	dot := filepath.Join(dir, ".catalog")
	for _, name := range []string{"catalog-123.tmp", "catalog-456.tmp", "config.toml", "notes.tmp"} {
		if err := os.WriteFile(filepath.Join(dot, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Publish(dir, "text\n"); err != nil {
		t.Fatal(err)
	}

	var names []string
	entries, _ := os.ReadDir(dot)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if fmt.Sprint(names) != "[catalog.md config.toml notes.tmp]" {
		t.Errorf("entries = %v, want [catalog.md config.toml notes.tmp]", names)
	}
}

// catalog.md summarizes files that may be private, so only its owner may
// read it. One an earlier version left readable by others is replaced with a
// private one, even when its text hasn't changed.
func TestPublishReplacesAnUnchangedCatalogReadableByOthers(t *testing.T) {
	dir := catalogDirWithDot(t)
	path := filepath.Join(dir, ".catalog", "catalog.md")
	if err := os.WriteFile(path, []byte("same\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Publish(dir, "same\n"); err != nil {
		t.Fatal(err)
	}

	assertPrivateFile(t, path, "same\n")
}

// A catalog.md that is a link is replaced, and the file it points to is left
// alone.
func TestPublishReplacesALinkWithoutTouchingItsTarget(t *testing.T) {
	dir := catalogDirWithDot(t)
	path := filepath.Join(dir, ".catalog", "catalog.md")
	target := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.WriteFile(target, []byte("same\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if _, err := Publish(dir, "same\n"); err != nil {
		t.Fatal(err)
	}

	assertPrivateFile(t, path, "same\n")
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("link target mode = %v, %v; want it left at 0644", info.Mode().Perm(), err)
	}
}

// assertPrivateFile fails unless path is a regular file, not a link, with
// mode 0600 and the given text.
func assertPrivateFile(t *testing.T, path, text string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Errorf("%s is %v, want a regular file with mode 0600", path, info.Mode())
	}
	if got, _ := os.ReadFile(path); string(got) != text {
		t.Errorf("%s holds %q, want %q", path, got, text)
	}
}
