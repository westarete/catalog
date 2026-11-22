package install

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// configTemplate is the config.toml that InitCatalog writes. It sets
// nothing, and shows each setting with a line on what it does.
const configTemplate = `# Settings for this catalog. Each list adds to the lists in the daemon's
# global.toml and to its built-in defaults. A file is in the catalog if it
# matches an include pattern and no exclude pattern.
#
# Patterns for more files to catalog, relative to this folder:
# include = ["**/*.go"]
#
# Patterns for files to leave out, even if an include pattern matches:
# exclude = ["drafts/**"]
`

// gitignoreContent keeps the catalog and the temporary file the publisher
// writes it to (tempPattern in internal/publish/publish.go) out of commits.
const gitignoreContent = "/catalog.md\n/catalog-*.tmp\n"

// InitCatalog makes folder a catalog: it writes .catalog/config.toml, and,
// when folder is in a Git repository, .catalog/.gitignore, and says on out
// which files it created and which it left as they were. It never changes a
// file that exists. The files get the default modes, since they travel with
// the folder. It fails without creating anything if folder doesn't exist or
// its .catalog isn't a folder, and fails if config.toml is there but isn't a
// regular file, since the daemon doesn't count that folder as a catalog.
func InitCatalog(folder string, out io.Writer) error {
	info, err := os.Stat(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s doesn't exist", folder)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s isn't a folder", folder)
	}
	dir := filepath.Join(folder, ".catalog")
	if err := makeCatalogDir(dir); err != nil {
		return err
	}
	files := []struct{ name, content string }{{"config.toml", configTemplate}}
	if InGitRepository(folder) {
		files = append(files, struct{ name, content string }{".gitignore", gitignoreContent})
	}
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		created, err := createSharedFile(path, f.content)
		if err != nil {
			return err
		}
		if !created && f.name == "config.toml" && !isRegularFile(path) {
			return fmt.Errorf("%s isn't a regular file, so %s isn't a catalog; replace it with a file", path, folder)
		}
		verb := "Left %s as it was\n"
		if created {
			verb = "Created %s\n"
		}
		if _, err := fmt.Fprintf(out, verb, path); err != nil {
			return err
		}
	}
	return nil
}

// makeCatalogDir creates the .catalog folder at dir, or leaves it as it is if
// it's a folder already. It refuses a symbolic link or a file there, so init
// never writes through a link to somewhere else.
func makeCatalogDir(dir string) error {
	info, err := os.Lstat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s isn't a folder", dir)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Mkdir(dir, 0o777) //nolint:gosec // the folder travels with the catalog, so it gets the default mode, narrowed by the umask
}

// isRegularFile reports whether path is a regular file, without following a
// symbolic link.
func isRegularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// createSharedFile writes content to a new file at path, with the default
// mode, and reports whether it did. It never changes a file that exists.
func createSharedFile(path, content string) (created bool, err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666) //nolint:gosec // shared with the folder on purpose; the umask narrows it
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(content); err != nil {
		return false, errors.Join(err, f.Close(), os.Remove(path))
	}
	if err := f.Close(); err != nil {
		return false, errors.Join(err, os.Remove(path))
	}
	return true, nil
}

// InGitRepository reports whether folder or one of its parents holds an
// entry named .git, a folder or, in a worktree, a file. It never runs git.
func InGitRepository(folder string) bool {
	for dir := folder; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		if parent := filepath.Dir(dir); parent == dir {
			return false
		}
	}
}
