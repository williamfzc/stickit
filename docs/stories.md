---
type: Concept
title: User stories
description: The scenarios stickit must serve, each tied to the design decisions it exercises.
---

# User stories

Two actors appear throughout: **human** (the developer) and **agent** (any
coding agent with shell access — Claude Code, Codex, or a swarm of them).

These stories are the acceptance source for v1: a story not traceable to a
command sequence here is out of scope, and a command sequence here that no
story needs should not exist. Mechanisms referenced below are specified in
[design](design.md).

## S1 — Read before editing

**Situation.** An agent is told to fix the retry logic in `src/client.ts`.
Before touching the file, it follows the skill and asks what the codebase
already knows about it.

**Trace.**

```
stickit ls src/client.ts
```

One note comes back: lines 40–48, `#gotcha`, "the server drops idle
connections after 30s — a retry without re-auth fails silently", pinned last
month by another agent. The agent re-plans and handles re-auth.

**Guarantees.** One call returns everything for a path, machine-readable.
An empty result is exit 0 with an empty array, never an error — absence of
notes is normal, not a failure.

## S2 — Pin what would otherwise evaporate

**Situation.** An agent spends twenty steps discovering that a dependency has
an upstream bug, worked around by a timeout bump. The session ends; without
stickit the discovery dies with it.

**Trace.**

```
stickit add src/vendor/api.py:112 "timeout 5s here works around upstream #2141; drop when we upgrade past 2.3 #gotcha"
```

**Guarantees.** One command, no required flags. The anchor carries the line;
the hashtag carries the lifecycle; the body carries everything else. The next
agent — or the same agent next week — meets the knowledge in S1.

## S3 — Human briefs the agent, asynchronously

**Situation.** A human reviews an agent's draft at 11pm. Tomorrow's session
starts fresh; they don't want to re-explain in a chat prompt.

**Trace.**

```
stickit add src/sync.ts:30-44 "this swallows RejectError — rethrow or log it"
# next morning, the agent:
stickit ls src/sync.ts        # reads the note
# ...fixes the code...
stickit resolve <id>          # or, if blocked: stickit add src/sync.ts --reply-to <id> "..."
```

**Guarantees.** The briefing survives with no conversation at all, and
line-anchoring makes "this" unambiguous. Resolution is an explicit command —
the loop closes visibly, and a blocked agent replies instead of pretending.
This is stickit's complement to review tools: the comment outlives the
session.

## S4 — A swarm shares one board

**Situation.** Two agents work the same repo from different git worktrees.
Agent A (in `worktree-alpha`) discovers that the e2e suite requires a live DB
and fails when tests run in the wrong order. Agent B (in `worktree-beta`) is
about to touch those tests.

**Trace.**

```
# in worktree-alpha:
stickit add tests/e2e.go:77 "suite needs the DB up; order matters — run migrations first #gotcha"
# in worktree-beta:
stickit ls tests/e2e.go       # A's note is there, author reads `alpha-agent`
```

**Guarantees.** The global store keyed by repo root makes notes visible
across worktrees — the swarm coordinates through the board, not through a
shared chat. Concurrent writes are SQLite WAL's problem, not a locking scheme
in the CLI. The author field (via `NOTES_AGENT`) says who knew what.

## S5 — Code moves, notes follow — or flag

**Situation.** A refactor shifts a function by thirty lines. Weeks later, a
rewrite replaces it outright.

**Trace.** After the move, a read of the note shows status `drifted`: the
anchor re-attached to the same content at its new position, silently. After
the rewrite, the note shows status `stale`: still listed, visibly flagged.

**Guarantees.** Lazy re-validation happens on read; there is no daemon and no
doctor command. A stale note is loud, never silent — staleness may not lie,
and only `resolve` or GC archives. An agent or human decides what survives.

## S6 — Why-code questions, answered

**Situation.** A new teammate (or a reviewer) hits a block that looks
duplicated and weird, and wants the rationale without archaeology.

**Trace.**

```
stickit ls src/legacy/
stickit ls src/legacy/ "schema"     # full-text across bodies
```

A `#decision` note explains: "this mirrors the v1 schema because billing
reads the frozen tables; removal is blocked on the 2027 migration".

**Guarantees.** On a TTY the output is a readable table, not JSON — the human
path needs no jq. Search spans bodies via FTS5; rationale pinned once is
found everywhere after.

## S7 — Handoff with a shelf life

**Situation.** An agent is mid-task when a review gate interrupts it. The
next agent needs the working state, but only until the task completes.

**Trace.**

```
stickit add src/migrate.py "review pending on the schema PR; do not widen this migration until it merges #handoff"
# task completes later; the note is archived by normal write-path GC
```

**Guarantees.** Lifecycle hangs off the hashtag, not a flag. Expiry is
behavior of normal reads and writes — there is no cron, no GC command, and
nothing for the agent to remember to clean up.

## S8 — Absent tool, absent problem

**Situation.** The same skill text runs in a CI container where stickit is
not installed.

**Trace.** The agent runs `stickit ls src/client.ts`; the shell reports
command not found; the agent continues without notes. Nothing is written,
nothing breaks.

**Guarantees.** The skill instructs one-time setup for humans and graceful
skip for everyone else. stickit is never a hard dependency of anyone's
workflow — losing it must cost nothing that was already there.
