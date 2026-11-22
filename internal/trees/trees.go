// Package trees finds the catalogs inside a watched folder and lists the
// files that belong to each one.
package trees

import (
	"io/fs"
	"path"
	"slices"
)

const (
	catalogDir = ".catalog"
	gitDir     = ".git"
)

// FindCatalogs returns every directory in watched, "." included, that
// contains a .catalog directory, as sorted slash paths. It never descends
// into .git or .catalog.
func FindCatalogs(watched fs.FS) ([]string, error) {
	var found []string
	err := fs.WalkDir(watched, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		switch d.Name() {
		case gitDir:
			return fs.SkipDir
		case catalogDir:
			found = append(found, path.Dir(p))
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(found)
	return found, nil
}

// Members returns the sorted slash paths of the regular files in catalog that
// match an include glob and no exclude glob. It leaves out anything under a
// directory listed in nested (relative to catalog), under .git, or under
// .catalog.
func Members(catalog fs.FS, include, exclude, nested []string) ([]string, error) {
	var members []string
	err := fs.WalkDir(catalog, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipsDir(p, d.Name(), nested) {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && isMember(p, include, exclude) {
			members = append(members, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(members)
	return members, nil
}

func skipsDir(p, name string, nested []string) bool {
	if p == "." {
		return false
	}
	return name == gitDir || name == catalogDir || slices.Contains(nested, p)
}

func isMember(p string, include, exclude []string) bool {
	return matchesAny(include, p) && !matchesAny(exclude, p)
}

func matchesAny(patterns []string, p string) bool {
	return slices.ContainsFunc(patterns, func(pattern string) bool {
		return Match(pattern, p)
	})
}
