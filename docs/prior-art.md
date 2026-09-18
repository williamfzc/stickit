---
type: Concept
title: Prior art
description: States and mechanisms across mature review, collaboration and comment tools, and what stickit keeps, already has, or defers.
---

# Prior art

Surveyed to sanity-check stickit's model against products that solved
anchored discussion at scale. Observations, not commitments — a mechanism
from this page is adopted only per the rule in [stories](stories.md)
(a real collaboration runs into the need first).

## Who was surveyed

- **GitHub PR reviews** — review verdicts (comment / approve / request
  changes), conversation resolve, `Outdated` diff badges, dismissal of
  stale approvals, suggested changes.
- **GitLab MRs** — thread resolve/unresolve, named diff versions,
  approval rules, draft state.
- **Gerrit** — change states (`NEW` / `MERGED` / `ABANDONED`), Code-Review
  labels `-2..+2` where `-2` blocks submit, per-patchset votes that reset
  on a new patchset, CI `Verified` label.
- **Reviewable** (Google Critique lineage) — consensus resolution: every
  participant marks a discussion satisfied; discussions resurface when a
  revision changes under them.
- **Google Docs / Figma comments** — open/resolved threads, resolved kept
  in a history view, @-assignment as action items.

## State inventories

| product | leaf object | states | who closes | closure typing |
|---|---|---|---|---|
| GitHub | conversation | open / resolved (+ orthogonal `Outdated`) | anyone with write | none |
| GitLab | thread | open / resolved | note author or anyone | none |
| Gerrit | change (not note) | NEW / MERGED / ABANDONED | owner/maintainer | via labels, not states |
| Reviewable | discussion | unresolved / satisfied per participant | consensus of participants | none |
| Docs / Figma | comment / thread | open / resolved (resolved kept in history) | anyone with edit | none |
| Linear | issue | typed categories: started / completed / canceled | mover | closed states are typed |

## Per-product details worth keeping

Raw mechanism notes, so the survey survives without re-fetching:

- **GitHub**: a review is drafted *privately* (pending) and submitted as
  one verdict — comment, approve, or request changes; the first two may
  block merging under branch settings, and unresolved conversations can
  be set to block independently. Line comments anchor to a diff position:
  new commits mark them `Outdated` (visible, collapsible, never
  auto-resolved). Approvals are *dismissed* when changes are significant,
  forcing re-review. A reviewer can attach a **suggested change** — an
  actionable diff the author accepts with one click; adopting it
  auto-resolves the thread.
- **GitLab**: threads resolve/unresolve manually (no auto-resolution);
  MR diffs are versioned and comparable — you review against a named
  snapshot. Approval *rules* (minimum count, eligible approvers) are
  separate from threads. Draft state gates merging, not editing.
- **Gerrit**: reviews attach to a **patchset**; pushing a new one resets
  votes (configurable) — re-review is the default, not the exception.
  `-2` hard-blocks submit regardless of other votes; CI posts a separate
  `Verified` label. `NEW` / `MERGED` / `ABANDONED` are the only change
  states (DRAFT was removed in 2.15 in favor of WIP/private changes).
- **Reviewable**: a discussion resolves only when *every participant*
  marks it satisfied — replying is never enough. Its discussion matrix
  groups by to-reply / unresolved / resolved; when a revision changes
  under a discussion it resurfaces rather than silently staying closed.
  Descended from Google's Critique, where every comment expects a
  disposition and intensity lives in text conventions (`nit:`).
- **Docs / Figma**: resolve is stored as a special reply (Docs API models
  resolution as a flag on a reply), resolved threads collapse but stay in
  a history view forever. @-assignment converts a comment into an owned
  action item tracked until resolved; Figma threads anchor to canvas
  points with the same open/resolved pair.

## Mechanisms that recur

1. **Open/closed binary at the leaf.** Every product converges on two
   leaf states; richer models (Gerrit labels, review verdicts) live one
   level up, on verdicts — not on individual notes.
2. **Resolution is a verb, never a side effect.** Replying does not
   resolve (Reviewable's core stance); a human — or now, an agent —
   states "done". Reopen always exists.
3. **Closed ≠ deleted.** Resolved threads collapse into a history view
   (Docs comment history, Figma panel, GitHub's resolved-but-visible).
   Notably, every surveyed product keeps resolved history *forever*;
   stickit's 30-day GC of archived notes is more aggressive than all of
   them — worth revisiting if an audit need ever appears.
4. **Anchoring plus outdating.** Notes are pinned to a position; when the
   content moves, products badge (`Outdated`), resurface (Reviewable), or
   invalidate attached verdicts (Gerrit vote reset, GitHub approval
   dismissal) — none silently drop the note. stickit's lazy re-anchor +
   `stale` flag is the same family, with auto-recovery most of them lack.
5. **Blocking lives at the exit.** Request-changes, unresolved-thread
   settings, and Gerrit `-2` all block the *merge/submit* gate, not the
   edit. stickit's pre-commit gate ([hooks](hooks.md)) is this pattern
   transplanted to commits.
6. **Severity by convention, not schema.** Critique's `nit:` prefixes and
   Gerrit's cultural use of `-1` vs `-2` keep intensity in text. stickit's
   `#hashtags` take the same bet; a severity column would be a schema bet
   none of the leaf models made.
7. **Assignment turns a comment into a task.** Docs/Slack @-assignment
   creates an owner and a "for you" filter. Trigger observed in the wild:
   the arrival of a *second human*, not agents.
8. **Pending review state is anti-noise batching** (GitHub's private
   draft review). A local single-writer tool has no equivalent need.

## Where stickit already matches

- Lazy drift re-anchoring and the `stale` flag — mechanism 4, plus
  auto-recovery on content match.
- `resolve` → archived as the only terminal state, kept visible via
  `--all` — mechanisms 1 and 2.
- Branch provenance on every note — the per-revision attribution Gerrit
  makes explicit.
- `#hashtags` instead of a type column — mechanism 6.
- The pre-commit gate — mechanism 5.

## Candidates this survey surfaced

All deferred per [stories](stories.md); listed so the next collision is
recognized instead of reinvented:

- **Typed closure** (won't-fix / obsolete, à la Linear's canceled) — only
  when a loop drowns in notes that were "addressed by making them wrong".
- **Assignee / mentions** — when a human joins the loop (mechanism 7).
- **Retention question** — if a resolved-note audit need appears, revisit
  the 30-day GC before reaching for it.

## Sources

- [GitHub: approving a PR with required reviews](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes/approving-a-pull-request-with-required-reviews)
- [GitLab: merge request reviews](https://docs.gitlab.com/ee/user/project/merge_requests/reviews/)
- [Gerrit: review labels](https://gerrit.wikimedia.org/r/Documentation/config-labels.html)
- [Gerrit: change states (REST API)](https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html)
- [Reviewable: code review discussions](https://docs.reviewable.io/discussions)
- [From Critique to Reviewable](https://www.reviewable.io/blog/from-critique-to-reviewable)
- [Google Docs: comments, action items & reactions](https://support.google.com/docs/answer/65129?hl=en)
- [Figma: guide to comments](https://help.figma.com/hc/en-us/articles/360040328693-Guide-to-comments-in-Figma)
