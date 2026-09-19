---
type: Log
title: Repository Change Log
description: Compact durable history for decisions, migrations, and boundary changes.
---

# Repository Change Log

Record only changes a future maintainer or agent is likely to need: boundary
changes, contract changes, migrations, operational lessons, and decisions that
changed how the repo is worked in. Keep entries short and link to the canonical
document when one exists.

## Entries

### 2026-09-19 (workspace-local storage)

- The board's database moved into the workspace — `<git-dir>/stickit/`
  for repositories (all worktrees share it via the common dir; `git
  clean` cannot reach it), `.stickit/` for plain directories. Decided
  by the user; motivated by the rehearsal's orphaned-board evidence.
  The rename acceptance test then caught the last path-hash scoping
  orphaning every note anyway, so the `repo` column and board keys
  retired together with the global database: **the file is the board**.
  `STICKIT_DB` remains the override (tests, explicit merges); `dump`
  backs up the current board.

### 2026-09-19 (subtraction)

- The periphery had outgrown the product; cut back. The gate's policy
  options and the pre-tool-use injection client — both built before any
  real use, the client before its runtime schema was even verified —
  are removed. The gate returns to one simple script whose copied form
  the adopter edits as their own; swarm scoping, injection wiring and
  whatever comes next are the adopter's scripts around the three verbs,
  not product surface. The expectation pinned down: an agent installs
  the CLI, reads `--help`, and knows both how to use it and how to wire
  it into a project — much of that wiring is theirs to write.

### 2026-09-19 (permanent retention, adopter-owned enforcement)

- The 30-day GC is gone: archived notes are permanent. Every surveyed
  product keeps resolved history forever, resolved history is the audit
  trail (why-code), and the rows cost nothing. `#handoff` expiry is
  unchanged — it archives, never deletes. Decision with the user.
- Swarm review surfaced that the board-wide gate deadlocks parallel
  agents: one agent's open WIP notes blocked every colleague's commit.
  Resolution per the user's principle: enforcement is the adopter's to
  shape, so the gate now carries a policy choice (`all` default, `own`
  for swarms) in the copied script, overridable via
  `STICKIT_GATE_POLICY`. No CLI change — the gate is a client.
- docs/hooks.md reframed as Clients & integrations, stating the product
  principle: the three verbs are the whole protocol; integrations are
  clients the adopter copies and owns (git hooks, agent-runtime hooks,
  future plugins). stickit ships reference clients only. The edit-time
  injection client (`hooks/pre-tool-use`) joins the gate as the second
  reference client — the entry half of enforcement.

### 2026-09-19 (review pass dispositions)

- Full-repo review pass; three taste-level findings dispositioned:
  (1) targets with a colon in the filename now parse — the whole spec
  naming an existing file wins over the `:lines` split, malformed line
  specs keep their instructive error; line counting unified on
  anchor.ReadLines, which also fixed blank-line files reporting one line.
  (2) The README carries the skill snippet verbatim on purpose — it is
  the front door, the one place duplication is load-bearing;
  [design](design.md) stays canonical for the contract, and this entry
  is the named resolution the charter demands.

### 2026-09-19 (threads removed)

- Removed `--reply-to` and the threads table. The audit question was not
  "who would reply" — a reply targets a note, not a known person — but
  whether any [story](stories.md) loop needs it: co-development never
  replies, the review loop's cycle is pin → fix → resolve, and a wrong
  note is resolved as-is with the corrected fact as a fresh note (under
  the gate, an unresolved disagreement arguably *should* stay on the
  board). Threads were the largest non-core surface chunk: threads
  table, seq, `--reply-to`, a `replies` key in every note, and FTS over
  replies (which forced the note-level AND). Existing databases keep
  their tables; nothing reads them. One concept fewer; unsure meant no.

### 2026-09-19 (skill alias trimmed)

- Removed the bare `skill` verb — the undocumented alias `--skill` carried
  since the same day it shipped. Audit against the [prior
  art](prior-art.md) found it was the only surface that was neither
  convention (bare `help` stays; git/go/docker all accept it) nor
  load-bearing (docs, gate and AGENTS.md all use `--skill`). With zero
  external users the removal is free; one concept, one spelling.

### 2026-09-18 (commit provenance)

- Notes now record the HEAD revision at write time, next to the branch.
  Not a new concept: it is the same provenance axis at finer grain, and
  the [prior art](prior-art.md) survey backs it (Gerrit pins votes to a
  patchset; GitHub dismisses approvals that a revision change made
  stale). Automatic, zero flag, null outside git and before the first
  commit — mirroring `branch`. Filtering by revision stays deferred.
  The `notes.commit` column is added by an idempotent guarded ALTER on
  open (the name is a reserved word, so it is quoted in SQL); older
  databases migrate in place, old notes serving a null commit.

