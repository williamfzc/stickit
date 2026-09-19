---
type: Concept
title: Clients & integrations
description: The principle that integrations are adopter-owned clients of the CLI, and the two reference clients shipped with the repo — the pre-commit gate and edit-time injection.
---

# Clients & integrations

The CLI is the whole protocol; everything an adopting repo adds around it
is a **client the adopter copies and owns** — a git hook, an agent-runtime
hook, a future plugin. stickit ships reference clients in `hooks/`, never
product surface: nothing here is a command, flag, or schema change, and a
repo that never copies them loses nothing. The two that matter:

- **the pre-commit gate** — the exit: a commit is refused while notes the
  adopter's policy holds open remain unresolved.
- **edit-time injection** — the entry: an agent is shown a file's notes in
  the same turn it decides to edit, so the gate stays a backstop instead
  of the first notice.

## Reference client: the pre-commit gate

`hooks/pre-commit` runs `stickit ls` against the committing repo's board
and fails the commit while open notes remain — `active` or `stale` count
as open; only `resolve` (which archives) clears one.

**The blocking policy is the adopter's choice**, set once in the copied
script (or overridden per run with `STICKIT_GATE_POLICY`):

| policy | blocks on | right for |
|---|---|---|
| `all` (default) | every open note | one agent or human working a repo at a time — "the board is clear" is the loop's exit |
| `own` | notes authored by this agent (`NOTES_AGENT`) or unauthored; other agents' notes are listed, not blocking | parallel swarms sharing one board, where co-workers legitimately carry open WIP notes |

`own` exists because the shared board makes the `all` policy deadlock a
swarm: one agent's mid-flight `#handoff` would block every colleague's
commit. Humans have no `NOTES_AGENT`, so a human commit is always gated
by everything.

### Install (per repo)

```sh
mkdir -p hooks
curl -fsSL https://raw.githubusercontent.com/williamfzc/stickit/main/hooks/pre-commit -o hooks/pre-commit
# or: cp <stickit-checkout>/hooks/pre-commit hooks/
chmod +x hooks/pre-commit
git config core.hooksPath hooks
```

`core.hooksPath` is what makes the gate apply to every clone that checks
out the config; `.git/hooks` would be per-clone and untracked.

## Reference client: edit-time injection

`hooks/pre-tool-use` is an agent-runtime hook: it reads the runtime's
tool event, asks the board what is pinned on the file about to be
edited, and returns it as context for that exact turn. For Claude Code,
wire it in the repo's `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [{ "type": "command", "command": "python3 hooks/pre-tool-use" }]
      }
    ]
  }
}
```

Silence is free: no notes, no binary, no output — the hook must never
become the thing that slows an unrelated edit down.

## Deliberate properties (both clients)

- **Fail-open**: without the binary, or on a store error, work proceeds.
  Agents must degrade gracefully when stickit is absent (see
  [design](design.md)); a gate that wedges work would violate that.
- **No bypass advertised to agents**: the gate's refusal never mentions
  `--no-verify`. Humans keep it as the escape hatch.
- **Copied, not imported**: each repo owns its copy and its policy; the
  reference clients in this repo are starting points, not a framework.
