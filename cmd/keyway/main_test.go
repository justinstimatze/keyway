package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		var out bytes.Buffer
		run([]string{"keyway", flag}, os.Stdin, &out)
		if out.String() != usage {
			t.Errorf("run(%q) output = %q, want usage text", flag, out.String())
		}
	}
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	run([]string{"keyway", "--version"}, os.Stdin, &out)
	want := buildVersion() + "\n"
	if out.String() != want {
		t.Errorf("run(--version) = %q, want %q", out.String(), want)
	}
}

// --help and --version must short-circuit before anything touches stdin —
// neither takes a *os.File that could block.
func TestRunHelpAndVersionDoNotReadStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close() // never written to; a read would hang if run() tried
	var out bytes.Buffer
	run([]string{"keyway", "--version"}, r, &out)
	run([]string{"keyway", "--help"}, r, &out)
}

// A curious manual run (./keyway with nothing piped in) must not hang
// forever on io.ReadAll — this is the footgun the character-device check
// exists to prevent. Mirrors basanite's TestCompactedSessionIgnoresATerminal.
func TestRunIgnoresATerminal(t *testing.T) {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		t.Skip("no controlling terminal in this environment")
	}
	defer tty.Close()

	var out bytes.Buffer
	run([]string{"keyway"}, tty, &out)
	if out.String() != usage {
		t.Errorf("run() on a terminal stdin = %q, want usage text", out.String())
	}
}

func TestRunDisabled(t *testing.T) {
	t.Setenv("KEYWAY_DISABLE", "1")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		w.WriteString(`{"model":"claude-opus-5"}`)
		w.Close()
	}()
	defer r.Close()

	var out bytes.Buffer
	run([]string{"keyway"}, r, &out)
	if out.Len() != 0 {
		t.Errorf("run() with KEYWAY_DISABLE=1 wrote %q, want nothing", out.String())
	}
}

// End-to-end: a real SessionStart-shaped payload piped in, against a real
// global+project model-tiers tree under a temp HOME, must produce global
// base+tier then project base+tier, in that order.
func TestRunEndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "") // use the HOME fallback, not a leftover from the outer env

	globalTiers := filepath.Join(home, ".claude", "model-tiers")
	if err := os.MkdirAll(globalTiers, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(globalTiers, "_base.md"), []byte("GLOBAL BASE\n"), 0o644)
	os.WriteFile(filepath.Join(globalTiers, "opus-5.md"), []byte("GLOBAL OPUS-5\n"), 0o644)

	proj := filepath.Join(home, "Documents", "someproj")
	projTiers := filepath.Join(proj, ".claude", "model-tiers")
	if err := os.MkdirAll(projTiers, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(projTiers, "_base.md"), []byte("PROJECT BASE\n"), 0o644)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		w.WriteString(`{"hook_event_name":"SessionStart","source":"startup","model":"claude-opus-5","cwd":"` + proj + `"}`)
		w.Close()
	}()
	defer r.Close()

	var out bytes.Buffer
	run([]string{"keyway"}, r, &out)

	want := "GLOBAL BASE\nGLOBAL OPUS-5\nPROJECT BASE\n"
	if out.String() != want {
		t.Errorf("run() = %q, want %q", out.String(), want)
	}
}

// Light e2e suite across a few real model names. Haiku and Sonnet only,
// not the full lineup — the mechanism under test is tier selection, which
// doesn't care which model wins, so a couple of fast-tier names exercise
// it the same way a longer list would at a fraction of the upkeep.
func TestRunAcrossModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	globalTiers := filepath.Join(home, ".claude", "model-tiers")
	if err := os.MkdirAll(globalTiers, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"_base.md":  "BASE\n",
		"haiku.md":  "HAIKU\n",
		"sonnet.md": "SONNET\n",
		// sonnet-5.md deliberately longer than sonnet.md, so a model
		// string matching both resolves to this one.
		"sonnet-5.md": "SONNET-5\n",
	} {
		if err := os.WriteFile(filepath.Join(globalTiers, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name  string
		model string
		want  string
	}{
		{"haiku", "claude-haiku-4-5", "BASE\nHAIKU\n"},
		{"sonnet 5, longest match wins", "claude-sonnet-5", "BASE\nSONNET-5\n"},
		{"older sonnet, no -5 suffix to match", "claude-sonnet-4-6", "BASE\nSONNET\n"},
		{
			"model field absent — only the base applies, regression for the README claim resection flagged",
			"",
			"BASE\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				w.WriteString(`{"hook_event_name":"SessionStart","source":"startup","model":"` + tt.model + `"}`)
				w.Close()
			}()
			defer r.Close()

			var out bytes.Buffer
			run([]string{"keyway"}, r, &out)
			if out.String() != tt.want {
				t.Errorf("run() for model %q = %q, want %q", tt.model, out.String(), tt.want)
			}
		})
	}
}

func TestGlobalDir(t *testing.T) {
	t.Run("CLAUDE_CONFIG_DIR set", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "/custom/config")
		if got, want := globalDir(), filepath.Join("/custom/config", "model-tiers"); got != want {
			t.Errorf("globalDir() = %q, want %q", got, want)
		}
	})
	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		home := t.TempDir()
		t.Setenv("HOME", home)
		if got, want := globalDir(), filepath.Join(home, ".claude", "model-tiers"); got != want {
			t.Errorf("globalDir() = %q, want %q", got, want)
		}
	})
}
