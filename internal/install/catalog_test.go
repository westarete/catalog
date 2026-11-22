package install

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"westarete.com/catalog/internal/settings"
)

func initCatalog(t *testing.T, folder string) string {
	t.Helper()
	var out bytes.Buffer
	if err := InitCatalog(folder, &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestInitCreatesConfig(t *testing.T) {
	folder := t.TempDir()
	config := filepath.Join(folder, ".catalog", "config.toml")

	out := initCatalog(t, folder)

	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != configTemplate {
		t.Errorf("config.toml:\n%s", data)
	}
	c, err := settings.ParseCatalog(data)
	if err != nil {
		t.Fatalf("template doesn't parse: %v", err)
	}
	if len(c.Include) != 0 || len(c.Exclude) != 0 {
		t.Errorf("template sets %+v, want nothing", c)
	}
	for _, line := range []string{"# include = ", "# exclude = "} {
		if !strings.Contains(string(data), line) {
			t.Errorf("template lacks %q", line)
		}
	}
	if want := "Created " + config + "\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

func TestInitWritesGitignoreOnlyInRepository(t *testing.T) {
	for name, mark := range map[string]func(t *testing.T, dir string){
		"git folder": func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"git file, as in a worktree": func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			mark(t, repo)
			folder := filepath.Join(repo, "docs")
			if err := os.Mkdir(folder, 0o755); err != nil {
				t.Fatal(err)
			}
			ignore := filepath.Join(folder, ".catalog", ".gitignore")

			out := initCatalog(t, folder)

			data, err := os.ReadFile(ignore)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "/catalog.md\n/catalog-*.tmp\n" {
				t.Errorf(".gitignore = %q", data)
			}
			want := "Created " + filepath.Join(folder, ".catalog", "config.toml") + "\nCreated " + ignore + "\n"
			if out != want {
				t.Errorf("output %q, want %q", out, want)
			}
		})
	}

	t.Run("no repository", func(t *testing.T) {
		folder := t.TempDir()
		initCatalog(t, folder)
		_, err := os.Stat(filepath.Join(folder, ".catalog", ".gitignore"))
		if !os.IsNotExist(err) {
			t.Errorf("stat .gitignore: %v, want not exist", err)
		}
	})
}

func TestInitLeavesExistingFiles(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repo, ".catalog")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	config, ignore := filepath.Join(dir, "config.toml"), filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(config, []byte("include = [\"a\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignore, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := initCatalog(t, repo)

	want := "Left " + config + " as it was\nLeft " + ignore + " as it was\n"
	if out != want {
		t.Errorf("output %q, want %q", out, want)
	}
	for path, content := range map[string]string{config: "include = [\"a\"]\n", ignore: "mine\n"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != content {
			t.Errorf("%s = %q, %v; want %q", path, data, err, content)
		}
	}
}

func TestInitMissingFolder(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "nope")

	err := InitCatalog(folder, &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), folder+" doesn't exist") {
		t.Errorf("error %v, want one naming %s", err, folder)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Errorf("stat folder: %v, want not exist", err)
	}
}

func TestInitRefusesAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	err := InitCatalog(file, &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), file+" isn't a folder") {
		t.Errorf("error %v, want one saying %s isn't a folder", err, file)
	}
}

func TestInitReportsUnwritableFolder(t *testing.T) {
	folder := t.TempDir()
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })
	if f, err := os.Create(filepath.Join(folder, "probe")); err == nil {
		_ = f.Close()
		t.Skip("folder stays writable, as for root")
	}

	if err := InitCatalog(folder, &bytes.Buffer{}); err == nil {
		t.Error("no error for an unwritable folder")
	}
}

func TestInitFilesUseDefaultModes(t *testing.T) {
	folder := t.TempDir()
	if err := os.Mkdir(filepath.Join(folder, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	initCatalog(t, folder)

	for path, want := range map[string]fs.FileMode{
		filepath.Join(folder, ".catalog"):                0o755,
		filepath.Join(folder, ".catalog", "config.toml"): 0o644,
		filepath.Join(folder, ".catalog", ".gitignore"):  0o644,
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode %o, want %o", path, got, want)
		}
	}
}

func TestInGitRepository(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if InGitRepository(deep) {
		t.Skip("the temporary folder is itself inside a repository")
	}
	if err := os.Mkdir(filepath.Join(root, "a", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !InGitRepository(deep) {
		t.Error("a .git two folders up isn't found")
	}
	if !InGitRepository(filepath.Join(root, "a")) {
		t.Error(".git in the folder itself isn't found")
	}
	if InGitRepository(root) {
		t.Error("a .git below the folder was counted")
	}
}
