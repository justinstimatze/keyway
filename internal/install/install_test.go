package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The settings file belongs to the user and holds hooks from many tools. The
// one unacceptable outcome is losing any of it, so this fixture carries a
// foreign hook on an event keyway also writes to, a stale keyway
// registration at an old path, plus an unrelated event and top-level keys.
const fixture = `{
  "model": "opus",
  "permissions": {"allow": ["Bash(git *)"]},
  "hooks": {
    "SessionStart": [
      {"matcher": "", "hooks": [{"type": "command", "command": "/usr/bin/other inject"}]},
      {"matcher": "", "hooks": [{"type": "command", "command": "/old/path/keyway"}]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "/usr/bin/guard"}]}
    ]
  }
}`

func loadFixture(t *testing.T) (*Settings, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestApplyPreservesEverythingElse(t *testing.T) {
	s, _ := loadFixture(t)
	s.Apply("/new/bin/keyway")

	var got map[string]any
	b, _ := s.Bytes()
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["model"] != "opus" {
		t.Errorf("an unrelated top-level key was lost: %v", got["model"])
	}
	if _, ok := got["permissions"]; !ok {
		t.Error("permissions were lost")
	}
	hooks := got["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("an event keyway does not use was dropped")
	}
	if !strings.Contains(string(b), "/usr/bin/other inject") {
		t.Error("a foreign hook sharing an event with ours was lost")
	}
}

// Re-running after installing to a new path must repoint the hook, not stack
// a second registration that keeps running the old binary too.
func TestApplyRetargetsInsteadOfDuplicating(t *testing.T) {
	s, _ := loadFixture(t)
	changes := s.Apply("/new/bin/keyway")

	byEvent := map[string]Change{}
	for _, c := range changes {
		byEvent[c.Hook.Event] = c
	}
	if c := byEvent["SessionStart"]; c.Action != "updated" || c.Was != "/old/path/keyway" {
		t.Errorf("stale registration should be updated in place, got %+v", c)
	}
	if c := byEvent["PostModelSwitch"]; c.Action != "added" {
		t.Errorf("a missing hook should be added, got %+v", c)
	}

	b, _ := s.Bytes()
	if strings.Contains(string(b), "/old/path/keyway") {
		t.Errorf("the old path survived:\n%s", b)
	}
	if n := strings.Count(string(b), "/new/bin/keyway"); n != 2 {
		t.Errorf("want exactly 2 registrations (SessionStart + PostModelSwitch), got %d:\n%s", n, b)
	}

	// Idempotent: a second identical run changes nothing.
	second := s.Apply("/new/bin/keyway")
	for _, c := range second {
		if c.Action != "unchanged" {
			t.Errorf("re-running a settled install reported %q for %s", c.Action, c.Hook.Event)
		}
	}
	// ...and must not write, or the backup becomes a copy of the already
	// modified file and the original is gone.
	if !Settled(second) {
		t.Error("a fully-unchanged run must report settled so the caller skips the write")
	}
	if Settled(changes) {
		t.Error("the first run changed things and must not report settled")
	}
	if Settled(nil) {
		t.Error("no changes at all is not a settled install")
	}
}

func TestRemoveLeavesForeignHooksAlone(t *testing.T) {
	s, _ := loadFixture(t)
	s.Apply("/new/bin/keyway")
	removed := s.Remove()

	for _, c := range removed {
		if c.Action != "removed" {
			t.Errorf("expected %s to be removed, got %q", c.Hook.Event, c.Action)
		}
	}

	b, _ := s.Bytes()
	if strings.Contains(string(b), "keyway") {
		t.Errorf("a keyway registration survived uninstall:\n%s", b)
	}
	if !strings.Contains(string(b), "/usr/bin/other inject") {
		t.Errorf("uninstall took a foreign hook with it:\n%s", b)
	}
	if !strings.Contains(string(b), "/usr/bin/guard") {
		t.Errorf("uninstall dropped an unrelated event:\n%s", b)
	}
	var got map[string]any
	json.Unmarshal(b, &got)
	if _, ok := got["hooks"].(map[string]any)["PostModelSwitch"]; ok {
		t.Error("an event left with no hooks should be dropped, not left empty")
	}
}

func TestRemoveOnNothingInstalledIsSettled(t *testing.T) {
	s, _ := loadFixture(t) // fixture has no PostModelSwitch entry at all
	changes := s.Remove()
	byEvent := map[string]Change{}
	for _, c := range changes {
		byEvent[c.Hook.Event] = c
	}
	if c := byEvent["PostModelSwitch"]; c.Action != "unchanged" {
		t.Errorf("removing a hook that was never registered reported %q, want unchanged", c.Action)
	}
}

func TestRemoveOnEmptySettingsDoesNotPanic(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	changes := s.Remove()
	if !Settled(changes) {
		t.Error("uninstalling from a settings file with nothing registered should be settled")
	}
}

func TestSaveBacksUpAndRoundTrips(t *testing.T) {
	s, path := loadFixture(t)
	s.Apply("/new/bin/keyway")
	backup, err := s.Save(path)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(backup); err != nil || string(b) != fixture {
		t.Errorf("the backup must hold the original bytes verbatim (err=%v)", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	reg := reloaded.Registered()
	if reg["SessionStart"] != "/new/bin/keyway" {
		t.Errorf("registration did not survive the write: %v", reg)
	}
	if len(reg) != len(Hooks) {
		t.Errorf("want all %d hooks registered, got %v", len(Hooks), reg)
	}
}

func TestBackupPathAvoidsCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	first := backupPath(path)
	if err := os.WriteFile(first, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := backupPath(path)
	if second == first {
		t.Fatalf("backupPath returned the same name twice: %s", second)
	}
	if !strings.HasPrefix(second, first) {
		t.Errorf("want the collision name to extend the first, got %s vs %s", second, first)
	}
}

func TestLoadMissingFileIsEmptyNotError(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("a missing settings file is a fine starting point, got %v", err)
	}
	if len(s.Registered()) != 0 {
		t.Error("nothing should be registered in an empty file")
	}
	if changes := s.Apply("/bin/keyway"); len(changes) != len(Hooks) {
		t.Errorf("want %d hooks added, got %d", len(Hooks), len(changes))
	}
}

func TestLoadRejectsMalformedRatherThanClobbering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte("{not json"), 0o600)
	if _, err := Load(path); err == nil {
		t.Fatal("malformed settings must be an error — overwriting them loses the file")
	}
}

func TestIsKeywayCmd(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"/home/x/go/bin/keyway", true},
		{"keyway", true},
		{"/usr/bin/keyway-other", false},
		{"/usr/bin/other keyway", false},
		{"keyway install", false}, // a hook command is always bare; this is not one of ours
		{"", false},
	}
	for _, c := range cases {
		if got := isKeywayCmd(c.cmd); got != c.want {
			t.Errorf("isKeywayCmd(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestSavePreservesFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Apply("/new/bin/keyway")
	if _, err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("want mode preserved at 0600, got %o", info.Mode().Perm())
	}
}
