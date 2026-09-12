// Package tiers resolves model-specific instruction content from a
// directory convention: an optional _base.md that always applies, plus
// *.md files named after a model-name substring they should match.
// Adding a tier is dropping in a file — no code or config change.
package tiers

import (
	"os"
	"path/filepath"
	"strings"
)

// Load returns _base.md's content (if present) followed by whichever
// other *.md file's stem is the longest case-insensitive substring match
// against model. Longest match wins so "opus-5.md" beats "opus.md" for a
// model string containing both. Returns "" if dir doesn't exist or has no
// matching files — never an error, since a hook must not fail the session.
func Load(dir, model string) string {
	if dir == "" {
		return ""
	}
	var out strings.Builder
	appendFile := func(name string) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return
		}
		out.Write(b)
		if len(b) > 0 && b[len(b)-1] != '\n' {
			out.WriteByte('\n')
		}
	}
	appendFile("_base.md")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return out.String()
	}
	lowerModel := strings.ToLower(model)
	best := ""
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "_base.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		if stem == "" || !strings.Contains(lowerModel, strings.ToLower(stem)) {
			continue
		}
		if len(stem) > len(best) {
			best = stem
		}
	}
	if best != "" {
		appendFile(best + ".md")
	}
	return out.String()
}

// FindProjectDir walks up from cwd looking for a .claude/model-tiers
// directory, the way git walks up looking for .git. Stops at home (the
// global tiers dir is handled separately, by the caller) or filesystem
// root. Returns "" if none is found.
func FindProjectDir(cwd, home string) string {
	if cwd == "" {
		return ""
	}
	dir := filepath.Clean(cwd)
	homeClean := filepath.Clean(home)
	for {
		if dir == homeClean || dir == string(filepath.Separator) {
			return ""
		}
		cand := filepath.Join(dir, ".claude", "model-tiers")
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
