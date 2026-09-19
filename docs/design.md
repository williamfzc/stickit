---
type: Concept
title: stickit design
description: Positioning, storage model, interface contract, anchoring algorithm and v1 non-goals for stickit.
---

# stickit — Design

Sticky notes pinned to files — source code or plain documents — exchanged
between humans and AI agents.

## Positioning

Not a review tool (that loop is Crit's territory) and not a session-memory store
(Mem0/Cognee territory). `stickit` is a **repo-scoped, line-anchored, managed note
layer**: AGENTS.md is project-level memory injected wholesale into prompts; stickit
is file/line-level memory queried on demand.

## Core model

```
CLI is the protocol.  Agents only see commands; SQLite is an implementation detail.
```

- One binary, installed globally on the user's machine.
- One SQLite database per **workspace**, stored inside the workspace
  itself: in the git dir (`<git-common-dir>/stickit/board.db`) for
  repositories — keyed by the common directory, never the worktree path,
  so all worktrees share one board — and in a `.stickit/` directory
  otherwise. The board is born, moves and dies with its project:
  renaming it keeps every note, deleting it removes the board, and
  nothing is orphaned on the machine. `STICKIT_DB` overrides the
  location (tests, explicit backups). Cross-repo search remains out of
  scope for v1 (it was once sketched as `--all`,
  which the interface contract now spends on "include archived" instead).
- Every write records the current branch and HEAD commit automatically — no
  flag — and reads show that origin, so a note pinned on `feature-x` never
  reads as a statement about `main`, and one pinned at commit `abc1234`
  stays a statement about that revision even after the branch moves on.
  Branch-scoped *filtering* is deferred until a real
  collaboration needs it; provenance first, hiding later.
- Any text file qualifies. Outside a git repo, the board keys on the
  directory itself, so a plain folder of documents behaves the same.
- Machine-first interface: JSON when stdout is piped, pretty tables on a TTY.
  Stable exit codes. No interactive prompts. No color when piped.

## Interface (the whole contract)

```
stickit add <file[:line[-line]]> "body"
stickit ls  [path] ["keyword"]                 # --all to include archived
stickit resolve <id>
```

Zero required flags. Everything else dissolves:

| concern | mechanism |
|---|---|
| note type (`gotcha`/`decision`/`handoff`) | `#hashtags` parsed from body; lifecycle rules attach to them |
| author identity | `NOTES_AGENT` env var (agents), git user.name fallback (humans) |
| full-text search | positional keyword arg → FTS5 (multi-token = AND over the note's body) |
| drift / staleness | lazy re-validation inside every read; no `doctor` verb |
| expiry | lazy, on write paths; `#handoff` notes archive when task completes — nothing is ever deleted |
| backup | hidden `stickit dump` → JSONL (not part of the agent surface) |

Interface-area metric: **everything an agent must learn fits in three skill lines.**

Machine surface, stable across versions: stdout is JSON whenever it is piped
(tables only on a TTY); a failed command writes exactly one `{"error": ...}`
object to piped stderr. Exit codes: `0` ok, `1` usage error, `2` note id not
found, `3` storage failure.

## Schema (v1)

```sql
CREATE TABLE notes (
  id          TEXT PRIMARY KEY,
  repo        TEXT NOT NULL,        -- hash of board root path
  file        TEXT NOT NULL,        -- worktree-relative, slash-separated
  start_line  INTEGER,              -- NULL for file-level notes
  end_line    INTEGER,
  content_hash TEXT,                -- hash of the exact anchored lines
  norm_hash   TEXT,                 -- whitespace-collapsed hash, for re-anchoring
  body        TEXT NOT NULL,
  tags        TEXT,                 -- parsed from #hashtags in body
  author      TEXT NOT NULL,        -- agent name or git user
  branch      TEXT,                 -- branch at write time (provenance)
  commit      TEXT,                 -- HEAD revision at write time (provenance)
  status      TEXT NOT NULL DEFAULT 'active',  -- active | stale | archived
  drifted_at  TEXT,                 -- set when an anchor silently moved
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
-- FTS5 virtual table over notes.body
-- (columns: body, kind UNINDEXED, ref_id UNINDEXED)
```

All writes go through the CLI in `BEGIN IMMEDIATE` transactions; readers are
unbounded. Concurrency safety is SQLite WAL's job, not ours.

## Anchoring & drift (the technical heart)

An anchor is `(file, start_line, end_line, content_hash)` where the hash covers the
exact lines at write time. Validation is **lazy on read**:

1. Hash the current lines in range. Match → note is fresh; serve it.
2. Mismatch → fuzzy re-anchor: scan ±N lines (e.g. 50) for a whitespace-insensitive
   match of the original content. Found → silently move the anchor, mark `drifted`,
   and re-baseline the hashes to the new position so the next read is a pure read.
3. Still no match → mark `stale`. Stale notes are shown flagged, never silently.

Staleness is a live view, not history: a stale note whose content matches again is
restored to `active`. Resolved (archived) notes are terminal — later reads never
re-stale or resurrect them. `#gotcha`-style notes that go stale stay
visible-but-flagged; `#handoff` notes expire outright. Staleness never destroys
data — only `resolve` does. Archived notes are permanent: resolved history
is the audit trail, and it costs nothing to keep.

## Deliberate non-goals (v1)

- MCP server — the CLI is already agent-native; add later from the same DB.
- Editor plugin — same store, later client.
- Repo-committed export ("route B") — data model stays append-friendly so
  `dump`/`rebuild` can become the git-tracked layer when sharing matters.
- Deletion by agents — notes are resolved or archived, never erased mid-flight.

## Risks & mitigations

- **Data loss** (all notes live in one dotfile): `stickit dump`; document backup.
- **Boards don't travel**: the database lives in the workspace, so fresh
  clones and other machines (SSH remotes, CI containers) start from an
  empty board; `dump` is the seam for moving a board when that matters.
- **Adoption**: ship a copy-paste skill/AGENTS.md snippet (`stickit --skill`,
  routed to from `--help`); agents must degrade gracefully when the binary is
  absent (CI/containers).
