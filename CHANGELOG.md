# Changelog

## Unreleased

- Fixed a README overclaim caught by a resection drift audit: an absent
  `model` field means no model-specific tier *file* gets chosen, not that
  nothing gets emitted — `_base.md`, if present, still applies. Added a
  named regression test (`TestLoad_baseAppliesRegardlessOfModel`) and a
  light multi-model e2e suite (`TestRunAcrossModels`, haiku/sonnet) to
  lock in correct behavior across several models at once.

## v0.1.0

- `SessionStart`/`PostModelSwitch` hook that resolves global
  (`~/.claude/model-tiers`) then nearest-project (`.claude/model-tiers`,
  walking up from `cwd`) tier content by longest model-name-substring
  match, and prints it to stdout for Claude Code to add as context.
- `--help`/`-h`, `--version`, and a character-device check on stdin so a
  manual run without piped input prints usage instead of hanging forever.
- `KEYWAY_DISABLE` escape hatch for eval harnesses that start fresh
  sessions and don't want tier content in a controlled probe.
- CI (gofmt, vet, staticcheck, race tests, build), dependabot, and a
  CI-gated auto-merge for non-major dependency bumps — mirrors this
  user's `vidette` house convention.
- `SECURITY.md` scoped to the actual surface: no network, no disk writes,
  reads two kinds of local file trusted the same way `CLAUDE.md` is.
- No prior art found for model-conditional `CLAUDE.md` content via
  Claude Code's hooks — checked GitHub code search and a handful of
  public `.claude/settings.json` examples before building this.
  `keyway` — a keyway admits only the correctly-shaped key — checked
  against `gauge`, `relay`, `switchboard`, `shim`, `yoke`, `spacer`, and
  `fitting` for name collisions first.
