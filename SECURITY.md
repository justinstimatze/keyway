# Security

## Reporting a vulnerability

Email **justin@justinstimatze.com** with `keyway-security` in the subject.
Please don't file a public GitHub issue for a suspected vulnerability.

## Threat model

keyway makes no network calls and writes nothing to disk. It reads two
kinds of local file — `~/.claude/model-tiers/*.md` and
`<project>/.claude/model-tiers/*.md` — and prints their content to
stdout, which Claude Code then adds to the session's context.

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
- **No supply-chain surface beyond Go itself.** keyway is a pure-Go binary
  with no third-party runtime dependencies — verifiable via `go.mod`
  having no `require` entries. Install via
  `go install github.com/justinstimatze/keyway/cmd/keyway@VERSION`; pin a
  tagged version in scripted installs.
