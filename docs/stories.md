---
type: Concept
title: User stories
description: What stickit is for — agents exchanging information while collaborating on a codebase.
---

# User stories

stickit exists for one primitive: **an agent pins a note where it learned
something; another agent reads it where it acts on it.** The file can be
source code or a plain document — the primitive does not care. Collaboration
shapes are that primitive applied. Mechanisms live in [design](design.md);
this page states only what should be possible.

## Co-development

Several agents build one thing in the same repo — different modules, or
sequential turns on the same one. Whatever one learns that the others will
need — an ordering constraint, a dead end, where it left off — it pins at the
line it concerns. Before starting on a file, an agent reads what is pinned
there instead of rediscovering it.

```
agent A:  stickit add src/importer.py:40 "parser here assumes UTF-8; the legacy feed is GBK #gotcha"
agent B:  stickit ls src/importer.py
```

## Review loop

One agent (or the human) writes; another reviews by pinning comments on the
lines it questions — a source file, a design doc, a draft. The author reads
them, fixes, and resolves what it addressed. Round and round until the board
is clear — a small local loop, with no PR or hosted service in the way.

```
reviewer: stickit add docs/plan.md:23 "this goal contradicts section 2"
author:   stickit ls docs/plan.md
author:   stickit resolve <id>
```

## Separate worktrees

Co-development usually runs as one worktree per agent. The board belongs to
the repo, not to a worktree: agents in different worktrees read and write
the same notes. Provenance is what keeps sharing from becoming noise — every
note carries the branch it was pinned from and shows it, so "this fails on
empty input" pinned on `feature-x` never reads as a statement about `main`.
Interpreting a note never requires archaeology.

```
# in worktree-alpha (branch feature-a):
stickit add tests/e2e.go:77 "suite needs the DB up; run migrations first #gotcha"
# in worktree-beta (branch feature-b):
stickit ls tests/e2e.go        # same board; origin reads `feature-a`
```

## Adopting a repo

An agent brings stickit into a repository it works in. The tool is
self-teaching and the wiring is the adopter's:

```
curl -fsSL https://raw.githubusercontent.com/williamfzc/stickit/main/install.sh | sh   # once, machine-wide
stickit --skill                                                                        # the contract, from the tool itself
```

- **work**: `ls <file>` before editing, `add <file:line> "..."` when
  something non-obvious is learned, `resolve <id>` when done — pasting
  those three lines into the repo's AGENTS.md starts every future
  session knowing it
- **close the loop mechanically**: copy `hooks/pre-commit`, point git
  at it ([hooks](hooks.md)); the copied script is the repo's to edit
- **anything else** (edit-time injection, session digests): the
  adopter's own scripts around the three verbs

The bar: an agent that installs the CLI and reads `--help` can do all
of this unaided — usage, contract, and the shape of the wiring are
visible from the tool and the docs it links.

## Candidates, not commitments

Written down when a real collaboration runs into them, not before:

- a human joining the same loop — briefing an agent, or reading why-code
- handoff across sessions — where a half-finished task left off
- accumulating rationale — why this code looks this way, for whoever asks
  next

The first multi-agent rehearsal (three unguided sessions sharing one
board) surfaced a further candidate, with evidence attached:

- **a note is a thread; closing it has a reason.** Both author agents
  spontaneously reached for threaded replies on the shipped version:
  recording why a note stopped applying, and that a drifted note was
  re-checked and still holds. Discussion on a note is a demonstrated
  need, not speculation. Shape: replies as the discussion surface on a
  note; `resolve` gaining an optional reason kept with the note's
  record, retrievable via `ls --all`. Typed closure beyond that waits
  for a third real disposition to appear.
