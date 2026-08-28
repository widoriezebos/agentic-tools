# Turn-boundary embedding review: should the mission runner speak goal-verb?

The review turn-boundary-embedding-review asked for (Appetite: 4h,
origin human): whether and how the mission runner speaks goal-verb at
the turn boundary — a decision, not an implementation. Written
2026-08-22 by mac-coordinator from the runner author's seat. The
DECISION section at the end is Wido's; everything above it is the
review.

## What the boundary already speaks today (verified in code)

1. **Per-turn READ is live.** `AssemblePrompt` runs at every turn
   open and emits the optional `## Serving goal` block —
   `<id> — <intent>` from `goal.Store.CurrentProjection()` — between
   the mission contract and the ledger tail (GOAL-09,
   `internal/mission/prompt.go:613`). A missing, absent, or degraded
   ledger produces no line; assembly never degrades and never blocks
   on goal state. The turn-prompt grammar validates the block's
   exact position (`internal/validate/turnprompt.go`). Dispatch
   briefs have the sibling `ServingGoalSection` (GOAL-08,
   orchestrator-chosen, default off).
2. **Mutation is frozen while a mission runs.** Every goal-mutation
   verb refuses while the checkout has an active mission, fail-closed
   when liveness is indeterminate (GOAL-21, `refuseMissionSeat`,
   `internal/goal/goalverbs.go:120`). The refusal text names the law:
   "the mission's intent is the contract — conclude or park the
   mission first." This rule survived a six-round critique chain
   (r5→r10) precisely because the recorded-fact substrate is hard to
   hold; it is load-bearing, not incidental.
3. **The doctrine sentence** ("programs start with `goal open`, turn
   ends read `goal next`", AGENTS.md) is an INTERACTIVE-agent duty.
   For missions, the read half is discharged mechanically by (1);
   the write half is deliberately reserved to the human/coordinator
   seat by (2).

One observed gap: `CurrentProjection` reads the LOCAL accepted tree,
which advances only on `goal fetch` or a local verb. A long mission
on a machine where nobody fetches embeds a serving-goal line that can
go stale against the canonical ledger — including a stale appetite
picture while claim-age accrues.

## The options, with costs

**A. Status quo — the runner speaks no verb; per-turn read stays as
is.** The coordinator claims the goal, launches the mission,
concludes the goal after the mission ends; the goal thread learns
outcomes when the coordinator writes them. Cost: nothing new to
build or hold. Residual cost: mid-mission, the goal ledger shows
only a claim aging against its appetite (which is exactly what the
breach banner needs — arguably a feature); outcome capture stays
manual; the staleness gap above persists. Risk: none — every
invariant already proven stays untouched.

**B. Turn-grain read hardening — the runner refreshes the accepted
tree before each prompt.** At turn open, best-effort `goal fetch`
(never blocking: offline or refused fetch keeps the stale line under
the existing never-degrade rule), and the serving-goal block could
carry the next step's Appetite token alongside the intent, so a
breach banner reaches the mission host mid-flight. Cost: small
(≈30m–1h); one network touch per turn (missions already fetch origin
at preflight); no authority change; grammar widens by one optional
line (assembler and validator move in the same commit, as GOAL-09
already requires). Risk: low — read-only, fail-open by design.

**C. Mission-boundary write — the runner annotates the serving goal
at completion/park.** Mechanical outcome capture: the terminal write
appends the mission's conclusion to the goal's next step or a
dedicated annotation. Cost: HIGH — it inverts GOAL-21 (the terminal
write happens while the mission is still active by record, so the
freeze must gain a runner-identity carve-out, the same shape as the
wall's runner-owned surfaces and wall-o19's tier-2 path); the write
must be lease-serialized against the canonical-branch publication
model (every mutation pushes origin) and ordered against the
runner's own two-phase conclusion; the integrity chain gains a
non-human, non-coordinator writer. That is a designed item with a
critique loop, not a bounded fix — appetite would be 1d+, and the
GOAL-21 critique history says the first design will be wrong.
Benefit: the thread of intent learns mission outcomes without a
human in the loop — which the conclude-owning seat may not even
want.

**D. Turn-grain write — the runner mutates goal state every turn.**
Rejected without reservation: every turn would be a ledger mutation
plus a canonical push (the multi-machine model publishes per
mutation), the freeze inverts wholesale, and the goal ledger becomes
a mission telemetry channel — the mission ledger already IS that
channel. No benefit survives the cost.

## Recommendation (the reviewer's, not the decision)

**A as the ruling, B as a small optional follow-up, C explicitly
deferred, D rejected.** The boundary already reads at exactly the
right grain and mutation-freeze is the single most-critiqued and
hardest-won invariant in the goal system; nothing observed in a full
day of multi-machine use argues for weakening it. If mid-mission
appetite visibility is wanted, B is a one-hour item (`Appetite: 1h`,
"the serving-goal block refreshes the accepted tree best-effort and
carries the appetite token"). If mechanical outcome capture is ever
wanted, C enters as its own designed item with its own appetite and
critique loop — not before a real instance of the manual conclude
being the bottleneck.

## Decision (Wido's)

- [ ] PENDING — the options above await Wido's ruling; the goal
  concludes only `--by wido` after his word.
