# keyway

A rule that's essential guidance for a smaller model can be redundant
noise for a stronger one reading the exact same `CLAUDE.md` — and
there's no native way to vary it by model. `keyway` does: drop one
markdown file per model, and Claude Code injects only the file that
matches whichever model is actually running.

Named for the slot cut into a shaft or lock that only admits the
correctly-shaped key.

## Demo

Two tier files, genuinely opposite instructions, the same prompt against
the same file, run through real Claude Code sessions — not a mechanism
check, the actual payoff:

```bash
mkdir -p demo/.claude/model-tiers demo-config

cat > demo/.claude/model-tiers/haiku.md <<'EOF'
Before making any change, state your plan in exactly one sentence
starting with "Plan:". Then make the change. After the change, report
only "Done." and the filename — no other commentary.
EOF

cat > demo/.claude/model-tiers/opus.md <<'EOF'
Skip any plan statement. Make the change directly, then explain in two
sentences what you changed and why you chose that wording over an
alternative.
EOF

cat > demo/hello.go <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("hello")
}
EOF

cat > demo-config/settings.json <<EOF
{
  "hooks": {
    "SessionStart": [{ "matcher": "", "hooks": [{ "type": "command", "command": "$(which keyway)" }] }],
    "PostModelSwitch": [{ "matcher": "", "hooks": [{ "type": "command", "command": "$(which keyway)" }] }]
  }
}
EOF

cd demo
printf '%s\n' \
  '{"type":"user","message":{"role":"user","content":"/model haiku"}}' \
  '{"type":"user","message":{"role":"user","content":"Add a one-line comment above main() in hello.go explaining what it does."}}' \
  | CLAUDE_CONFIG_DIR="$PWD/../demo-config" claude -p --input-format stream-json \
      --output-format stream-json --dangerously-skip-permissions \
  | jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="text") | .text'
```

```
Set model to `Haiku 4.5` for this session only
Plan: Add a comment above main() explaining that it prints a hello message.
Done. hello.go
```

Reset `hello.go` and run the same two lines again with `/model opus` in
place of `/model haiku`:

```
Set model to `Opus 5` for this session only
Added `// main prints a greeting to standard output.` at hello.go:5.

