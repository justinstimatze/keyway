# Security

## Reporting a vulnerability

Email **justin@justinstimatze.com** with `keyway-security` in the subject.
Please don't file a public GitHub issue for a suspected vulnerability.

## Threat model

keyway makes no network calls, ever. As a hook, it writes nothing to
disk — it reads two kinds of local file, `~/.claude/model-tiers/*.md` and
`<project>/.claude/model-tiers/*.md`, and prints their content to stdout,
which Claude Code then adds to the session's context.

`install`/`uninstall` are the one path that does write: the
`settings.json` you point them at (`~/.claude/settings.json` by default,
or `./.claude/settings.json` with `--project`), and only after backing up
the existing file first. `install` never writes without `--write` passed
explicitly. Both operations touch only the hook entries they recognize as
keyway's own (matched by binary basename, not by scanning or rewriting
unrelated content) and round-trip every other key in the file unchanged
— see `internal/install/install_test.go` for the tests that hold this
property, including one that a foreign hook sharing the same event
survives both an install and an uninstall.

**Tier files are trusted the same way `CLAUDE.md` is trusted.** Anyone
who can write to either `model-tiers` directory can put arbitrary text in
front of Claude the next time the matching model is active — the same
capability already implied by write access to `CLAUDE.md`,
`settings.json`, or any hook command Claude Code is configured to run.
keyway adds no new write-access-implies-injection surface; it reuses the
one Claude Code's hook system already accepts.

**Don't point a `model-tiers` directory at a location others can write
to** — a shared filesystem mount, a world-writable CI workspace, a
project cloned from an untrusted source with its own `.claude/model-tiers`
already populated. keyway follows symlinks when reading tier files, same
as `os.ReadFile` always does; a tier directory under someone else's
control can redirect a filename to content outside it.

## Known limitations (document-only)

- **`cwd` comes from the calling hook's JSON, unvalidated.** keyway walks
  up from it looking for `.claude/model-tiers`, the way git walks up
  looking for `.git`. The trust boundary here is the same as running any
  hook command at all — the value comes from the same process that would
  also be invoking keyway's shell command in the first place.
- **Whether `cwd` reflects the literal process working directory, or a
  project root Claude Code resolves first, isn't independently
  confirmed** — only inferred from the hooks reference's description.
  `FindProjectDir` walks up from whatever value arrives, so it still
  finds a project's `model-tiers` when launched from a subdirectory
  either way; this would only matter if project-tier lookup seemed to
  miss a directory it should have found.
- **`PostModelSwitch` confirmed firing — directly observed, not inferred.**
  Three earlier headless `claude -p` calls with different `--model`/
  `--resume` flags logged zero `PostModelSwitch` events; that result
  meant nothing, since a one-shot `-p` process starts with one model and
  exits, with no later moment inside it for a "switch" to happen against.
  The real mechanism (`hooks.md:3208`) is an Agent SDK `set_model` control
  request sent *inside* one continuous `--input-format stream-json`
  process. Sent one (`{"type":"control_request","request":{"subtype":
  "set_model","model":"claude-opus-5"}}`) mid-stream, between two user
  turns, against an isolated config whose `PreModelSwitch`/
  `PostModelSwitch` hooks logged raw stdin verbatim: both fired, with
  `from_model: claude-sonnet-5`, `to_model: claude-opus-5`,
  `source: "sdk"` — matching the documented shape exactly. Then sent the
  literal text `/model opus` as an ordinary user turn in the same
  stream-json process, no pty involved: both hooks fired again, this time
  `source: "command"` and `requested_model: "opus"` — the exact
  documented primary path (a live `/model <name>`), confirmed the same
  way, not just the SDK fallback. Nothing about `PostModelSwitch` is
  still resting on the docs alone.
- **`model` missing from `SessionStart` in headless mode, unexplained.**
  The same three test calls each logged a `SessionStart` event, and none
  carried a `model` field — including the first, a plain fresh session
  with `model` set explicitly via `--model` at launch. The docs name
  exactly two cases where `model` can be omitted: after `/clear`, or a
  conversation-recovery restore. A fresh `startup` session is neither.
  Checked the installed binary's own schema directly
  (`model:o().optional()` — confirms the field is legitimately optional
  in the type, says nothing about when it's actually populated) rather
  than guess a cause. No explanation found. There's no code-level fix
  keyway itself can make for a field the harness doesn't send — the
  degraded behavior (no model, no tier-matched file chosen, `_base.md`
  still applies if present) is the same tested fallback as any other
  missing-model case, not a crash or a wrong answer. Driving keyway from
  a headless or scripted context specifically: verify this against your
  own setup before relying on model-specific tiers firing there.
- **`install`/`uninstall` reformat the whole settings file, not just the
  lines they touch.** Both round-trip `settings.json` through a generic
  JSON map, which re-indents and alphabetizes every key — confirmed live:
  a hook entry's `"type"` and `"command"` fields swap order because `c`
  sorts before `t`. No key, value, or hook is lost or altered, but a diff
  against the file from before will show the whole file as changed, not
  a small patch. The same property holds for every sibling tool this
  pattern was adapted from.
- **No supply-chain surface beyond Go itself.** keyway is a pure-Go binary
  with no third-party runtime dependencies — verifiable via `go.mod`
  having no `require` entries. Install via
  `go install github.com/justinstimatze/keyway/cmd/keyway@VERSION`; pin a
  tagged version in scripted installs.
