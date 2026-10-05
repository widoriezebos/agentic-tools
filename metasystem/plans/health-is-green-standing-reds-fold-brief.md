# Design fold brief: health-is-green-when-the-seat-is-healthy, the standing reds, revision 2

## Revision

Revision: 2 (fold of the Astra critique round 1, design-critic-33b212e1cd19505db69c369f, six material findings)

Reason: the coordinator (m1e, in Wido's word, 2026-10-05 07:00 CEST) accepts all six findings. Revise `plans/designs/health-is-green-standing-reds.md` in place: keep its structure, its Id and `Status: draft`; change only what each decision below needs; add a "## Critique round 1" section at the end with the six dispositions. The page stays under 110 lines.

## Context pack

Read `plans/designs/health-is-green-standing-reds.md` (the draft) and `plans/health-is-green-standing-reds-design-brief.md` (its brief, with the code facts). Open a code file only to check one line a decision below names. Batch independent reads.

## Decisions (all accepted; argue only with evidence)

| Finding | Decision | Change |
| --- | --- | --- |
| MOVED-EFFECTS-CAPABILITY-PROBE (high): the snapshot remedy names the adapter self-test, which refuses without a default model and launches agent jobs | accepted | The capability-snapshots row's remedy is the probe admission itself uses (name its function and file from `internal/adapter` or `internal/dispatch`, the one that writes the snapshot), run at most once per runtime per tick, launching no agent job; the moved-effects row names that operation and owner. |
| CLOSED-RULE-ROLE-APPLICABILITY (high): the checkout kinds leave a healthy lane checking hook evidence it never produces | accepted | Replace the per-kind role lists with one closed rule: a role applies to a checkout only when that checkout runs the producer or duty the role checks (name, per role, the producer and how the code can tell it runs there: a registration, a running component, a hook installed); the landing lane runs no session and no hooks, so hook-freshness, stop-hook-duration, session-main and context-budget do not apply to it; give the fixture and the test for a new lane checkout reading green. |
| HEALTH-STANDING-READERS (medium): the standing-failure rule changes the tick's aggregate but not the interactive health check or the Stop evaluation | accepted | The standing decision lives in one place both readers call (name the function); read-only projections apply it without advancing the observation counter; a test that the tick, `metasystem health` and the Stop evaluation agree on the first transition. |
| HEALTH-STANDING-REASON-IDENTITY (medium): resetting the count whenever the reason text changes keeps an aging message from ever reaching five observations | accepted | The standing count keys on the role and a stable cause (the reason with ages, counts and timestamps stripped, or an explicit cause field the check sets); a changed cause resets, changed diagnostic text does not; a test with a reason that carries an age. |
| HEALTH-LEDGER-EXISTING-MOVE (medium): the ledger remedy runs only on promotion and misses already-promoted unexamined moves | accepted | The ledger-attention remedy also runs once at steward start and whenever the role first reads dead, covering existing state; a test with an idle seat installed after the move. |
| MOVED-EFFECTS-STANDING-NOTICE (medium): the transfer from the health alert episode to a per-role standing-defect episode has no owner in the moved-effects table | accepted | Add the moved-effects row (`| Effect | From | To | Code |`) for the health notice's suppression and the standing-defect episode, name the owner that closes the old notice when the standing defect opens, and the test that the first handoff never leaves two notices for one failure. |

Re-estimate the units table after the fold (production and test lines separately; each unit under 1,000 changed lines; step 1 stays the smallest set that turns today's evidence green on a healthy seat).

## Tool-call budget

Maximum reader tool calls: 40