I wrote it in Go doc-comment form (starting with the identifier name,
full sentence) rather than something like `// entry point` — the latter
restates what `func main` already tells a Go reader, while this says
what the program actually does.
```

One states a plan, then reports tersely with no elaboration. The other
skips the plan, acts, then explains a wording tradeoff. Same file, same
prompt — the difference is entirely the tier file the active model
happened to read.

**Why `/model <name>` mid-session, not a plain `--model` flag at
startup**: it looks like `claude -p --model haiku ...` should trigger
this more directly, but in this author's testing it doesn't reliably —
`claude -p`'s `SessionStart` event carries no `model` field at all (see
[SECURITY.md](SECURITY.md#known-limitations-document-only)), so a
model-specific tier never gets picked on session start alone. Switching
model *during* the session fires `PostModelSwitch`, which does carry it
— the one path confirmed to. Type `/model haiku` into an ordinary
interactive session and the mechanism is the same; the two-line JSONL
above just scripts that switch so this demo runs non-interactively.

**Same effect, zero tokens, no model switch needed** — pipe synthetic
hook JSON straight into the binary:

```bash
echo '{"hook_event_name":"SessionStart","model":"claude-opus-5","cwd":"'"$PWD"'/demo"}' | keyway
echo '{"hook_event_name":"SessionStart","model":"claude-haiku-4-5","cwd":"'"$PWD"'/demo"}' | keyway
```

```
Skip any plan statement. Make the change directly, then explain in two
sentences what you changed and why you chose that wording over an
alternative.
```

```
Before making any change, state your plan in exactly one sentence
starting with "Plan:". Then make the change. After the change, report
only "Done." and the filename — no other commentary.
```

This only proves file selection, not behavior — the real Claude Code
run above is the actual demonstration; this is the free, deterministic
way to check the mechanism in isolation.

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
  fallback, entering or leaving `opusplan` (Claude Code's mode that plans
  on Opus and executes on a cheaper model) — none of which restart the
  session, so `SessionStart` alone would never see them. Its JSON carries
  `to_model` instead. Confirmed directly, including the literal
  `/model <name>` path — see
  [SECURITY.md](SECURITY.md#known-limitations-document-only) for the
  tests.

`keyway` reads whichever field is present, resolves matching tier
content (below), and prints it to stdout — which Claude Code adds to
context for the next turn. A missing model field just means no
model-specific tier *file* gets chosen; `_base.md`, if one exists, still
applies, since it isn't model-specific in the first place.

**Tier content lives in two optional places:**

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

- A model substring in the filename is the whole interface — no
  manifest, no `{match, file}` list to keep in sync.
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
git tag. If `keyway --version` can't find the binary, that directory
isn't on `PATH` yet — add it, or use the absolute path to the binary in
the hook `command` below.

## Wire it into Claude Code

Read [SECURITY.md](SECURITY.md#threat-model) before wiring anything into
your real `~/.claude/settings.json` — tier files are trusted exactly the
same way `CLAUDE.md` already is, no new surface.

```
keyway install
```

Prints what it would change and writes nothing. It finds its own
absolute path, so you never paste `/home/you/go/bin/keyway` into JSON by
hand — and if `hooks.SessionStart`/`hooks.PostModelSwitch` already exist,
it merges into them rather than asking you to. Nothing touches
`settings.json` until you add `--write`:

```
keyway install --write
```

This backs up the existing file first (`settings.json.keyway-backup-<timestamp>`),
then writes. Re-running `install --write` later — after `go install` put
the binary at a new path, say — repoints the existing registration in
place instead of stacking a second copy; re-running it with nothing to
change prints `already wired, nothing to do` and touches nothing, so a
second run is always safe to try. Add `--project` to wire into
`./.claude/settings.json` for one project instead of your global config.

**Uninstall** removes exactly what `install` added and nothing else —
hooks you or another tool registered survive untouched, same backup-first
guarantee:

```
keyway uninstall
```

That only removes the hook registration. To remove the binary too:
`rm "$(which keyway)"`.

**Verify it's wired correctly**:

```
keyway status
```

reports what's actually registered in `settings.json` right now — not
what a past `install` run claimed, the live file. That confirms
registration; to confirm tier content is actually reaching a session,
create `~/.claude/model-tiers/_base.md` with placeholder text, start a
new session, and ask it what context it has. Remove the file once
confirmed, or leave it if you want it permanently.

**Manual wiring**, if you'd rather not run an installer: `install --write`
does exactly this —

```json
{
  "hooks": {
    "SessionStart": [
      { "matcher": "", "hooks": [{ "type": "command", "command": "/absolute/path/to/keyway" }] }
    ],
    "PostModelSwitch": [
      { "matcher": "", "hooks": [{ "type": "command", "command": "/absolute/path/to/keyway" }] }
    ]
  }
}
```

merged into your existing `hooks` key rather than pasted over it (pasting
the whole object above over an existing `hooks` key silently deletes
every other hook you have configured — no diff, no warning). Replace
`/absolute/path/to/keyway` with the output of `which keyway` — Claude
Code's hook runner doesn't reliably inherit a shell's `PATH`, so every
hook entry in a real `settings.json` tends to use an absolute path rather
than the bare command name, and this is the one place in this README
where copying the placeholder as-is would fail silently (see Failure
mode, below). `matcher` is shown empty above to match that same
convention; `keyway` itself doesn't read it.

**Failure mode**: as a hook, `keyway` always exits `0` and writes nothing
to stderr, by design — a hook must never block or fail the session. If
it's wired correctly but nothing is appearing in context, run `keyway
status` to confirm registration, check that `command` in `settings.json`
points at a binary that actually exists (`which keyway` or the absolute
path), and confirm at least one tier file matches — an empty
`model-tiers/` directory correctly produces no output at all.
`install`/`uninstall`/`status` are the opposite on purpose: they exit
nonzero and print to stderr on a real failure, since a silent installer
is a bad installer.

Set `KEYWAY_DISABLE=1` to suppress the hook entirely, e.g. for an eval
harness that starts fresh sessions and doesn't want tier content in a
controlled probe.

## Status

Built, unit-tested (`go test ./...`, including a multi-model end-to-end
suite — see `cmd/keyway/main_test.go`), and wired into this author's own
live `settings.json` as of 2026-09-11.

One hypothesis behind this tool didn't survive contact with real
content: that an existing, well-curated `CLAUDE.md` has material worth
splitting by model. A full pass over two dense real instruction files —
a global `CLAUDE.md` and an actively-shipped private project's own
`CLAUDE.md` — found nothing to split in either. Both read as
incident-evidenced and non-derivable throughout: the kind of thing a
strong model still needs spelled out, not boilerplate it would reinvent
on its own. [Anthropic's own post on context engineering for Claude
5-generation
models](https://claude.com/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models)
names exactly that distinction — overconstraint shows up in generic,
rediscoverable rules, and this author's own files don't have many of
those.

That's a finding about splitting an existing file, not about the
mechanism — the mechanism is independently confirmed in SECURITY.md. No
global tier content exists on this machine right now. The one tier
content actually deployed lives in this repo's own
`.claude/model-tiers/` — real, non-placeholder content written for
contributing to keyway itself, not the demo text above. The demo has
never been anyone's real configuration. If `keyway` has a real payoff
left beyond that, it's more likely in scaffolding a cheaper or faster
tier doing mechanical, repetitive work than in trimming a stronger one —
unconfirmed, not yet tried.

## License

[MIT](LICENSE). See [SECURITY.md](SECURITY.md) to report a vulnerability.
