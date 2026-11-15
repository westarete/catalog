package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/settings"
)

// reloadGlobal rereads global.toml from GlobalPath, if it's set. If the file
// can't be read, doesn't parse, or has no API key, it journals the error and
// keeps the settings from the last file that loaded. When the key or the
// model changes, it builds a new Provider.
func (d *Daemon) reloadGlobal(now time.Time) error {
	if d.GlobalPath == "" {
		return nil
	}
	g, err := readGlobal(d.GlobalPath)
	if err != nil {
		return d.report(journal.Event{
			Time: now, Kind: journal.KindConfigError,
			Dir: filepath.Dir(d.GlobalPath), Rel: filepath.Base(d.GlobalPath),
			Error: err.Error(),
		})
	}
	if g.APIKey != d.Global.APIKey || g.Model != d.Global.Model {
		d.Provider = d.NewProvider(g.APIKey, g.Model)
	}
	d.Global = g
	return nil
}

func readGlobal(path string) (settings.Global, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return settings.Global{}, err
	}
	g, err := settings.ParseGlobal(data)
	if err != nil {
		return settings.Global{}, err
	}
	if g.APIKey == "" {
		return settings.Global{}, errors.New("no api_key")
	}
	return g, nil
}
