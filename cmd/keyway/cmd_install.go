package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/justinstimatze/keyway/internal/install"
)

// cmdInstall wires keyway's hooks into a Claude Code settings file. It
// prints the change and stops there unless --write is passed: settings.json
// is the user's file and the behavior of every future session, so touching
// it should be something they asked for twice, not a side effect of running
// the installer once.
func cmdInstall(args []string, stdout io.Writer) {
	fl := flag.NewFlagSet("install", flag.ContinueOnError)
	fl.SetOutput(os.Stderr)
	write := fl.Bool("write", false, "apply the change to settings.json instead of printing it")
	project := fl.Bool("project", false, "wire into ./.claude/settings.json instead of ~/.claude/settings.json")
	if err := fl.Parse(args); err != nil {
		os.Exit(2)
	}

	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway install: cannot find own path: %s\n", err)
		os.Exit(1)
	}
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		bin = resolved // hooks run from an arbitrary cwd and outlive a symlink
	}

	path, err := settingsPath(*project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway install: %s\n", err)
		os.Exit(1)
	}

	settings, err := install.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway install: %s\n", err)
		os.Exit(1)
	}
	changes := settings.Apply(bin)

	if !*write {
		fmt.Fprintf(stdout, "target: %s\n\n", path)
		fmt.Fprintln(stdout, "keyway install --write would apply:")
		fmt.Fprint(stdout, install.Render(changes, bin))
		fmt.Fprintln(stdout, "\nRe-run with --write to apply (the existing file is backed up first).")
		return
	}

	if install.Settled(changes) {
		fmt.Fprintf(stdout, "%s: already wired, nothing to do\n", path)
		return
	}

	backup, err := settings.Save(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway install: %s\n", err)
		os.Exit(1)
	}
	fmt.Fprint(stdout, install.Render(changes, bin))
	if backup != "" {
		fmt.Fprintf(stdout, "\nbackup: %s\n", backup)
	}
	fmt.Fprintln(stdout, "Restart Claude Code for the new hooks to take effect.")
}

// cmdUninstall strips exactly what install added and nothing else — it
// writes immediately, on the same reasoning nowcast uses: a hook that fails
// silently by design is one you cannot turn off by watching it, so turning
// it off should not need a second flag to confirm.
func cmdUninstall(args []string, stdout io.Writer) {
	fl := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fl.SetOutput(os.Stderr)
	project := fl.Bool("project", false, "operate on ./.claude/settings.json")
	if err := fl.Parse(args); err != nil {
		os.Exit(2)
	}

	path, err := settingsPath(*project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway uninstall: %s\n", err)
		os.Exit(1)
	}
	settings, err := install.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway uninstall: %s\n", err)
		os.Exit(1)
	}
	changes := settings.Remove()

	if install.Settled(changes) {
		fmt.Fprintf(stdout, "%s: nothing registered, nothing to do\n", path)
		return
	}

	backup, err := settings.Save(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway uninstall: %s\n", err)
		os.Exit(1)
	}
	fmt.Fprint(stdout, install.Render(changes, ""))
	if backup != "" {
		fmt.Fprintf(stdout, "\nbackup: %s\n", backup)
	}
}

// cmdStatus reports what's actually registered right now, not what a prior
// install claimed to do — the live file is the only source that can answer
// "is it actually on?".
func cmdStatus(args []string, stdout io.Writer) {
	fl := flag.NewFlagSet("status", flag.ContinueOnError)
	fl.SetOutput(os.Stderr)
	project := fl.Bool("project", false, "check ./.claude/settings.json instead of ~/.claude/settings.json")
	if err := fl.Parse(args); err != nil {
		os.Exit(2)
	}

	path, err := settingsPath(*project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway status: %s\n", err)
		os.Exit(1)
	}
	settings, err := install.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keyway status: %s\n", err)
		os.Exit(1)
	}
	fmt.Fprint(stdout, install.RenderStatus(settings.Registered(), path))
}

func settingsPath(project bool) (string, error) {
	if project {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return install.ProjectPath(cwd), nil
	}
	return install.DefaultPath()
}
