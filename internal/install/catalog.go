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
// the folder. It fails without creating anything if folder doesn't exist.
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
	// The folder is shared with the catalog's folder; the umask narrows 0777.
	if err := os.Mkdir(dir, 0o777); err != nil && !errors.Is(err, fs.ErrExist) { //nolint:gosec // see above
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
