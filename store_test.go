package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenStoreCreatesTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	db, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

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
	db.Close()

	db2, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

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
