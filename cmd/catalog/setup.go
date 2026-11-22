package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/term"

	"westarete.com/catalog/internal/install"
	"westarete.com/catalog/internal/settings"
)

// launchctl and onMac are variables so tests never run the real launchctl
// and can pretend to be on, or off, a Mac.
var (
	launchctl install.Launchctl = install.RunLaunchctl
	onMac                       = runtime.GOOS == "darwin"
)

// runSetup carries out `catalog setup` with the arguments that follow it. It
// asks for settings unless --agent-only is given, and then, on a Mac, loads
// the launch agent. With --agent-only it never reads stdin.
func runSetup(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("catalog setup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	agentOnly := flags.Bool("agent-only", false, "install the launch agent without asking for settings")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errUsage
	}
	stateDir, err := privateStateDir()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if !*agentOnly {
		in := bufio.NewReader(stdin)
		prompt := install.Prompt{In: in, Out: stdout, ReadKey: keyReader(stdin, in), Home: home}
		if err := install.AskSettings(prompt, filepath.Join(stateDir, settings.GlobalFile)); err != nil {
			return err
		}
	}
	if !onMac {
		_, _ = fmt.Fprintln(stdout, "Setup can start the daemon only on a Mac.")
		return nil
	}
	return installAgent(home, stdout)
}

// installAgent loads the launch agent for the running program and says what
// it did.
func installAgent(home string, stdout io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	program, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	agent := install.Agent{Home: home, UID: os.Getuid(), Launchctl: launchctl}
	previous, err := agent.Install(program)
	if err != nil {
		return err
	}
	if previous != "" {
		_, _ = fmt.Fprintf(stdout, "The daemon was running %s, and now it runs %s.\n", previous, program)
	}
	_, _ = fmt.Fprintln(stdout, "The catalog daemon is running. To see what it's doing, run `catalog status`.")
	return nil
}

// keyReader returns a function that reads the API key. If stdin is a
// terminal, it reads without echo; otherwise it reads one line from in, the
// reader the folders come from too. It prints nothing.
func keyReader(stdin io.Reader, in *bufio.Reader) func() (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return func() (string, error) {
			key, err := term.ReadPassword(int(f.Fd()))
			return string(key), err
		}
	}
	return func() (string, error) {
		line, err := in.ReadString('\n')
		if errors.Is(err, io.EOF) && line != "" {
			err = nil
		}
		return strings.TrimRight(line, "\r\n"), err
	}
}

// settingsState reports on global.toml in stateDir for status: whether it
// exists, holds an API key, or can't be read. It never returns the key.
func settingsState(stateDir string) install.Settings {
	path := filepath.Join(stateDir, settings.GlobalFile)
	data, err := os.ReadFile(path) //nolint:gosec // path is global.toml in the state folder
	if errors.Is(err, fs.ErrNotExist) {
		return install.Settings{Path: path}
	}
	if err != nil {
		return install.Settings{Path: path, Found: true, Err: err}
	}
	g, err := settings.ParseGlobal(data)
	if err != nil {
		return install.Settings{Path: path, Found: true, Err: err}
	}
	return install.Settings{Path: path, Found: true, HasKey: g.APIKey != ""}
}