### 2026-09-18 (pre-commit gate)

- Shipped `hooks/pre-commit` as a client-side enforcement hook and enabled
  it in this repo via `core.hooksPath`. Semantics pinned: any non-archived
  note on the committing repo's board blocks any commit — the review-loop
  exit condition ("board is clear") stated literally. Diff-intersection
  scoping was considered and dropped as harder to reason about. The gate
  fails open (binary absent or store broken → commit passes) and never
  advertises `--no-verify` to the reader; that bypass stays documented here
  and in [hooks](hooks.md) for humans. Canonical statement in
  [hooks](hooks.md).

### 2026-09-18 (--skill added)

- Added the `--skill` root flag (print the agent skill snippet, exit 0) and
  an agent-routing line at the end of `--help`. Named justification: the
  existing `skill` verb could not carry discoverability — an agent skims the
  options block of `--help`, where a "print X and exit" flag is the
  convention (modeled on `herdr --skill`). The verb stays as an undocumented
  alias so earlier callers keep working; one concept, one documented
  spelling.

### 2026-09-18 (v1 implemented)

- v1 implemented and tested: `add` / `ls` / `resolve` plus the auxiliary
  `skill` and `dump`. E2E suite in `e2e/` drives the built binary through
  every [story](stories.md) shape; unit tests cover anchoring, pathspecs,
  store lifecycle and flag parsing. `scripts/run_checks.sh` now runs
  `go vet` and `go test` alongside the docs validation.
- Exit codes fixed as contract: `0` ok, `1` usage, `2` note not found,
  `3` store error. Failures emit exactly one `{"error": ...}` JSON object
  on piped stderr — including pre-parse failures (no/unknown command),
  which previously printed plain usage text. Canonical statement in
  [design](design.md).
- Named conflict, then resolved: design.md once used `--all` for both
  "cross-repo search" and "include archived". The interface contract keeps
  `--all` = include archived; cross-repo search is out of scope for v1
  (listing another board's files would lazily stale them against a worktree
  that cannot contain those files).
- Schema in [design](design.md) synced with the implementation: `norm_hash`
  and `drifted_at` columns documented; drift is recorded via `drifted_at`,
  not as a status.
- Anchoring semantics pinned down: a drifted anchor re-baselines its hashes
  so a steady-state read performs no write; a stale note whose content
  matches again returns to `active`; archived notes are terminal. A
  multi-token keyword is an AND at note level (across body and replies) —
  FTS5's row-level AND would otherwise miss notes whose tokens split
  between note and reply.
- Worktree fix: user paths are made relative to the working tree
  (`--show-toplevel`), not the board root — keying by common dir while
  resolving paths against the main root rejected every add from a linked
  worktree. All board paths are canonicalized through symlinks (macOS
  `/tmp` → `/private/tmp`) so board keys cannot fragment by path form.
- Branch provenance survives unborn branches (`git branch --show-current`
  instead of `--abbrev-ref HEAD`, which fails before the first commit).
- Cold-start hardening: concurrent first opens of a fresh database retry
  through the WAL/schema initialization race, which `busy_timeout` alone
  does not cover (a `journal_mode(WAL)` switch can fail fast with
  SQLITE_BUSY).

### 2026-09-18

- Repository chartered. Name `stickit` chosen over `pinote`, `notepin` and
  `agentnotes` after an npm/GitHub availability check (all shorter candidates
  were taken on npm).
- Positioning settled: a managed, line-anchored sticky-note layer — not an
  ephemeral review loop, not a session-memory store. Canonical statement in
  [design](design.md).
- Storage: one global SQLite database (WAL) keyed by repo root. No
  repo-committed store in v1; `dump`/`rebuild` keeps that door open.
- Interface collapsed to three verbs with zero required flags — `add`, `ls`,
  `resolve`. Maintenance (drift detection, expiry, GC) is behavior of normal
  reads and writes, not commands.
- User stories added in [stories](stories.md): one primitive — agents
  exchanging notes where they act — in two collaboration shapes,
  co-development and a review loop. Stories state what should be possible;
  mechanisms stay in [design](design.md).
- Worktree support pinned down: the board is keyed by
  `git rev-parse --git-common-dir` — keying by `--show-toplevel` (the
  original design) would have fragmented one repo into one board per
  worktree. Notes record branch provenance at write time; provenance over
  filtering until proven needed.
- Scope widened to any text file — code or plain documents; the review-loop
  example is now a markdown review. Outside git, a board keys on its
  directory.
