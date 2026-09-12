// Package install registers keyway's hooks in a Claude Code settings file.
//
// Wiring two hooks by hand means locating the binary, writing its absolute
// path into nested JSON twice, and getting "merge, don't replace" right
// against whatever else is already in the file. This does it from the
// running binary, which knows where it actually is.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Hook is one registration: the event keyway needs to fire on, and why.
// Unlike a tool with several subcommands, keyway registers the same bare
// binary path for every event — there is no per-hook subcommand to match on.
type Hook struct {
	Event string
	Why   string
}

// Hooks is the full set keyway needs. Order is the order they'd run in a
// session.
var Hooks = []Hook{
	{Event: "SessionStart", Why: "inject tier content when a session starts"},
	{Event: "PostModelSwitch", Why: "inject tier content again when the model changes mid-session"},
}

// Change describes one hook's outcome, for the report to the user.
type Change struct {
	Hook   Hook
	Action string // "added", "updated", "unchanged", "removed"
	Was    string // the previous command, when updated
}

// Settings is a Claude Code settings file held as generic JSON so that every
// key keyway does not know about survives the round-trip. Only the hooks
// this tool owns are ever touched.
type Settings struct {
	raw map[string]any
}

// Load reads a settings file; a missing one is an empty settings object,
// since registering a hook is a reasonable way to create it.
func Load(path string) (*Settings, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Settings{raw: map[string]any{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return &Settings{raw: m}, nil
}

// Apply registers every hook at bin, returning what changed. An existing
// keyway registration is rewritten in place rather than duplicated, so
// re-running after `go install` to a new path repoints the hooks instead of
// stacking a second copy that runs the old binary too.
func (s *Settings) Apply(bin string) []Change {
	hooks, _ := s.raw["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		s.raw["hooks"] = hooks
	}
	var changes []Change
	for _, h := range Hooks {
		groups, _ := hooks[h.Event].([]any)
		if found, was := retarget(groups, bin); found {
			action := "updated"
			if was == bin {
				action = "unchanged"
			}
			changes = append(changes, Change{Hook: h, Action: action, Was: was})
			continue
		}
		hooks[h.Event] = append(groups, map[string]any{
			"matcher": "",
			"hooks":   []any{map[string]any{"type": "command", "command": bin}},
		})
		changes = append(changes, Change{Hook: h, Action: "added"})
	}
	return changes
}

// retarget points an existing keyway registration at want, in place. It
// matches by binary basename rather than full path precisely so that a
// hook left over from an older install location is repaired, not
// duplicated.
func retarget(groups []any, want string) (found bool, was string) {
	for _, g := range groups {
		gm, _ := g.(map[string]any)
		inner, _ := gm["hooks"].([]any)
		for _, e := range inner {
			em, _ := e.(map[string]any)
			cmd, _ := em["command"].(string)
			if !isKeywayCmd(cmd) {
				continue
			}
			em["command"] = want
			return true, cmd
		}
	}
	return false, ""
}

// isKeywayCmd reports whether cmd invokes keyway as a bare hook command —
// the binary path and nothing else. keyway takes no arguments as a hook
// (it reads the event off stdin), so unlike a tool with named subcommands,
// a command carrying any extra field is never one of ours.
func isKeywayCmd(cmd string) bool {
	f := strings.Fields(cmd)
	if len(f) != 1 {
		return false
	}
	return filepath.Base(f[0]) == "keyway"
}

// Remove strips every keyway registration, dropping groups left empty. The
// uninstall path exists because a hook that fails silently by design is one
// you cannot turn off by watching it.
func (s *Settings) Remove() []Change {
	hooks, _ := s.raw["hooks"].(map[string]any)
	if hooks == nil {
		var changes []Change
		for _, h := range Hooks {
			changes = append(changes, Change{Hook: h, Action: "unchanged"})
		}
		return changes
	}
	var changes []Change
	for _, h := range Hooks {
		groups, _ := hooks[h.Event].([]any)
		kept := make([]any, 0, len(groups))
		removed := false
		for _, g := range groups {
			gm, _ := g.(map[string]any)
			inner, _ := gm["hooks"].([]any)
			keptInner := make([]any, 0, len(inner))
			for _, e := range inner {
				em, _ := e.(map[string]any)
				cmd, _ := em["command"].(string)
				if isKeywayCmd(cmd) {
					removed = true
					continue
				}
				keptInner = append(keptInner, e)
			}
			if len(keptInner) == 0 {
				continue // the group held only our hook
			}
			gm["hooks"] = keptInner
			kept = append(kept, gm)
		}
		if len(kept) == 0 {
			delete(hooks, h.Event)
		} else {
			hooks[h.Event] = kept
		}
		action := "unchanged"
		if removed {
			action = "removed"
		}
		changes = append(changes, Change{Hook: h, Action: action})
	}
	if len(hooks) == 0 {
		delete(s.raw, "hooks")
	}
	return changes
}

// Registered reports the command currently registered for each hook, for
// the status view — the answer to "is it actually on?", which a tracker
// cannot give and only the live file can.
func (s *Settings) Registered() map[string]string {
	out := map[string]string{}
	hooks, _ := s.raw["hooks"].(map[string]any)
	for _, h := range Hooks {
		groups, _ := hooks[h.Event].([]any)
		for _, g := range groups {
			gm, _ := g.(map[string]any)
			inner, _ := gm["hooks"].([]any)
			for _, e := range inner {
				em, _ := e.(map[string]any)
				if cmd, _ := em["command"].(string); isKeywayCmd(cmd) {
					out[h.Event] = cmd
				}
			}
		}
	}
	return out
}

// Bytes renders the settings as they will be written.
func (s *Settings) Bytes() ([]byte, error) {
	b, err := json.MarshalIndent(s.raw, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Save backs up the current file, then writes atomically via a temp file
// and a rename, preserving the original file's permission bits — a
// settings.json can hold MCP env values, so rewriting must not widen them.
// The backup is the point: this edits a file the user did not write and
// cannot easily reconstruct.
func (s *Settings) Save(path string) (backup string, err error) {
	b, err := s.Bytes()
	if err != nil {
		return "", err
	}
	mode := os.FileMode(0o600)
	if old, err := os.ReadFile(path); err == nil {
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		backup = backupPath(path)
		if err := os.WriteFile(backup, old, mode); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return backup, err
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.tmp")
	if err != nil {
		return backup, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return backup, err
	}
	if err := tmp.Close(); err != nil {
		return backup, err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return backup, err
	}
	return backup, os.Rename(tmp.Name(), path)
}

// backupPath names a timestamped backup, appending -2, -3, ... on the rare
// case a prior backup from the same second is still there — a plain
// timestamp name would otherwise silently overwrite it.
func backupPath(path string) string {
	base := path + ".keyway-backup-" + time.Now().Format("20060102-150405")
	candidate := base
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

// Settled reports whether every hook is already where it should be, so a
// re-run can skip the write. Rewriting on a no-op would replace the backup
// with the already-modified file, quietly losing the only copy of what the
// settings looked like before keyway first touched them. Works for both
// Apply (all "unchanged" means fully installed already) and Remove (all
// "unchanged" means there was nothing to remove).
func Settled(changes []Change) bool {
	for _, c := range changes {
		if c.Action != "unchanged" {
			return false
		}
	}
	return len(changes) > 0
}

// DefaultPath is the user-level Claude Code settings file.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// ProjectPath is the project-level Claude Code settings file for dir.
func ProjectPath(dir string) string {
	return filepath.Join(dir, ".claude", "settings.json")
}

// Render formats the changes for the terminal.
func Render(changes []Change, bin string) string {
	var b strings.Builder
	if bin != "" {
		fmt.Fprintf(&b, "keyway %s\n\n", bin)
	}
	for _, c := range changes {
		fmt.Fprintf(&b, "  %-9s %-16s %s\n", c.Action, c.Hook.Event, c.Hook.Why)
		if c.Action == "updated" && c.Was != "" {
			fmt.Fprintf(&b, "  %-9s %-16s was: %s\n", "", "", c.Was)
		}
	}
	return b.String()
}

// RenderStatus formats what is registered right now.
func RenderStatus(reg map[string]string, path string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "hooks in %s\n\n", path)
	for _, h := range Hooks {
		mark, detail := "not registered", ""
		if cmd, ok := reg[h.Event]; ok {
			mark, detail = "registered", cmd
		}
		fmt.Fprintf(&b, "  %-16s %-15s %s\n", h.Event, mark, detail)
	}
	return b.String()
}
