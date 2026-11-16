package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"reflect"
	"runtime"
	"testing"
	"testing/fstest"
	"time"

	"westarete.com/catalog/internal/model"
)

var (
	modTime = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	now     = time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
)

// helloHash is the SHA-256 of the bytes "hello".
const helloHash = model.Hash("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")

func TestScanNewFile(t *testing.T) {
	catalog := fstest.MapFS{
		"notes.md": {Data: []byte("hello"), ModTime: modTime},
	}

	got := Scan(catalog, []string{"notes.md"}, nil, now)

	want := []model.FileState{{
		Rel:       "notes.md",
		Size:      5,
		ModTime:   modTime,
		Hash:      helloHash,
		ChangedAt: now,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan() = %+v, want %+v", got, want)
	}
}

func TestScanUnchanged(t *testing.T) {
	// The content differs from the recorded hash but has the same size and
	// mtime, so Scan must trust the record and not read the file.
	catalog := fstest.MapFS{
		"notes.md": {Data: []byte("world"), ModTime: modTime},
	}
	prev := map[string]model.FileState{
		"notes.md": {
			Rel:       "notes.md",
			Size:      5,
			ModTime:   modTime,
			Hash:      "recorded-not-recomputed",
			ChangedAt: modTime,
		},
	}

	got := Scan(catalog, []string{"notes.md"}, prev, now)

	if want := []model.FileState{prev["notes.md"]}; !reflect.DeepEqual(got, want) {
		t.Errorf("Scan() = %+v, want %+v", got, want)
	}
}

func TestScanChanged(t *testing.T) {
	cases := []struct {
		name    string
		prevSz  int64
		prevMod time.Time
	}{
		{name: "size differs", prevSz: 3, prevMod: modTime},
		{name: "mtime differs", prevSz: 5, prevMod: modTime.Add(-time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			catalog := fstest.MapFS{
				"notes.md": {Data: []byte("hello"), ModTime: modTime},
			}
			prev := map[string]model.FileState{
				"notes.md": {
					Rel:       "notes.md",
					Size:      c.prevSz,
					ModTime:   c.prevMod,
					Hash:      "old-hash",
					ChangedAt: modTime,
				},
			}

			got := Scan(catalog, []string{"notes.md"}, prev, now)

			want := []model.FileState{{
				Rel:       "notes.md",
				Size:      5,
				ModTime:   modTime,
				Hash:      helloHash,
				ChangedAt: now,
			}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Scan() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestScanKeepsRelOrder(t *testing.T) {
	catalog := fstest.MapFS{
		"a.md": {Data: []byte("hello"), ModTime: modTime},
		"b.md": {Data: []byte("hello"), ModTime: modTime},
	}
	prev := map[string]model.FileState{
		"a.md": {Rel: "a.md", Size: 5, ModTime: modTime, Hash: helloHash, ChangedAt: modTime},
	}

	got := Scan(catalog, []string{"b.md", "a.md"}, prev, now)

	if len(got) != 2 || got[0].Rel != "b.md" || got[1].Rel != "a.md" {
		t.Fatalf("Scan() rels = %v, want [b.md a.md]", rels(got))
	}
	if got[1].ChangedAt != modTime {
		t.Errorf("unchanged a.md ChangedAt = %v, want %v", got[1].ChangedAt, modTime)
	}
}

func TestScanVanished(t *testing.T) {
	catalog := fstest.MapFS{
		"here.md": {Data: []byte("hello"), ModTime: modTime},
	}

	got := Scan(catalog, []string{"gone.md", "here.md"}, nil, now)

	if len(got) != 1 || got[0].Rel != "here.md" {
		t.Errorf("Scan() rels = %v, want [here.md]", rels(got))
	}
}

func TestScanUnreadable(t *testing.T) {
	catalog := unreadableFS{
		FS: fstest.MapFS{
			"bad.md":  {Data: []byte("hello"), ModTime: modTime},
			"good.md": {Data: []byte("hello"), ModTime: modTime},
		},
		name: "bad.md",
	}

	got := Scan(catalog, []string{"bad.md", "good.md"}, nil, now)

	if len(got) != 1 || got[0].Rel != "good.md" {
		t.Errorf("Scan() rels = %v, want [good.md]", rels(got))
	}
}

// unreadableFS stats name normally but fails every read of it.
type unreadableFS struct {
	fs.FS
	name string
}

func (u unreadableFS) Open(name string) (fs.File, error) {
	f, err := u.FS.Open(name)
	if err != nil || name != u.name {
		return f, err
	}
	return failingReads{f}, nil
}

type failingReads struct {
	fs.File
}

func (failingReads) Read([]byte) (int, error) {
	return 0, errors.New("i/o error")
}

func rels(states []model.FileState) []string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = s.Rel
	}
	return out
}

// Scan hashes a file as it reads it, so even a large file costs little
// memory.
func TestScanStreamsLargeFile(t *testing.T) {
	const size = 64 << 20
	catalog := zerosFS{size: size}
	want := sha256.New()
	if _, err := io.Copy(want, io.LimitReader(zeros{}, size)); err != nil {
		t.Fatal(err)
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got := Scan(catalog, []string{"big.csv"}, nil, now)
	runtime.ReadMemStats(&after)

	if len(got) != 1 || got[0].Hash != model.Hash(hex.EncodeToString(want.Sum(nil))) || got[0].Size != size {
		t.Fatalf("Scan() = %+v", got)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 1<<20 {
		t.Errorf("Scan allocated %d bytes for a %d-byte file", alloc, size)
	}
}

// zerosFS holds one file, big.csv, of size zero bytes, made up as it's read.
type zerosFS struct{ size int64 }

func (z zerosFS) Open(name string) (fs.File, error) {
	if name != "big.csv" {
		return nil, fs.ErrNotExist
	}
	return &zerosFile{size: z.size, r: io.LimitReader(zeros{}, z.size)}, nil
}

type zerosFile struct {
	size int64
	r    io.Reader
}

func (f *zerosFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *zerosFile) Close() error               { return nil }
func (f *zerosFile) Stat() (fs.FileInfo, error) { return zerosInfo{f.size}, nil }

type zerosInfo struct{ size int64 }

func (zerosInfo) Name() string       { return "big.csv" }
func (i zerosInfo) Size() int64      { return i.size }
func (zerosInfo) Mode() fs.FileMode  { return 0o644 }
func (zerosInfo) ModTime() time.Time { return modTime }
func (zerosInfo) IsDir() bool        { return false }
func (zerosInfo) Sys() any           { return nil }

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
