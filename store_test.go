package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func openTestStore(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.db")
	db, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing store: %v", err)
		}
	})
	return db
}

func TestStoreExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	if storeExists(path) {
		t.Error("storeExists on a path with nothing there should be false")
	}
	db, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if !storeExists(path) {
		t.Error("storeExists after openStore created the file should be true")
	}
}

func TestOpenStoreCreatesTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	db, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing store: %v", err)
		}
	}()

	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'profiles'`).Scan(&name)
	if err != nil {
		t.Fatalf("profiles table not created: %v", err)
	}
}

func TestOpenStorePreservesExistingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	db, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO profiles (path, content_hash, profile) VALUES (?, ?, ?)`,
		"a.md", "hash1", "profile text")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db2.Close(); err != nil {
			t.Errorf("closing store: %v", err)
		}
	}()

	var profile string
	err = db2.QueryRow(`SELECT profile FROM profiles WHERE path = ?`, "a.md").Scan(&profile)
	if err != nil {
		t.Fatalf("existing row lost: %v", err)
	}
	if profile != "profile text" {
		t.Errorf("profile = %q, want %q", profile, "profile text")
	}
}

func TestOpenStoreRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := openStore(path)
	if err == nil {
		t.Fatal("openStore on a corrupt file: got nil error, want an error")
	}
}

func TestContentHashDeterministic(t *testing.T) {
	content := []byte("hello")
	first := contentHash(content)
	second := contentHash(content)
	if first != second {
		t.Error("identical content produced different hashes")
	}
}

func TestContentHashDiffers(t *testing.T) {
	if contentHash([]byte("hello")) == contentHash([]byte("world")) {
		t.Error("different content produced the same hash")
	}
}

func TestContentHashKnownValue(t *testing.T) {
	// sha256("hello") — a fixed value so a regression to a different hash
	// (or a broken implementation like appending instead of hashing) is
	// caught even if it happens to still be deterministic.
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got := contentHash([]byte("hello")); got != want {
		t.Errorf("contentHash(%q) = %s, want %s", "hello", got, want)
	}
}

func TestReadProfilesEmpty(t *testing.T) {
	db := openTestStore(t)
	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("readProfiles on empty store = %v, want empty", rows)
	}
}

func TestWriteThenReadProfile(t *testing.T) {
	db := openTestStore(t)
	want := profileRow{path: "a.md", contentHash: "hash1", profile: "profile text"}
	if err := writeProfile(db, want); err != nil {
		t.Fatal(err)
	}

	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0] != want {
		t.Errorf("readProfiles = %v, want [%v]", rows, want)
	}
}

func TestWriteProfileUpsertsExistingPath(t *testing.T) {
	db := openTestStore(t)
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: "hash1", profile: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: "hash2", profile: "new"}); err != nil {
		t.Fatal(err)
	}

	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	want := profileRow{path: "a.md", contentHash: "hash2", profile: "new"}
	if len(rows) != 1 || rows[0] != want {
		t.Errorf("readProfiles after upsert = %v, want [%v] (one row, not two)", rows, want)
	}
}

func TestWriteProfileMultipleRows(t *testing.T) {
	db := openTestStore(t)
	for _, r := range []profileRow{
		{path: "a.md", contentHash: "h1", profile: "p1"},
		{path: "b.md", contentHash: "h2", profile: "p2"},
	} {
		if err := writeProfile(db, r); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	want := []profileRow{
		{path: "a.md", contentHash: "h1", profile: "p1"},
		{path: "b.md", contentHash: "h2", profile: "p2"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("readProfiles = %v, want %v", rows, want)
	}
}

func TestDeleteProfile(t *testing.T) {
	db := openTestStore(t)
	if err := writeProfile(db, profileRow{path: "a.md", contentHash: "h1", profile: "p1"}); err != nil {
		t.Fatal(err)
	}
	if err := deleteProfile(db, "a.md"); err != nil {
		t.Fatal(err)
	}

	rows, err := readProfiles(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("readProfiles after delete = %v, want empty", rows)
	}
}

func TestDeleteProfileMissingPathIsNotAnError(t *testing.T) {
	db := openTestStore(t)
	if err := deleteProfile(db, "never-existed.md"); err != nil {
		t.Errorf("deleteProfile on a missing path returned an error: %v", err)
	}
}
