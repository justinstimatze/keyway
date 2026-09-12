// Command keyway is a Claude Code SessionStart/PostModelSwitch hook that
// injects model- and project-specific instruction content, so guidance
// that's load-bearing for one model isn't overconstraint noise for another.
//
// Convention: ~/.claude/model-tiers/ (global) and <project>/.claude/model-tiers/
// (nearest ancestor of cwd) each hold an optional _base.md plus *.md files
// named after a model-name substring, e.g. opus-5.md, opus.md, sonnet.md.
// Global emits before project, so project reads as the more specific layer.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/justinstimatze/keyway/internal/hookio"
	"github.com/justinstimatze/keyway/internal/tiers"
)

var version = "dev"

const usage = `keyway is a Claude Code SessionStart/PostModelSwitch hook.
It's not meant to be run by hand — Claude Code invokes it with hook JSON
on stdin. See https://github.com/justinstimatze/keyway for setup.

  keyway --version   print the installed version
  keyway --help      print this message
`

func buildVersion() string {
	if version != "dev" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, dirty string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 12 {
				rev = s.Value[:12]
			} else {
				rev = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev == "" {
		return version
	}
	return rev + dirty
}

// globalDir resolves ~/.claude/model-tiers, honoring CLAUDE_CONFIG_DIR for
// a relocated Claude Code config directory.
func globalDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "model-tiers")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "model-tiers")
}

// run holds all of main's logic behind a signature a test can drive: args
// in place of os.Args, stdin as an *os.File (so a test can pass a real
// /dev/tty handle, the same way it would arrive as os.Stdin) rather than
// an io.Reader, since the char-device check below needs Stat.
func run(args []string, stdin *os.File, stdout io.Writer) {
	if len(args) > 1 {
		switch args[1] {
		case "--version":
			fmt.Fprintln(stdout, buildVersion())
			return
		case "--help", "-h":
			fmt.Fprint(stdout, usage)
			return
		}
	}
	if hookio.Disabled() {
		return
	}
	// Claude Code always writes the hook JSON then closes stdin, so
	// reading to EOF below is correct there. Run by hand with stdin still
	// attached to a terminal, that read blocks forever with no feedback —
	// print the same usage instead of hanging.
	if fi, err := stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(stdout, usage)
		return
	}

	in := hookio.Parse(stdin)
	model := in.ActiveModel()

	var out string
	out += tiers.Load(globalDir(), model)
	if home, err := os.UserHomeDir(); err == nil {
		if projDir := tiers.FindProjectDir(in.Cwd, home); projDir != "" {
			out += tiers.Load(projDir, model)
		}
	}
	fmt.Fprint(stdout, out)
}

func main() {
	run(os.Args, os.Stdin, os.Stdout)
}
