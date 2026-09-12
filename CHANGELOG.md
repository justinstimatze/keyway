# Changelog

## Unreleased

- Added `keyway install`/`uninstall`/`status`, replacing hand-edited JSON
  as the primary wiring path. `install` prints what it would change and
  writes nothing until `--write` is passed; repoints an existing
  registration in place rather than duplicating it when the binary moves;
  backs up `settings.json` with a collision-safe timestamped name before
  every write; and round-trips every hook and key it doesn't own
  untouched. `--project` targets `./.claude/settings.json`. Pattern
  adapted from this author's own `basanite`/`couloir` (the `Hook` table +
  `Apply`/`Remove`/`Settled` shape, matched on binary basename) and
  `nowcast` (print-by-default, `--write` to actually mutate — the one
  safety rail neither `basanite` nor `couloir` has, and the one this
  tool adopts for a file the user didn't write and can't easily
  reconstruct). Manual JSON wiring stays documented as a fallback.
- Ran the real editorial split against two dense instruction files (the
  global `CLAUDE.md`, an actively-shipped project's own `CLAUDE.md`) and
  found no content worth tiering by model in either — both are incident-evidenced
  and non-derivable throughout, the category that survives a
  capability-based cut regardless of model. Rewrote the README Status
  section to say so plainly instead of leaving the split listed as
  not-yet-attempted.
- Fixed a README overclaim caught by `resection` (a separate tool that
  diffs a project's stated claims against its actual implementation): an
  absent
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
