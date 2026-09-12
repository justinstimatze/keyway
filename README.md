# keyway

A rule that's essential guidance for a smaller model can be redundant
noise for a stronger one reading the exact same `CLAUDE.md` — and
there's no native way to vary it by model. `keyway` does: drop one
markdown file per model, and Claude Code injects only the file that
matches whichever model is actually running.

Named for the slot cut into a shaft or lock that only admits the
correctly-shaped key.

## Demo

Two tier files, genuinely opposite instructions, no Claude Code session
required to see the effect:

```bash
mkdir -p demo/.claude/model-tiers

cat > demo/.claude/model-tiers/opus.md <<'EOF'
Trust your own judgment on ambiguous edge cases — don't narrate every
step, just make the change and report the result.
EOF

cat > demo/.claude/model-tiers/haiku.md <<'EOF'
Before editing any file, restate your plan in one sentence first. If
anything is ambiguous, stop and ask rather than guessing.
EOF

echo '{"hook_event_name":"SessionStart","model":"claude-opus-5","cwd":"'"$PWD"'/demo"}' | keyway
echo '{"hook_event_name":"SessionStart","model":"claude-haiku-4-5","cwd":"'"$PWD"'/demo"}' | keyway
```

```
Trust your own judgment on ambiguous edge cases — don't narrate every
step, just make the change and report the result.
```

```
Before editing any file, restate your plan in one sentence first. If
anything is ambiguous, stop and ask rather than guessing.
```

Same mechanism, same two files, either one's content — real Claude Code
wiring just replaces the hand-typed JSON above with what Claude Code
actually pipes in.

**Global and project layers compose**, global first. A global tier
(`CLAUDE_CONFIG_DIR` stands in for `~/.claude` here, so this doesn't
touch your real config) plus a project tier both apply on the same run:

```bash
mkdir -p demo-global/model-tiers
echo '(global) Follow this project'"'"'s CI checks before committing.' \
  > demo-global/model-tiers/_base.md

echo '(project) This repo'"'"'s tests double as its spec — read them before changing behavior.' \
  > demo/.claude/model-tiers/_base.md

echo '{"hook_event_name":"SessionStart","model":"claude-opus-5","cwd":"'"$PWD"'/demo"}' \
  | CLAUDE_CONFIG_DIR="$PWD/demo-global" keyway
```

```
(global) Follow this project's CI checks before committing.
(project) This repo's tests double as its spec — read them before changing behavior.
```

## How it works

`keyway` runs as a `SessionStart` and `PostModelSwitch` hook.

- `SessionStart` fires once, when a session starts. Its JSON carries
  `model` — the active model's canonical name — though that field can be
  omitted after `/clear` or a conversation-recovery restore (see
  [Claude Code's hooks reference](https://code.claude.com/docs/en/hooks.md),
  "SessionStart input").
- `PostModelSwitch` fires on every model change *during* an
  already-running session — `/model`, fast-mode toggling, automatic
  fallback, entering or leaving `opusplan` — none of which restart the
  session, so `SessionStart` alone would never see them. Its JSON carries
  `to_model` instead.

`keyway` reads whichever field is present, resolves matching tier
content (below), and prints it to stdout — which Claude Code adds to
context for the next turn. A missing model field just means no
model-specific tier *file* gets chosen; `_base.md`, if one exists, still
applies, since it isn't model-specific in the first place.

**Tier content lives in two places, both optional:**

- `~/.claude/model-tiers/` — global, every project
- `<project>/.claude/model-tiers/` — nearest `.claude/model-tiers` found
  walking up from the session's `cwd`, the way git finds `.git`

**Each directory can hold:**

- `_base.md` — always emitted, if present
- any other `*.md`, named after a substring of the model's canonical
  name — `opus-5.md`, `opus.md`, `sonnet.md`, `fable.md`. The longest
  matching stem wins, so `opus-5.md` is picked over `opus.md` on Opus 5.

Global emits before project, so project-level content reads as the more
specific layer. Adding a tier is dropping in a file — no code or config
change.

## Design

- Tier files, not a manifest. A model substring in the filename is the
  whole interface — no `{match, file}` list to keep in sync.
- Longest matching stem wins, so a more specific file (`opus-5.md`) beats
  a less specific one (`opus.md`) without separate precedence rules.
- One binary resolves global then project, in that fixed order, inside a
  single process. Nothing in Claude Code's hooks reference promises an
  order across two separately-registered parallel hooks, so this doesn't
  register twice and hope.
- `FindProjectDir` walks up from `cwd` the way git walks up looking for
  `.git`.

This became possible recently: Claude Code shipped `PostModelSwitch` for
exactly this ("give Claude model-specific guidance without editing every
CLAUDE.md") not long before this was built, and nothing had been built on
it yet.

## Install

```
make install
keyway --version   # confirms the binary is on PATH and built correctly
```

Installs to `$GOBIN`/`$GOPATH/bin` as `keyway`, version baked in from the
git tag.

## Wire it into Claude Code

**Merge, don't replace.** If `~/.claude/settings.json` already has a
`hooks` key, pasting the whole object below over it silently deletes
every other hook you have configured — no diff, no warning. Append
`{ "hooks": [{ "type": "command", "command": "keyway" }] }` to your
existing `hooks.SessionStart` array instead, and the same for
`PostModelSwitch`.

Starting from nothing (no existing `hooks` key):

```json
{
  "hooks": {
    "SessionStart": [
      { "hooks": [{ "type": "command", "command": "keyway" }] }
    ],
    "PostModelSwitch": [
      { "hooks": [{ "type": "command", "command": "keyway" }] }
    ]
  }
}
```

No matcher needed — `keyway` resolves which tier(s) apply from the hook's
own JSON input, not from the hook registration.

(Paths above assume `~/go/bin` is on `PATH`; use the absolute path to the
installed binary in `command` otherwise.)

**Verify it's wired correctly**: create `~/.claude/model-tiers/_base.md`
with any placeholder text, start a new Claude Code session, and ask it
what context it has — the placeholder text should be there. Remove the
file once confirmed, or leave it if you want it permanently.

**Failure mode**: `keyway` always exits `0` and writes nothing to stderr,
by design — a hook must never block or fail the session. If it's wired
correctly but nothing is appearing in context, check that `command` in
`settings.json` points at a binary that actually exists (`which keyway`
or the absolute path), and that at least one tier file matches — an empty
`model-tiers/` directory correctly produces no output at all.

Set `KEYWAY_DISABLE=1` to suppress it entirely, e.g. for an eval harness
that starts fresh sessions and doesn't want tier content in a controlled
probe.

## Status

Built, unit-tested (`go test ./...`, including a multi-model end-to-end
suite — see `cmd/keyway/main_test.go`), and wired into this author's own
live `settings.json` as of 2026-09-11. The tier files actually deployed are still the demo text above rather
than a considered split of a real `CLAUDE.md`. The mechanism is proven;
the editorial work — deciding what's genuinely model-invariant versus
overconstraint-prone on a stronger model — is the actual payoff this
tool exists for, and it hasn't happened yet.

## License

[MIT](LICENSE). See [SECURITY.md](SECURITY.md) to report a vulnerability.
