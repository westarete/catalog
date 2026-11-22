package install

import (
	"fmt"
	"os"
	"strings"
)

// Settings describes global.toml for Describe. cmd/catalog fills it in.
type Settings struct {
	Path  string // global.toml's path
	Found bool   // the file exists
	Err   error  // reading or parsing failed; its text must quote no value
}

// Keys describes keys.toml for Describe. cmd/catalog fills it in.
type Keys struct {
	Path   string // keys.toml's path
	Found  bool   // the file exists
	HasKey bool   // it has the key global.toml asks for
	Err    error  // reading or parsing failed; its text must quote no value or name
}

// Describe returns the lines that say where the launch agent, the
// settings, the API keys, and the log are. It prints paths and whether the
// key is set, never a key or its name. It leaves out the launch agent line
// when onMac is false.
// Paths are absolute and unquoted, as pwd and git print them: a path that
// starts with ~ stops working once someone quotes it for the space in
// "Application Support", and an agent's file tools need an absolute path.
func Describe(home string, s Settings, k Keys, onMac bool) string {
	var b strings.Builder
	if onMac {
		fmt.Fprintf(&b, "Launch agent: %s\n", launchAgentState(home))
	}
	fmt.Fprintf(&b, "Settings: %s (%s)\n", s.Path, fileState(s.Found, s.Err, "valid"))
	fmt.Fprintf(&b, "Keys: %s (%s)\n", k.Path, keysState(k))
	fmt.Fprintf(&b, "Log: %s\n", LogPath(home))
	return b.String()
}

func launchAgentState(home string) string {
	// Any Stat error, not only "does not exist", counts as not installed:
	// if we can't see the file, we can't say the agent is there.
	if _, err := os.Stat(PlistPath(home)); err != nil {
		return "not installed; run catalog setup"
	}
	return PlistPath(home)
}

// fileState says whether a settings file can be read, and loaded if it
// can.
func fileState(found bool, err error, loaded string) string {
	if err != nil {
		return "can't be read: " + err.Error()
	}
	if !found {
		return "not found; run catalog setup"
	}
	return loaded
}

func keysState(k Keys) string {
	loaded := "no key"
	if k.HasKey {
		loaded = "key set"
	}
	return fileState(k.Found, k.Err, loaded)
}
