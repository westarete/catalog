package main

import (
	"reflect"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name      string
		docHashes map[string]string
		rows      []profileRow
		want      docStatus
	}{
		{
			name:      "empty store, no docs",
			docHashes: map[string]string{},
			rows:      nil,
			want:      docStatus{},
		},
		{
			name:      "new: on disk, no row",
			docHashes: map[string]string{"a.md": "h1"},
			rows:      nil,
			want:      docStatus{new: []string{"a.md"}},
		},
		{
			name:      "unchanged: hash matches",
			docHashes: map[string]string{"a.md": "h1"},
			rows:      []profileRow{{path: "a.md", contentHash: "h1", profile: "p"}},
			want:      docStatus{},
		},
		{
			name:      "modified: hash differs",
			docHashes: map[string]string{"a.md": "h2"},
			rows:      []profileRow{{path: "a.md", contentHash: "h1", profile: "p"}},
			want:      docStatus{modified: []string{"a.md"}},
		},
		{
			name:      "deleted: row exists, no doc",
			docHashes: map[string]string{},
			rows:      []profileRow{{path: "a.md", contentHash: "h1", profile: "p"}},
			want:      docStatus{deleted: []string{"a.md"}},
		},
		{
			name: "one of each, sorted independently",
			docHashes: map[string]string{
				"new.md":      "h1",
				"modified.md": "h2",
				"same.md":     "h3",
			},
			rows: []profileRow{
				{path: "modified.md", contentHash: "old", profile: "p"},
				{path: "same.md", contentHash: "h3", profile: "p"},
				{path: "deleted.md", contentHash: "h4", profile: "p"},
			},
			want: docStatus{
				new:      []string{"new.md"},
				modified: []string{"modified.md"},
				deleted:  []string{"deleted.md"},
			},
		},
		{
			// A doc excluded by a config.toml change looks identical to a
			// deleted file: it's simply absent from docHashes. See FUTURE.md
			// — deliberately one category, not two.
			name:      "excluded-by-config doc reads the same as deleted",
			docHashes: map[string]string{},
			rows:      []profileRow{{path: "excluded.md", contentHash: "h1", profile: "p"}},
			want:      docStatus{deleted: []string{"excluded.md"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classify(c.docHashes, c.rows)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("classify() = %+v, want %+v", got, c.want)
			}
		})
	}
}
