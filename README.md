# stickit

**Sticky notes pinned to your files** — a CLI for humans and AI agents to exchange line-anchored notes.

```
stickit add <file[:line[-line]]> "..."
stickit ls  [path] ["keyword"]
stickit resolve <id>
```

## Why

AI coding agents constantly re-learn (and re-forget) the same things about a codebase:
hidden constraints, past mistakes, why-that-weird-block-exists. AGENTS.md puts all of
that into every prompt at project level. `stickit` puts it **on the code, at the line,
queried on demand** — a managed sticky-note layer that accumulates while agents work.
The same loop covers plain documents: prose gets reviewed where the review belongs,
on the line.

The entire contract, as an agent skill:

```
Before editing a file:      stickit ls <file>
Learned something non-obvious: stickit add <file:line> "... #gotcha"
Done with a note:           stickit resolve <id>
```

Three verbs, zero required flags. Interface area = lines of skill text.

## Design at a glance

- **CLI is the protocol.** Agents only ever see commands; the database is an
  implementation detail. Machine-first output (JSON when piped, pretty on TTY).
- **Global store, repo-scoped.** One SQLite database per machine, keyed by repo root —
  notes are shared across git worktrees, isolated across repos.
- **Line anchors that survive edits.** Anchors carry a content hash; reads lazily
  re-validate and fuzzy-re-anchor, so notes drift gracefully instead of lying.
- **Maintenance is behavior, not commands.** Stale detection, expiry and GC are
  built into normal reads/writes — there is no `doctor` verb.

See [docs/design.md](docs/design.md) for the full design: schema, anchoring algorithm, and
deliberate non-goals. The scenarios this must serve are written out in
[docs/stories.md](docs/stories.md).

## Status

✅ v1 implemented and tested — `go test ./...` covers the unit surface and an
end-to-end suite in [`e2e/`](e2e) drives the built binary through every user
story: co-development, the review loop, shared worktrees, drift, expiry,
isolation and concurrent writers.

One CLI install serves every repo on the machine:

```sh
curl -fsSL https://raw.githubusercontent.com/williamfzc/stickit/main/install.sh | sh
```

From a checkout, `make` builds `./bin/stickit`, `make check` runs
vet + tests + docs validation, and `go install .` installs the binary.

Agents identify themselves via `NOTES_AGENT`; the global store lives at
`$STICKIT_DB` (default: XDG data home). `stickit --skill` prints the
three-line snippet to paste into an agent's instructions (`--help` points
agents there).

## Adopting in a repo

Nothing to configure: install the binary, then point agents at the
contract — paste `stickit --skill` (three lines) into the repo's
AGENTS.md, or rely on the agent finding it through `stickit --help`.
Finding notes by tag works through plain search: `stickit ls handoff`.
From there, integrations are your own scripts around the three verbs —
the [pre-commit gate](docs/hooks.md) is the one reference client worth
copying.

## License

MIT
