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
