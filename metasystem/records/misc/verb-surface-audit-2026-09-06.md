# Verb surface audit: do the verbs match intent or internal structure?

Asked by Wido on 2026-09-06 after three refused attempts to raise one
goal's budget at his terminal: "verbs should not reflect internal
structure, they should match intent as close as possible. Can you have
a look at all the verbs?" Audited by seat m1d from the engine's own
help page (329 verbs in 33 families at commit 47591ac1) and from the
day's refusals. Read it, not run it, except where a count is given.

## The verdict

The surface is organized by the engine's packages, not by what a person
or a seat is trying to do. Eight patterns, ranked by how often they cost
a turn.

| # | Pattern | Evidence | Who pays |
|---|---|---|---|
| 1 | Refusals name codes and flags, never the command that would have worked | "--budget is box"; "--budget and the five explicit budget limits are mutually exclusive"; "budgets on unclaimed work are the human's approval act, set through goal approve with --budget"; "evidence must be root:<jobId>, finding:<jobId>/<id>, or refusal:<code>"; REFUSED-OPID-MISMATCH; goal-item-not-held; BRIEF_AUTHORITY_REFUSED. Three refusals for one budget raise today; five refused dispatches on 2026-09-05; three refused briefs this morning | human and seat, every time |
| 2 | The human's act is split by the goal's internal state | a budget on a claimed goal is `set-budget`, on an unclaimed one `approve` with limits; parking or concluding the Current goal needs `--then` or `--and-none`; the human must know the state machine to pick the verb | human |
| 3 | Tuples are spelled as their fields | the box is five long flags, all-or-nothing, while every record and every message writes it as `1d/10/720m/1/3`; risk is `--risk severity=,novelty=,exposure=,accumulation=` plus `--basis` plus `--evidence root:<job>` | human and seat |
| 4 | Values that are really switches, and dispatcher vocabulary on the front page | `--budget box`; `--goal none-explicit`; `--destructive-reach MECHANICAL/DESIGN-BEARING/DESTRUCTIVE-REACH` where a seat thinks "mechanical" or "design"; `--op <id>` needed on a retry because the first refusal pinned a fingerprint | seat |
| 5 | Identity plumbing repeated on every call | `--by` on every human verb though the enrolled terminal knows who enrolled it; `--lineage` or an environment variable on every seat mutation though the checkout announces its own main; `--pid --start-time` on `up` | both |
| 6 | Two names for one thing | the checkout is `--root` in 109 verbs and `--repo` in 47, both on the first screen (`up --repo`, `watch --root`); `metasystem watch --job` and `dispatch.sh watch --job`; `land.sh`, `commit.sh` and `landing observe` | seat |
| 7 | Plumbing and intent share one page | the `job` family lists about fifty verbs (record-cas, prefork-mark, launch-capability-consume, critique-register-advance) beside `goal approve`; only 8 of 329 verbs are marked internal; family names are implementation nouns (behavior-surface, lease, landing, proof-run, covenant, gate) | anyone reading help |
| 8 | The workflows have no verb | "land this reviewed chain" is: `validate conformance --stage review --job`, `dispatch.sh watch`, `job critique-register-advance --root-job --round-job`, `dispatch.sh close --job`, `git apply --index --directory=metasystem`, `land.sh --chain --goal --staged-only --direct-fix register-carriage --allow-new-plan`, `goal done --why --conclude`; "review this design" needs `--design`, `--outputs` (a manifest file), `--op`, then a register rendered by hand when the critic is read-only. The seats keep memory files of these sequences, which is the smell itself | seat, every chain |

## What "matching intent" would look like

- A human page of about eight verbs, each an act: approve, budget,
  stop, restart, answer, enroll, decide (accept-risk), done; each takes
  the intent's own grammar (a box as `1d/10/720m/1/3` or `norm` or
  `keep`; a reason as free text), defaults `--by` to the enrolled name,
  and sorts out queued/parked/claimed inside.
- A seat page of about ten verbs that are the way of working: claim,
  brief, build, review, fold, close, land, done, ask, fleet; `land`
  performs the seven-step sequence; `review` computes the conformance
  diff, dispatches the critic and renders the register; a retry never
  needs a fresh opid.
- Everything else behind one word (`metasystem internal ...`) or out
  of the default help, with the eight-of-329 internal marker applied
  to all of it.
- One name for the checkout everywhere; one name for a reason
  everywhere.
- Every refusal ends with the one complete command that would have
  succeeded with the values it saw, or with the human act it needs.
- A fixture per intent verb that drives the refusal and runs the
  printed command.

## What it must not change

Authority: every human-only act stays at the enrolled terminal; every
history line and ledger byte stays as it is; the intent verbs call the
same transactions the long forms call today. Agnosticism: no runtime
named in core. Decisions stay in Go; scripts relay.
