# Repository conventions

## Before you act

Read `docs/index.md` before changing anything. The product is a contract, and
the contract is small: `add`, `ls`, `resolve`, zero required flags. Every
addition to the CLI must name the existing verb or flag that cannot carry the
need. Between two designs take the one with fewer concepts; unsure means no.

The CLI is the protocol; SQLite is an implementation detail. Nothing outside
the command surface may become load-bearing for agents.

Search for the source of truth before restating it. The design lives in
`docs/design.md`; decisions live in `docs/log.md`. If two documents disagree,
name the conflict instead of silently picking one.

## While you write

- Machine-first output: JSON when stdout is piped, pretty only on a TTY.
  Stable exit codes. No interactive prompts, no color when piped.
- Code, identifiers, comments, commits and repo-level documents are English.
- Durable knowledge goes in `docs/`: YAML frontmatter (`type`, `title`,
  `description`), one concept per document, registered in its directory
  index, ordinary relative links. `scripts/validate_docs.py` enforces this.
- Concurrency safety belongs to SQLite (WAL, immediate transactions) — never
  ad-hoc locking in the CLI.

## When you finish

Run `scripts/run_checks.sh`. It fails on missing frontmatter, unregistered
pages and broken links — fix the cause, not the check.

Add a `CHANGELOG.md` entry for anything a user of the CLI would notice: a
changed command, flag, output shape or status rule.

## What this repo refuses

- new verbs, flags or concepts that an existing one already carries
- required flags: decisions the tool should default, not delegate
- maintenance as commands: drift detection, expiry and GC are behavior of
  normal reads and writes
- an agent-facing deletion path: notes are resolved or archived
- v1 scope creep: MCP servers, editor plugins, repo-committed stores
- duplicated statements of the contract; link to `docs/design.md`
