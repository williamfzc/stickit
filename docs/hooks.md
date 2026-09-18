---
type: Concept
title: Enforcement hooks
description: The pre-commit gate that refuses commits until the board's notes are handled, and how to install it.
---

# Enforcement hooks

The CLI carries no enforcement — hooks are clients of the protocol, not
part of it. The shipped one closes the review loop in
[stories](stories.md): a commit is refused while the board still has
unprocessed notes, so "round until the board is clear" is mechanical
rather than behavioral.

## What it checks

`hooks/pre-commit` runs `stickit ls` against the board of the repo being
committed to and fails the commit while any note comes back — `active`
or `stale` count as open; only `resolve` (which archives) clears one.
The refusal lists every open note with id, location, status, author and
body, and the two ways to handle one:

- address it, then `stickit resolve <id>`
- disagree in its thread (`stickit add --reply-to <id> "..."`), then resolve

## Install (per repo)

Copy `hooks/pre-commit` from the stickit checkout into the target repo,
make it executable, and point git at it:

```sh
mkdir -p hooks && cp <stickit-checkout>/hooks/pre-commit hooks/
chmod +x hooks/pre-commit
git config core.hooksPath hooks
```

`core.hooksPath` is what makes the gate apply to every clone that checks
out the config; `.git/hooks` would be per-clone and untracked.

## Deliberate properties

- **Fail-open**: without the binary, or on a store error, commits pass.
  Agents must degrade gracefully when stickit is absent (see
  [design](design.md)); a gate that wedges work would violate that.
- **Board-wide, not diff-scoped**: any open note blocks any commit. This
  is the story's exit condition stated literally; diff intersection was
  considered and dropped as a harder-to-reason-about variant.
- **No bypass advertised to agents**: the refusal message never mentions
  `--no-verify`. Humans keep it as the escape hatch.
