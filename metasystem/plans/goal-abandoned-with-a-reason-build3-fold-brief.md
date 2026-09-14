Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

Round 4 of chain gawr-build3, the landing side. Two things. Your worktree
is at the tip (design page revision 7) with the carry of round 3 applied.
Do not commit.

# 1. The tier-one bed scenario regressed under the carry

The seat ran metasystem/scripts/agents/land-fixtures.sh outside the
delegate sandbox, twice. On unmodified trunk all 26 scenarios pass. On the
carried tree, 24 pass and two fail: receipt-cutover (item 2 below) and
tier-one, which trunk passes. Its output:

```
land tier-one fixture: receipt creation or forwarding failed
== STEP: tier-1 test receipt
-- ok
== STEP: commit
!! STEP FAILED: commit (exit 83)
land fixture commit refused: would-refuse code=tier1-declaration-refused
```

One difference the seat found while reading: trunk's declaration check in
metasystem/internal/landing/observe.go refuses a tier-1 direct fix when
the root job, the GOAL or the test receipt is missing; the carried tree's
version of that same check omits the goal condition. The carry also joined
ten conflicts in metasystem/scripts/agents/commit.sh, which forwards the
declaration. Find the true cause, which may be either side or the bed's
own tier-one leg, and fix it so that trunk's tier-1 rule and this slice's
goal-and-revision binding both hold. Do not weaken the tier-1 declaration
rule to make the scenario pass.

# 2. Revision 7: the landing-promotion record carries its version

Section 10a of metasystem/plans/goal-abandoned-with-a-reason-design.md
(revision 7) decides the compatibility seam your round 3 reported as a
gap. Build it: the reader in metasystem/internal/landing/promotion.go
enforces each version's vocabulary; version 1 keeps the fields and the
complete code vocabulary the fixture's pinned engine accepts; version 2
has the same fields and adds exactly the three observation codes this
slice introduces; metasystem/scripts/agents/landing-promotion.json becomes
version 2 and keeps every existing entry. A version-2 reader also accepts a
valid version-1 record with its original meaning; a version-1 reader
accepts version 1 only; no reader accepts a version above its maximum, and
a reader that meets one refuses the whole record with the page's exit and
message. The receipt-cutover leg keeps its pinned engine and gains the
disagreement case, as section 13 says.

# What the round must satisfy

- `go test ./internal/landing/ -count=1` and
  `go test ./cmd/metasystem/ -run 'Land|Landing|Carry|Held' -count=1` pass.
- The fast gate passes.
- Say in whatWasDone what the tier-one cause was. If your sandbox cannot
  run the land bed, say so as a gap; the seat runs it outside the sandbox
  and expects all 27 scenarios green.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with `{command, observed, level}` evidence replayable from the worktree's
repository root, one per command above, plus `git -C metasystem status --short`.
Every path in your return starts with `metasystem/`.

# Constraints

Wall clock: 75 minutes. A partial round returns with its tests green for
what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.

# Required testing contract

Use the committed shared testing contract for goal goal-abandoned-with-a-reason (recorded gate width full). The proof of your round is made by the orchestrator, not inside your sandbox: when you return, the orchestrator's enrolled engine runs the risk-selected public `metasystem test` plan on your worktree as you left it, HEAD plus every change in the working tree, the same snapshot conformance reviews (`metasystem job prove-round`), a diagnostic run that collects every failed group, and the chain lands only when the landing's own delivery receipt, which reuses that run's passed groups by execution identity, is sufficient. Retained proof is reused across rounds and attempts by execution identity, so a round that changed no group's inputs proves in seconds and the landing reuses the last round's attempt. Your worktree carries no enrolled engine: do not run `metasystem test run`, `test plan` or `test verify` there. While developing, run the focused tests and the fast gate for what you change; leave every change in the worktree and do not commit (the dispatcher and the proof read the worktree); report the commands you ran. A failed group comes back to you as a follow-up with its evidence. The build cache is provided for the whole chain (GOCACHE, GOTMPDIR and STATICCHECK_CACHE are set): never set, unset or strip them, and never run a gate under env -u or env -i.

# Return path form

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
