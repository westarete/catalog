package install

import (
	"fmt"
	"os"
	"strings"
)

// Settings describes global.toml for Describe. cmd/catalog fills it in.
type Settings struct {
	Path   string // global.toml's path
	Found  bool   // the file exists
	HasKey bool   // api_key has a value
	Err    error  // reading or parsing failed; its text must quote no value
}

// Describe returns the lines that say where the launch agent, the
// settings, and the log are. It prints paths and whether a key is set,
// never the key. It leaves out the launch agent line when onMac is false.
func Describe(home string, s Settings, onMac bool) string {
	var b strings.Builder
	if onMac {
		fmt.Fprintf(&b, "Launch agent: %s\n", launchAgentState(home))
	}
	fmt.Fprintf(&b, "Settings: %s (%s)\n", Tilde(home, s.Path), settingsState(s))
	fmt.Fprintf(&b, "Log: %s\n", Tilde(home, LogPath(home)))
	return b.String()
}

func launchAgentState(home string) string {
	// Any Stat error, not only "does not exist", counts as not installed:
	// if we can't see the file, we can't say the agent is there.
	if _, err := os.Stat(PlistPath(home)); err != nil {
		return "not installed; run catalog setup"
	}
	return Tilde(home, PlistPath(home))
}

func settingsState(s Settings) string {
	switch {
	case s.Err != nil:
		return "can't be read: " + s.Err.Error()
	case !s.Found:
		return "not found; run catalog setup"
	case !s.HasKey:
		return "no API key"
	default:
		return "API key set"
	}
}
