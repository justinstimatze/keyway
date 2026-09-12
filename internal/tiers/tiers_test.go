package tiers

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		model string
		want  string
	}{
		{
			name:  "base only, no model match",
			files: map[string]string{"_base.md": "base\n"},
			model: "claude-sonnet-5",
			want:  "base\n",
		},
		{
			name: "most specific tier wins over less specific",
			files: map[string]string{
				"_base.md":  "base\n",
				"opus.md":   "opus\n",
				"opus-5.md": "opus-5\n",
			},
			model: "claude-opus-5",
			want:  "base\nopus-5\n",
		},
		{
			name: "less specific tier matches when specific absent",
			files: map[string]string{
				"opus.md": "opus\n",
			},
			model: "claude-opus-4-6",
			want:  "opus\n",
		},
		{
			name:  "empty model matches nothing",
			files: map[string]string{"opus.md": "opus\n"},
			model: "",
			want:  "",
		},
		{
			name:  "empty dir",
			files: map[string]string{},
			model: "claude-opus-5",
			want:  "",
		},
		{
			name:  "file missing trailing newline still gets one",
			files: map[string]string{"_base.md": "base"},
			model: "",
			want:  "base\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				writeFile(t, dir, name, content)
			}
			got := Load(dir, tt.model)
			if got != tt.want {
				t.Errorf("Load() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Regression for an overclaim resection caught in README.md: an absent
// model means no *tier* file gets chosen, not that Load emits nothing —
// _base.md isn't model-specific, so it still applies. Named explicitly so
// this property can't quietly drop out from under an unrelated edit to
// one of the table cases above that happens to exercise it incidentally.
func TestLoad_baseAppliesRegardlessOfModel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "_base.md", "base\n")
	writeFile(t, dir, "opus.md", "opus\n")
	if got, want := Load(dir, ""), "base\n"; got != want {
		t.Errorf("Load(dir, \"\") = %q, want %q", got, want)
	}
}

func TestLoad_missingDir(t *testing.T) {
	if got := Load("/nonexistent/path/xyz", "claude-opus-5"); got != "" {
		t.Errorf("Load() on missing dir = %q, want empty", got)
	}
}

func TestLoad_emptyDirArg(t *testing.T) {
	if got := Load("", "claude-opus-5"); got != "" {
		t.Errorf("Load() with empty dir = %q, want empty", got)
	}
}

func TestFindProjectDir(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "Documents", "someproj")
	nested := filepath.Join(proj, "internal", "deep")
	if err := os.MkdirAll(filepath.Join(proj, ".claude", "model-tiers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindProjectDir(nested, home)
	want := filepath.Join(proj, ".claude", "model-tiers")
	if got != want {
		t.Errorf("FindProjectDir() = %q, want %q", got, want)
	}
}

// cwd outside home entirely (e.g. Claude Code launched from /opt/app with
// home elsewhere) must walk to the filesystem root and stop there, not loop
// forever waiting to hit a home that's never on the path.
func TestFindProjectDir_outsideHomeWalksToRoot(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir() // a second temp dir, guaranteed not under home
	if got := FindProjectDir(cwd, home); got != "" {
		t.Errorf("FindProjectDir() = %q, want empty", got)
	}
}

func TestFindProjectDir_none(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "Documents", "someproj")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindProjectDir(cwd, home); got != "" {
		t.Errorf("FindProjectDir() = %q, want empty", got)
	}
}

func TestFindProjectDir_stopsAtHome(t *testing.T) {
	home := t.TempDir()
	if got := FindProjectDir(home, home); got != "" {
		t.Errorf("FindProjectDir() = %q, want empty", got)
	}
}

func TestFindProjectDir_emptyCwd(t *testing.T) {
	if got := FindProjectDir("", "/home/x"); got != "" {
		t.Errorf("FindProjectDir() with empty cwd = %q, want empty", got)
	}
}
