package main

import (
	"bufio"
	"context"
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
// the launch agent. With --agent-only it never reads stdin. If ctx ends, as
// it does on Ctrl-C, setup stops at once with nothing more written or loaded.
func runSetup(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
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
		in := bufio.NewReader(interruptible{ctx, stdin})
		prompt := install.Prompt{In: in, Out: stdout, ReadKey: keyReader(ctx, stdin, in), Home: home}
		globalPath, keysPath := filepath.Join(stateDir, settings.GlobalFile), filepath.Join(stateDir, settings.KeysFile)
		checkKeys := func() install.KeyCheck { return keyCheck(stateDir) }
		if err := install.AskSettings(prompt, globalPath, keysPath, checkKeys); err != nil {
			return interrupted(ctx, err)
		}
	}
	if ctx.Err() != nil {
		return errInterrupted
	}
	if !onMac {
		_, _ = fmt.Fprintln(stdout, "Setup can start the daemon only on a Mac.")
		return nil
	}
	return installAgent(home, stdout)
}

var errInterrupted = errors.New("setup interrupted")

// interrupted returns errInterrupted if ctx has ended, and err otherwise.
func interrupted(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errInterrupted
	}
	return err
}

// interruptible reads from r, but stops waiting when ctx ends. A terminal
// still catches Ctrl-C as a signal while it waits for input, so the read
// alone would never return. The read it abandons ends with the process.
type interruptible struct {
	ctx context.Context
	r   io.Reader
}

func (i interruptible) Read(p []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	got := make(chan result, 1)
	buf := make([]byte, len(p))
	go func() {
		n, err := i.r.Read(buf)
		got <- result{n, err}
	}()
	select {
	case r := <-got:
		return copy(p, buf[:r.n]), r.err
	case <-i.ctx.Done():
		return 0, i.ctx.Err()
	}
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
func keyReader(ctx context.Context, stdin io.Reader, in *bufio.Reader) func() (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return func() (string, error) { return readHiddenKey(ctx, int(f.Fd())) }
	}
	return func() (string, error) {
		line, err := in.ReadString('\n')
		if errors.Is(err, io.EOF) && line != "" {
			err = nil
		}
		return strings.TrimRight(line, "\r\n"), err
	}
}

// readHiddenKey reads a line from the terminal fd with echo off. If ctx ends
// first, it turns echo back on and returns ctx's error, because Ctrl-C leaves
// the terminal as ReadPassword set it.
func readHiddenKey(ctx context.Context, fd int) (string, error) {
	saved, err := term.GetState(fd)
	if err != nil {
		return "", err
	}
	type result struct {
		key []byte
		err error
	}
	got := make(chan result, 1)
	go func() {
		key, err := term.ReadPassword(fd)
		got <- result{key, err}
	}()
	select {
	case r := <-got:
		return string(r.key), r.err
	case <-ctx.Done():
		return "", errors.Join(ctx.Err(), term.Restore(fd, saved))
	}
}

// settingsState reports on global.toml and keys.toml in stateDir for
// status: whether each exists or can't be read, and whether keys.toml has
// the key global.toml asks for. It never returns a key or its name.
func settingsState(stateDir string) (install.Settings, install.Keys) {
	path := filepath.Join(stateDir, settings.GlobalFile)
	// A global.toml that doesn't load gives the zero Global, whose KeyName
	// is DefaultKeyName.
	g, err := settings.ReadGlobalFile(path)
	s := install.Settings{Path: path, Found: !errors.Is(err, fs.ErrNotExist)}
	if s.Found {
		s.Err = err
	}
	return s, keysState(stateDir, g.KeyName())
}

// keysState reports on keys.toml in stateDir for status, and whether it has
// the key called name.
func keysState(stateDir, name string) install.Keys {
	path := filepath.Join(stateDir, settings.KeysFile)
	keys, err := settings.ReadKeysFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return install.Keys{Path: path}
	}
	if err != nil {
		return install.Keys{Path: path, Found: true, Err: err}
	}
	return install.Keys{Path: path, Found: true, HasKey: keys[name] != ""}
}

// keyCheck says whether keys.toml in stateDir has the key global.toml asks
// for, or default if global.toml is missing or doesn't load.
func keyCheck(stateDir string) install.KeyCheck {
	// A global.toml that doesn't load gives the zero Global, whose KeyName
	// is DefaultKeyName; status reports the error.
	g, _ := settings.ReadGlobalFile(filepath.Join(stateDir, settings.GlobalFile))
	k := keysState(stateDir, g.KeyName())
	if k.Err != nil {
		return install.KeyUnknown
	}
	if k.HasKey {
		return install.KeyFound
	}
	if g.KeyName() == settings.DefaultKeyName {
		return install.DefaultKeyMissing
	}
	return install.NamedKeyMissing
}
