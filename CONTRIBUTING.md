# Contributing

## Documents

New durable knowledge goes in `docs/`, one concept per file, next to the
concern it belongs to. Create a directory only when a concern has earned its
own space; do not scaffold empty branches.

Every document carries YAML frontmatter with `type`, `title` and `description`
(the description in English), is registered in its directory's `index.md`, and
links with ordinary relative paths. A page absent from its index does not
exist; the check treats it that way.

## Checks

Run `scripts/run_checks.sh` before opening a change. It rejects:

- missing or malformed frontmatter
- a page not registered in its directory index
- a broken internal link

## Changes users notice

Anything that changes the CLI surface or observable behavior gets a
`CHANGELOG.md` entry.
