package daemon

import (
	"errors"
	"io/fs"
	"path/filepath"
	"time"

	"westarete.com/catalog/internal/journal"
	"westarete.com/catalog/internal/settings"
)

// reloadGlobal rereads global.toml from GlobalPath, if it's set, and then
// the key it names from keys.toml at KeysPath. If either file can't be read
// or doesn't parse, it journals the error and keeps what it loaded from that
// file last; while global.toml fails, it doesn't load keys.toml, but still
// checks its mode. If either file is world-accessible, it drops the key, so
// the daemon summarizes nothing until the owner fixes the file's mode. If keys.toml has no key by
// the name global.toml asks for, it drops the key too and journals that.
// When the key or the model changes, reloadGlobal builds a new Provider and
// ends any pause.
func (d *Daemon) reloadGlobal(now time.Time) error {
	if d.GlobalPath == "" {
		return nil
	}
	g, err := readSettingsFile(d.GlobalPath, settings.ReadGlobalFile)
	if err != nil {
		return d.globalFailed(now, err)
	}
	key, keyErr := d.reloadKey(g.KeyName())
	if key != d.apiKey || g.Model != d.Global.Model {
		d.useProvider(key, g.Model)
	}
	d.Global = g
	if keyErr != nil {
		return d.reportConfig(now, d.KeysPath, keyErr)
	}
	return nil
}

// globalFailed journals err, the error from reading global.toml, and keeps
// the settings loaded last. It drops the key if global.toml or keys.toml is
// world-accessible.
func (d *Daemon) globalFailed(now time.Time, err error) error {
	errs := []error{d.reportConfig(now, d.GlobalPath, err)}
	keysErr := keysWorldAccessible(d.KeysPath)
	if keysErr != nil {
		errs = append(errs, d.reportConfig(now, d.KeysPath, keysErr))
	}
	if errors.Is(err, settings.ErrWorldAccessible) || keysErr != nil {
		d.useProvider("", d.Global.Model)
	}
	return errors.Join(errs...)
}

// keysWorldAccessible returns settings.ErrWorldAccessible, with the mode, if
// the keys.toml at path is world-accessible, and nil otherwise. It reads the
// file through settings, which checks the mode of the file it opened.
func keysWorldAccessible(path string) error {
	_, err := settings.ReadKeysFile(path)
	if errors.Is(err, settings.ErrWorldAccessible) {
		return err
	}
	return nil
}

// errNoKey never names the key global.toml asks for, since a person may have
// pasted the key itself there.
var errNoKey = errors.New("no key under the name global.toml asks for")

// reloadKey returns the key called name in keys.toml at KeysPath. If the
// file is world-accessible or has no such key, it returns no key. If the
// file can't be read or doesn't parse, it returns the key loaded last.
// Either way, it returns the error.
func (d *Daemon) reloadKey(name string) (string, error) {
	keys, err := readSettingsFile(d.KeysPath, settings.ReadKeysFile)
	if errors.Is(err, settings.ErrWorldAccessible) {
		return "", err
	}
	if err != nil {
		return d.apiKey, err
	}
	if keys[name] == "" {
		return "", errNoKey
	}
	return keys[name], nil
}

// useProvider replaces the Provider with one for apiKey and model, or none
// if apiKey is empty. The pause was for the old provider; a fixed key
// shouldn't wait out a backoff of up to 6 hours. A success still journals
// the resume.
func (d *Daemon) useProvider(apiKey, model string) {
	d.apiKey = apiKey
	d.Provider = nil
	if apiKey != "" {
		d.Provider = d.NewProvider(apiKey, model)
	}
	d.pausedUntil, d.pauseReason = time.Time{}, ""
}

// reportConfig journals err as a config error in the settings file at path.
func (d *Daemon) reportConfig(now time.Time, path string, err error) error {
	return d.report(journal.Event{
		Time: now, Kind: journal.KindConfigError,
		Dir: filepath.Dir(path), Rel: filepath.Base(path),
		Error: err.Error(),
	})
}

// readSettingsFile reads the settings file at path with read, and says how
// to fix a missing one.
func readSettingsFile[T any](path string, read func(string) (T, error)) (T, error) {
	v, err := read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return v, errors.New("not found; run catalog setup")
	}
	return v, err
}
