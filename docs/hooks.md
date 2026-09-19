---
type: Concept
title: Enforcement hooks
description: The pre-commit gate — a client script the adopting repo copies and owns — and the rule that every other integration is the adopter's own script.
---

# Enforcement hooks

The CLI is the whole protocol; an integrating repo adds its own clients
around it, and writes them itself. This repo ships one reference client
and uses it on itself:

## The pre-commit gate

`hooks/pre-commit` refuses a commit while `stickit ls` still returns
notes — active or stale; only `resolve` clears one. It fails open when
the binary is absent, and never mentions `--no-verify`: that escape
hatch stays with humans.

Install (per repo):

```sh
mkdir -p hooks
curl -fsSL https://raw.githubusercontent.com/williamfzc/stickit/main/hooks/pre-commit -o hooks/pre-commit
chmod +x hooks/pre-commit
git config core.hooksPath hooks
```

The copied script is **yours**: scope blocking to your own notes when
several agents share one board, ignore certain tags, reword the
refusal — edit it like any file you own.

## Writing your own

Every client is a few lines of shell around the three verbs. Edit-time
injection, for example, is `stickit ls <file>` inside your agent
runtime's pre-edit hook, feeding the output into context; a session
digest is `stickit ls` at startup. The [interface
contract](design.md) is the whole surface — if a client needs more
than it offers, that is a product need, not a script hack.
