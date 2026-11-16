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
func Publish(catalogDir, text string) (changed bool, err error) {
	dir := filepath.Join(catalogDir, ".catalog")
	target := filepath.Join(dir, "catalog.md")

	if existing, readErr := os.ReadFile(target); readErr == nil && bytes.Equal(existing, []byte(text)) {
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
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	return true, nil
}

// tempPattern names the temporary files Publish writes in .catalog/.
const tempPattern = "catalog-*.tmp"

// removeLeftovers removes the temporary files in dir that an earlier
// Publish left when its process was killed before the rename. If one can't
// be removed, the next Publish tries again.
func removeLeftovers(dir string) {
	leftovers, _ := filepath.Glob(filepath.Join(dir, tempPattern))
	for _, path := range leftovers {
		_ = os.Remove(path)
	}
}

func writeAndClose(f *os.File, text string) error {
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
