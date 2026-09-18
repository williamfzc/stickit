# Changelog

## Unreleased

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
