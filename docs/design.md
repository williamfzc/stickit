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
- One global SQLite database (WAL mode), keyed by the repo's **common
  directory** (`git rev-parse --git-common-dir`), never by the worktree path
  (`--show-toplevel` differs per worktree and would fragment one repo into
  one board per worktree). Consequences, both intended:
  - worktrees of the same repo share one board (swarm-friendly),
  - repos are isolated from each other by default; cross-repo search is
    deliberately out of scope for v1 (it was once sketched as `--all`,
    which the interface contract now spends on "include archived" instead).
- Every write records the current branch automatically — no flag — and reads
  show that origin, so a note pinned on `feature-x` never reads as a
  statement about `main`. Branch-scoped *filtering* is deferred until a real
  collaboration needs it; provenance first, hiding later.
- Any text file qualifies. Outside a git repo, the board keys on the
  directory itself, so a plain folder of documents behaves the same.
- Machine-first interface: JSON when stdout is piped, pretty tables on a TTY.
  Stable exit codes. No interactive prompts. No color when piped.

## Interface (the whole contract)

```
stickit add <file[:line[-line]]> "body"        # --reply-to <id> for threads
stickit ls  [path] ["keyword"]                 # --all to include archived
stickit resolve <id>
```

Zero required flags. Everything else dissolves:

| concern | mechanism |
|---|---|
| note type (`gotcha`/`decision`/`handoff`) | `#hashtags` parsed from body; lifecycle rules attach to them |
| author identity | `NOTES_AGENT` env var (agents), git user.name fallback (humans) |
| full-text search | positional keyword arg → FTS5 (multi-token = AND over the note's body and replies) |
| drift / staleness | lazy re-validation inside every read; no `doctor` verb |
| expiry & GC | lazy, on write paths; `#handoff` notes archive when task completes |
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
  status      TEXT NOT NULL DEFAULT 'active',  -- active | stale | archived
  drifted_at  TEXT,                 -- set when an anchor silently moved
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
CREATE TABLE threads (
  note_id TEXT NOT NULL REFERENCES notes(id),
  seq     INTEGER NOT NULL,
  author  TEXT NOT NULL,
  body    TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (note_id, seq)
);
-- FTS5 virtual table over notes.body + threads.body
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
data — only `resolve`/GC does.

## Deliberate non-goals (v1)

- MCP server — the CLI is already agent-native; add later from the same DB.
- Editor plugin — same store, later client.
- Repo-committed export ("route B") — data model stays append-friendly so
  `dump`/`rebuild` can become the git-tracked layer when sharing matters.
- Deletion by agents — notes are resolved or archived, never erased mid-flight.

## Risks & mitigations

- **Data loss** (all notes live in one dotfile): `stickit dump`; document backup.
- **Cross-repo leakage on shared machines**: strict repo scoping by default.
- **Adoption**: ship a copy-paste skill/AGENTS.md snippet (`stickit skill`);
  agents must degrade gracefully when the binary is absent (CI/containers).
