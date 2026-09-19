# Changelog

## Unreleased

- The store moved into the workspace: git repositories keep their board
  in `.git/stickit/` (shared by every worktree, untouchable by `git
  clean`), plain directories in `.stickit/`. Renaming a project keeps
  its notes; deleting it removes the board with them. The global
  database, hashed board keys and orphaned boards are gone; `STICKIT_DB`
  still overrides, and `dump` backs up the current board.
- Trimmed the periphery back to the product: the pre-commit gate is one
  simple script the adopting repo copies and edits as its own (the
  blocking-policy options and the pre-tool-use injection client were
  removed unused). Integrations are the adopter's own scripts around
  the three verbs; `--help` and the agent skill remain the whole
  onboarding.
- Removed the 30-day GC: archived notes are permanent — resolved history
  is the audit trail and costs nothing to keep. `#handoff` expiry is
  unchanged (archiving, not deletion).
- `add` targets accept filenames containing colons: when the whole spec
  names an existing file, a non-numeric `:suffix` is part of the name.
  Line counting against the real file also unified, so empty and
  blank-line files report their true length in target errors.
- Removed `--reply-to` and note threads: no collaboration shape needed
  them — a wrong note is resolved as-is (archived notes stay reachable
  via `ls --all`) and a corrected fact is a fresh note. Notes no longer
  carry a `replies` key; keyword search covers note bodies only.
- Removed the undocumented `skill` verb alias (shipped in 0.1.0):
  `stickit --skill` is the only spelling of the agent contract snippet.

## 0.1.0 - 2026-09-18

- Added `install.sh`: the one-line install (`curl … | sh`). Prefers a
  prebuilt release asset, falls back to building the checkout it runs in,
  then `go install …@latest`; destination is `$STICKIT_INSTALL_DIR`,
  `$GOBIN` or `~/.local/bin`, verified via `stickit --skill`.
- README gained the adoption loop: install, point agents at
  `stickit --skill`, optionally the pre-commit gate. This repo's own
  AGENTS.md now tells agents to use stickit here.
- Notes record the git commit (HEAD) at write time alongside the branch:
  served as `commit` in every JSON note (null outside git and before the
  first commit), short hash in TTY tables, carried in `dump`. Existing
  databases are migrated in place on open.
- Added `hooks/pre-commit`, an installable git gate that refuses commits
  while the repo's board has unprocessed notes (anything not archived).
  Fails open when stickit is absent or broken. See
  [docs/hooks.md](docs/hooks.md).
- Added `--skill`: prints the agent skill snippet and exits — the canonical
  spelling, visible from `--help`, which now routes agents to it. The bare
  `skill` verb still works as an undocumented alias.
- Implemented the v1 CLI: `add` (with `--reply-to`), `ls` (with `--all`),
  `resolve`, plus auxiliary `skill` and `dump`. Full JSON machine surface
  on piped stdout, tables on a TTY.
- Stable exit codes: `0` ok, `1` usage error, `2` note id not found, `3`
  storage failure. Failures print exactly one `{"error": ...}` JSON object
  on piped stderr.
- Notes carry provenance (`author` from `NOTES_AGENT` or git `user.name`,
  `branch` recorded on every write, shown on every read) and lifecycle
  tags (`#gotcha`, `#handoff`, …) parsed out of the body.
- Anchors survive edits: lazy re-validation on every read, fuzzy
  re-anchoring within ±50 lines (whitespace-insensitive), stale notes
  flagged and recoverable, `#handoff` notes archived when their anchor is
  lost, archived notes garbage-collected after 30 days.
- Boards are repo-scoped in one global SQLite store (`$STICKIT_DB`), shared
  across git worktrees and isolated across repos; plain non-git directories
  work as boards too.
- Chartered the repository: docs tree with validation, repository conventions
  in AGENTS.md, contribution instructions.
- Established the v1 CLI contract: `add` / `ls` / `resolve`, zero required
  flags. See [docs/design.md](docs/design.md).
