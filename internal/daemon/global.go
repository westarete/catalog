package daemon

import (
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/settings"
)

// reloadGlobal rereads global.toml from GlobalPath, if it's set. If the file
// can't be read or doesn't parse, it journals the error and keeps the
// settings from the last file that loaded. If the file is
// world-accessible, it also drops the key, so the daemon summarizes nothing
// until the owner fixes the file's mode. A file with no API key is used
// for everything else, and journaled; the daemon then has no Provider and
// summarizes nothing. When the key or the model changes, reloadGlobal builds
// a new Provider and ends any pause.
func (d *Daemon) reloadGlobal(now time.Time) error {
	if d.GlobalPath == "" {
		return nil
	}
	g, err := readGlobalFile(d.GlobalPath)
	if errors.Is(err, settings.ErrWorldAccessible) {
		d.Global.APIKey = ""
		d.useProvider(d.Global)
	}
	if err != nil {
		return d.reportGlobal(now, err)
	}
	if g.APIKey != d.Global.APIKey || g.Model != d.Global.Model {
		d.useProvider(g)
	}
	d.Global = g
	if g.APIKey == "" {
		return d.reportGlobal(now, errNoAPIKey)
	}
	return nil
}

var errNoAPIKey = errors.New("no api_key")

// useProvider replaces the Provider with one for g's key and model, or none
// if g has no key. The pause was for the old provider; a fixed key shouldn't
// wait out a backoff of up to 6 hours. A success still journals the resume.
func (d *Daemon) useProvider(g settings.Global) {
	d.Provider = nil
	if g.APIKey != "" {
		d.Provider = d.NewProvider(g.APIKey, g.Model)
	}
	d.pausedUntil, d.pauseReason = time.Time{}, ""
}

func (d *Daemon) reportGlobal(now time.Time, err error) error {
	return d.report(journal.Event{
		Time: now, Kind: journal.KindConfigError,
		Dir: filepath.Dir(d.GlobalPath), Rel: filepath.Base(d.GlobalPath),
		Error: err.Error(),
	})
}

// readGlobalFile reads and parses global.toml.
func readGlobalFile(path string) (settings.Global, error) {
	g, err := settings.ReadGlobalFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings.Global{}, errors.New("not found; create it with your api_key and the folders to watch")
	}
	return g, err
}
