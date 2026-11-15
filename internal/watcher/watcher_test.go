package watcher

import (
	"errors"
	"io/fs"
	"reflect"
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
