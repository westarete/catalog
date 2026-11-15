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
// already holds text, Publish writes nothing. It never creates .catalog/.
func Publish(catalogDir, text string) (changed bool, err error) {
	dir := filepath.Join(catalogDir, ".catalog")
	target := filepath.Join(dir, "catalog.md")

	if existing, readErr := os.ReadFile(target); readErr == nil && bytes.Equal(existing, []byte(text)) {
		return false, nil
	}

	tmp, err := os.CreateTemp(dir, "catalog-*.tmp")
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

func writeAndClose(f *os.File, text string) error {
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
