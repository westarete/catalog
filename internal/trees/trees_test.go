package trees

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		pat, rel string
		want     bool
	}{
		{"leadership/**/*.md", "leadership/about.md", true},
		{"leadership/**/*.md", "leadership/sub/deep.md", true},
		{"leadership/**/*.md", "outreach/about.md", false},
		{"leadership/**/*.md", "leadership/notes.txt", false},
		{"**/README.md", "README.md", true},
		{"**/README.md", "operations/README.md", true},
		{"**/README.md", "operations/skills/x/README.md", true},
		{"**/README.md", "operations/notes.md", false},
		{"operations/skills/**/*", "operations/skills/markdown/SKILL.md", true},
		{"operations/skills/**/*", "operations/skills/a/b/c.txt", true},
		{"operations/skills/**/*", "operations/context-system.md", false},
		{"product/**/*.md", "product/offering.md", true},
		{"*.md", "MANIFEST.md", true},
		{"*.md", "leadership/about.md", false},
		{"a/**", "a/b/c", true},
		{"a/b", "a/b/c", false},
		{"a/b/c", "a/b", false},
		{"[", "a", false},
		{"a/**/**", "a", true},
		{"**/*.md", "x.txt", false},
	}
	for _, c := range cases {
		if got := Match(c.pat, c.rel); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pat, c.rel, got, c.want)
		}
	}
}

func TestFindCatalogs(t *testing.T) {
	watched := fstest.MapFS{
		"a/.catalog/config.toml":    {},
		"a/b/.catalog/config.toml":  {},
		"a/.git/x/.catalog/x":       {},
		"c/file.md":                 {},
		".catalog/config.toml":      {},
		"a/.catalog/.catalog/y.md":  {},
		"d/.catalog/catalog.md":     {},
		"e/.catalog/config.toml/zz": {},
	}
	got, err := FindCatalogs(watched)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".", "a", "a/b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindCatalogs = %v, want %v", got, want)
	}
}

func TestFindCatalogsNeedsConfig(t *testing.T) {
	watched := fstest.MapFS{"a/.catalog": {Mode: fs.ModeDir}}
	got, err := FindCatalogs(watched)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("FindCatalogs = %v, want none", got)
	}
}

func TestFindCatalogsNested(t *testing.T) {
	watched := fstest.MapFS{
		"a/.catalog/config.toml":     {},
		"a/b/c/.catalog/config.toml": {},
	}
	got, err := FindCatalogs(watched)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "a/b/c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindCatalogs = %v, want %v", got, want)
	}
}

func TestFindCatalogsIgnoresLinkedConfig(t *testing.T) {
	root := t.TempDir()
	dot := filepath.Join(root, "a", ".catalog")
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "elsewhere.toml")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dot, "config.toml")); err != nil {
		t.Fatal(err)
	}
	got, err := FindCatalogs(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("FindCatalogs = %v, want none", got)
	}
}

func TestFindCatalogsUnreadableCatalogDirIsNotACatalog(t *testing.T) {
	watched := errorFS{MapFS: fstest.MapFS{"a/.catalog/config.toml": {}}, bad: "a/.catalog"}
	got, err := FindCatalogs(watched)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("FindCatalogs = %v, want none", got)
	}
}

// errorFS fails to list the directory named bad.
type errorFS struct {
	fstest.MapFS
	bad string
}

func (e errorFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == e.bad {
		return nil, fs.ErrPermission
	}
	return e.MapFS.ReadDir(name)
}

func TestFindCatalogsIgnoresCatalogFile(t *testing.T) {
	watched := fstest.MapFS{"a/.catalog": {Data: []byte("not a dir")}}
	got, err := FindCatalogs(watched)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("FindCatalogs = %v, want none", got)
	}
}

func TestMembersExcludesNested(t *testing.T) {
	catalog := fstest.MapFS{"a/doc.md": {}, "a/b/doc.md": {}}
	got, err := Members(catalog, []string{"**/*.md"}, nil, []string{"a/b"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a/doc.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Members = %v, want %v", got, want)
	}
}

func TestMembersSkipsCatalogAndGit(t *testing.T) {
	catalog := fstest.MapFS{
		".catalog/catalog.md": {},
		".git/x.md":           {},
		"README.md":           {},
		"notes.md":            {},
	}
	got, err := Members(catalog, []string{"**/*.md"}, []string{"README.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"notes.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Members = %v, want %v", got, want)
	}
}

func TestMembersWorktreeFolders(t *testing.T) {
	catalog := fstest.MapFS{".claude/worktrees/w/doc.md": {}}
	got, err := Members(catalog, []string{"**/*.md"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".claude/worktrees/w/doc.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Members = %v, want %v", got, want)
	}
}

func TestMembersFiltersAndSorts(t *testing.T) {
	catalog := fstest.MapFS{
		"z.md":         {},
		"b.md":         {},
		"skip.txt":     {},
		"a/ignored.md": {},
		"ab/keep.md":   {},
	}
	got, err := Members(catalog, []string{"**/*.md"}, []string{"a/**"}, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ab/keep.md", "b.md", "z.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Members = %v, want %v", got, want)
	}
}

func TestMembersIgnoreWithoutNested(t *testing.T) {
	catalog := fstest.MapFS{"a/x.md": {}, "b.md": {}}
	got, err := Members(catalog, []string{"**/*.md"}, []string{"a/**"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"b.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Members = %v, want %v", got, want)
	}
}
