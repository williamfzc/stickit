# stickit

**Sticky notes pinned to your code** — a CLI for humans and AI agents to exchange line-anchored notes.

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
deliberate non-goals.

## Status

🚧 Design phase. The interface above is the contract we are building against.

## License

MIT
