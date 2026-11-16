// Package publish writes catalog.md into a catalog's .catalog/ folder.
package publish

import (
	"bytes"
	"os"
	"path/filepath"
)

// Publish writes text to <catalogDir>/.catalog/catalog.md and reports whether
// the file changed. It writes through a temporary file in .catalog/ and
// renames it into place, so readers never see a partial file. If the file
// already holds text, Publish writes nothing. Before writing, it removes any
// temporary file an earlier Publish left behind. It never creates .catalog/.
//
// Only the owner can read catalog.md, since it summarizes files that may be
// private; the agents that read it run as the owner.
func Publish(catalogDir, text string) (changed bool, err error) {
	dir := filepath.Join(catalogDir, ".catalog")
	target := filepath.Join(dir, "catalog.md")

	if isCurrent(target, text) {
		return false, nil
	}

	removeLeftovers(dir)
	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	if err := writeAndClose(tmp, text); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	if err := os.Chmod(tmpPath, privateMode); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	return true, nil
}

// isCurrent reports whether the file at target is a private regular file
// that already holds text. Anything else, such as a catalog.md an earlier
// version left readable by others or a link, is replaced, which changes
// neither a link's target nor a file another account owns.
func isCurrent(target, text string) bool {
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != privateMode {
		return false
	}
	existing, err := os.ReadFile(target)
	return err == nil && bytes.Equal(existing, []byte(text))
}

// tempPattern names the temporary files Publish writes in .catalog/.
const tempPattern = "catalog-*.tmp"

// privateMode lets only the owner read and write catalog.md.
const privateMode = 0o600

// removeLeftovers removes the temporary files in dir that an earlier
// Publish left when its process was killed before the rename. If one can't
// be removed, the next Publish tries again.
// It matches only the names in dir against tempPattern, so characters such as
// "[" or "*" in the catalog's own path aren't read as part of the pattern.
func removeLeftovers(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if ok, _ := filepath.Match(tempPattern, e.Name()); ok {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func writeAndClose(f *os.File, text string) error {
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
