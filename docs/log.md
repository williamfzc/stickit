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
